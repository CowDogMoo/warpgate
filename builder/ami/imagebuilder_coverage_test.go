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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	ibtypes "github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVolumeType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected ibtypes.EbsVolumeType
	}{
		{"gp2", "gp2", ibtypes.EbsVolumeTypeGp2},
		{"gp3", "gp3", ibtypes.EbsVolumeTypeGp3},
		{"io1", "io1", ibtypes.EbsVolumeTypeIo1},
		{"io2", "io2", ibtypes.EbsVolumeTypeIo2},
		{"sc1", "sc1", ibtypes.EbsVolumeTypeSc1},
		{"st1", "st1", ibtypes.EbsVolumeTypeSt1},
		{"standard", "standard", ibtypes.EbsVolumeTypeStandard},
		{"unknown defaults to gp3", "unknown", ibtypes.EbsVolumeTypeGp3},
		{"empty defaults to gp3", "", ibtypes.EbsVolumeTypeGp3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVolumeType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAMIID_WithTypes(t *testing.T) {
	t.Parallel()
	ib := &ImageBuilder{}

	tests := []struct {
		name    string
		image   *ibtypes.Image
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid image with AMI",
			image: &ibtypes.Image{
				OutputResources: &ibtypes.OutputResources{
					Amis: []ibtypes.Ami{
						{Image: aws.String("ami-1234567890abcdef0")},
					},
				},
			},
			want: "ami-1234567890abcdef0",
		},
		{
			name: "nil output resources",
			image: &ibtypes.Image{
				OutputResources: nil,
			},
			wantErr: true,
			errMsg:  "no AMI output found",
		},
		{
			name: "empty AMIs list",
			image: &ibtypes.Image{
				OutputResources: &ibtypes.OutputResources{
					Amis: []ibtypes.Ami{},
				},
			},
			wantErr: true,
			errMsg:  "no AMI output found",
		},
		{
			name: "nil AMI ID",
			image: &ibtypes.Image{
				OutputResources: &ibtypes.OutputResources{
					Amis: []ibtypes.Ami{
						{Image: nil},
					},
				},
			},
			wantErr: true,
			errMsg:  "AMI ID is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ib.extractAMIID(tt.image)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestSetupBuild(t *testing.T) {
	t.Parallel()
	ib := &ImageBuilder{
		config: ClientConfig{Region: "us-east-1"},
	}

	tests := []struct {
		name    string
		config  builder.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with AMI target",
			config: builder.Config{
				Targets: []builder.Target{
					{Type: "ami", Region: "us-east-1"},
				},
			},
			wantErr: false,
		},
		{
			name: "no AMI target",
			config: builder.Config{
				Targets: []builder.Target{
					{Type: "container"},
				},
			},
			wantErr: true,
			errMsg:  "no AMI target found",
		},
		{
			name: "empty targets",
			config: builder.Config{
				Targets: []builder.Target{},
			},
			wantErr: true,
			errMsg:  "no AMI target found",
		},
		{
			name: "AMI target with region in target",
			config: builder.Config{
				Targets: []builder.Target{
					{Type: "ami", Region: "us-west-2"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ib.setupBuild(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, target)
			}
		})
	}
}

func TestImageBuilderSetMonitorConfig(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)

	ib := &ImageBuilder{
		pipelineManager: pm,
	}

	mc := MonitorConfig{
		StreamLogs:    true,
		ShowEC2Status: true,
	}

	ib.SetMonitorConfig(mc)
	assert.Equal(t, mc, ib.monitorConfig)
}

func TestSetNamingPrefix(t *testing.T) {
	t.Parallel()
	ib := &ImageBuilder{}
	ib.SetNamingPrefix("myprefix")
	assert.Equal(t, "myprefix", ib.namingPrefix)
}

func TestGetBuildID(t *testing.T) {
	t.Parallel()
	ib := &ImageBuilder{buildID: "20260101-120000-abcd1234"}
	assert.Equal(t, "20260101-120000-abcd1234", ib.GetBuildID())
}

func TestSetForceRecreate(t *testing.T) {
	t.Parallel()
	ib := &ImageBuilder{}
	assert.False(t, ib.forceRecreate)
	ib.SetForceRecreate(true)
	assert.True(t, ib.forceRecreate)
	ib.SetForceRecreate(false)
	assert.False(t, ib.forceRecreate)
}

func TestClose(t *testing.T) {
	t.Parallel()
	ib := &ImageBuilder{}
	err := ib.Close()
	assert.NoError(t, err)
}

func TestDryRun(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()

	ib := &ImageBuilder{
		clients: clients,
		config:  ClientConfig{Region: "us-east-1"},
	}

	tests := []struct {
		name    string
		config  builder.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: builder.Config{
				Name:    "test-image",
				Version: "1.0.0",
				Targets: []builder.Target{
					{
						Type:   "ami",
						Region: "us-east-1",
					},
				},
				Provisioners: []builder.Provisioner{
					{
						Type:   "shell",
						Inline: []string{"echo hello"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no AMI target",
			config: builder.Config{
				Targets: []builder.Target{
					{Type: "container"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ib.DryRun(context.Background(), tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestShare(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	ib := &ImageBuilder{
		operations: NewAMIOperations(clients, nil),
	}

	err := ib.Share(context.Background(), "ami-12345", []string{"123456789012"})
	assert.NoError(t, err)
}

func TestDeregister_SameRegion(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		return &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId: aws.String("ami-12345"),
				},
			},
		}, nil
	}

	ib := &ImageBuilder{
		operations: NewAMIOperations(clients, nil),
		config:     ClientConfig{Region: "us-east-1"},
	}

	err := ib.Deregister(context.Background(), "ami-12345", "us-east-1")
	assert.NoError(t, err)
}

func TestDeregister_EmptyRegion(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		return &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{ImageId: aws.String("ami-12345")},
			},
		}, nil
	}

	ib := &ImageBuilder{
		operations: NewAMIOperations(clients, nil),
		config:     ClientConfig{Region: "us-east-1"},
	}

	err := ib.Deregister(context.Background(), "ami-12345", "")
	assert.NoError(t, err)
}

func TestCreatedResourcesCleanup_AllFields(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	manager := NewResourceManager(clients)

	resources := &CreatedResources{
		PipelineARN:   "arn:pipeline",
		RecipeARN:     "arn:recipe",
		DistARN:       "arn:dist",
		InfraARN:      "arn:infra",
		ComponentARNs: []string{"arn:comp1", "arn:comp2"},
	}

	// Should not panic; cleanup logs warnings but does not return errors
	resources.Cleanup(context.Background(), manager)
}

func TestCreatedResourcesCleanup_NoFields(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	manager := NewResourceManager(clients)

	resources := &CreatedResources{}
	resources.Cleanup(context.Background(), manager)
}

func TestCleanupResourcesForBuild(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	// Mock pipeline lookup returning not-found
	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{},
		}, nil
	}
	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{},
		}, nil
	}
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{
			DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{},
		}, nil
	}
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{},
		}, nil
	}

	manager := NewResourceManager(clients)
	err := manager.CleanupResourcesForBuild(context.Background(), "test-build", false)
	assert.NoError(t, err)
}

