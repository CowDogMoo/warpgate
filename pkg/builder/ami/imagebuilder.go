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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// ImageBuilder implements the AMIBuilder interface
type ImageBuilder struct {
	clients         *AWSClients
	componentGen    *ComponentGenerator
	pipelineManager *PipelineManager
	operations      *AMIOperations
	config          ClientConfig
}

// NewImageBuilder creates a new AMI builder
func NewImageBuilder(ctx context.Context, config ClientConfig) (*ImageBuilder, error) {
	// Create AWS clients
	clients, err := NewAWSClients(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS clients: %w", err)
	}

	return &ImageBuilder{
		clients:         clients,
		componentGen:    NewComponentGenerator(clients),
		pipelineManager: NewPipelineManager(clients),
		operations:      NewAMIOperations(clients),
		config:          config,
	}, nil
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

	// Create AWS Image Builder resources
	resources, err := b.createBuildResources(ctx, config, amiTarget)
	if err != nil {
		return nil, err
	}

	// Execute the build pipeline
	amiID, err := b.executePipeline(ctx, resources)
	if err != nil {
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
func (b *ImageBuilder) createBuildResources(ctx context.Context, config builder.Config, amiTarget *builder.Target) (*buildResources, error) {
	// Create components from provisioners
	componentARNs, err := b.createComponents(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create components: %w", err)
	}

	// Create infrastructure configuration
	infraARN, err := b.createInfrastructureConfig(ctx, config.Name, amiTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to create infrastructure config: %w", err)
	}

	// Create distribution configuration
	distARN, err := b.createDistributionConfig(ctx, config.Name, amiTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to create distribution config: %w", err)
	}

	// Create image recipe
	recipeARN, err := b.createImageRecipe(ctx, config, componentARNs, amiTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to create image recipe: %w", err)
	}

	// Create pipeline
	pipelineConfig := PipelineConfig{
		Name:             fmt.Sprintf("%s-pipeline", config.Name),
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
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	return &buildResources{
		ComponentARNs: componentARNs,
		InfraARN:      infraARN,
		DistARN:       distARN,
		RecipeARN:     recipeARN,
		PipelineARN:   *pipelineARN,
	}, nil
}

// executePipeline runs the Image Builder pipeline and waits for completion
func (b *ImageBuilder) executePipeline(ctx context.Context, resources *buildResources) (string, error) {
	// Start pipeline execution
	imageARN, err := b.pipelineManager.StartPipeline(ctx, resources.PipelineARN)
	if err != nil {
		return "", fmt.Errorf("failed to start pipeline: %w", err)
	}

	// Wait for pipeline completion
	image, err := b.pipelineManager.WaitForPipelineCompletion(ctx, *imageARN, 30*time.Second)
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
		regionOps := NewAMIOperations(regionClients)
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

// createComponents creates Image Builder components from provisioners
func (b *ImageBuilder) createComponents(ctx context.Context, config builder.Config) ([]string, error) {
	var componentARNs []string

	for i, provisioner := range config.Provisioners {
		logging.Info("Creating component: %s (index: %d)", provisioner.Type, i)

		arn, err := b.componentGen.GenerateComponent(
			ctx,
			provisioner,
			fmt.Sprintf("%s-%d", config.Name, i),
			config.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create component %d: %w", i, err)
		}

		componentARNs = append(componentARNs, *arn)
	}

	return componentARNs, nil
}

// createInfrastructureConfig creates an infrastructure configuration
func (b *ImageBuilder) createInfrastructureConfig(ctx context.Context, name string, target *builder.Target) (string, error) {
	logging.Info("Creating infrastructure configuration")

	// Determine instance type
	instanceType := target.InstanceType
	if instanceType == "" {
		instanceType = "t3.medium" // Default
	}

	input := &imagebuilder.CreateInfrastructureConfigurationInput{
		Name:          aws.String(fmt.Sprintf("%s-infra", name)),
		InstanceTypes: []string{instanceType},
		Description:   aws.String(fmt.Sprintf("Infrastructure config for %s", name)),
		Tags: map[string]string{
			"warpgate:name": name,
		},
	}

	// Add subnet ID if specified
	if target.SubnetID != "" {
		input.SubnetId = aws.String(target.SubnetID)
	}

	result, err := b.clients.ImageBuilder.CreateInfrastructureConfiguration(ctx, input)
	if err != nil {
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

	distribution := types.Distribution{
		Region: aws.String(region),
		AmiDistributionConfiguration: &types.AmiDistributionConfiguration{
			Name:        aws.String(amiName),
			Description: aws.String(fmt.Sprintf("AMI for %s", name)),
			AmiTags:     target.AMITags,
		},
	}

	input := &imagebuilder.CreateDistributionConfigurationInput{
		Name:          aws.String(fmt.Sprintf("%s-dist", name)),
		Description:   aws.String(fmt.Sprintf("Distribution config for %s", name)),
		Distributions: []types.Distribution{distribution},
		Tags: map[string]string{
			"warpgate:name": name,
		},
	}

	result, err := b.clients.ImageBuilder.CreateDistributionConfiguration(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create distribution config: %w", err)
	}

	return *result.DistributionConfigurationArn, nil
}

// createImageRecipe creates an image recipe
func (b *ImageBuilder) createImageRecipe(ctx context.Context, config builder.Config, componentARNs []string, target *builder.Target) (string, error) {
	logging.Info("Creating image recipe")

	// Determine parent image (base AMI)
	parentImage := config.Base.Image
	if parentImage == "" {
		// Default to Amazon Linux 2023
		parentImage = "ami-0c55b159cbfafe1f0"
	}

	// Create component configurations
	components := make([]types.ComponentConfiguration, 0, len(componentARNs))
	for _, arn := range componentARNs {
		components = append(components, types.ComponentConfiguration{
			ComponentArn: aws.String(arn),
		})
	}

	// Determine volume size
	volumeSize := int32(target.VolumeSize)
	if volumeSize == 0 {
		volumeSize = 8 // Default 8 GB
	}

	input := &imagebuilder.CreateImageRecipeInput{
		Name:            aws.String(fmt.Sprintf("%s-recipe", config.Name)),
		SemanticVersion: aws.String(config.Version),
		ParentImage:     aws.String(parentImage),
		Components:      components,
		Description:     aws.String(fmt.Sprintf("Image recipe for %s", config.Name)),
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &types.EbsInstanceBlockDeviceSpecification{
					VolumeSize:          aws.Int32(volumeSize),
					VolumeType:          types.EbsVolumeTypeGp3,
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
