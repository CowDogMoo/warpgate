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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// ResourceManager handles idempotent creation and management of Image Builder resources
type ResourceManager struct {
	clients *AWSClients
}

// NewResourceManager creates a new resource manager
func NewResourceManager(clients *AWSClients) *ResourceManager {
	return &ResourceManager{
		clients: clients,
	}
}

// ResourceExistsError indicates a resource already exists
type ResourceExistsError struct {
	ResourceType string
	ResourceName string
	ResourceARN  string
}

func (e *ResourceExistsError) Error() string {
	return fmt.Sprintf("%s '%s' already exists (ARN: %s). Use --force to delete and recreate, or use a different name", e.ResourceType, e.ResourceName, e.ResourceARN)
}

// GetInfrastructureConfig retrieves an infrastructure configuration by name
func (m *ResourceManager) GetInfrastructureConfig(ctx context.Context, name string) (*types.InfrastructureConfiguration, error) {
	// List infrastructure configurations and filter by name with pagination
	input := &imagebuilder.ListInfrastructureConfigurationsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{name},
			},
		},
	}

	for {
		result, err := m.clients.ImageBuilder.ListInfrastructureConfigurations(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list infrastructure configurations: %w", err)
		}

		// Find exact match
		for _, summary := range result.InfrastructureConfigurationSummaryList {
			if summary.Name != nil && *summary.Name == name {
				// Get full configuration
				getInput := &imagebuilder.GetInfrastructureConfigurationInput{
					InfrastructureConfigurationArn: summary.Arn,
				}
				getResult, err := m.clients.ImageBuilder.GetInfrastructureConfiguration(ctx, getInput)
				if err != nil {
					return nil, fmt.Errorf("failed to get infrastructure configuration: %w", err)
				}
				return getResult.InfrastructureConfiguration, nil
			}
		}

		// Check for more pages
		if result.NextToken == nil {
			break
		}
		input.NextToken = result.NextToken
	}

	return nil, nil
}

// GetDistributionConfig retrieves a distribution configuration by name
func (m *ResourceManager) GetDistributionConfig(ctx context.Context, name string) (*types.DistributionConfiguration, error) {
	input := &imagebuilder.ListDistributionConfigurationsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{name},
			},
		},
	}

	for {
		result, err := m.clients.ImageBuilder.ListDistributionConfigurations(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list distribution configurations: %w", err)
		}

		for _, summary := range result.DistributionConfigurationSummaryList {
			if summary.Name != nil && *summary.Name == name {
				getInput := &imagebuilder.GetDistributionConfigurationInput{
					DistributionConfigurationArn: summary.Arn,
				}
				getResult, err := m.clients.ImageBuilder.GetDistributionConfiguration(ctx, getInput)
				if err != nil {
					return nil, fmt.Errorf("failed to get distribution configuration: %w", err)
				}
				return getResult.DistributionConfiguration, nil
			}
		}

		// Check for more pages
		if result.NextToken == nil {
			break
		}
		input.NextToken = result.NextToken
	}

	return nil, nil
}

// GetImageRecipe retrieves an image recipe by name and version
func (m *ResourceManager) GetImageRecipe(ctx context.Context, name, version string) (*types.ImageRecipe, error) {
	input := &imagebuilder.ListImageRecipesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{name},
			},
		},
	}

	for {
		result, err := m.clients.ImageBuilder.ListImageRecipes(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list image recipes: %w", err)
		}

		for _, summary := range result.ImageRecipeSummaryList {
			if summary.Name != nil && *summary.Name == name {
				getInput := &imagebuilder.GetImageRecipeInput{
					ImageRecipeArn: summary.Arn,
				}
				getResult, err := m.clients.ImageBuilder.GetImageRecipe(ctx, getInput)
				if err != nil {
					return nil, fmt.Errorf("failed to get image recipe: %w", err)
				}
				return getResult.ImageRecipe, nil
			}
		}

		// Check for more pages
		if result.NextToken == nil {
			break
		}
		input.NextToken = result.NextToken
	}

	return nil, nil
}

