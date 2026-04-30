/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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

package ami

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"golang.org/x/sync/errgroup"
)

// componentForceBumpMaxAttempts caps the bump-and-retry loop used when --force
// races with concurrent builds (or accumulated stale components) for the same
// next-available patch version. Each attempt re-queries Image Builder for the
// current highest patch, so even with hundreds of stale versions the loop
// converges quickly — but we bound it to keep a misbehaving environment from
// looping forever.
const componentForceBumpMaxAttempts = 10

// ImageBuilder implements the AMIBuilder interface
type ImageBuilder struct {
	clients         *AWSClients
	componentGen    *ComponentGenerator
	pipelineManager *PipelineManager
	operations      *AMIOperations
	resourceManager *ResourceManager
	fileStager      *FileStager // nil when no staging bucket is configured
	config          ClientConfig
	globalConfig    *config.Config
	forceRecreate   bool
	cleanupOnFinish bool          // Delete all build resources (components, configs, recipe, pipeline) after build
	buildID         string        // Unique identifier for this build
	namingPrefix    string        // Optional prefix for resource naming
	monitorConfig   MonitorConfig // Configuration for build monitoring
}

// Verify that ImageBuilder implements builder.AMIBuilder at compile time
var _ builder.AMIBuilder = (*ImageBuilder)(nil)

// NewImageBuilder creates a new AMI builder
func NewImageBuilder(ctx context.Context, clientConfig ClientConfig) (*ImageBuilder, error) {
	return NewImageBuilderWithOptions(ctx, clientConfig, false)
}

// NewImageBuilderWithOptions creates a new AMI builder with additional options
func NewImageBuilderWithOptions(ctx context.Context, clientConfig ClientConfig, forceRecreate bool) (*ImageBuilder, error) {
	return NewImageBuilderWithAllOptions(ctx, clientConfig, forceRecreate, MonitorConfig{})
}

// NewImageBuilderWithAllOptions creates a new AMI builder with all options including monitoring
func NewImageBuilderWithAllOptions(ctx context.Context, clientConfig ClientConfig, forceRecreate bool, monitorConfig MonitorConfig) (*ImageBuilder, error) {
	globalCfg, err := config.Load()
	if err != nil && !config.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	clients, err := NewAWSClients(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS clients: %w", err)
	}

	buildID := generateBuildID()
	pipelineManager := NewPipelineManagerWithMonitor(clients, monitorConfig)

	var fileStager *FileStager
	if globalCfg != nil {
		fileStager = NewFileStager(clients.S3, globalCfg.AWS.AMI.FileStagingBucket)
	}

	return &ImageBuilder{
		clients:         clients,
		componentGen:    NewComponentGenerator(clients),
		pipelineManager: pipelineManager,
		operations:      NewAMIOperations(clients, globalCfg),
		resourceManager: NewResourceManager(clients),
		fileStager:      fileStager,
		config:          clientConfig,
		globalConfig:    globalCfg,
		forceRecreate:   forceRecreate,
		buildID:         buildID,
		monitorConfig:   monitorConfig,
	}, nil
}

// SetMonitorConfig sets the monitor configuration
func (b *ImageBuilder) SetMonitorConfig(config MonitorConfig) {
	b.monitorConfig = config
	b.pipelineManager.SetMonitorConfig(config)
}

// SetCleanupOnFinish controls whether all build resources (components,
// infrastructure configs, distribution configs, recipes, and pipelines)
// are deleted after a successful build. Default is false, which only
// deletes the pipeline.
func (b *ImageBuilder) SetCleanupOnFinish(cleanup bool) {
	b.cleanupOnFinish = cleanup
}

// generateBuildID creates a unique identifier for this build
func generateBuildID() string {
	timestamp := time.Now().Format("20060102-150405")
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to timestamp only if random fails
		return timestamp
	}
	return fmt.Sprintf("%s-%s", timestamp, hex.EncodeToString(randomBytes))
}

// SetNamingPrefix sets a custom prefix for resource naming
func (b *ImageBuilder) SetNamingPrefix(prefix string) {
	b.namingPrefix = prefix
}

// GetBuildID returns the unique build identifier
func (b *ImageBuilder) GetBuildID() string {
	return b.buildID
}

