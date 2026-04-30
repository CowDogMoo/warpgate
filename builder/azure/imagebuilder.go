/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package azure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// defaultBuildTimeoutMinutes is the AIB image template build timeout used
// when no value is configured. Matches AIB's own default of 4 hours.
const defaultBuildTimeoutMinutes int32 = 240

// MonitorConfig holds runtime monitoring options for an Azure build.
type MonitorConfig struct {
	// StreamLogs enables streaming AIB build logs to stdout (via the staging
	// storage account log blob). Not yet wired up — reserved for follow-up.
	StreamLogs bool
}

// ImageBuilder builds Azure VM images via Azure VM Image Builder (AIB) and
// publishes them to a Compute Gallery.
type ImageBuilder struct {
	clients         *AzureClients
	fileStager      FileStagerAPI
	forceRecreate   bool
	cleanupOnFinish bool
	monitor         MonitorConfig

	// buildID uniquely identifies this builder so staged file paths cannot
	// collide across concurrent builds. Mirrors the AMI builder's buildID.
	buildID string

	// buildTimeoutMinutes overrides the AIB image template timeout when > 0.
	buildTimeoutMinutes int32

	// pollingInterval overrides the Azure SDK LRO polling frequency when > 0.
	pollingInterval time.Duration

	// runnerFactory builds the AIB pipelineOps used by Build. Defaults to
	// newPipelineRunner; tests can substitute a fake.
	runnerFactory func(*AzureClients, string, time.Duration) pipelineOps
}

// generateBuildID creates a unique identifier for this build. Mirrors the
// AMI builder's helper so staging keys are formatted the same way.
func generateBuildID() string {
	timestamp := time.Now().UTC().Format("20060102-150405")
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return timestamp
	}
	return fmt.Sprintf("%s-%s", timestamp, hex.EncodeToString(randomBytes))
}

// NewImageBuilderWithAllOptions constructs an ImageBuilder with the given
// client config, force-recreate flag, and monitoring configuration. When
// cfg.FileStagingAccount and cfg.FileStagingContainer are both set, a file
// stager is created so file provisioners can reference local paths.
func NewImageBuilderWithAllOptions(ctx context.Context, cfg ClientConfig, forceRecreate bool, monitor MonitorConfig) (*ImageBuilder, error) {
	clients, err := NewAzureClients(ctx, cfg)
	if err != nil {
		return nil, err
	}
	b := &ImageBuilder{
		clients:       clients,
		forceRecreate: forceRecreate,
		monitor:       monitor,
		buildID:       generateBuildID(),
		runnerFactory: newPipelineRunner,
	}
	if stager := NewFileStager(clients.BlobStaging, clients.FileStagingAccount, clients.FileStagingContainer); stager != nil {
		b.fileStager = stager
	}
	return b, nil
}

// GetBuildID returns the unique build identifier. Useful for log
// correlation and matches the AMI builder's accessor.
func (b *ImageBuilder) GetBuildID() string {
	return b.buildID
}

// SetCleanupOnFinish controls whether the AIB image template resource is
// deleted on a successful build. The published gallery image version always
// remains.
func (b *ImageBuilder) SetCleanupOnFinish(v bool) {
	b.cleanupOnFinish = v
}

// SetBuildTimeoutMinutes overrides the AIB image template build timeout.
// Pass 0 to revert to the default.
func (b *ImageBuilder) SetBuildTimeoutMinutes(v int32) {
	b.buildTimeoutMinutes = v
}

// SetPollingInterval overrides the default Azure SDK LRO polling frequency.
// Pass 0 to let the SDK default apply.
func (b *ImageBuilder) SetPollingInterval(v time.Duration) {
	b.pollingInterval = v
}

// Build executes the full Azure image pipeline: generate AIB image template,
// submit, run, read the resulting gallery image version, and (optionally)
// delete the template. The published gallery image version is returned in
// BuildResult.GalleryImageVersionID.
func (b *ImageBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	start := time.Now()

	target, err := b.resolveTarget(cfg)
	if err != nil {
		return nil, err
	}

	stamp := fmtBuildStamp(start)
	tplName := imageTemplateName(cfg.Name, stamp)

	tpl, stagedFiles, err := b.prepareTemplate(ctx, cfg, target, start)
	if err != nil {
		return nil, err
	}
	defer b.cleanupStagedFiles(ctx, stagedFiles)

	artifactID, err := b.runPipeline(ctx, target, tplName, tpl)
	if err != nil {
		return nil, err
	}

	logging.InfoContext(ctx, "Published gallery image version: %s", artifactID)

	return &builder.BuildResult{
		GalleryImageVersionID: artifactID,
		Location:              target.Location,
		Duration:              time.Since(start).String(),
	}, nil
}