// GetImagePipeline retrieves an image pipeline by name
func (m *ResourceManager) GetImagePipeline(ctx context.Context, name string) (*types.ImagePipeline, error) {
	input := &imagebuilder.ListImagePipelinesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{name},
			},
		},
	}

	for {
		result, err := m.clients.ImageBuilder.ListImagePipelines(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list image pipelines: %w", err)
		}

		for _, summary := range result.ImagePipelineList {
			if summary.Name != nil && *summary.Name == name {
				return &summary, nil
			}
		}

		// Check for more pages
		if result.NextToken == nil {
			break
		}
		input.NextToken = result.NextToken
	}

	return nil, nil
}

// DeleteInfrastructureConfig deletes an infrastructure configuration
func (m *ResourceManager) DeleteInfrastructureConfig(ctx context.Context, arn string) error {
	input := &imagebuilder.DeleteInfrastructureConfigurationInput{
		InfrastructureConfigurationArn: aws.String(arn),
	}
	_, err := m.clients.ImageBuilder.DeleteInfrastructureConfiguration(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete infrastructure configuration: %w", err)
	}
	logging.Info("Deleted existing infrastructure configuration: %s", arn)
	return nil
}

// DeleteDistributionConfig deletes a distribution configuration
func (m *ResourceManager) DeleteDistributionConfig(ctx context.Context, arn string) error {
	input := &imagebuilder.DeleteDistributionConfigurationInput{
		DistributionConfigurationArn: aws.String(arn),
	}
	_, err := m.clients.ImageBuilder.DeleteDistributionConfiguration(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete distribution configuration: %w", err)
	}
	logging.Info("Deleted existing distribution configuration: %s", arn)
	return nil
}

// DeleteImageRecipe deletes an image recipe
func (m *ResourceManager) DeleteImageRecipe(ctx context.Context, arn string) error {
	input := &imagebuilder.DeleteImageRecipeInput{
		ImageRecipeArn: aws.String(arn),
	}
	_, err := m.clients.ImageBuilder.DeleteImageRecipe(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete image recipe: %w", err)
	}
	logging.Info("Deleted existing image recipe: %s", arn)
	return nil
}

// DeleteImagePipeline deletes an image pipeline
func (m *ResourceManager) DeleteImagePipeline(ctx context.Context, arn string) error {
	input := &imagebuilder.DeleteImagePipelineInput{
		ImagePipelineArn: aws.String(arn),
	}
	_, err := m.clients.ImageBuilder.DeleteImagePipeline(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete image pipeline: %w", err)
	}
	logging.Info("Deleted existing image pipeline: %s", arn)
	return nil
}

// DeleteComponent deletes a component
func (m *ResourceManager) DeleteComponent(ctx context.Context, arn string) error {
	input := &imagebuilder.DeleteComponentInput{
		ComponentBuildVersionArn: aws.String(arn),
	}
	_, err := m.clients.ImageBuilder.DeleteComponent(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete component: %w", err)
	}
	logging.Info("Deleted existing component: %s", arn)
	return nil
}

// GetComponentVersions retrieves all versions of a component by name
func (m *ResourceManager) GetComponentVersions(ctx context.Context, name string) ([]types.ComponentVersion, error) {
	input := &imagebuilder.ListComponentsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{name},
			},
		},
	}

	result, err := m.clients.ImageBuilder.ListComponents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list components: %w", err)
	}

	return result.ComponentVersionList, nil
}

// GetNextComponentVersion determines the next available version for a component
func (m *ResourceManager) GetNextComponentVersion(ctx context.Context, name, baseVersion string) (string, error) {
	versions, err := m.GetComponentVersions(ctx, name)
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return baseVersion, nil
	}

	// Parse the base version parts
	baseParts := strings.Split(baseVersion, ".")
	if len(baseParts) < 3 {
		baseParts = append(baseParts, make([]string, 3-len(baseParts))...)
		for i := range baseParts {
			if baseParts[i] == "" {
				baseParts[i] = "0"
			}
		}
	}

	// Find the highest build number for matching major.minor
	prefix := baseParts[0] + "." + baseParts[1] + "."
	highestPatch := -1

	for _, v := range versions {
		if v.Version == nil {
			continue
		}
		ver := *v.Version
		if strings.HasPrefix(ver, prefix) {
			patchStr := strings.TrimPrefix(ver, prefix)
			var patch int
			if _, err := fmt.Sscanf(patchStr, "%d", &patch); err == nil {
				if patch > highestPatch {
					highestPatch = patch
				}
			}
		}
	}

	if highestPatch >= 0 {
		return fmt.Sprintf("%s%d", prefix, highestPatch+1), nil
	}

	return baseVersion, nil
}

