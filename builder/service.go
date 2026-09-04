/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/opencontainers/go-digest"
)

// BuildService encapsulates the complete build workflow including building,
// pushing, and manifest creation. It coordinates between builders, orchestrators,
// and configuration to execute complex multi-stage builds.
type BuildService struct {
	globalConfig *config.Config

	// Builder creation function
	buildKitCreator BuilderCreatorFunc
}

// NewBuildService creates a new build service with the given configuration.
// The creator function initializes BuildKit builders.
func NewBuildService(cfg *config.Config, buildKitCreator BuilderCreatorFunc) *BuildService {
	return &BuildService{
		globalConfig:    cfg,
		buildKitCreator: buildKitCreator,
	}
}

// ExecuteContainerBuild performs a complete container build workflow.
func (s *BuildService) ExecuteContainerBuild(ctx context.Context, config Config, opts BuildOptions) ([]BuildResult, error) {
	logging.InfoContext(ctx, "Executing container build")

	ApplyOverrides(ctx, &config, opts, s.globalConfig)

	bldr, err := s.buildKitCreator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}
	defer func() {
		if err := bldr.Close(); err != nil {
			logging.ErrorContext(ctx, "Failed to close builder", "error", err)
		}
	}()

	// Determine if this is a multi-arch build
	if len(config.Architectures) > 1 {
		return s.executeMultiArchBuild(ctx, &config, bldr, opts)
	}

	// Single architecture build
	result, err := s.executeSingleArchBuild(ctx, &config, bldr, opts)
	if err != nil {
		return nil, err
	}

	return []BuildResult{*result}, nil
}

// ExecuteAMIBuild performs a complete AMI build workflow with the provided AMI builder.
// It handles region resolution and AMI target selection before delegating to the builder.
// The caller is responsible for creating and closing the AMI builder to avoid import cycles.
func (s *BuildService) ExecuteAMIBuild(ctx context.Context, config Config, opts BuildOptions, amiBuilder AMIBuilder) (*BuildResult, error) {
	logging.InfoContext(ctx, "Executing AMI build")

	// Find AMI target in configuration
	var amiTarget *Target
	for i := range config.Targets {
		if config.Targets[i].Type == "ami" {
			amiTarget = &config.Targets[i]
			break
		}
	}

	if amiTarget == nil {
		return nil, fmt.Errorf("no AMI target found in configuration")
	}

	// Resolve region from multiple sources (CLI flag > target config > global config)
	region := opts.Region
	if region == "" {
		region = amiTarget.Region
	}
	if region == "" && s.globalConfig != nil {
		region = s.globalConfig.AWS.Region
	}
	if region == "" {
		return nil, fmt.Errorf("AWS region must be specified (use --region flag, set in template, or configure in global config)")
	}

	opts.Region = region

	result, err := amiBuilder.Build(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("AMI build failed: %w", err)
	}

	logging.InfoContext(ctx, "AMI build completed successfully: %s", result.AMIID)
	return result, nil
}

// ExecuteAzureBuild performs a complete Azure image build workflow with the
// provided Azure builder. Mirrors ExecuteAMIBuild: handles target selection
// and location/identity resolution, then delegates to the builder.
// The caller is responsible for creating and closing the Azure builder to
// avoid import cycles.
func (s *BuildService) ExecuteAzureBuild(ctx context.Context, config Config, opts BuildOptions, azBuilder AzureImageBuilder) (*BuildResult, error) {
	logging.InfoContext(ctx, "Executing Azure image build")

	azureTarget := findAzureTarget(config.Targets)
	if azureTarget == nil {
		return nil, fmt.Errorf("no azure target found in configuration")
	}

	s.applyAzureGlobalDefaults(azureTarget)

	if azureTarget.Location == "" {
		return nil, fmt.Errorf("azure location must be specified (target.location, --location, or global config azure.location)")
	}

	result, err := azBuilder.Build(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("azure build failed: %w", err)
	}

	if len(azureTarget.ShareWith) > 0 {
		logging.InfoContext(ctx, "Sharing gallery image version with %d principal(s)", len(azureTarget.ShareWith))
		if err := azBuilder.Share(ctx, result.GalleryImageVersionID, azureTarget.ShareWith); err != nil {
			return nil, fmt.Errorf("azure share failed: %w", err)
		}
	}

	logging.InfoContext(ctx, "Azure image build completed: %s", result.GalleryImageVersionID)
	return result, nil
}