func TestCleanupResourcesForBuild_WithExistingResources(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	pipelineARN := "arn:aws:imagebuilder:us-east-1:123456789012:image-pipeline/test-build-pipeline"
	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{
					Name: aws.String("test-build-pipeline"),
					Arn:  aws.String(pipelineARN),
				},
			},
		}, nil
	}
	mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
		return &imagebuilder.DeleteImagePipelineOutput{}, nil
	}
	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{},
		}, nil
	}
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{
			DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{},
		}, nil
	}
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{},
		}, nil
	}

	manager := NewResourceManager(clients)
	err := manager.CleanupResourcesForBuild(context.Background(), "test-build", true)
	assert.NoError(t, err)
}

func TestCleanupResource(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	manager := NewResourceManager(clients)

	t.Run("resource does not exist", func(t *testing.T) {
		err := manager.cleanupResource(context.Background(), "pipeline",
			func() (string, error) { return "", nil },
			func(ctx context.Context, arn string) error { return nil },
			false,
		)
		assert.NoError(t, err)
	})

	t.Run("resource exists and deletion succeeds", func(t *testing.T) {
		deleted := false
		err := manager.cleanupResource(context.Background(), "pipeline",
			func() (string, error) { return "arn:test", nil },
			func(ctx context.Context, arn string) error { deleted = true; return nil },
			false,
		)
		assert.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("resource exists and deletion fails without force", func(t *testing.T) {
		err := manager.cleanupResource(context.Background(), "pipeline",
			func() (string, error) { return "arn:test", nil },
			func(ctx context.Context, arn string) error { return fmt.Errorf("delete failed") },
			false,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete pipeline")
	})

	t.Run("resource exists and deletion fails with force", func(t *testing.T) {
		err := manager.cleanupResource(context.Background(), "pipeline",
			func() (string, error) { return "arn:test", nil },
			func(ctx context.Context, arn string) error { return fmt.Errorf("delete failed") },
			true,
		)
		// With force, continues instead of erroring
		assert.NoError(t, err)
	})

	t.Run("getARN returns error", func(t *testing.T) {
		err := manager.cleanupResource(context.Background(), "pipeline",
			func() (string, error) { return "", fmt.Errorf("not found") },
			func(ctx context.Context, arn string) error { return nil },
			false,
		)
		// Error in getARN means resource doesn't exist
		assert.NoError(t, err)
	})
}

func TestGetPipelineARN(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{
					Name: aws.String("test-pipeline"),
					Arn:  aws.String("arn:pipeline:test"),
				},
			},
		}, nil
	}
	manager := NewResourceManager(clients)
	arn, err := manager.getPipelineARN(context.Background(), "test-pipeline")
	require.NoError(t, err)
	assert.Equal(t, "arn:pipeline:test", arn)
}