// Build implements the Builder interface for AMI builds
func (b *ImageBuilder) Build(ctx context.Context, config builder.Config) (*builder.BuildResult, error) {
	startTime := time.Now()
	logging.InfoContext(ctx, "Starting AMI build: %s (version: %s)", config.Name, config.Version)

	amiTarget, err := b.setupBuild(config)
	if err != nil {
		return nil, err
	}

	if b.forceRecreate {
		logging.InfoContext(ctx, "Force recreate enabled, cleaning up existing resources")
		if err := b.resourceManager.CleanupResourcesForBuild(ctx, config.Name, true); err != nil {
			logging.WarnContext(ctx, "Force cleanup encountered errors (continuing): %v", err)
		}
	}

	createdResources := &CreatedResources{}

	// Suppress cleanup logs when a StatusCallback is set, since a progress
	// display is likely active and log output would corrupt the terminal.
	quietCleanup := b.monitorConfig.StatusCallback != nil

	stagedFiles, err := b.stageFileProvisioners(ctx, config)
	if err != nil {
		return nil, err
	}
	defer b.cleanupStagedFiles(ctx, stagedFiles)

	resources, err := b.createBuildResources(ctx, config, amiTarget, createdResources, stagedFiles)
	if err != nil {
		createdResources.Cleanup(ctx, b.resourceManager, quietCleanup)
		return nil, err
	}

	amiID, err := b.executePipeline(ctx, resources, config.Name)
	if err != nil {
		createdResources.Cleanup(ctx, b.resourceManager, quietCleanup)
		return nil, err
	}

	b.finalizeBuild(ctx, amiID, amiTarget, resources.PipelineARN, createdResources)

	duration := time.Since(startTime)
	logging.InfoContext(ctx, "AMI build completed: %s (duration: %s)", amiID, duration)

	return &builder.BuildResult{
		AMIID:    amiID,
		Region:   amiTarget.Region,
		Duration: duration.String(),
		Notes:    []string{fmt.Sprintf("Built in region: %s", amiTarget.Region)},
	}, nil
}

// buildResources holds the ARNs of created AWS resources
type buildResources struct {
	ComponentARNs []string
	InfraARN      string
	DistARN       string
	RecipeARN     string
	PipelineARN   string
}

// setupBuild validates and prepares the build configuration
func (b *ImageBuilder) setupBuild(config builder.Config) (*builder.Target, error) {
	// Find AMI target configuration
	var amiTarget *builder.Target
	for i := range config.Targets {
		if config.Targets[i].Type == "ami" {
			amiTarget = &config.Targets[i]
			break
		}
	}

	if amiTarget == nil {
		return nil, fmt.Errorf("no AMI target found in configuration")
	}

	// Validate AMI configuration
	if err := b.validateConfig(amiTarget); err != nil {
		return nil, fmt.Errorf("invalid AMI configuration: %w", err)
	}

	return amiTarget, nil
}

// createBuildResources creates all necessary AWS resources for the build
func (b *ImageBuilder) createBuildResources(ctx context.Context, config builder.Config, amiTarget *builder.Target, created *CreatedResources, stagedFiles map[int]*StagedFile) (*buildResources, error) {
	componentARNs, err := b.createComponents(ctx, config, created, stagedFiles)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create components")
	}

	infraARN, err := b.getOrCreateInfrastructureConfig(ctx, config.Name, amiTarget, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create infrastructure config")
	}

	distARN, err := b.getOrCreateDistributionConfig(ctx, config.Name, amiTarget, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create distribution config")
	}

	recipeARN, err := b.getOrCreateImageRecipe(ctx, config, componentARNs, amiTarget, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create image recipe")
	}

	pipelineARN, err := b.getOrCreatePipeline(ctx, config, recipeARN, infraARN, distARN, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create pipeline")
	}

	return &buildResources{
		ComponentARNs: componentARNs,
		InfraARN:      infraARN,
		DistARN:       distARN,
		RecipeARN:     recipeARN,
		PipelineARN:   pipelineARN,
	}, nil
}

// executePipeline runs the Image Builder pipeline and waits for completion
func (b *ImageBuilder) executePipeline(ctx context.Context, resources *buildResources, imageName string) (string, error) {
	imageARN, err := b.pipelineManager.StartPipeline(ctx, resources.PipelineARN)
	if err != nil {
		return "", fmt.Errorf("failed to start pipeline: %w", err)
	}

	image, err := b.pipelineManager.WaitForPipelineCompletionWithImageName(ctx, *imageARN, 30*time.Second, imageName)
	if err != nil {
		return "", fmt.Errorf("pipeline execution failed: %w", err)
	}

	amiID, err := b.extractAMIID(image)
	if err != nil {
		return "", fmt.Errorf("failed to extract AMI ID: %w", err)
	}

	return amiID, nil
}

