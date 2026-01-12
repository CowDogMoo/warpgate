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
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// ResourceInfo contains information about an AWS Image Builder resource
type ResourceInfo struct {
	Type      string // "Component", "InfrastructureConfiguration", "DistributionConfiguration", "ImageRecipe", "ImagePipeline"
	Name      string
	ARN       string
	BuildName string // Extracted from warpgate:name tag
	Version   string // For components
}

// ResourceCleaner handles cleanup of Image Builder resources
type ResourceCleaner struct {
	clients *AWSClients
}

// NewResourceCleaner creates a new resource cleaner
func NewResourceCleaner(clients *AWSClients) *ResourceCleaner {
	return &ResourceCleaner{
		clients: clients,
	}
}

// ListWarpgateResources lists all resources created by warpgate (those with warpgate: tags)
func (c *ResourceCleaner) ListWarpgateResources(ctx context.Context) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	// List pipelines
	pipelines, err := c.listPipelines(ctx)
	if err != nil {
		logging.Warn("Failed to list pipelines: %v", err)
	} else {
		resources = append(resources, pipelines...)
	}

	// List recipes
	recipes, err := c.listRecipes(ctx)
	if err != nil {
		logging.Warn("Failed to list recipes: %v", err)
	} else {
		resources = append(resources, recipes...)
	}

	// List distribution configurations
	distConfigs, err := c.listDistributionConfigs(ctx)
	if err != nil {
		logging.Warn("Failed to list distribution configs: %v", err)
	} else {
		resources = append(resources, distConfigs...)
	}

	// List infrastructure configurations
	infraConfigs, err := c.listInfrastructureConfigs(ctx)
	if err != nil {
		logging.Warn("Failed to list infrastructure configs: %v", err)
	} else {
		resources = append(resources, infraConfigs...)
	}

	// List components
	components, err := c.listComponents(ctx)
	if err != nil {
		logging.Warn("Failed to list components: %v", err)
	} else {
		resources = append(resources, components...)
	}

	return resources, nil
}

// ListResourcesForBuild lists all resources for a specific build name
func (c *ResourceCleaner) ListResourcesForBuild(ctx context.Context, buildName string) ([]ResourceInfo, error) {
	allResources, err := c.ListWarpgateResources(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []ResourceInfo
	for _, r := range allResources {
		// Match by build name in tags or by resource name pattern
		if r.BuildName == buildName || strings.HasPrefix(r.Name, buildName+"-") || r.Name == buildName {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// DeleteResources deletes the specified resources in the correct order
func (c *ResourceCleaner) DeleteResources(ctx context.Context, resources []ResourceInfo) error {
	grouped := c.groupResourcesByType(resources)
	var deleteErrors []string

	// Delete in dependency order: pipelines -> recipes -> configs -> components
	deleteErrors = c.deleteResourceGroup(ctx, grouped["ImagePipeline"], "pipeline", c.deletePipeline, deleteErrors)
	deleteErrors = c.deleteResourceGroup(ctx, grouped["ImageRecipe"], "recipe", c.deleteRecipe, deleteErrors)
	deleteErrors = c.deleteResourceGroup(ctx, grouped["DistributionConfiguration"], "distribution config", c.deleteDistributionConfig, deleteErrors)
	deleteErrors = c.deleteResourceGroup(ctx, grouped["InfrastructureConfiguration"], "infrastructure config", c.deleteInfrastructureConfig, deleteErrors)
	deleteErrors = c.deleteResourceGroup(ctx, grouped["Component"], "component", c.deleteComponent, deleteErrors)

	if len(deleteErrors) > 0 {
		return fmt.Errorf("some resources failed to delete:\n  - %s", strings.Join(deleteErrors, "\n  - "))
	}

	logging.Info("Successfully deleted %d resources", len(resources))
	return nil
}

// groupResourcesByType groups resources by their type
func (c *ResourceCleaner) groupResourcesByType(resources []ResourceInfo) map[string][]ResourceInfo {
	grouped := make(map[string][]ResourceInfo)
	for _, r := range resources {
		grouped[r.Type] = append(grouped[r.Type], r)
	}
	return grouped
}

// deleteResourceGroup deletes a group of resources using the provided delete function
func (c *ResourceCleaner) deleteResourceGroup(ctx context.Context, resources []ResourceInfo, typeName string, deleteFn func(context.Context, string) error, errors []string) []string {
	for _, r := range resources {
		logging.Info("Deleting %s: %s", typeName, r.Name)
		if err := deleteFn(ctx, r.ARN); err != nil {
			errors = append(errors, fmt.Sprintf("%s %s: %v", typeName, r.Name, err))
		}
	}
	return errors
}

// listPipelines lists all Image Builder pipelines with warpgate tags
func (c *ResourceCleaner) listPipelines(ctx context.Context) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	input := &imagebuilder.ListImagePipelinesInput{}
	paginator := imagebuilder.NewListImagePipelinesPaginator(c.clients.ImageBuilder, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list pipelines: %w", err)
		}

		for _, p := range output.ImagePipelineList {
			if p.Tags != nil {
				if buildName, ok := p.Tags["warpgate:name"]; ok {
					resources = append(resources, ResourceInfo{
						Type:      "ImagePipeline",
						Name:      aws.ToString(p.Name),
						ARN:       aws.ToString(p.Arn),
						BuildName: buildName,
					})
				}
			}
		}
	}

	return resources, nil
}

// listRecipes lists all Image Builder recipes with warpgate tags
func (c *ResourceCleaner) listRecipes(ctx context.Context) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	input := &imagebuilder.ListImageRecipesInput{
		Owner: "Self",
	}
	paginator := imagebuilder.NewListImageRecipesPaginator(c.clients.ImageBuilder, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list recipes: %w", err)
		}

		for _, r := range output.ImageRecipeSummaryList {
			if r.Tags != nil {
				if buildName, ok := r.Tags["warpgate:name"]; ok {
					resources = append(resources, ResourceInfo{
						Type:      "ImageRecipe",
						Name:      aws.ToString(r.Name),
						ARN:       aws.ToString(r.Arn),
						BuildName: buildName,
					})
				}
			}
		}
	}

	return resources, nil
}