func TestGetPipelineARN_NotFound(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{},
		}, nil
	}

	manager := NewResourceManager(clients)
	arn, err := manager.getPipelineARN(context.Background(), "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, arn)
}

func TestGetRecipeARN(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
				{
					Name: aws.String("test-recipe"),
					Arn:  aws.String("arn:recipe:test"),
				},
			},
		}, nil
	}
	mocks.imageBuilder.GetImageRecipeFunc = func(ctx context.Context, params *imagebuilder.GetImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error) {
		return &imagebuilder.GetImageRecipeOutput{
			ImageRecipe: &ibtypes.ImageRecipe{
				Arn:  aws.String("arn:recipe:test"),
				Name: aws.String("test-recipe"),
			},
		}, nil
	}

	manager := NewResourceManager(clients)
	arn, err := manager.getRecipeARN(context.Background(), "test-recipe")
	require.NoError(t, err)
	assert.Equal(t, "arn:recipe:test", arn)
}

func TestGetDistConfigARN(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{
			DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{
				{
					Name: aws.String("test-dist"),
					Arn:  aws.String("arn:dist:test"),
				},
			},
		}, nil
	}
	mocks.imageBuilder.GetDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error) {
		return &imagebuilder.GetDistributionConfigurationOutput{
			DistributionConfiguration: &ibtypes.DistributionConfiguration{
				Arn: aws.String("arn:dist:test"),
			},
		}, nil
	}

	manager := NewResourceManager(clients)
	arn, err := manager.getDistConfigARN(context.Background(), "test-dist")
	require.NoError(t, err)
	assert.Equal(t, "arn:dist:test", arn)
}

func TestGetInfraConfigARN(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
				{
					Name: aws.String("test-infra"),
					Arn:  aws.String("arn:infra:test"),
				},
			},
		}, nil
	}
	mocks.imageBuilder.GetInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
		return &imagebuilder.GetInfrastructureConfigurationOutput{
			InfrastructureConfiguration: &ibtypes.InfrastructureConfiguration{
				Arn:  aws.String("arn:infra:test"),
				Name: aws.String("test-infra"),
			},
		}, nil
	}

	manager := NewResourceManager(clients)
	arn, err := manager.getInfraConfigARN(context.Background(), "test-infra")
	require.NoError(t, err)
	assert.Equal(t, "arn:infra:test", arn)
}