// finalizeBuild tags the AMI and cleans up temporary resources. When
// cleanupOnFinish is true, all build resources are deleted; otherwise
// only the pipeline is removed.
func (b *ImageBuilder) finalizeBuild(ctx context.Context, amiID string, target *builder.Target, pipelineARN string, created *CreatedResources) {
	// Tag the AMI
	if len(target.AMITags) > 0 {
		if err := b.operations.TagAMI(ctx, amiID, target.AMITags); err != nil {
			logging.WarnContext(ctx, "Failed to tag AMI: %v", err)
		}
	}

	if b.cleanupOnFinish {
		logging.InfoContext(ctx, "Cleaning up all build resources (cleanup-on-finish enabled)")
		created.Cleanup(ctx, b.resourceManager)
	} else {
		// Default: only delete the pipeline.
		if err := b.pipelineManager.DeletePipeline(ctx, pipelineARN); err != nil {
			logging.WarnContext(ctx, "Failed to delete pipeline: %v", err)
		}
	}
}

// stageFileProvisioners uploads any `file` provisioner sources to the
// configured S3 staging bucket so the build instance can fetch them via
// S3Download. Returns a map keyed by provisioner index. If no `file`
// provisioners are present, returns an empty map and no error.
func (b *ImageBuilder) stageFileProvisioners(ctx context.Context, cfg builder.Config) (map[int]*StagedFile, error) {
	staged := make(map[int]*StagedFile)

	hasFileProvisioner := false
	for _, prov := range cfg.Provisioners {
		if prov.Type == "file" {
			hasFileProvisioner = true
			break
		}
	}
	if !hasFileProvisioner {
		return staged, nil
	}

	if b.fileStager == nil {
		return nil, fmt.Errorf("file provisioner requires aws.ami.file_staging_bucket to be configured")
	}

	for i, prov := range cfg.Provisioners {
		if prov.Type != "file" {
			continue
		}
		if prov.Source == "" {
			return nil, fmt.Errorf("file provisioner %d: source is required", i)
		}
		keyPrefix := fmt.Sprintf("warpgate-staging/%s/%d", b.buildID, i)
		s, err := b.fileStager.Stage(ctx, prov.Source, keyPrefix)
		if err != nil {
			b.cleanupStagedFiles(ctx, staged)
			return nil, fmt.Errorf("stage file provisioner %d: %w", i, err)
		}
		staged[i] = s
	}
	return staged, nil
}

// cleanupStagedFiles removes any S3 objects uploaded for `file` provisioners.
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

// Share implements the AMIBuilder interface
func (b *ImageBuilder) Share(ctx context.Context, amiID string, accountIDs []string) error {
	return b.operations.ShareAMI(ctx, amiID, accountIDs)
}

// Copy implements the AMIBuilder interface
func (b *ImageBuilder) Copy(ctx context.Context, amiID, sourceRegion, destRegion string) (string, error) {
	return b.operations.CopyAMI(ctx, amiID, sourceRegion, destRegion, fmt.Sprintf("%s-copy", amiID))
}

// Deregister implements the AMIBuilder interface
func (b *ImageBuilder) Deregister(ctx context.Context, amiID, region string) error {
	// If region is different from current, create a new client for that region
	if region != "" && region != b.config.Region {
		regionConfig := b.config
		regionConfig.Region = region
		regionClients, err := NewAWSClients(ctx, regionConfig)
		if err != nil {
			return fmt.Errorf("failed to create clients for region %s: %w", region, err)
		}
		regionOps := NewAMIOperations(regionClients, b.globalConfig)
		return regionOps.DeregisterAMI(ctx, amiID, true)
	}

	return b.operations.DeregisterAMI(ctx, amiID, true)
}

// Close implements the Builder interface
func (b *ImageBuilder) Close() error {
	// Create a background context since we don't have one passed in
	ctx := context.Background()
	logging.InfoContext(ctx, "Closing AMI builder")
	// No resources to clean up currently
	return nil
}