// ExecuteProxmoxBuild performs a complete Proxmox template build workflow
// with the provided Proxmox builder. Mirrors ExecuteAzureBuild: handles
// target selection and node defaulting, then delegates to the builder.
// The caller is responsible for creating and closing the Proxmox builder
// to avoid import cycles.
func (s *BuildService) ExecuteProxmoxBuild(ctx context.Context, config Config, opts BuildOptions, pmBuilder ProxmoxImageBuilder) (*BuildResult, error) {
	logging.InfoContext(ctx, "Executing Proxmox template build")

	proxmoxTarget := findProxmoxTarget(config.Targets)
	if proxmoxTarget == nil {
		return nil, fmt.Errorf("no proxmox target found in configuration")
	}

	s.applyProxmoxGlobalDefaults(proxmoxTarget)

	if proxmoxTarget.Node == "" {
		return nil, fmt.Errorf("proxmox node must be specified (target.node or global config proxmox.node)")
	}

	result, err := pmBuilder.Build(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("proxmox build failed: %w", err)
	}

	logging.InfoContext(ctx, "Proxmox template build completed: VMID %d (%s)", result.TemplateVMID, result.TemplateName)
	return result, nil
}

// findProxmoxTarget returns a pointer to the first proxmox target in
// targets, or nil if none exists.
func findProxmoxTarget(targets []Target) *Target {
	for i := range targets {
		if targets[i].Type == "proxmox" {
			return &targets[i]
		}
	}
	return nil
}

// applyProxmoxGlobalDefaults fills empty fields on proxmoxTarget from the
// service's global config (if set). Fields already populated on the target
// take precedence.
func (s *BuildService) applyProxmoxGlobalDefaults(proxmoxTarget *Target) {
	if s.globalConfig == nil {
		return
	}
	if proxmoxTarget.Node == "" {
		proxmoxTarget.Node = s.globalConfig.Proxmox.Node
	}
	if proxmoxTarget.Storage == "" {
		proxmoxTarget.Storage = s.globalConfig.Proxmox.Storage
	}
	if proxmoxTarget.Pool == "" {
		proxmoxTarget.Pool = s.globalConfig.Proxmox.Pool
	}
}

// findAzureTarget returns a pointer to the first azure target in targets, or
// nil if none exists.
func findAzureTarget(targets []Target) *Target {
	for i := range targets {
		if targets[i].Type == "azure" {
			return &targets[i]
		}
	}
	return nil
}

// applyAzureGlobalDefaults fills empty fields on azureTarget from the service's
// global config (if set). Fields already populated on the target take precedence.
func (s *BuildService) applyAzureGlobalDefaults(azureTarget *Target) {
	if s.globalConfig == nil {
		return
	}
	if azureTarget.Location == "" {
		azureTarget.Location = s.globalConfig.Azure.Location
	}
	if azureTarget.ResourceGroup == "" {
		azureTarget.ResourceGroup = s.globalConfig.Azure.ResourceGroup
	}
	if azureTarget.IdentityID == "" {
		azureTarget.IdentityID = s.globalConfig.Azure.IdentityID
	}
	if azureTarget.SubscriptionID == "" {
		azureTarget.SubscriptionID = s.globalConfig.Azure.SubscriptionID
	}
	if azureTarget.VMSize == "" {
		azureTarget.VMSize = s.globalConfig.Azure.Image.VMSize
	}
}

// Push pushes build results to a registry and optionally saves digests.
// For multi-arch builds, it also creates and pushes a multi-arch manifest.
func (s *BuildService) Push(ctx context.Context, config Config, results []BuildResult, opts BuildOptions) error {
	if opts.Registry == "" {
		return fmt.Errorf("registry must be specified for push")
	}

	// Resolve the same overrides the build resolved. ExecuteContainerBuild takes
	// its Config by value, so --tag, --registry and the resolved architectures
	// never reach the caller's copy, and the caller passes that unresolved copy
	// here. Composing manifest names from it published a release under the
	// template's own version instead of the tag the operator asked for.
	ApplyOverrides(ctx, &config, opts, s.globalConfig)

	bldr, err := s.buildKitCreator(ctx)
	if err != nil {
		return fmt.Errorf("failed to create builder for push: %w", err)
	}
	defer func() {
		if closeErr := bldr.Close(); closeErr != nil {
			logging.ErrorContext(ctx, "Failed to close builder after push", "error", closeErr)
		}
	}()

	// Single arch push
	if len(results) == 1 {
		return s.pushSingleArch(ctx, &config, results[0], bldr, opts)
	}

	// Multi-arch push
	return s.pushMultiArch(ctx, &config, results, bldr, opts)
}