func TestOptimizedResourceCleanup(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	// All resources exist
	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{Name: aws.String("test-pipeline"), Arn: aws.String("arn:pipeline:test")},
			},
		}, nil
	}
	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
				{Name: aws.String("test-recipe"), Arn: aws.String("arn:recipe:test")},
			},
		}, nil
	}
	mocks.imageBuilder.GetImageRecipeFunc = func(ctx context.Context, params *imagebuilder.GetImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error) {
		return &imagebuilder.GetImageRecipeOutput{
			ImageRecipe: &ibtypes.ImageRecipe{Arn: aws.String("arn:recipe:test"), Name: aws.String("test-recipe")},
		}, nil
	}
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{
			DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{
				{Name: aws.String("test-dist"), Arn: aws.String("arn:dist:test")},
			},
		}, nil
	}
	mocks.imageBuilder.GetDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error) {
		return &imagebuilder.GetDistributionConfigurationOutput{
			DistributionConfiguration: &ibtypes.DistributionConfiguration{Arn: aws.String("arn:dist:test")},
		}, nil
	}
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
				{Name: aws.String("test-infra"), Arn: aws.String("arn:infra:test")},
			},
		}, nil
	}
	mocks.imageBuilder.GetInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
		return &imagebuilder.GetInfrastructureConfigurationOutput{
			InfrastructureConfiguration: &ibtypes.InfrastructureConfiguration{Arn: aws.String("arn:infra:test"), Name: aws.String("test-infra")},
		}, nil
	}
	mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
		return &imagebuilder.DeleteImagePipelineOutput{}, nil
	}
	mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
		return &imagebuilder.DeleteImageRecipeOutput{}, nil
	}
	mocks.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
		return &imagebuilder.DeleteDistributionConfigurationOutput{}, nil
	}
	mocks.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
		return &imagebuilder.DeleteInfrastructureConfigurationOutput{}, nil
	}

	bo := NewBatchOperations(clients)
	err := bo.OptimizedResourceCleanup(context.Background(), "test")
	assert.NoError(t, err)
}

func TestOptimizedResourceCleanup_NoResources(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	// All resources don't exist
	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{}, nil
	}
	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{}, nil
	}
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
	}
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
	}

	bo := NewBatchOperations(clients)
	err := bo.OptimizedResourceCleanup(context.Background(), "nonexistent")
	assert.NoError(t, err)
}

func TestBatchGetRecipeARN(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{
			ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
				{Name: aws.String("test-recipe"), Arn: aws.String("arn:recipe:test")},
			},
		}, nil
	}
	mocks.imageBuilder.GetImageRecipeFunc = func(ctx context.Context, params *imagebuilder.GetImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error) {
		return &imagebuilder.GetImageRecipeOutput{
			ImageRecipe: &ibtypes.ImageRecipe{Arn: aws.String("arn:recipe:test"), Name: aws.String("test-recipe")},
		}, nil
	}

	bo := NewBatchOperations(clients)
	arn, err := bo.getRecipeARN(context.Background(), "test-recipe")
	require.NoError(t, err)
	assert.Equal(t, "arn:recipe:test", arn)
}

func TestBatchGetDistARN(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{
			DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{
				{Name: aws.String("test-dist"), Arn: aws.String("arn:dist:test")},
			},
		}, nil
	}
	mocks.imageBuilder.GetDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error) {
		return &imagebuilder.GetDistributionConfigurationOutput{
			DistributionConfiguration: &ibtypes.DistributionConfiguration{Arn: aws.String("arn:dist:test")},
		}, nil
	}

	bo := NewBatchOperations(clients)
	arn, err := bo.getDistARN(context.Background(), "test-dist")
	require.NoError(t, err)
	assert.Equal(t, "arn:dist:test", arn)
}

func TestBatchGetInfraARN(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
				{Name: aws.String("test-infra"), Arn: aws.String("arn:infra:test")},
			},
		}, nil
	}
	mocks.imageBuilder.GetInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
		return &imagebuilder.GetInfrastructureConfigurationOutput{
			InfrastructureConfiguration: &ibtypes.InfrastructureConfiguration{Arn: aws.String("arn:infra:test"), Name: aws.String("test-infra")},
		}, nil
	}

	bo := NewBatchOperations(clients)
	arn, err := bo.getInfraARN(context.Background(), "test-infra")
	require.NoError(t, err)
	assert.Equal(t, "arn:infra:test", arn)
}