// createComponents creates Image Builder components from provisioners in parallel using errgroup
func (b *ImageBuilder) createComponents(ctx context.Context, config builder.Config, created *CreatedResources, stagedFiles map[int]*StagedFile) ([]string, error) {
	numProvisioners := len(config.Provisioners)
	if numProvisioners == 0 {
		return nil, nil
	}

	logging.InfoContext(ctx, "Creating %d components in parallel", numProvisioners)

	componentARNs := make([]string, numProvisioners)
	g, ctx := errgroup.WithContext(ctx)

	for i, provisioner := range config.Provisioners {
		index := i
		prov := provisioner

		g.Go(func() error {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context cancelled before creating component %d: %w", index, err)
			}

			componentName := config.Name + "-" + fmt.Sprint(index)
			logging.InfoContext(ctx, "Creating component: %s (index: %d)", prov.Type, index)

			baseVersion := NormalizeSemanticVersion(config.Version)
			if prov.ComponentVersion != "" {
				baseVersion = NormalizeSemanticVersion(prov.ComponentVersion)
				logging.InfoContext(ctx, "Using provisioner-specified component version: %s", baseVersion)
			}

			opts := GenerateComponentOpts{BumpOnConflict: b.forceRecreate}
			if staged, ok := stagedFiles[index]; ok {
				opts.StagedFile = staged
			}

			fullComponentName := componentName + "-" + prov.Type
			arn, err := b.createComponentWithBump(ctx, prov, componentName, fullComponentName, baseVersion, opts)
			if err != nil {
				return fmt.Errorf("failed to create component %d (%s): %w", index, prov.Type, err)
			}

			componentARNs[index] = *arn
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		for _, arn := range componentARNs {
			if arn != "" {
				created.ComponentARNs = append(created.ComponentARNs, arn)
			}
		}
		return nil, err
	}

	created.ComponentARNs = append(created.ComponentARNs, componentARNs...)

	for i, arn := range componentARNs {
		if arn == "" {
			return nil, fmt.Errorf("component %d was not created", i)
		}
	}

	logging.InfoContext(ctx, "Successfully created %d components", numProvisioners)
	return componentARNs, nil
}

// createComponentWithBump creates an Image Builder component, retrying with
// the next available patch version when a duplicate-version conflict occurs.
// Conflicts are common with --force in two scenarios:
//
//  1. Stale leftover versions from prior failed builds inflate the version
//     space, but we still pick "highest+1" — fine on its own, but…
//  2. Two concurrent builds both compute the same "highest+1" between their
//     ListComponents and CreateComponent calls. CreateComponent is the only
//     authoritative arbiter, so on conflict we re-list and try again.
//
// When BumpOnConflict is unset (the non-force path) GenerateComponent latches
// onto the existing ARN exactly like before — this loop runs at most once.
func (b *ImageBuilder) createComponentWithBump(ctx context.Context, prov builder.Provisioner, componentName, fullComponentName, baseVersion string, opts GenerateComponentOpts) (*string, error) {
	version := baseVersion
	if opts.BumpOnConflict {
		nextVersion, err := b.resourceManager.GetNextComponentVersion(ctx, fullComponentName, version)
		if err == nil && nextVersion != version {
			logging.InfoContext(ctx, "Bumping component %s to next available version: %s", fullComponentName, nextVersion)
			version = nextVersion
		}
	}

	for attempt := 1; attempt <= componentForceBumpMaxAttempts; attempt++ {
		arn, err := b.componentGen.GenerateComponent(ctx, prov, componentName, version, opts)
		if err == nil {
			return arn, nil
		}
		if !errors.Is(err, ErrComponentVersionExists) {
			return nil, err
		}

		nextVersion, bumpErr := b.resourceManager.GetNextComponentVersion(ctx, fullComponentName, baseVersion)
		if bumpErr != nil {
			return nil, fmt.Errorf("conflict on %s@%s and re-bump failed: %w", fullComponentName, version, bumpErr)
		}
		if nextVersion == version {
			// Another writer must have raced us *and* taken the same patch we
			// just attempted. Synthesize the next slot manually so we make
			// forward progress on the very next try.
			nextVersion = bumpPatch(version)
		}
		logging.InfoContext(ctx, "Component %s@%s race lost (attempt %d/%d), retrying with %s",
			fullComponentName, version, attempt, componentForceBumpMaxAttempts, nextVersion)
		version = nextVersion
	}
	return nil, fmt.Errorf("exhausted %d bump attempts creating component %s; another build is concurrently bumping the same name", componentForceBumpMaxAttempts, fullComponentName)
}

// bumpPatch returns version with its third component incremented by one. If
// the input has fewer than three dotted components or a non-numeric patch, it
// falls back to "<input>.1" — close enough to the real format that the next
// GetNextComponentVersion call will recover.
func bumpPatch(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return version + ".1"
	}
	var patch int
	if _, err := fmt.Sscanf(parts[2], "%d", &patch); err != nil {
		return version + ".1"
	}
	parts[2] = fmt.Sprintf("%d", patch+1)
	return strings.Join(parts, ".")
}

