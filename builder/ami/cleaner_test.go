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

package ami

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	ibtypes "github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/stretchr/testify/assert"
)

func TestNewResourceCleaner(t *testing.T) {
	t.Parallel()

	cleaner := NewResourceCleaner(nil)
	assert.NotNil(t, cleaner)
	assert.Nil(t, cleaner.clients)
}

func TestGroupResourcesByType(t *testing.T) {
	t.Parallel()

	cleaner := NewResourceCleaner(nil)

	resources := []ResourceInfo{
		{Type: "Component", Name: "comp1", ARN: "arn:comp1"},
		{Type: "ImagePipeline", Name: "pipe1", ARN: "arn:pipe1"},
		{Type: "Component", Name: "comp2", ARN: "arn:comp2"},
		{Type: "ImageRecipe", Name: "recipe1", ARN: "arn:recipe1"},
		{Type: "ImagePipeline", Name: "pipe2", ARN: "arn:pipe2"},
	}

	grouped := cleaner.groupResourcesByType(resources)

	assert.Len(t, grouped["Component"], 2)
	assert.Len(t, grouped["ImagePipeline"], 2)
	assert.Len(t, grouped["ImageRecipe"], 1)
	assert.Len(t, grouped["DistributionConfiguration"], 0)
}

func TestGroupResourcesByTypeEmpty(t *testing.T) {
	t.Parallel()

	cleaner := NewResourceCleaner(nil)
	grouped := cleaner.groupResourcesByType(nil)
	assert.Empty(t, grouped)
}

// --- listPipelines tests ---

func TestListPipelines_WithWarpgateTag(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{
					Name: aws.String("my-pipeline"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image-pipeline/my-pipeline"),
					Tags: map[string]string{"warpgate:name": "my-build"},
				},
			},
		}, nil
	}

	resources, err := cleaner.listPipelines(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "ImagePipeline", resources[0].Type)
	assert.Equal(t, "my-pipeline", resources[0].Name)
	assert.Equal(t, "my-build", resources[0].BuildName)
}

func TestListPipelines_WithoutTag_Skipped(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{
					Name: aws.String("unrelated-pipeline"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image-pipeline/unrelated"),
					Tags: map[string]string{"env": "prod"},
				},
				{
					Name: aws.String("no-tags-pipeline"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image-pipeline/no-tags"),
				},
			},
		}, nil
	}

	resources, err := cleaner.listPipelines(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, resources)
}

func TestListPipelines_Paginated(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	callCount := 0
	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		callCount++
		if callCount == 1 {
			return &imagebuilder.ListImagePipelinesOutput{
				ImagePipelineList: []ibtypes.ImagePipeline{
					{
						Name: aws.String("pipeline-1"),
						Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image-pipeline/pipeline-1"),
						Tags: map[string]string{"warpgate:name": "build-a"},
					},
				},
				NextToken: aws.String("page2token"),
			}, nil
		}
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{
					Name: aws.String("pipeline-2"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image-pipeline/pipeline-2"),
					Tags: map[string]string{"warpgate:name": "build-b"},
				},
			},
		}, nil
	}

	resources, err := cleaner.listPipelines(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 2)
	assert.Equal(t, "pipeline-1", resources[0].Name)
	assert.Equal(t, "pipeline-2", resources[1].Name)
	assert.Equal(t, 2, callCount)
}

func TestListPipelines_APIError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return nil, fmt.Errorf("access denied")
	}

	resources, err := cleaner.listPipelines(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list pipelines")
	assert.Nil(t, resources)
}

// --- listRecipes tests ---

func TestListRecipes_WithWarpgateTag(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
				{
					Name: aws.String("my-recipe"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image-recipe/my-recipe/1.0.0"),
					Tags: map[string]string{"warpgate:name": "my-build"},
				},
			},
		}, nil
	}

	resources, err := cleaner.listRecipes(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "ImageRecipe", resources[0].Type)
	assert.Equal(t, "my-recipe", resources[0].Name)
	assert.Equal(t, "my-build", resources[0].BuildName)
}

func TestListRecipes_APIError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return nil, fmt.Errorf("throttled")
	}

	resources, err := cleaner.listRecipes(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list recipes")
	assert.Nil(t, resources)
}

// --- listDistributionConfigs tests ---

func TestListDistributionConfigs_WithWarpgateTag(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{
			DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{
				{
					Name: aws.String("my-dist"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:distribution-configuration/my-dist"),
					Tags: map[string]string{"warpgate:name": "my-build"},
				},
			},
		}, nil
	}

	resources, err := cleaner.listDistributionConfigs(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "DistributionConfiguration", resources[0].Type)
	assert.Equal(t, "my-dist", resources[0].Name)
	assert.Equal(t, "my-build", resources[0].BuildName)
}