// listDistributionConfigs lists all distribution configurations with warpgate tags
func (c *ResourceCleaner) listDistributionConfigs(ctx context.Context) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	input := &imagebuilder.ListDistributionConfigurationsInput{}
	paginator := imagebuilder.NewListDistributionConfigurationsPaginator(c.clients.ImageBuilder, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list distribution configs: %w", err)
		}

		for _, d := range output.DistributionConfigurationSummaryList {
			if d.Tags != nil {
				if buildName, ok := d.Tags["warpgate:name"]; ok {
					resources = append(resources, ResourceInfo{
						Type:      "DistributionConfiguration",
						Name:      aws.ToString(d.Name),
						ARN:       aws.ToString(d.Arn),
						BuildName: buildName,
					})
				}
			}
		}
	}

	return resources, nil
}

// listInfrastructureConfigs lists all infrastructure configurations with warpgate tags
func (c *ResourceCleaner) listInfrastructureConfigs(ctx context.Context) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	input := &imagebuilder.ListInfrastructureConfigurationsInput{}
	paginator := imagebuilder.NewListInfrastructureConfigurationsPaginator(c.clients.ImageBuilder, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list infrastructure configs: %w", err)
		}

		for _, i := range output.InfrastructureConfigurationSummaryList {
			if i.Tags != nil {
				if buildName, ok := i.Tags["warpgate:name"]; ok {
					resources = append(resources, ResourceInfo{
						Type:      "InfrastructureConfiguration",
						Name:      aws.ToString(i.Name),
						ARN:       aws.ToString(i.Arn),
						BuildName: buildName,
					})
				}
			}
		}
	}

	return resources, nil
}

// listComponents lists all components with warpgate tags
func (c *ResourceCleaner) listComponents(ctx context.Context) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	input := &imagebuilder.ListComponentsInput{
		Owner: "Self",
	}
	paginator := imagebuilder.NewListComponentsPaginator(c.clients.ImageBuilder, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list components: %w", err)
		}

		for _, comp := range output.ComponentVersionList {
			// Get component details to check tags
			getInput := &imagebuilder.GetComponentInput{
				ComponentBuildVersionArn: comp.Arn,
			}
			detail, err := c.clients.ImageBuilder.GetComponent(ctx, getInput)
			if err != nil {
				continue // Skip if we can't get details
			}

			if detail.Component != nil && detail.Component.Tags != nil {
				if buildName, ok := detail.Component.Tags["warpgate:name"]; ok {
					resources = append(resources, ResourceInfo{
						Type:      "Component",
						Name:      aws.ToString(comp.Name),
						ARN:       aws.ToString(comp.Arn),
						BuildName: buildName,
						Version:   aws.ToString(comp.Version),
					})
				}
			}
		}
	}

	return resources, nil
}

// Delete functions
func (c *ResourceCleaner) deletePipeline(ctx context.Context, arn string) error {
	_, err := c.clients.ImageBuilder.DeleteImagePipeline(ctx, &imagebuilder.DeleteImagePipelineInput{
		ImagePipelineArn: aws.String(arn),
	})
	return err
}

func (c *ResourceCleaner) deleteRecipe(ctx context.Context, arn string) error {
	_, err := c.clients.ImageBuilder.DeleteImageRecipe(ctx, &imagebuilder.DeleteImageRecipeInput{
		ImageRecipeArn: aws.String(arn),
	})
	return err
}

func (c *ResourceCleaner) deleteDistributionConfig(ctx context.Context, arn string) error {
	_, err := c.clients.ImageBuilder.DeleteDistributionConfiguration(ctx, &imagebuilder.DeleteDistributionConfigurationInput{
		DistributionConfigurationArn: aws.String(arn),
	})
	return err
}

func (c *ResourceCleaner) deleteInfrastructureConfig(ctx context.Context, arn string) error {
	_, err := c.clients.ImageBuilder.DeleteInfrastructureConfiguration(ctx, &imagebuilder.DeleteInfrastructureConfigurationInput{
		InfrastructureConfigurationArn: aws.String(arn),
	})
	return err
}

func (c *ResourceCleaner) deleteComponent(ctx context.Context, arn string) error {
	_, err := c.clients.ImageBuilder.DeleteComponent(ctx, &imagebuilder.DeleteComponentInput{
		ComponentBuildVersionArn: aws.String(arn),
	})
	return err
}