// createInfrastructureConfig creates an infrastructure configuration
func (b *ImageBuilder) createInfrastructureConfig(ctx context.Context, name string, target *builder.Target) (string, error) {
	logging.InfoContext(ctx, "Creating infrastructure configuration")

	instanceType := target.InstanceType
	if instanceType == "" {
		instanceType = b.globalConfig.AWS.AMI.InstanceType
	}

	instanceProfile := target.InstanceProfileName
	if instanceProfile == "" {
		instanceProfile = b.globalConfig.AWS.AMI.InstanceProfileName
	}
	if instanceProfile == "" {
		return "", fmt.Errorf("instance_profile_name must be specified in template config or global config (aws.ami.instance_profile_name)")
	}

	infraName := fmt.Sprintf("%s-infra", name)
	input := &imagebuilder.CreateInfrastructureConfigurationInput{
		Name:                aws.String(infraName),
		InstanceTypes:       []string{instanceType},
		InstanceProfileName: aws.String(instanceProfile),
		Description:         aws.String(fmt.Sprintf("Infrastructure config for %s", name)),
		Tags: map[string]string{
			"warpgate:name": name,
		},
	}

	if target.SubnetID != "" {
		input.SubnetId = aws.String(target.SubnetID)
	}

	if len(target.SecurityGroupIDs) > 0 {
		input.SecurityGroupIds = target.SecurityGroupIDs
	}

	result, err := b.clients.ImageBuilder.CreateInfrastructureConfiguration(ctx, input)
	if err != nil {
		if IsResourceExistsError(err) {
			logging.InfoContext(ctx, "Infrastructure configuration already exists, retrieving: %s", infraName)
			existing, getErr := b.resourceManager.GetInfrastructureConfig(ctx, infraName)
			if getErr != nil {
				return "", fmt.Errorf("failed to create infrastructure config (already exists) and failed to retrieve: %w", err)
			}
			if existing != nil && existing.Arn != nil {
				logging.InfoContext(ctx, "Reusing existing infrastructure configuration: %s", *existing.Arn)
				return *existing.Arn, nil
			}
			return "", fmt.Errorf("failed to create infrastructure config (already exists) but could not retrieve: %w", err)
		}
		return "", fmt.Errorf("failed to create infrastructure config: %w", err)
	}

	return *result.InfrastructureConfigurationArn, nil
}

// createDistributionConfig creates a distribution configuration
func (b *ImageBuilder) createDistributionConfig(ctx context.Context, name string, target *builder.Target) (string, error) {
	logging.InfoContext(ctx, "Creating distribution configuration")

	region := target.Region
	if region == "" {
		region = b.clients.GetRegion()
	}

	amiName := normalizeAMIName(target.AMIName, name)

	amiDistConfig := &types.AmiDistributionConfiguration{
		Name:        aws.String(amiName),
		Description: aws.String(fmt.Sprintf("AMI for %s", name)),
		AmiTags:     target.AMITags,
	}

	distribution := types.Distribution{
		Region:                       aws.String(region),
		AmiDistributionConfiguration: amiDistConfig,
	}

	// Configure Windows Fast Launch if enabled
	if target.FastLaunchEnabled {
		fastLaunchConfig := b.buildFastLaunchConfiguration(target)
		distribution.FastLaunchConfigurations = []types.FastLaunchConfiguration{fastLaunchConfig}
	}

	distName := fmt.Sprintf("%s-dist", name)
	input := &imagebuilder.CreateDistributionConfigurationInput{
		Name:          aws.String(distName),
		Description:   aws.String(fmt.Sprintf("Distribution config for %s", name)),
		Distributions: []types.Distribution{distribution},
		Tags: map[string]string{
			"warpgate:name": name,
		},
	}

	result, err := b.clients.ImageBuilder.CreateDistributionConfiguration(ctx, input)
	if err != nil {
		if IsResourceExistsError(err) {
			logging.InfoContext(ctx, "Distribution configuration already exists, retrieving: %s", distName)
			existing, getErr := b.resourceManager.GetDistributionConfig(ctx, distName)
			if getErr != nil {
				return "", fmt.Errorf("failed to create distribution config (already exists) and failed to retrieve: %w", err)
			}
			if existing != nil && existing.Arn != nil {
				logging.InfoContext(ctx, "Reusing existing distribution configuration: %s", *existing.Arn)
				return *existing.Arn, nil
			}
			return "", fmt.Errorf("failed to create distribution config (already exists) but could not retrieve: %w", err)
		}
		return "", fmt.Errorf("failed to create distribution config: %w", err)
	}

	return *result.DistributionConfigurationArn, nil
}

func (b *ImageBuilder) buildFastLaunchConfiguration(target *builder.Target) types.FastLaunchConfiguration {
	maxParallelLaunches := target.FastLaunchMaxParallelLaunches
	if maxParallelLaunches == 0 {
		maxParallelLaunches = 6 // AWS default
	}

	targetResourceCount := target.FastLaunchTargetResourceCount
	if targetResourceCount == 0 {
		targetResourceCount = 5
	}

	return types.FastLaunchConfiguration{
		Enabled:             true,
		MaxParallelLaunches: aws.Int32(int32(maxParallelLaunches)),
		SnapshotConfiguration: &types.FastLaunchSnapshotConfiguration{
			TargetResourceCount: aws.Int32(int32(targetResourceCount)),
		},
	}
}