// CleanupOldComponentVersions deletes old versions of a component, keeping only the specified number of most recent versions
func (m *ResourceManager) CleanupOldComponentVersions(ctx context.Context, name string, keepCount int) ([]string, error) {
	if keepCount < 1 {
		keepCount = 1 // Always keep at least one version
	}

	versions, err := m.GetComponentVersions(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to list component versions: %w", err)
	}

	if len(versions) <= keepCount {
		logging.Info("Component %s has %d versions, keeping all (threshold: %d)", name, len(versions), keepCount)
		return nil, nil
	}

	// Sort versions by semantic version (descending) to keep the latest
	sortedVersions := SortComponentVersions(versions)

	// Determine versions to delete (all except the keepCount newest)
	versionsToDelete := sortedVersions[keepCount:]

	deletedARNs := []string{}
	for _, v := range versionsToDelete {
		if v.Arn == nil {
			continue
		}

		// Get all build versions for this component version to delete them
		buildVersions, err := m.getComponentBuildVersions(ctx, *v.Arn)
		if err != nil {
			logging.Warn("Failed to get build versions for %s: %v", *v.Arn, err)
			continue
		}

		for _, buildARN := range buildVersions {
			if err := m.DeleteComponent(ctx, buildARN); err != nil {
				logging.Warn("Failed to delete component version %s: %v", buildARN, err)
			} else {
				deletedARNs = append(deletedARNs, buildARN)
			}
		}
	}

	logging.Info("Cleaned up %d old component versions for %s", len(deletedARNs), name)
	return deletedARNs, nil
}

// getComponentBuildVersions retrieves all build versions for a component version ARN
func (m *ResourceManager) getComponentBuildVersions(ctx context.Context, componentVersionARN string) ([]string, error) {
	input := &imagebuilder.ListComponentBuildVersionsInput{
		ComponentVersionArn: aws.String(componentVersionARN),
	}

	result, err := m.clients.ImageBuilder.ListComponentBuildVersions(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list component build versions: %w", err)
	}

	arns := make([]string, 0, len(result.ComponentSummaryList))
	for _, summary := range result.ComponentSummaryList {
		if summary.Arn != nil {
			arns = append(arns, *summary.Arn)
		}
	}

	return arns, nil
}

// ListComponentsByPrefix lists all components matching a name prefix
func (m *ResourceManager) ListComponentsByPrefix(ctx context.Context, prefix string) ([]types.ComponentVersion, error) {
	input := &imagebuilder.ListComponentsInput{}

	result, err := m.clients.ImageBuilder.ListComponents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list components: %w", err)
	}

	matching := []types.ComponentVersion{}
	for _, v := range result.ComponentVersionList {
		if v.Name != nil && strings.HasPrefix(*v.Name, prefix) {
			matching = append(matching, v)
		}
	}

	return matching, nil
}

// CleanupResourcesForBuild deletes all resources associated with a build name
func (m *ResourceManager) CleanupResourcesForBuild(ctx context.Context, buildName string, force bool) error {
	logging.Info("Cleaning up existing resources for build: %s", buildName)

	// Define cleanup operations in dependency order
	cleanupOps := []struct {
		suffix   string
		typeName string
		getARN   func() (string, error)
		deleteFn func(context.Context, string) error
	}{
		{"pipeline", "pipeline", func() (string, error) { return m.getPipelineARN(ctx, buildName+"-pipeline") }, m.DeleteImagePipeline},
		{"recipe", "image recipe", func() (string, error) { return m.getRecipeARN(ctx, buildName+"-recipe") }, m.DeleteImageRecipe},
		{"dist", "distribution config", func() (string, error) { return m.getDistConfigARN(ctx, buildName+"-dist") }, m.DeleteDistributionConfig},
		{"infra", "infrastructure config", func() (string, error) { return m.getInfraConfigARN(ctx, buildName+"-infra") }, m.DeleteInfrastructureConfig},
	}

	for _, op := range cleanupOps {
		if err := m.cleanupResource(ctx, op.typeName, op.getARN, op.deleteFn, force); err != nil {
			return err
		}
	}

	logging.Info("Resource cleanup completed for: %s", buildName)
	return nil
}