// DetermineTargetType determines the target type from configuration and options.
func DetermineTargetType(config *Config, opts BuildOptions) string {
	// CLI override takes precedence
	if opts.TargetType != "" {
		return opts.TargetType
	}

	if len(config.Targets) > 0 {
		return config.Targets[0].Type
	}

	// Default to container
	return "container"
}

// executeSingleArchBuild executes a single-architecture build
func (s *BuildService) executeSingleArchBuild(ctx context.Context, config *Config, bldr ContainerBuilder, opts BuildOptions) (*BuildResult, error) {
	if config.Base.Platform == "" && len(config.Architectures) > 0 {
		config.Base.Platform = fmt.Sprintf("linux/%s", config.Architectures[0])
	}

	bldr.SetCacheOptions(ctx, opts.CacheFrom, opts.CacheTo)

	result, err := bldr.Build(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("container build failed: %w", err)
	}

	return result, nil
}

// executeMultiArchBuild executes a multi-architecture build
func (s *BuildService) executeMultiArchBuild(ctx context.Context, config *Config, bldr ContainerBuilder, opts BuildOptions) ([]BuildResult, error) {
	logging.InfoContext(ctx, "Executing multi-arch build for %d architectures: %v", len(config.Architectures), config.Architectures)

	bldr.SetCacheOptions(ctx, opts.CacheFrom, opts.CacheTo)

	concurrency := DefaultMaxConcurrency
	if s.globalConfig != nil && s.globalConfig.Build.Concurrency > 0 {
		concurrency = s.globalConfig.Build.Concurrency
	}

	orchestrator := NewBuildOrchestrator(concurrency)
	requests := CreateBuildRequests(ctx, config)
	results, err := orchestrator.BuildMultiArch(ctx, requests, bldr)
	if err != nil {
		return nil, fmt.Errorf("multi-arch build failed: %w", err)
	}

	return results, nil
}

// pushAdditionalTags publishes the extra references the template declared for an
// image. A digest push publishes no tag at all, so the extra references are
// skipped there instead of being pushed as tags the caller asked not to create.
func pushAdditionalTags(ctx context.Context, refs []string, registry string, pushDigest bool, bldr ContainerBuilder) error {
	if len(refs) == 0 {
		return nil
	}

	if pushDigest {
		logging.InfoContext(ctx, "Skipping %d additional tag(s): --push-digest publishes by digest, not by tag", len(refs))
		return nil
	}

	for _, ref := range refs {
		logging.InfoContext(ctx, "Pushing additional tag: %s", ref)

		if _, err := bldr.Push(ctx, ref, registry); err != nil {
			return fmt.Errorf("failed to push additional tag %q: %w", ref, err)
		}
	}

	return nil
}

// pushSingleArch pushes a single architecture image
func (s *BuildService) pushSingleArch(ctx context.Context, config *Config, result BuildResult, bldr ContainerBuilder, opts BuildOptions) error {
	logging.InfoContext(ctx, "Pushing to registry: %s", opts.Registry)

	pushFn := bldr.Push
	if opts.PushDigest {
		pushFn = bldr.PushDigest
	}

	digest, err := pushFn(ctx, result.ImageRef, opts.Registry)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	if err := pushAdditionalTags(ctx, result.AdditionalRefs, opts.Registry, opts.PushDigest, bldr); err != nil {
		return err
	}

	// Use the digest from Push if available, otherwise fall back to result.Digest
	if digest == "" {
		digest = result.Digest
	}

	if opts.SaveDigests && digest != "" {
		arch := result.Architecture
		if arch == "" {
			arch = "unknown"
			if len(config.Architectures) > 0 {
				arch = config.Architectures[0]
			}
		}
		if err := manifests.SaveDigestToFile(ctx, config.Name, arch, digest, opts.DigestDir); err != nil {
			logging.WarnContext(ctx, "Failed to save digest: %v", err)
		} else {
			logging.InfoContext(ctx, "Saved digest for %s: %s", arch, digest)
		}
	}

	logging.InfoContext(ctx, "Successfully pushed to %s", opts.Registry)
	return nil
}