// parseVolumeType converts a volume type string to the AWS EbsVolumeType enum.
func parseVolumeType(volumeTypeStr string) types.EbsVolumeType {
	volumeTypes := map[string]types.EbsVolumeType{
		"gp2":      types.EbsVolumeTypeGp2,
		"gp3":      types.EbsVolumeTypeGp3,
		"io1":      types.EbsVolumeTypeIo1,
		"io2":      types.EbsVolumeTypeIo2,
		"sc1":      types.EbsVolumeTypeSc1,
		"st1":      types.EbsVolumeTypeSt1,
		"standard": types.EbsVolumeTypeStandard,
	}
	if vt, ok := volumeTypes[volumeTypeStr]; ok {
		return vt
	}
	return types.EbsVolumeTypeGp3
}

// resolveSSMParameterARN resolves a full SSM parameter ARN to its value (an AMI ID).
// If the input is not an SSM parameter ARN, it is returned unchanged.
func (b *ImageBuilder) resolveSSMParameterARN(ctx context.Context, parentImage string) (string, error) {
	if !strings.HasPrefix(parentImage, "arn:aws:ssm:") || !strings.Contains(parentImage, ":parameter/") {
		return parentImage, nil
	}

	paramName := parentImage[strings.Index(parentImage, ":parameter")+len(":parameter"):]
	logging.InfoContext(ctx, "Resolving SSM parameter to AMI ID: %s", paramName)

	ssmOutput, err := b.clients.SSM.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(paramName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve SSM parameter %q to AMI ID: %w", paramName, err)
	}

	if ssmOutput.Parameter != nil && ssmOutput.Parameter.Value != nil {
		logging.InfoContext(ctx, "Resolved parent image to AMI: %s", *ssmOutput.Parameter.Value)
		return *ssmOutput.Parameter.Value, nil
	}

	return parentImage, nil
}

// resolveAMIFilters resolves AMI filters to the latest matching AMI ID using EC2 DescribeImages.
func (b *ImageBuilder) resolveAMIFilters(ctx context.Context, filters *builder.AMIFilterConfig) (string, error) {
	if len(filters.Owners) == 0 {
		return "", fmt.Errorf("ami_filters.owners must specify at least one owner")
	}
	if len(filters.Filters) == 0 {
		return "", fmt.Errorf("ami_filters.filters must specify at least one filter")
	}

	ec2Filters := make([]ec2types.Filter, 0, len(filters.Filters))
	for name, value := range filters.Filters {
		ec2Filters = append(ec2Filters, ec2types.Filter{
			Name:   aws.String(name),
			Values: []string{value},
		})
	}

	// Always filter for available images unless the user explicitly set "state"
	if _, hasState := filters.Filters["state"]; !hasState {
		ec2Filters = append(ec2Filters, ec2types.Filter{
			Name:   aws.String("state"),
			Values: []string{"available"},
		})
	}

	input := &ec2.DescribeImagesInput{
		Owners:  filters.Owners,
		Filters: ec2Filters,
	}

	logging.InfoContext(ctx, "Resolving AMI from filters (owners: %v, filters: %d)", filters.Owners, len(filters.Filters))

	output, err := b.clients.EC2.DescribeImages(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to resolve AMI from filters: %w", err)
	}

	if len(output.Images) == 0 {
		filterDesc := make([]string, 0, len(filters.Filters))
		for k, v := range filters.Filters {
			filterDesc = append(filterDesc, fmt.Sprintf("%s=%s", k, v))
		}
		return "", fmt.Errorf("no AMIs found matching filters (owners: %v, filters: [%s])",
			filters.Owners, strings.Join(filterDesc, ", "))
	}

	// Sort by CreationDate descending (RFC 3339 strings sort lexicographically)
	sort.Slice(output.Images, func(i, j int) bool {
		return aws.ToString(output.Images[i].CreationDate) > aws.ToString(output.Images[j].CreationDate)
	})

	selected := output.Images[0]
	amiID := aws.ToString(selected.ImageId)
	amiName := aws.ToString(selected.Name)
	logging.InfoContext(ctx, "Resolved AMI from filters: %s (%s, created: %s)",
		amiID, amiName, aws.ToString(selected.CreationDate))

	return amiID, nil
}

// resolveParentImage resolves the parent image from config, handling:
//   - AMI Filters (resolved via EC2 DescribeImages)
//   - Direct AMI IDs (passthrough)
//   - SSM Parameter ARNs (resolved via SSM GetParameter)
func (b *ImageBuilder) resolveParentImage(ctx context.Context, config builder.Config) (string, error) {
	if config.Base.AMIFilters != nil {
		return b.resolveAMIFilters(ctx, config.Base.AMIFilters)
	}

	parentImage := config.Base.Image
	if parentImage == "" {
		parentImage = b.globalConfig.AWS.AMI.DefaultParentImage
		if parentImage == "" {
			return "", fmt.Errorf("parent image must be specified via base.image, base.ami_filters, or global config (aws.ami.default_parent_image)")
		}
	}

	return b.resolveSSMParameterARN(ctx, parentImage)
}