func TestListDistributionConfigs_APIError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return nil, fmt.Errorf("service unavailable")
	}

	resources, err := cleaner.listDistributionConfigs(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list distribution configs")
	assert.Nil(t, resources)
}

// --- listInfrastructureConfigs tests ---

func TestListInfrastructureConfigs_WithWarpgateTag(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
				{
					Name: aws.String("my-infra"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:infrastructure-configuration/my-infra"),
					Tags: map[string]string{"warpgate:name": "my-build"},
				},
			},
		}, nil
	}

	resources, err := cleaner.listInfrastructureConfigs(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "InfrastructureConfiguration", resources[0].Type)
	assert.Equal(t, "my-infra", resources[0].Name)
	assert.Equal(t, "my-build", resources[0].BuildName)
}

func TestListInfrastructureConfigs_APIError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return nil, fmt.Errorf("network error")
	}

	resources, err := cleaner.listInfrastructureConfigs(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list infrastructure configs")
	assert.Nil(t, resources)
}

// --- listComponents tests ---

func TestListComponents_WithWarpgateTag(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{
					Name:    aws.String("my-component"),
					Arn:     aws.String("arn:aws:imagebuilder:us-east-1:123456789012:component/my-component/1.0.0/1"),
					Version: aws.String("1.0.0"),
				},
			},
		}, nil
	}

	mocks.imageBuilder.GetComponentFunc = func(ctx context.Context, params *imagebuilder.GetComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetComponentOutput, error) {
		return &imagebuilder.GetComponentOutput{
			Component: &ibtypes.Component{
				Name: aws.String("my-component"),
				Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123456789012:component/my-component/1.0.0/1"),
				Tags: map[string]string{"warpgate:name": "my-build"},
			},
		}, nil
	}

	resources, err := cleaner.listComponents(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "Component", resources[0].Type)
	assert.Equal(t, "my-component", resources[0].Name)
	assert.Equal(t, "my-build", resources[0].BuildName)
	assert.Equal(t, "1.0.0", resources[0].Version)
}

func TestListComponents_GetComponentError_Skipped(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{
					Name:    aws.String("bad-component"),
					Arn:     aws.String("arn:aws:imagebuilder:us-east-1:123456789012:component/bad-component/1.0.0/1"),
					Version: aws.String("1.0.0"),
				},
				{
					Name:    aws.String("good-component"),
					Arn:     aws.String("arn:aws:imagebuilder:us-east-1:123456789012:component/good-component/1.0.0/1"),
					Version: aws.String("1.0.0"),
				},
			},
		}, nil
	}

	callCount := 0
	mocks.imageBuilder.GetComponentFunc = func(ctx context.Context, params *imagebuilder.GetComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetComponentOutput, error) {
		callCount++
		if aws.ToString(params.ComponentBuildVersionArn) == "arn:aws:imagebuilder:us-east-1:123456789012:component/bad-component/1.0.0/1" {
			return nil, fmt.Errorf("component not accessible")
		}
		return &imagebuilder.GetComponentOutput{
			Component: &ibtypes.Component{
				Name: aws.String("good-component"),
				Tags: map[string]string{"warpgate:name": "my-build"},
			},
		}, nil
	}

	resources, err := cleaner.listComponents(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "good-component", resources[0].Name)
	assert.Equal(t, 2, callCount)
}

func TestListComponents_APIError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return nil, fmt.Errorf("access denied")
	}

	resources, err := cleaner.listComponents(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list components")
	assert.Nil(t, resources)
}

// --- ListWarpgateResources tests ---

