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
	"github.com/cowdogmoo/warpgate/v3/pkg/logging"
)

// PipelineManager manages EC2 Image Builder pipelines
type PipelineManager struct {
	clients *AWSClients
}

// PipelineConfig contains configuration for creating a pipeline
type PipelineConfig struct {
	Name             string
	Description      string
	ImageRecipeARN   string
	InfraConfigARN   string
	DistConfigARN    string
	Tags             map[string]string
	EnhancedMetadata bool
}

// NewPipelineManager creates a new pipeline manager
func NewPipelineManager(clients *AWSClients) *PipelineManager {
	return &PipelineManager{
		clients: clients,
	}
}

// CreatePipeline creates an Image Builder pipeline
func (m *PipelineManager) CreatePipeline(ctx context.Context, config PipelineConfig) (*string, error) {
	logging.Info("Creating Image Builder pipeline: %s", config.Name)

	input := &imagebuilder.CreateImagePipelineInput{
		Name:                           aws.String(config.Name),
		Description:                    aws.String(config.Description),
		ImageRecipeArn:                 aws.String(config.ImageRecipeARN),
		InfrastructureConfigurationArn: aws.String(config.InfraConfigARN),
		DistributionConfigurationArn:   aws.String(config.DistConfigARN),
		EnhancedImageMetadataEnabled:   aws.Bool(config.EnhancedMetadata),
		Status:                         types.PipelineStatusEnabled,
		Tags:                           config.Tags,
	}

	result, err := m.clients.ImageBuilder.CreateImagePipeline(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	logging.Info("Pipeline created successfully: %s", *result.ImagePipelineArn)
	return result.ImagePipelineArn, nil
}

// StartPipeline starts an image pipeline execution
func (m *PipelineManager) StartPipeline(ctx context.Context, pipelineARN string) (*string, error) {
	logging.Info("Starting pipeline execution: %s", pipelineARN)

	input := &imagebuilder.StartImagePipelineExecutionInput{
		ImagePipelineArn: aws.String(pipelineARN),
	}

	result, err := m.clients.ImageBuilder.StartImagePipelineExecution(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to start pipeline: %w", err)
	}

	logging.Info("Pipeline execution started: %s", *result.ImageBuildVersionArn)
	return result.ImageBuildVersionArn, nil
}

// WaitForPipelineCompletion waits for a pipeline execution to complete
func (m *PipelineManager) WaitForPipelineCompletion(ctx context.Context, imageARN string, pollInterval time.Duration) (*types.Image, error) {
	logging.Info("Waiting for pipeline completion: %s", imageARN)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for pipeline: %w", ctx.Err())
		case <-ticker.C:
			image, err := m.getImageStatus(ctx, imageARN)
			if err != nil {
				return nil, fmt.Errorf("failed to get image status: %w", err)
			}

			status := image.State.Status
			logging.Debug("Pipeline status: %s (image: %s)", status, imageARN)

			switch status {
			case types.ImageStatusAvailable:
				logging.Info("Pipeline completed successfully: %s", imageARN)
				return image, nil
			case types.ImageStatusFailed, types.ImageStatusCancelled, types.ImageStatusDeprecated:
				reason := "unknown"
				if image.State.Reason != nil {
					reason = *image.State.Reason
				}
				return nil, fmt.Errorf("pipeline failed with status %s: %s", status, reason)
			case types.ImageStatusBuilding, types.ImageStatusCreating, types.ImageStatusPending, types.ImageStatusTesting, types.ImageStatusDistributing, types.ImageStatusIntegrating:
				// Continue waiting
				continue
			default:
				return nil, fmt.Errorf("unexpected pipeline status: %s", status)
			}
		}
	}
}

// getImageStatus retrieves the status of an image build
func (m *PipelineManager) getImageStatus(ctx context.Context, imageARN string) (*types.Image, error) {
	input := &imagebuilder.GetImageInput{
		ImageBuildVersionArn: aws.String(imageARN),
	}

	result, err := m.clients.ImageBuilder.GetImage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	return result.Image, nil
}

// GetPipeline retrieves pipeline details
func (m *PipelineManager) GetPipeline(ctx context.Context, pipelineARN string) (*types.ImagePipeline, error) {
	input := &imagebuilder.GetImagePipelineInput{
		ImagePipelineArn: aws.String(pipelineARN),
	}

	result, err := m.clients.ImageBuilder.GetImagePipeline(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline: %w", err)
	}

	return result.ImagePipeline, nil
}

// DeletePipeline deletes an image pipeline
func (m *PipelineManager) DeletePipeline(ctx context.Context, pipelineARN string) error {
	logging.Info("Deleting pipeline: %s", pipelineARN)

	input := &imagebuilder.DeleteImagePipelineInput{
		ImagePipelineArn: aws.String(pipelineARN),
	}

	_, err := m.clients.ImageBuilder.DeleteImagePipeline(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete pipeline: %w", err)
	}

	logging.Info("Pipeline deleted successfully: %s", pipelineARN)
	return nil
}

// CleanupResources cleans up Image Builder resources
func (m *PipelineManager) CleanupResources(ctx context.Context, recipeARN, infraARN, distARN string) error {
	logging.Info("Cleaning up Image Builder resources")

	// Delete distribution configuration
	if distARN != "" {
		if err := m.deleteDistributionConfig(ctx, distARN); err != nil {
			logging.Warn("Failed to delete distribution config: %v", err)
		}
	}

	// Delete infrastructure configuration
	if infraARN != "" {
		if err := m.deleteInfrastructureConfig(ctx, infraARN); err != nil {
			logging.Warn("Failed to delete infrastructure config: %v", err)
		}
	}

	// Delete image recipe
	if recipeARN != "" {
		if err := m.deleteImageRecipe(ctx, recipeARN); err != nil {
			logging.Warn("Failed to delete image recipe: %v", err)
		}
	}

	return nil
}

// deleteImageRecipe deletes an image recipe
func (m *PipelineManager) deleteImageRecipe(ctx context.Context, recipeARN string) error {
	input := &imagebuilder.DeleteImageRecipeInput{
		ImageRecipeArn: aws.String(recipeARN),
	}

	_, err := m.clients.ImageBuilder.DeleteImageRecipe(ctx, input)
	return err
}

// deleteInfrastructureConfig deletes an infrastructure configuration
func (m *PipelineManager) deleteInfrastructureConfig(ctx context.Context, infraARN string) error {
	input := &imagebuilder.DeleteInfrastructureConfigurationInput{
		InfrastructureConfigurationArn: aws.String(infraARN),
	}

	_, err := m.clients.ImageBuilder.DeleteInfrastructureConfiguration(ctx, input)
	return err
}

// deleteDistributionConfig deletes a distribution configuration
func (m *PipelineManager) deleteDistributionConfig(ctx context.Context, distARN string) error {
	input := &imagebuilder.DeleteDistributionConfigurationInput{
		DistributionConfigurationArn: aws.String(distARN),
	}

	_, err := m.clients.ImageBuilder.DeleteDistributionConfiguration(ctx, input)
	return err
}