// pushMultiArch pushes multi-architecture images and creates a manifest
func (s *BuildService) pushMultiArch(ctx context.Context, config *Config, results []BuildResult, bldr ContainerBuilder, opts BuildOptions) error {
	logging.InfoContext(ctx, "Pushing multi-arch images to registry: %s", opts.Registry)

	// Use configured concurrency
	concurrency := DefaultMaxConcurrency
	if s.globalConfig != nil && s.globalConfig.Build.Concurrency > 0 {
		concurrency = s.globalConfig.Build.Concurrency
	}

	orchestrator := NewBuildOrchestrator(concurrency)

	// Push individual architecture images
	if err := orchestrator.PushMultiArch(ctx, results, opts.Registry, opts.PushDigest, bldr); err != nil {
		return fmt.Errorf("failed to push multi-arch images: %w", err)
	}

	// Save digests if requested
	if opts.SaveDigests {
		s.saveDigests(ctx, config.Name, results, opts.DigestDir)
	}

	if err := s.publishManifestTags(ctx, config, results, bldr, opts); err != nil {
		return err
	}

	logging.InfoContext(ctx, "Successfully pushed multi-arch images to %s", opts.Registry)
	return nil
}

// publishManifestTags publishes the template's version and additional tags as
// manifest lists spanning the architecture images just pushed. A multi-arch
// build produces one image per architecture, each tagged with its architecture,
// so a release tag can only name the list that unites them; tagging one
// architecture with it would publish a release that resolves to whichever
// architecture happened to finish last.
func (s *BuildService) publishManifestTags(ctx context.Context, config *Config, results []BuildResult, bldr ContainerBuilder, opts BuildOptions) error {
	if opts.PushDigest {
		logging.InfoContext(ctx, "Skipping manifest tags: --push-digest publishes by digest, not by tag")
		return nil
	}

	// The architecture images are pushed by now, so a manifest that cannot be
	// described leaves the release half-published: name the command that
	// finishes the job rather than failing silently.
	entries, err := CreateManifestEntries(ctx, results)
	if err != nil {
		return fmt.Errorf("%w; publish the tags with 'warpgate manifests create'", err)
	}

	refs := append([]string{PrimaryImageRef(*config)}, AdditionalTagRefs(*config)...)
	for _, ref := range refs {
		logging.InfoContext(ctx, "Publishing multi-arch manifest: %s", ref)

		if err := bldr.CreateAndPushManifest(ctx, ref, entries); err != nil {
			return fmt.Errorf("failed to publish manifest %q: %w", ref, err)
		}
	}

	return nil
}

// saveDigests saves digests for all architectures
func (s *BuildService) saveDigests(ctx context.Context, imageName string, results []BuildResult, digestDir string) {
	logging.InfoContext(ctx, "Saving image digests to %s", digestDir)
	for _, result := range results {
		if result.Digest != "" {
			if err := manifests.SaveDigestToFile(ctx, imageName, result.Architecture, result.Digest, digestDir); err != nil {
				logging.WarnContext(ctx, "Failed to save digest for %s: %v", result.Architecture, err)
			}
		}
	}
}

// CreateManifestEntries describes every build result as a manifest entry. The
// actual manifest creation and push should be done in the command layer since it
// requires platform-specific handling.
//
// Every result has to be describable. Dropping the one whose digest will not
// parse would publish a manifest list silently missing an architecture, which is
// worse than leaving the previous list in place, so an undescribable result
// fails the whole set instead of shrinking it.
func CreateManifestEntries(ctx context.Context, results []BuildResult) ([]manifests.ManifestEntry, error) {
	entries := make([]manifests.ManifestEntry, 0, len(results))
	var undescribable []string

	for _, result := range results {
		var imageDigest digest.Digest
		if result.Digest != "" {
			parsed, err := digest.Parse(result.Digest)
			if err != nil {
				name := describeResult(result)
				logging.WarnContext(ctx, "Failed to parse digest for %s: %v", name, err)
				undescribable = append(undescribable, name)
				continue
			}
			imageDigest = parsed
		}

		platformInfo := manifests.ParsePlatform(result.Platform)

		entries = append(entries, manifests.ManifestEntry{
			ImageRef:     result.ImageRef,
			Digest:       imageDigest,
			Platform:     result.Platform,
			Architecture: platformInfo.Architecture,
			OS:           platformInfo.OS,
			Variant:      platformInfo.Variant,
		})
	}

	if len(undescribable) > 0 {
		return nil, fmt.Errorf("only %d of %d architectures could be described for the manifest: %s",
			len(entries), len(results), strings.Join(undescribable, ", "))
	}

	return entries, nil
}

// describeResult names a build result for an error message, preferring the
// architecture and falling back through the fields that are still set when a
// push returned nothing usable.
func describeResult(result BuildResult) string {
	for _, name := range []string{result.Architecture, result.Platform, result.ImageRef} {
		if name != "" {
			return name
		}
	}

	return "unknown architecture"
}