func TestListWarpgateResources_ReturnsAllTypes(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{Name: aws.String("pipe-1"), Arn: aws.String("arn:pipe-1"), Tags: map[string]string{"warpgate:name": "build-a"}},
			},
		}, nil
	}

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
				{Name: aws.String("recipe-1"), Arn: aws.String("arn:recipe-1"), Tags: map[string]string{"warpgate:name": "build-a"}},
			},
		}, nil
	}

	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{
			DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{
				{Name: aws.String("dist-1"), Arn: aws.String("arn:dist-1"), Tags: map[string]string{"warpgate:name": "build-a"}},
			},
		}, nil
	}

	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
				{Name: aws.String("infra-1"), Arn: aws.String("arn:infra-1"), Tags: map[string]string{"warpgate:name": "build-a"}},
			},
		}, nil
	}

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{Name: aws.String("comp-1"), Arn: aws.String("arn:comp-1"), Version: aws.String("1.0.0")},
			},
		}, nil
	}

	mocks.imageBuilder.GetComponentFunc = func(ctx context.Context, params *imagebuilder.GetComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetComponentOutput, error) {
		return &imagebuilder.GetComponentOutput{
			Component: &ibtypes.Component{
				Name: aws.String("comp-1"),
				Tags: map[string]string{"warpgate:name": "build-a"},
			},
		}, nil
	}

	resources, err := cleaner.ListWarpgateResources(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resources, 5)

	typeCount := map[string]int{}
	for _, r := range resources {
		typeCount[r.Type]++
	}
	assert.Equal(t, 1, typeCount["ImagePipeline"])
	assert.Equal(t, 1, typeCount["ImageRecipe"])
	assert.Equal(t, 1, typeCount["DistributionConfiguration"])
	assert.Equal(t, 1, typeCount["InfrastructureConfiguration"])
	assert.Equal(t, 1, typeCount["Component"])
}

func TestListWarpgateResources_HandlesIndividualListErrors(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	// Pipelines error
	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return nil, fmt.Errorf("pipeline list error")
	}

	// Recipes succeed
	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
				{Name: aws.String("recipe-1"), Arn: aws.String("arn:recipe-1"), Tags: map[string]string{"warpgate:name": "build-a"}},
			},
		}, nil
	}

	// Dist configs error
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return nil, fmt.Errorf("dist list error")
	}

	// Infra configs error
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return nil, fmt.Errorf("infra list error")
	}

	// Components error
	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return nil, fmt.Errorf("component list error")
	}

	resources, err := cleaner.ListWarpgateResources(context.Background())
	// ListWarpgateResources returns nil error even with individual failures
	assert.NoError(t, err)
	// Only recipe succeeded
	assert.Len(t, resources, 1)
	assert.Equal(t, "ImageRecipe", resources[0].Type)
}

// --- ListResourcesForBuild tests ---

func TestListResourcesForBuild_FiltersByBuildName(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{Name: aws.String("my-build-pipeline"), Arn: aws.String("arn:pipe-1"), Tags: map[string]string{"warpgate:name": "my-build"}},
				{Name: aws.String("other-pipeline"), Arn: aws.String("arn:pipe-2"), Tags: map[string]string{"warpgate:name": "other-build"}},
			},
		}, nil
	}

	// Empty results for the rest
	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{}, nil
	}
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
	}
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
	}
	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{}, nil
	}

	resources, err := cleaner.ListResourcesForBuild(context.Background(), "my-build")
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "my-build", resources[0].BuildName)
}

func TestListResourcesForBuild_MatchesByNamePrefix(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{
					Name: aws.String("my-build-pipeline"),
					Arn:  aws.String("arn:pipe-1"),
					Tags: map[string]string{"warpgate:name": "different-name"},
				},
			},
		}, nil
	}

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
				{
					Name: aws.String("my-build"),
					Arn:  aws.String("arn:recipe-exact"),
					Tags: map[string]string{"warpgate:name": "something-else"},
				},
			},
		}, nil
	}
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
	}
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
	}
	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{}, nil
	}

	resources, err := cleaner.ListResourcesForBuild(context.Background(), "my-build")
	assert.NoError(t, err)
	// pipeline matches by prefix "my-build-", recipe matches by exact name "my-build"
	assert.Len(t, resources, 2)
}

// --- DeleteResources tests ---

func TestDeleteResources_DeletesInOrder(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	var deleteOrder []string

	mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
		deleteOrder = append(deleteOrder, "pipeline")
		return &imagebuilder.DeleteImagePipelineOutput{}, nil
	}
	mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
		deleteOrder = append(deleteOrder, "recipe")
		return &imagebuilder.DeleteImageRecipeOutput{}, nil
	}
	mocks.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
		deleteOrder = append(deleteOrder, "dist")
		return &imagebuilder.DeleteDistributionConfigurationOutput{}, nil
	}
	mocks.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
		deleteOrder = append(deleteOrder, "infra")
		return &imagebuilder.DeleteInfrastructureConfigurationOutput{}, nil
	}
	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		deleteOrder = append(deleteOrder, "component")
		return &imagebuilder.DeleteComponentOutput{}, nil
	}

	resources := []ResourceInfo{
		{Type: "Component", Name: "comp-1", ARN: "arn:comp-1"},
		{Type: "ImagePipeline", Name: "pipe-1", ARN: "arn:pipe-1"},
		{Type: "ImageRecipe", Name: "recipe-1", ARN: "arn:recipe-1"},
		{Type: "DistributionConfiguration", Name: "dist-1", ARN: "arn:dist-1"},
		{Type: "InfrastructureConfiguration", Name: "infra-1", ARN: "arn:infra-1"},
	}

	err := cleaner.DeleteResources(context.Background(), resources)
	assert.NoError(t, err)
	assert.Equal(t, []string{"pipeline", "recipe", "dist", "infra", "component"}, deleteOrder)
}