func TestBatchGetRecipeARN_NotFound(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{}, nil
	}

	bo := NewBatchOperations(clients)
	arn, err := bo.getRecipeARN(context.Background(), "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, arn)
}

func TestBatchGetDistARN_NotFound(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
	}

	bo := NewBatchOperations(clients)
	arn, err := bo.getDistARN(context.Background(), "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, arn)
}

func TestBatchGetInfraARN_NotFound(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
	}

	bo := NewBatchOperations(clients)
	arn, err := bo.getInfraARN(context.Background(), "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, arn)
}

func TestBatchGetRecipeARN_ListError(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return nil, fmt.Errorf("api error")
	}

	bo := NewBatchOperations(clients)
	_, err := bo.getRecipeARN(context.Background(), "test")
	assert.Error(t, err)
}

func TestBatchGetDistARN_ListError(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return nil, fmt.Errorf("api error")
	}

	bo := NewBatchOperations(clients)
	_, err := bo.getDistARN(context.Background(), "test")
	assert.Error(t, err)
}

func TestBatchGetInfraARN_ListError(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return nil, fmt.Errorf("api error")
	}

	bo := NewBatchOperations(clients)
	_, err := bo.getInfraARN(context.Background(), "test")
	assert.Error(t, err)
}

func newTestGlobalConfig() *config.Config {
	return &config.Config{
		AWS: config.AWSConfig{
			AMI: config.AMIConfig{
				InstanceType:        "m5.large",
				InstanceProfileName: "test-profile",
				DefaultParentImage:  "ami-12345",
				VolumeSize:          30,
				VolumeType:          "gp3",
				DeviceName:          "/dev/sda1",
				PollingIntervalSec:  30,
				BuildTimeoutMin:     60,
			},
		},
	}
}

func TestCreateInfrastructureConfig_Success(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	expectedARN := "arn:aws:imagebuilder:us-east-1:123456789012:infrastructure-configuration/test-infra"
	mocks.imageBuilder.CreateInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.CreateInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateInfrastructureConfigurationOutput, error) {
		return &imagebuilder.CreateInfrastructureConfigurationOutput{
			InfrastructureConfigurationArn: aws.String(expectedARN),
		}, nil
	}

	ib := &ImageBuilder{
		clients:         clients,
		globalConfig:    newTestGlobalConfig(),
		resourceManager: NewResourceManager(clients),
	}

	target := &builder.Target{
		Type:             "ami",
		Region:           "us-east-1",
		SubnetID:         "subnet-12345",
		SecurityGroupIDs: []string{"sg-12345"},
	}

	arn, err := ib.createInfrastructureConfig(context.Background(), "test", target)
	require.NoError(t, err)
	assert.Equal(t, expectedARN, arn)
}

func TestCreateInfrastructureConfig_AlreadyExists(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.CreateInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.CreateInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateInfrastructureConfigurationOutput, error) {
		return nil, fmt.Errorf("ResourceAlreadyExistsException: already exists")
	}
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{
			InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
				{Name: aws.String("test-infra"), Arn: aws.String("arn:infra:existing")},
			},
		}, nil
	}
	mocks.imageBuilder.GetInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
		return &imagebuilder.GetInfrastructureConfigurationOutput{
			InfrastructureConfiguration: &ibtypes.InfrastructureConfiguration{
				Arn:  aws.String("arn:infra:existing"),
				Name: aws.String("test-infra"),
			},
		}, nil
	}

	ib := &ImageBuilder{
		clients:         clients,
		globalConfig:    newTestGlobalConfig(),
		resourceManager: NewResourceManager(clients),
	}

	target := &builder.Target{Type: "ami", Region: "us-east-1"}
	arn, err := ib.createInfrastructureConfig(context.Background(), "test", target)
	require.NoError(t, err)
	assert.Equal(t, "arn:infra:existing", arn)
}

