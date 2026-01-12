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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"golang.org/x/sync/errgroup"
)

// ImageBuilder implements the AMIBuilder interface
type ImageBuilder struct {
	clients         *AWSClients
	componentGen    *ComponentGenerator
	pipelineManager *PipelineManager
	operations      *AMIOperations
	resourceManager *ResourceManager
	config          ClientConfig
	globalConfig    *config.Config
	forceRecreate   bool
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
	// Load global config
	globalCfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	// Create AWS clients
	clients, err := NewAWSClients(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS clients: %w", err)
	}

	// Generate unique build ID
	buildID := generateBuildID()

	// Create pipeline manager with monitor config
	pipelineManager := NewPipelineManagerWithMonitor(clients, monitorConfig)

	return &ImageBuilder{
		clients:         clients,
		componentGen:    NewComponentGenerator(clients),
		pipelineManager: pipelineManager,
		operations:      NewAMIOperations(clients, globalCfg),
		resourceManager: NewResourceManager(clients),
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
	logging.Info("Starting AMI build: %s (version: %s)", config.Name, config.Version)

	// Setup and validate configuration
	amiTarget, err := b.setupBuild(config)
	if err != nil {
		return nil, err
	}

	// If force recreate is enabled, clean up existing resources first
	if b.forceRecreate {
		logging.Info("Force recreate enabled, cleaning up existing resources")
		if err := b.resourceManager.CleanupResourcesForBuild(ctx, config.Name, true); err != nil {
			logging.Warn("Force cleanup encountered errors (continuing): %v", err)
		}
	}

	// Track created resources for cleanup on failure
	createdResources := &CreatedResources{}

	// Create AWS Image Builder resources
	resources, err := b.createBuildResources(ctx, config, amiTarget, createdResources)
	if err != nil {
		// Clean up any resources that were created before the failure
		createdResources.Cleanup(ctx, b.resourceManager)
		return nil, err
	}

	// Execute the build pipeline
	amiID, err := b.executePipeline(ctx, resources, config.Name)
	if err != nil {
		// Clean up resources on pipeline failure
		createdResources.Cleanup(ctx, b.resourceManager)
		return nil, err
	}

	// Finalize the build
	b.finalizeBuild(ctx, amiID, amiTarget, resources.PipelineARN)

	duration := time.Since(startTime)
	logging.Info("AMI build completed: %s (duration: %s)", amiID, duration)

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
func (b *ImageBuilder) createBuildResources(ctx context.Context, config builder.Config, amiTarget *builder.Target, created *CreatedResources) (*buildResources, error) {
	// Create components from provisioners
	componentARNs, err := b.createComponents(ctx, config, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create components")
	}

	// Create or get infrastructure configuration
	infraARN, err := b.getOrCreateInfrastructureConfig(ctx, config.Name, amiTarget, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create infrastructure config")
	}

	// Create or get distribution configuration
	distARN, err := b.getOrCreateDistributionConfig(ctx, config.Name, amiTarget, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create distribution config")
	}

	// Create or get image recipe
	recipeARN, err := b.getOrCreateImageRecipe(ctx, config, componentARNs, amiTarget, created)
	if err != nil {
		return nil, WrapWithRemediation(err, "failed to create image recipe")
	}

	// Create or get pipeline
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
	// Start pipeline execution
	imageARN, err := b.pipelineManager.StartPipeline(ctx, resources.PipelineARN)
	if err != nil {
		return "", fmt.Errorf("failed to start pipeline: %w", err)
	}

	// Wait for pipeline completion with monitoring if enabled
	image, err := b.pipelineManager.WaitForPipelineCompletionWithImageName(ctx, *imageARN, 30*time.Second, imageName)
	if err != nil {
		return "", fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Extract AMI ID from image output
	amiID, err := b.extractAMIID(image)
	if err != nil {
		return "", fmt.Errorf("failed to extract AMI ID: %w", err)
	}

	return amiID, nil
}

// finalizeBuild tags the AMI and cleans up temporary resources
func (b *ImageBuilder) finalizeBuild(ctx context.Context, amiID string, target *builder.Target, pipelineARN string) {
	// Tag the AMI
	if len(target.AMITags) > 0 {
		if err := b.operations.TagAMI(ctx, amiID, target.AMITags); err != nil {
			logging.Warn("Failed to tag AMI: %v", err)
		}
	}

	// Clean up resources
	if err := b.pipelineManager.DeletePipeline(ctx, pipelineARN); err != nil {
		logging.Warn("Failed to delete pipeline: %v", err)
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
	logging.Info("Closing AMI builder")
	// No resources to clean up currently
	return nil
}

// createComponents creates Image Builder components from provisioners in parallel using errgroup
func (b *ImageBuilder) createComponents(ctx context.Context, config builder.Config, created *CreatedResources) ([]string, error) {
	numProvisioners := len(config.Provisioners)
	if numProvisioners == 0 {
		return nil, nil
	}

	logging.Info("Creating %d components in parallel", numProvisioners)

	// Pre-allocate results slice - each goroutine writes to its own index (safe)
	componentARNs := make([]string, numProvisioners)

	// Use errgroup for cleaner goroutine management with automatic context cancellation
	g, ctx := errgroup.WithContext(ctx)

	// Create components in parallel
	for i, provisioner := range config.Provisioners {
		index := i
		prov := provisioner

		g.Go(func() error {
			// Check context cancellation before starting work
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context cancelled before creating component %d: %w", index, err)
			}

			componentName := config.Name + "-" + fmt.Sprint(index)
			logging.Info("Creating component: %s (index: %d)", prov.Type, index)

			// Determine the version to use:
			// 1. Use provisioner's ComponentVersion if specified
			// 2. Fall back to config.Version
			// 3. Auto-increment if forceRecreate and conflicts exist
			version := config.Version
			if prov.ComponentVersion != "" {
				version = prov.ComponentVersion
				logging.Info("Using provisioner-specified component version: %s", version)
			}

			if b.forceRecreate {
				// With force, try to get the next available version
				nextVersion, err := b.resourceManager.GetNextComponentVersion(ctx, componentName+"-"+prov.Type, version)
				if err == nil && nextVersion != version {
					logging.Info("Component version conflict detected, using next version: %s", nextVersion)
					version = nextVersion
				}
			}

			arn, err := b.componentGen.GenerateComponent(ctx, prov, componentName, version)
			if err != nil {
				return fmt.Errorf("failed to create component %d (%s): %w", index, prov.Type, err)
			}

			componentARNs[index] = *arn
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		// Track any components that were created before the failure for cleanup
		for _, arn := range componentARNs {
			if arn != "" {
				created.ComponentARNs = append(created.ComponentARNs, arn)
			}
		}
		return nil, err
	}

	// Track all created components for cleanup
	created.ComponentARNs = append(created.ComponentARNs, componentARNs...)

	// Verify all components were created
	for i, arn := range componentARNs {
		if arn == "" {
			return nil, fmt.Errorf("component %d was not created", i)
		}
	}

	logging.Info("Successfully created %d components", numProvisioners)
	return componentARNs, nil
}

// createInfrastructureConfig creates an infrastructure configuration
func (b *ImageBuilder) createInfrastructureConfig(ctx context.Context, name string, target *builder.Target) (string, error) {
	logging.Info("Creating infrastructure configuration")

	// Determine instance type (target > globalConfig > default)
	instanceType := target.InstanceType
	if instanceType == "" {
		instanceType = b.globalConfig.AWS.AMI.InstanceType
	}

	// Determine instance profile (target > globalConfig)
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

	// Add subnet ID if specified
	if target.SubnetID != "" {
		input.SubnetId = aws.String(target.SubnetID)
	}

	// Add security group IDs if specified
	if len(target.SecurityGroupIDs) > 0 {
		input.SecurityGroupIds = target.SecurityGroupIDs
	}

	result, err := b.clients.ImageBuilder.CreateInfrastructureConfiguration(ctx, input)
	if err != nil {
		// If resource already exists, try to retrieve it instead
		if IsResourceExistsError(err) {
			logging.Info("Infrastructure configuration already exists, retrieving: %s", infraName)
			existing, getErr := b.resourceManager.GetInfrastructureConfig(ctx, infraName)
			if getErr != nil {
				return "", fmt.Errorf("failed to create infrastructure config (already exists) and failed to retrieve: %w", err)
			}
			if existing != nil && existing.Arn != nil {
				logging.Info("Reusing existing infrastructure configuration: %s", *existing.Arn)
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
	logging.Info("Creating distribution configuration")

	region := target.Region
	if region == "" {
		region = b.clients.GetRegion()
	}

	amiName := target.AMIName
	if amiName == "" {
		amiName = fmt.Sprintf("%s-{{imagebuilder:buildDate}}", name)
	}

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
		logging.Info("Windows Fast Launch enabled with %d target resources", target.FastLaunchTargetResourceCount)
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
		// If resource already exists, try to retrieve it instead
		if IsResourceExistsError(err) {
			logging.Info("Distribution configuration already exists, retrieving: %s", distName)
			existing, getErr := b.resourceManager.GetDistributionConfig(ctx, distName)
			if getErr != nil {
				return "", fmt.Errorf("failed to create distribution config (already exists) and failed to retrieve: %w", err)
			}
			if existing != nil && existing.Arn != nil {
				logging.Info("Reusing existing distribution configuration: %s", *existing.Arn)
				return *existing.Arn, nil
			}
			return "", fmt.Errorf("failed to create distribution config (already exists) but could not retrieve: %w", err)
		}
		return "", fmt.Errorf("failed to create distribution config: %w", err)
	}

	return *result.DistributionConfigurationArn, nil
}

// buildFastLaunchConfiguration creates the Fast Launch configuration for Windows AMIs
func (b *ImageBuilder) buildFastLaunchConfiguration(target *builder.Target) types.FastLaunchConfiguration {
	// Set defaults for Fast Launch parameters
	maxParallelLaunches := target.FastLaunchMaxParallelLaunches
	if maxParallelLaunches == 0 {
		maxParallelLaunches = 6 // AWS default
	}

	targetResourceCount := target.FastLaunchTargetResourceCount
	if targetResourceCount == 0 {
		targetResourceCount = 5 // Reasonable default for pre-provisioned snapshots
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

// createImageRecipe creates an image recipe
func (b *ImageBuilder) createImageRecipe(ctx context.Context, config builder.Config, componentARNs []string, target *builder.Target) (string, error) {
	logging.Info("Creating image recipe")

	// Determine parent image (base AMI)
	// Priority: config.Base.Image > globalConfig > error (no default)
	parentImage := config.Base.Image
	if parentImage == "" {
		parentImage = b.globalConfig.AWS.AMI.DefaultParentImage
		if parentImage == "" {
			return "", fmt.Errorf("parent image (base AMI) must be specified in template config or global config (aws.ami.default_parent_image)")
		}
	}

	// Create component configurations
	components := make([]types.ComponentConfiguration, 0, len(componentARNs))
	for _, arn := range componentARNs {
		components = append(components, types.ComponentConfiguration{
			ComponentArn: aws.String(arn),
		})
	}

	// Determine volume size (target > globalConfig > default)
	volumeSize := int32(target.VolumeSize)
	if volumeSize == 0 {
		volumeSize = int32(b.globalConfig.AWS.AMI.VolumeSize)
	}

	recipeName := fmt.Sprintf("%s-recipe", config.Name)
	input := &imagebuilder.CreateImageRecipeInput{
		Name:            aws.String(recipeName),
		SemanticVersion: aws.String(config.Version),
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
		// If resource already exists, try to retrieve it instead
		if IsResourceExistsError(err) {
			logging.Info("Image recipe already exists, retrieving: %s", recipeName)
			existing, getErr := b.resourceManager.GetImageRecipe(ctx, recipeName, config.Version)
			if getErr != nil {
				return "", fmt.Errorf("failed to create image recipe (already exists) and failed to retrieve: %w", err)
			}
			if existing != nil && existing.Arn != nil {
				logging.Info("Reusing existing image recipe: %s", *existing.Arn)
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

// getOrCreateInfrastructureConfig creates or retrieves an infrastructure configuration
func (b *ImageBuilder) getOrCreateInfrastructureConfig(ctx context.Context, name string, target *builder.Target, created *CreatedResources) (string, error) {
	infraName := fmt.Sprintf("%s-infra", name)

	// Check if infrastructure config already exists
	existing, err := b.resourceManager.GetInfrastructureConfig(ctx, infraName)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing infrastructure config: %w", err)
	}

	if existing != nil {
		if b.forceRecreate {
			logging.Info("Force recreate enabled, deleting existing infrastructure config: %s", *existing.Arn)
			if err := b.resourceManager.DeleteInfrastructureConfig(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing infrastructure config: %w", err)
			}
		} else {
			logging.Info("Reusing existing infrastructure configuration: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	// Create new infrastructure configuration
	arn, err := b.createInfrastructureConfig(ctx, name, target)
	if err != nil {
		return "", err
	}

	created.InfraARN = arn
	return arn, nil
}

// getOrCreateDistributionConfig creates or retrieves a distribution configuration
func (b *ImageBuilder) getOrCreateDistributionConfig(ctx context.Context, name string, target *builder.Target, created *CreatedResources) (string, error) {
	distName := fmt.Sprintf("%s-dist", name)

	// Check if distribution config already exists
	existing, err := b.resourceManager.GetDistributionConfig(ctx, distName)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing distribution config: %w", err)
	}

	if existing != nil {
		if b.forceRecreate {
			logging.Info("Force recreate enabled, deleting existing distribution config: %s", *existing.Arn)
			if err := b.resourceManager.DeleteDistributionConfig(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing distribution config: %w", err)
			}
		} else {
			logging.Info("Reusing existing distribution configuration: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	// Create new distribution configuration
	arn, err := b.createDistributionConfig(ctx, name, target)
	if err != nil {
		return "", err
	}

	created.DistARN = arn
	return arn, nil
}

// getOrCreateImageRecipe creates or retrieves an image recipe
func (b *ImageBuilder) getOrCreateImageRecipe(ctx context.Context, config builder.Config, componentARNs []string, target *builder.Target, created *CreatedResources) (string, error) {
	recipeName := fmt.Sprintf("%s-recipe", config.Name)

	// Check if recipe already exists
	existing, err := b.resourceManager.GetImageRecipe(ctx, recipeName, config.Version)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing image recipe: %w", err)
	}

	if existing != nil {
		if b.forceRecreate {
			logging.Info("Force recreate enabled, deleting existing image recipe: %s", *existing.Arn)
			if err := b.resourceManager.DeleteImageRecipe(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing image recipe: %w", err)
			}
		} else {
			logging.Info("Reusing existing image recipe: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	// Create new image recipe
	arn, err := b.createImageRecipe(ctx, config, componentARNs, target)
	if err != nil {
		return "", err
	}

	created.RecipeARN = arn
	return arn, nil
}

// getOrCreatePipeline creates or retrieves an image pipeline
func (b *ImageBuilder) getOrCreatePipeline(ctx context.Context, config builder.Config, recipeARN, infraARN, distARN string, created *CreatedResources) (string, error) {
	pipelineName := fmt.Sprintf("%s-pipeline", config.Name)

	// Check if pipeline already exists
	existing, err := b.resourceManager.GetImagePipeline(ctx, pipelineName)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing pipeline: %w", err)
	}

	if existing != nil {
		if b.forceRecreate {
			logging.Info("Force recreate enabled, deleting existing pipeline: %s", *existing.Arn)
			if err := b.resourceManager.DeleteImagePipeline(ctx, *existing.Arn); err != nil {
				return "", fmt.Errorf("failed to delete existing pipeline: %w", err)
			}
		} else {
			logging.Info("Reusing existing pipeline: %s", *existing.Arn)
			return *existing.Arn, nil
		}
	}

	// Create new pipeline
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
		// If resource already exists, try to retrieve it instead
		if IsResourceExistsError(err) {
			logging.Info("Pipeline already exists, retrieving: %s", pipelineName)
			existingPipeline, getErr := b.resourceManager.GetImagePipeline(ctx, pipelineName)
			if getErr != nil {
				return "", fmt.Errorf("failed to create pipeline (already exists) and failed to retrieve: %w", err)
			}
			if existingPipeline != nil && existingPipeline.Arn != nil {
				logging.Info("Reusing existing pipeline: %s", *existingPipeline.Arn)
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