// cleanupResource handles the get/delete pattern for a single resource type
func (m *ResourceManager) cleanupResource(ctx context.Context, typeName string, getARN func() (string, error), deleteFn func(context.Context, string) error, force bool) error {
	arn, err := getARN()
	if err != nil || arn == "" {
		return nil // Resource doesn't exist, nothing to delete
	}

	if err := deleteFn(ctx, arn); err != nil {
		if !force {
			return fmt.Errorf("failed to delete %s: %w", typeName, err)
		}
		logging.Warn("Failed to delete %s (continuing): %v", typeName, err)
	}
	return nil
}

// Helper functions to get ARNs
func (m *ResourceManager) getPipelineARN(ctx context.Context, name string) (string, error) {
	pipeline, err := m.GetImagePipeline(ctx, name)
	if err != nil || pipeline == nil {
		return "", err
	}
	return aws.ToString(pipeline.Arn), nil
}

func (m *ResourceManager) getRecipeARN(ctx context.Context, name string) (string, error) {
	recipe, err := m.GetImageRecipe(ctx, name, "")
	if err != nil || recipe == nil {
		return "", err
	}
	return aws.ToString(recipe.Arn), nil
}

func (m *ResourceManager) getDistConfigARN(ctx context.Context, name string) (string, error) {
	dist, err := m.GetDistributionConfig(ctx, name)
	if err != nil || dist == nil {
		return "", err
	}
	return aws.ToString(dist.Arn), nil
}

func (m *ResourceManager) getInfraConfigARN(ctx context.Context, name string) (string, error) {
	infra, err := m.GetInfrastructureConfig(ctx, name)
	if err != nil || infra == nil {
		return "", err
	}
	return aws.ToString(infra.Arn), nil
}

// IsResourceExistsError checks if an error is due to a resource already existing
func IsResourceExistsError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "ResourceAlreadyExistsException")
}

// CreatedResources tracks resources created during a build for cleanup on failure
type CreatedResources struct {
	ComponentARNs []string
	InfraARN      string
	DistARN       string
	RecipeARN     string
	PipelineARN   string
}

// Cleanup deletes all tracked resources
func (r *CreatedResources) Cleanup(ctx context.Context, manager *ResourceManager) {
	logging.Info("Cleaning up resources created during failed build")

	// Delete in reverse order of dependency
	if r.PipelineARN != "" {
		if err := manager.DeleteImagePipeline(ctx, r.PipelineARN); err != nil {
			logging.Warn("Failed to cleanup pipeline: %v", err)
		}
	}

	if r.RecipeARN != "" {
		if err := manager.DeleteImageRecipe(ctx, r.RecipeARN); err != nil {
			logging.Warn("Failed to cleanup image recipe: %v", err)
		}
	}

	if r.DistARN != "" {
		if err := manager.DeleteDistributionConfig(ctx, r.DistARN); err != nil {
			logging.Warn("Failed to cleanup distribution config: %v", err)
		}
	}

	if r.InfraARN != "" {
		if err := manager.DeleteInfrastructureConfig(ctx, r.InfraARN); err != nil {
			logging.Warn("Failed to cleanup infrastructure config: %v", err)
		}
	}

	for _, arn := range r.ComponentARNs {
		if err := manager.DeleteComponent(ctx, arn); err != nil {
			logging.Warn("Failed to cleanup component: %v", err)
		}
	}

	logging.Info("Resource cleanup completed")
}