func TestCreateInfrastructureConfig_MissingInstanceProfile(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()

	ib := &ImageBuilder{
		clients:      clients,
		globalConfig: &config.Config{},
	}

	target := &builder.Target{Type: "ami", Region: "us-east-1"}
	_, err := ib.createInfrastructureConfig(context.Background(), "test", target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instance_profile_name must be specified")
}

func TestCreateDistributionConfig_Success(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	expectedARN := "arn:aws:imagebuilder:us-east-1:123456789012:distribution-configuration/test-dist"
	mocks.imageBuilder.CreateDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.CreateDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateDistributionConfigurationOutput, error) {
		return &imagebuilder.CreateDistributionConfigurationOutput{
			DistributionConfigurationArn: aws.String(expectedARN),
		}, nil
	}

	ib := &ImageBuilder{
		clients:         clients,
		globalConfig:    newTestGlobalConfig(),
		resourceManager: NewResourceManager(clients),
	}

	target := &builder.Target{
		Type:    "ami",
		Region:  "us-east-1",
		AMIName: "my-ami-{{timestamp}}",
		AMITags: map[string]string{"env": "test"},
	}

	arn, err := ib.createDistributionConfig(context.Background(), "test", target)
	require.NoError(t, err)
	assert.Equal(t, expectedARN, arn)
}

func TestCreateDistributionConfig_WithFastLaunch(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	expectedARN := "arn:dist:fastlaunch"
	mocks.imageBuilder.CreateDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.CreateDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateDistributionConfigurationOutput, error) {
		// Verify fast launch is included
		assert.Len(t, params.Distributions, 1)
		assert.NotEmpty(t, params.Distributions[0].FastLaunchConfigurations)
		return &imagebuilder.CreateDistributionConfigurationOutput{
			DistributionConfigurationArn: aws.String(expectedARN),
		}, nil
	}

	ib := &ImageBuilder{
		clients:         clients,
		globalConfig:    newTestGlobalConfig(),
		resourceManager: NewResourceManager(clients),
	}

	target := &builder.Target{
		Type:              "ami",
		Region:            "us-east-1",
		FastLaunchEnabled: true,
	}

	arn, err := ib.createDistributionConfig(context.Background(), "test", target)
	require.NoError(t, err)
	assert.Equal(t, expectedARN, arn)
}

func TestCreateImageRecipe_Success(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	expectedARN := "arn:aws:imagebuilder:us-east-1:123456789012:image-recipe/test-recipe/1.0.0"
	mocks.imageBuilder.CreateImageRecipeFunc = func(ctx context.Context, params *imagebuilder.CreateImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImageRecipeOutput, error) {
		return &imagebuilder.CreateImageRecipeOutput{
			ImageRecipeArn: aws.String(expectedARN),
		}, nil
	}

	ib := &ImageBuilder{
		clients:         clients,
		globalConfig:    newTestGlobalConfig(),
		resourceManager: NewResourceManager(clients),
	}

	config := builder.Config{
		Name:    "test",
		Version: "1.0.0",
		Base:    builder.BaseImage{Image: "ami-base123"},
	}
	target := &builder.Target{Type: "ami", Region: "us-east-1"}
	componentARNs := []string{"arn:comp1", "arn:comp2"}

	arn, err := ib.createImageRecipe(context.Background(), config, componentARNs, target)
	require.NoError(t, err)
	assert.Equal(t, expectedARN, arn)
}

func TestCreateImageRecipe_MissingParentImage(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()

	ib := &ImageBuilder{
		clients:      clients,
		globalConfig: &config.Config{},
	}

	cfg := builder.Config{
		Name:    "test",
		Version: "1.0.0",
	}
	target := &builder.Target{Type: "ami"}

	_, err := ib.createImageRecipe(context.Background(), cfg, []string{"arn:comp1"}, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent image")
}

func TestValidateConfig_Comprehensive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  ClientConfig
		target  *builder.Target
		wantErr bool
	}{
		{
			name:    "region in client config",
			config:  ClientConfig{Region: "us-east-1"},
			target:  &builder.Target{},
			wantErr: false,
		},
		{
			name:    "region in target",
			config:  ClientConfig{},
			target:  &builder.Target{Region: "us-west-2"},
			wantErr: false,
		},
		{
			name:    "no region anywhere",
			config:  ClientConfig{},
			target:  &builder.Target{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ib := &ImageBuilder{config: tt.config}
			err := ib.validateConfig(tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