func TestDeleteResources_HandlesIndividualDeleteErrors(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
		return nil, fmt.Errorf("pipeline delete error")
	}
	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		return &imagebuilder.DeleteComponentOutput{}, nil
	}

	resources := []ResourceInfo{
		{Type: "ImagePipeline", Name: "pipe-1", ARN: "arn:pipe-1"},
		{Type: "Component", Name: "comp-1", ARN: "arn:comp-1"},
	}

	err := cleaner.DeleteResources(context.Background(), resources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline pipe-1")
	assert.Contains(t, err.Error(), "pipeline delete error")
}

// --- Delete helper function tests ---

func TestDeletePipeline_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	var capturedARN string
	mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
		capturedARN = aws.ToString(params.ImagePipelineArn)
		return &imagebuilder.DeleteImagePipelineOutput{}, nil
	}

	err := cleaner.deletePipeline(context.Background(), "arn:aws:imagebuilder:us-east-1:123:image-pipeline/test")
	assert.NoError(t, err)
	assert.Equal(t, "arn:aws:imagebuilder:us-east-1:123:image-pipeline/test", capturedARN)
}

func TestDeletePipeline_Error(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
		return nil, fmt.Errorf("pipeline in use")
	}

	err := cleaner.deletePipeline(context.Background(), "arn:pipe")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline in use")
}

func TestDeleteRecipe_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	var capturedARN string
	mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
		capturedARN = aws.ToString(params.ImageRecipeArn)
		return &imagebuilder.DeleteImageRecipeOutput{}, nil
	}

	err := cleaner.deleteRecipe(context.Background(), "arn:recipe-1")
	assert.NoError(t, err)
	assert.Equal(t, "arn:recipe-1", capturedARN)
}

func TestDeleteRecipe_Error(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
		return nil, fmt.Errorf("recipe in use")
	}

	err := cleaner.deleteRecipe(context.Background(), "arn:recipe-1")
	assert.Error(t, err)
}

func TestDeleteDistributionConfig_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	var capturedARN string
	mocks.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
		capturedARN = aws.ToString(params.DistributionConfigurationArn)
		return &imagebuilder.DeleteDistributionConfigurationOutput{}, nil
	}

	err := cleaner.deleteDistributionConfig(context.Background(), "arn:dist-1")
	assert.NoError(t, err)
	assert.Equal(t, "arn:dist-1", capturedARN)
}

func TestDeleteDistributionConfig_Error(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
		return nil, fmt.Errorf("dist config in use")
	}

	err := cleaner.deleteDistributionConfig(context.Background(), "arn:dist-1")
	assert.Error(t, err)
}

func TestDeleteInfrastructureConfig_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	var capturedARN string
	mocks.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
		capturedARN = aws.ToString(params.InfrastructureConfigurationArn)
		return &imagebuilder.DeleteInfrastructureConfigurationOutput{}, nil
	}

	err := cleaner.deleteInfrastructureConfig(context.Background(), "arn:infra-1")
	assert.NoError(t, err)
	assert.Equal(t, "arn:infra-1", capturedARN)
}

func TestDeleteInfrastructureConfig_Error(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
		return nil, fmt.Errorf("infra config in use")
	}

	err := cleaner.deleteInfrastructureConfig(context.Background(), "arn:infra-1")
	assert.Error(t, err)
}

func TestDeleteComponent_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	var capturedARN string
	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		capturedARN = aws.ToString(params.ComponentBuildVersionArn)
		return &imagebuilder.DeleteComponentOutput{}, nil
	}

	err := cleaner.deleteComponent(context.Background(), "arn:comp-1")
	assert.NoError(t, err)
	assert.Equal(t, "arn:comp-1", capturedARN)
}

func TestDeleteComponent_Error(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	cleaner := NewResourceCleaner(clients)

	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		return nil, fmt.Errorf("component in use")
	}

	err := cleaner.deleteComponent(context.Background(), "arn:comp-1")
	assert.Error(t, err)
}