func (b *ImageBuilder) createImageRecipe(ctx context.Context, config builder.Config, componentARNs []string, target *builder.Target) (string, error) {
	logging.InfoContext(ctx, "Creating image recipe")

	parentImage, err := b.resolveParentImage(ctx, config)
	if err != nil {
		return "", err
	}

	components := make([]types.ComponentConfiguration, 0, len(componentARNs))
	for _, arn := range componentARNs {
		components = append(components, types.ComponentConfiguration{
			ComponentArn: aws.String(arn),
		})
	}

	volumeSize := int32(target.VolumeSize)
	if volumeSize == 0 {
		volumeSize = int32(b.globalConfig.AWS.AMI.VolumeSize)
	}

	recipeName := fmt.Sprintf("%s-recipe", config.Name)
	normalizedVersion := NormalizeSemanticVersion(config.Version)
	input := &imagebuilder.CreateImageRecipeInput{
		Name:            aws.String(recipeName),
		SemanticVersion: aws.String(normalizedVersion),
		ParentImage:     aws.String(parentImage),
		Components:      components,
		Description:     aws.String(fmt.Sprintf("Image recipe for %s", config.Name)),
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(b.globalConfig.AWS.AMI.DeviceName),
				Ebs: &types.EbsInstanceBlockDeviceSpecification{
					VolumeSize:          aws.Int32(volumeSize),
					VolumeType:          parseVolumeType(b.globalConfig.AWS.AMI.VolumeType),
					DeleteOnTermination: aws.Bool(true),
				},
			},
		},
		Tags: map[string]string{
			"warpgate:name":    config.Name,
			"warpgate:version": config.Version,
		},
	}

	result, err := b.clients.ImageBuilder.CreateImageRecipe(ctx, input)
	if err != nil {
		if IsResourceExistsError(err) {
			logging.InfoContext(ctx, "Image recipe already exists, retrieving: %s", recipeName)
			existing, getErr := b.resourceManager.GetImageRecipe(ctx, recipeName, config.Version)
			if getErr != nil {
				return "", fmt.Errorf("failed to create image recipe (already exists) and failed to retrieve: %w", err)
			}
			if existing != nil && existing.Arn != nil {
				logging.InfoContext(ctx, "Reusing existing image recipe: %s", *existing.Arn)
				return *existing.Arn, nil
			}
			return "", fmt.Errorf("failed to create image recipe (already exists) but could not retrieve: %w", err)
		}
		return "", fmt.Errorf("failed to create image recipe: %w", err)
	}

	return *result.ImageRecipeArn, nil
}

// extractAMIID extracts the AMI ID from the image output
func (b *ImageBuilder) extractAMIID(image *types.Image) (string, error) {
	if image.OutputResources == nil || len(image.OutputResources.Amis) == 0 {
		return "", fmt.Errorf("no AMI output found in image")
	}

	amiID := image.OutputResources.Amis[0].Image
	if amiID == nil {
		return "", fmt.Errorf("AMI ID is nil in output")
	}

	return *amiID, nil
}

// validateConfig validates AMI target configuration
func (b *ImageBuilder) validateConfig(target *builder.Target) error {
	if target.Region == "" && b.config.Region == "" {
		return fmt.Errorf("region must be specified")
	}

	return nil
}