// resolveTarget extracts the Azure target from cfg and applies builder-level
// defaults (subscription, identity) before validating required fields.
func (b *ImageBuilder) resolveTarget(cfg builder.Config) (*builder.Target, error) {
	target, err := findAzureTarget(cfg)
	if err != nil {
		return nil, err
	}
	// Apply default subscription/identity onto target if not specified, since
	// the gallery image ID needs them.
	if target.SubscriptionID == "" {
		target.SubscriptionID = b.clients.SubscriptionID
	}
	if target.IdentityID == "" {
		target.IdentityID = b.clients.IdentityID
	}
	if err := requireAzureFields(target); err != nil {
		return nil, err
	}
	return target, nil
}

// prepareTemplate stages any local file provisioners and constructs the AIB
// image template. The returned stagedFiles map must be cleaned up by the
// caller via cleanupStagedFiles.
func (b *ImageBuilder) prepareTemplate(ctx context.Context, cfg builder.Config, target *builder.Target, start time.Time) (*armvirtualmachineimagebuilder.ImageTemplate, map[int]*StagedFile, error) {
	timeout := b.buildTimeoutMinutes
	if timeout <= 0 {
		timeout = defaultBuildTimeoutMinutes
	}

	stagedFiles, err := b.stageFileProvisioners(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	versionTag := nextGalleryVersion(start)
	tpl, err := buildImageTemplate(cfg, target, versionTag, timeout, buildTemplateOpts{StagedFiles: stagedFiles})
	if err != nil {
		return nil, stagedFiles, fmt.Errorf("build image template: %w", err)
	}
	return tpl, stagedFiles, nil
}

// runPipeline submits the template, runs it, and reads the resulting artifact
// ID. Honors forceRecreate (pre-delete) and cleanupOnFinish (post-delete).
func (b *ImageBuilder) runPipeline(ctx context.Context, target *builder.Target, tplName string, tpl *armvirtualmachineimagebuilder.ImageTemplate) (string, error) {
	factory := b.runnerFactory
	if factory == nil {
		factory = newPipelineRunner
	}
	runner := factory(b.clients, target.ResourceGroup, b.pollingInterval)

	if b.forceRecreate {
		runner.deleteTemplate(ctx, tplName)
	}

	if err := runner.submit(ctx, tplName, tpl); err != nil {
		return "", err
	}

	if err := runner.run(ctx, tplName); err != nil {
		if status, dErr := runner.describeLastRun(ctx, tplName); dErr == nil && status != "" {
			return "", fmt.Errorf("%w (last run: %s)", err, status)
		}
		return "", err
	}

	artifactID, err := runner.readArtifact(ctx, tplName)
	if err != nil {
		return "", err
	}

	if b.cleanupOnFinish {
		runner.deleteTemplate(ctx, tplName)
	}
	return artifactID, nil
}

// stageFileProvisioners uploads any `file` provisioner sources whose Source is
// a local path to the configured staging container so the AIB build VM can
// fetch them via the File customizer. Returns a map keyed by provisioner index;
// remote-URL file provisioners are absent from the map and will pass through
// to the customizer untouched. If no `file` provisioners need staging, the
// returned map is empty and no error is returned.
func (b *ImageBuilder) stageFileProvisioners(ctx context.Context, cfg builder.Config) (map[int]*StagedFile, error) {
	staged := make(map[int]*StagedFile)

	hasLocalFile := false
	for _, p := range cfg.Provisioners {
		if p.Type == "file" && isStageableLocalPath(p.Source) {
			hasLocalFile = true
			break
		}
	}
	if !hasLocalFile {
		return staged, nil
	}

	if b.fileStager == nil {
		return nil, fmt.Errorf("file provisioner with a local source requires azure.image.file_staging_storage_account and azure.image.file_staging_container to be configured")
	}

	buildID := b.buildID
	if buildID == "" {
		buildID = generateBuildID()
	}

	for i, p := range cfg.Provisioners {
		if p.Type != "file" {
			continue
		}
		if p.Source == "" {
			b.cleanupStagedFiles(ctx, staged)
			return nil, fmt.Errorf("provisioner[%d] file: source is required", i)
		}
		if !isStageableLocalPath(p.Source) {
			continue
		}
		keyPrefix := fmt.Sprintf("warpgate-staging/%s/%d", buildID, i)
		s, err := b.fileStager.Stage(ctx, p.Source, keyPrefix)
		if err != nil {
			b.cleanupStagedFiles(ctx, staged)
			return nil, fmt.Errorf("stage file provisioner %d: %w", i, err)
		}
		staged[i] = s
	}
	return staged, nil
}

// cleanupStagedFiles removes any blobs uploaded for `file` provisioners.
// Errors are logged inside FileStager.Cleanup; we never fail the build over
// orphaned staging objects.
func (b *ImageBuilder) cleanupStagedFiles(ctx context.Context, staged map[int]*StagedFile) {
	if b.fileStager == nil {
		return
	}
	for _, s := range staged {
		b.fileStager.Cleanup(ctx, s)
	}
}

// Replicate updates a gallery image version's TargetRegions to include
// targetRegions. The version's existing regions are preserved.
func (b *ImageBuilder) Replicate(ctx context.Context, versionID string, targetRegions []string) error {
	if versionID == "" {
		return fmt.Errorf("versionID is required")
	}
	if len(targetRegions) == 0 {
		return nil
	}
	return updateGalleryVersionRegions(ctx, b.clients, versionID, targetRegions)
}

// Share grants the Reader role on a published gallery image version to each
// principalID (typically the object ID of an Azure AD user, group, or service
// principal). Existing assignments are tolerated so re-running is idempotent.
func (b *ImageBuilder) Share(ctx context.Context, versionID string, principalIDs []string) error {
	if versionID == "" {
		return fmt.Errorf("versionID is required")
	}
	if len(principalIDs) == 0 {
		return nil
	}
	return shareGalleryImageVersion(ctx, b.clients.RoleAssignments, b.clients.SubscriptionID, versionID, principalIDs)
}

// Delete removes a gallery image version identified by its full resource ID.
func (b *ImageBuilder) Delete(ctx context.Context, versionID string) error {
	if versionID == "" {
		return fmt.Errorf("versionID is required")
	}
	return deleteGalleryVersion(ctx, b.clients, versionID)
}

// Close releases any resources held by the builder. Currently a no-op; the
// SDK clients use the credential's pooled HTTP transport.
func (b *ImageBuilder) Close() error {
	return nil
}

// findAzureTarget returns the first azure target in the config or an error.
func findAzureTarget(cfg builder.Config) (*builder.Target, error) {
	for i := range cfg.Targets {
		if cfg.Targets[i].Type == "azure" {
			return &cfg.Targets[i], nil
		}
	}
	return nil, fmt.Errorf("no azure target found in configuration")
}

// requireAzureFields enforces the minimum target fields needed for AIB +
// Compute Gallery. This is a backstop for the validator so calling Build
// directly (e.g., from tests) still fails loudly.
func requireAzureFields(t *builder.Target) error {
	if t.SubscriptionID == "" {
		return fmt.Errorf("azure target: subscription_id is required")
	}
	if t.ResourceGroup == "" {
		return fmt.Errorf("azure target: resource_group is required")
	}
	if t.Location == "" {
		return fmt.Errorf("azure target: location is required")
	}
	if t.Gallery == "" {
		return fmt.Errorf("azure target: gallery is required")
	}
	if t.GalleryImageDefinition == "" {
		return fmt.Errorf("azure target: gallery_image_definition is required")
	}
	if t.IdentityID == "" {
		return fmt.Errorf("azure target: identity_id is required")
	}
	return nil
}

// fmtBuildStamp produces a compact UTC timestamp suitable for resource names.
func fmtBuildStamp(t time.Time) string {
	return t.UTC().Format("20060102150405")
}

// Compile-time check that ImageBuilder satisfies the AzureImageBuilder interface.
var _ builder.AzureImageBuilder = (*ImageBuilder)(nil)