func (b *ImageBuilder) getOrCreateInfrastructureConfig(ctx context.Context, name string, target *builder.Target, created *CreatedResources) (string, error) {
	infraName := fmt.Sprintf("%s-infra", name)

	existing, err := b.resourceManager.GetInfrastructureConfig(ctx, infraName)
	if err != nil && !IsErrNotFound(err) {
		return "", fmt.Errorf("failed to check for existing infrastructure config: %w", err)
	}

	if existing != nil {
		if b.forceRecreate {
			logging.InfoContext(ctx, "Force recreate enabled, deleting existing infrastructure config: %s", *existing.Arn)
			if err := b.resourceManager.DeleteInfrastructureConfig(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing infrastructure config: %w", err)
			}
		} else {
			logging.InfoContext(ctx, "Reusing existing infrastructure configuration: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	arn, err := b.createInfrastructureConfig(ctx, name, target)
	if err != nil {
		return "", err
	}

	created.InfraARN = arn
	return arn, nil
}

func (b *ImageBuilder) getOrCreateDistributionConfig(ctx context.Context, name string, target *builder.Target, created *CreatedResources) (string, error) {
	distName := fmt.Sprintf("%s-dist", name)

	existing, err := b.resourceManager.GetDistributionConfig(ctx, distName)
	if err != nil && !IsErrNotFound(err) {
		return "", fmt.Errorf("failed to check for existing distribution config: %w", err)
	}

	if existing != nil {
		if b.forceRecreate {
			logging.InfoContext(ctx, "Force recreate enabled, deleting existing distribution config: %s", *existing.Arn)
			if err := b.resourceManager.DeleteDistributionConfig(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing distribution config: %w", err)
			}
		} else {
			logging.InfoContext(ctx, "Reusing existing distribution configuration: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	arn, err := b.createDistributionConfig(ctx, name, target)
	if err != nil {
		return "", err
	}

	created.DistARN = arn
	return arn, nil
}

func (b *ImageBuilder) getOrCreateImageRecipe(ctx context.Context, config builder.Config, componentARNs []string, target *builder.Target, created *CreatedResources) (string, error) {
	recipeName := fmt.Sprintf("%s-recipe", config.Name)
	normalizedVersion := NormalizeSemanticVersion(config.Version)

	existing, err := b.resourceManager.GetImageRecipe(ctx, recipeName, normalizedVersion)
	if err != nil && !IsErrNotFound(err) {
		return "", fmt.Errorf("failed to check for existing image recipe: %w", err)
	}

	if existing != nil {
		recreate := b.forceRecreate

		// When AMI filters are configured, resolve the current parent image
		// and compare with the cached recipe. The filter may now resolve to a
		// different AMI (e.g. a newer snapshot), so the recipe must be rebuilt.
		if !recreate && existing.ParentImage != nil {
			currentParent, err := b.resolveParentImage(ctx, config)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent image for recipe comparison: %w", err)
			}
			if currentParent != *existing.ParentImage {
				logging.InfoContext(ctx, "Parent image changed (%s -> %s), recreating image recipe",
					*existing.ParentImage, currentParent)
				recreate = true
			}
		}

		if recreate {
			logging.InfoContext(ctx, "Deleting existing image recipe: %s", *existing.Arn)
			if err := b.resourceManager.DeleteImageRecipe(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing image recipe: %w", err)
			}
		} else {
			logging.InfoContext(ctx, "Reusing existing image recipe: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	arn, err := b.createImageRecipe(ctx, config, componentARNs, target)
	if err != nil {
		return "", err
	}

	created.RecipeARN = arn
	return arn, nil
}

func (b *ImageBuilder) getOrCreatePipeline(ctx context.Context, config builder.Config, recipeARN, infraARN, distARN string, created *CreatedResources) (string, error) {
	pipelineName := fmt.Sprintf("%s-pipeline", config.Name)

	existing, err := b.resourceManager.GetImagePipeline(ctx, pipelineName)
	if err != nil && !IsErrNotFound(err) {
		return "", fmt.Errorf("failed to check for existing pipeline: %w", err)
	}

	if existing != nil {
		if b.forceRecreate {
			logging.InfoContext(ctx, "Force recreate enabled, deleting existing pipeline: %s", *existing.Arn)
			if err := b.resourceManager.DeleteImagePipeline(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing pipeline: %w", err)
			}
		} else {
			logging.InfoContext(ctx, "Reusing existing pipeline: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	pipelineConfig := PipelineConfig{
		Name:             pipelineName,
		Description:      fmt.Sprintf("Pipeline for %s", config.Name),
		ImageRecipeARN:   recipeARN,
		InfraConfigARN:   infraARN,
		DistConfigARN:    distARN,
		EnhancedMetadata: true,
		Tags: map[string]string{
			"warpgate:name":    config.Name,
			"warpgate:version": config.Version,
		},
	}

	pipelineARN, err := b.pipelineManager.CreatePipeline(ctx, pipelineConfig)
	if err != nil {
		if IsResourceExistsError(err) {
			logging.InfoContext(ctx, "Pipeline already exists, retrieving: %s", pipelineName)
			existingPipeline, getErr := b.resourceManager.GetImagePipeline(ctx, pipelineName)
			if getErr != nil {
				return "", fmt.Errorf("failed to create pipeline (already exists) and failed to retrieve: %w", err)
			}
			if existingPipeline != nil && existingPipeline.Arn != nil {
				logging.InfoContext(ctx, "Reusing existing pipeline: %s", *existingPipeline.Arn)
				return *existingPipeline.Arn, nil
			}
			return "", fmt.Errorf("failed to create pipeline (already exists) but could not retrieve: %w", err)
		}
		return "", fmt.Errorf("failed to create pipeline: %w", err)
	}

	created.PipelineARN = *pipelineARN
	return *pipelineARN, nil
}

// SetForceRecreate sets whether to force recreation of existing resources
func (b *ImageBuilder) SetForceRecreate(force bool) {
	b.forceRecreate = force
}
