package ami

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	ibtypes "github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// NewResourceManager
// ---------------------------------------------------------------------------

func TestNewResourceManager(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	rm := NewResourceManager(clients)
	assert.NotNil(t, rm)
	assert.Equal(t, clients, rm.clients)
}

// ---------------------------------------------------------------------------
// GetInfrastructureConfig
// ---------------------------------------------------------------------------

func TestGetInfrastructureConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		searchName  string
		setupMock   func(*mockClients)
		wantErr     bool
		errContains string
		wantARN     string
	}{
		{
			name:       "found on first page",
			searchName: "my-infra",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
					return &imagebuilder.ListInfrastructureConfigurationsOutput{
						InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
							{Name: aws.String("my-infra"), Arn: aws.String("arn:aws:imagebuilder:us-east-1:123456789012:infrastructure-configuration/my-infra")},
						},
					}, nil
				}
				mc.imageBuilder.GetInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
					return &imagebuilder.GetInfrastructureConfigurationOutput{
						InfrastructureConfiguration: &ibtypes.InfrastructureConfiguration{
							Arn:  params.InfrastructureConfigurationArn,
							Name: aws.String("my-infra"),
						},
					}, nil
				}
			},
			wantARN: "arn:aws:imagebuilder:us-east-1:123456789012:infrastructure-configuration/my-infra",
		},
		{
			name:       "found on second page via pagination",
			searchName: "my-infra",
			setupMock: func(mc *mockClients) {
				callCount := 0
				mc.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
					callCount++
					if callCount == 1 {
						return &imagebuilder.ListInfrastructureConfigurationsOutput{
							InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
								{Name: aws.String("other-infra"), Arn: aws.String("arn:other")},
							},
							NextToken: aws.String("page2"),
						}, nil
					}
					return &imagebuilder.ListInfrastructureConfigurationsOutput{
						InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
							{Name: aws.String("my-infra"), Arn: aws.String("arn:aws:imagebuilder:us-east-1:123456789012:infrastructure-configuration/my-infra")},
						},
					}, nil
				}
				mc.imageBuilder.GetInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
					return &imagebuilder.GetInfrastructureConfigurationOutput{
						InfrastructureConfiguration: &ibtypes.InfrastructureConfiguration{
							Arn:  params.InfrastructureConfigurationArn,
							Name: aws.String("my-infra"),
						},
					}, nil
				}
			},
			wantARN: "arn:aws:imagebuilder:us-east-1:123456789012:infrastructure-configuration/my-infra",
		},
		{
			name:       "not found",
			searchName: "missing-infra",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
					return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
				}
			},
			wantErr:     true,
			errContains: "resource not found",
		},
		{
			name:       "API error on list",
			searchName: "my-infra",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
					return nil, fmt.Errorf("API unavailable")
				}
			},
			wantErr:     true,
			errContains: "failed to list infrastructure configurations",
		},
		{
			name:       "API error on get",
			searchName: "my-infra",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
					return &imagebuilder.ListInfrastructureConfigurationsOutput{
						InfrastructureConfigurationSummaryList: []ibtypes.InfrastructureConfigurationSummary{
							{Name: aws.String("my-infra"), Arn: aws.String("arn:infra")},
						},
					}, nil
				}
				mc.imageBuilder.GetInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
					return nil, fmt.Errorf("get failed")
				}
			},
			wantErr:     true,
			errContains: "failed to get infrastructure configuration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			result, err := rm.GetInfrastructureConfig(context.Background(), tc.searchName)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				assert.Nil(t, result)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tc.wantARN, aws.ToString(result.Arn))
		})
	}
}

// ---------------------------------------------------------------------------
// GetDistributionConfig
// ---------------------------------------------------------------------------

func TestGetDistributionConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		searchName  string
		setupMock   func(*mockClients)
		wantErr     bool
		errContains string
		wantARN     string
	}{
		{
			name:       "found",
			searchName: "my-dist",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
					return &imagebuilder.ListDistributionConfigurationsOutput{
						DistributionConfigurationSummaryList: []ibtypes.DistributionConfigurationSummary{
							{Name: aws.String("my-dist"), Arn: aws.String("arn:dist")},
						},
					}, nil
				}
				mc.imageBuilder.GetDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.GetDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error) {
					return &imagebuilder.GetDistributionConfigurationOutput{
						DistributionConfiguration: &ibtypes.DistributionConfiguration{
							Arn: params.DistributionConfigurationArn,
						},
					}, nil
				}
			},
			wantARN: "arn:dist",
		},
		{
			name:       "not found",
			searchName: "missing-dist",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
					return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
				}
			},
			wantErr:     true,
			errContains: "resource not found",
		},
		{
			name:       "API error",
			searchName: "my-dist",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
					return nil, fmt.Errorf("API error")
				}
			},
			wantErr:     true,
			errContains: "failed to list distribution configurations",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			result, err := rm.GetDistributionConfig(context.Background(), tc.searchName)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				assert.Nil(t, result)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tc.wantARN, aws.ToString(result.Arn))
		})
	}
}

// ---------------------------------------------------------------------------
// GetImageRecipe
// ---------------------------------------------------------------------------

func TestGetImageRecipe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		searchName  string
		version     string
		setupMock   func(*mockClients)
		wantErr     bool
		errContains string
		wantARN     string
	}{
		{
			name:       "found",
			searchName: "my-recipe",
			version:    "1.0.0",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
					return &imagebuilder.ListImageRecipesOutput{
						ImageRecipeSummaryList: []ibtypes.ImageRecipeSummary{
							{Name: aws.String("my-recipe"), Arn: aws.String("arn:recipe")},
						},
					}, nil
				}
				mc.imageBuilder.GetImageRecipeFunc = func(ctx context.Context, params *imagebuilder.GetImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error) {
					return &imagebuilder.GetImageRecipeOutput{
						ImageRecipe: &ibtypes.ImageRecipe{
							Arn:  params.ImageRecipeArn,
							Name: aws.String("my-recipe"),
						},
					}, nil
				}
			},
			wantARN: "arn:recipe",
		},
		{
			name:       "not found",
			searchName: "missing-recipe",
			version:    "1.0.0",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
					return &imagebuilder.ListImageRecipesOutput{}, nil
				}
			},
			wantErr:     true,
			errContains: "resource not found",
		},
		{
			name:       "API error",
			searchName: "my-recipe",
			version:    "1.0.0",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
					return nil, fmt.Errorf("API error")
				}
			},
			wantErr:     true,
			errContains: "failed to list image recipes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			result, err := rm.GetImageRecipe(context.Background(), tc.searchName, tc.version)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				assert.Nil(t, result)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tc.wantARN, aws.ToString(result.Arn))
		})
	}
}

// ---------------------------------------------------------------------------
// GetImagePipeline
// ---------------------------------------------------------------------------

func TestGetImagePipeline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		searchName  string
		setupMock   func(*mockClients)
		wantErr     bool
		errContains string
		wantARN     string
	}{
		{
			name:       "found",
			searchName: "my-pipeline",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
					return &imagebuilder.ListImagePipelinesOutput{
						ImagePipelineList: []ibtypes.ImagePipeline{
							{Name: aws.String("my-pipeline"), Arn: aws.String("arn:pipeline")},
						},
					}, nil
				}
			},
			wantARN: "arn:pipeline",
		},
		{
			name:       "not found",
			searchName: "missing-pipeline",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
					return &imagebuilder.ListImagePipelinesOutput{}, nil
				}
			},
			wantErr:     true,
			errContains: "resource not found",
		},
		{
			name:       "API error",
			searchName: "my-pipeline",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
					return nil, fmt.Errorf("API error")
				}
			},
			wantErr:     true,
			errContains: "failed to list image pipelines",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			result, err := rm.GetImagePipeline(context.Background(), tc.searchName)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				assert.Nil(t, result)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tc.wantARN, aws.ToString(result.Arn))
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteInfrastructureConfig
// ---------------------------------------------------------------------------

func TestDeleteInfrastructureConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arn       string
		setupMock func(*mockClients)
		wantErr   bool
	}{
		{
			name: "success",
			arn:  "arn:infra",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
					return &imagebuilder.DeleteInfrastructureConfigurationOutput{}, nil
				}
			},
		},
		{
			name: "error",
			arn:  "arn:infra",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
					return nil, fmt.Errorf("delete failed")
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			err := rm.DeleteInfrastructureConfig(context.Background(), tc.arn)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to delete infrastructure configuration")
				return
			}
			assert.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteDistributionConfig
// ---------------------------------------------------------------------------

func TestDeleteDistributionConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arn       string
		setupMock func(*mockClients)
		wantErr   bool
	}{
		{
			name: "success",
			arn:  "arn:dist",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
					return &imagebuilder.DeleteDistributionConfigurationOutput{}, nil
				}
			},
		},
		{
			name: "error",
			arn:  "arn:dist",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
					return nil, fmt.Errorf("delete failed")
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			err := rm.DeleteDistributionConfig(context.Background(), tc.arn)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to delete distribution configuration")
				return
			}
			assert.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteImageRecipe
// ---------------------------------------------------------------------------

func TestDeleteImageRecipe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arn       string
		setupMock func(*mockClients)
		wantErr   bool
	}{
		{
			name: "success",
			arn:  "arn:recipe",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
					return &imagebuilder.DeleteImageRecipeOutput{}, nil
				}
			},
		},
		{
			name: "error",
			arn:  "arn:recipe",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
					return nil, fmt.Errorf("delete failed")
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			err := rm.DeleteImageRecipe(context.Background(), tc.arn)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to delete image recipe")
				return
			}
			assert.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteImagePipeline
// ---------------------------------------------------------------------------

func TestDeleteImagePipeline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arn       string
		setupMock func(*mockClients)
		wantErr   bool
	}{
		{
			name: "success",
			arn:  "arn:pipeline",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
					return &imagebuilder.DeleteImagePipelineOutput{}, nil
				}
			},
		},
		{
			name: "error",
			arn:  "arn:pipeline",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
					return nil, fmt.Errorf("delete failed")
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			err := rm.DeleteImagePipeline(context.Background(), tc.arn)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to delete image pipeline")
				return
			}
			assert.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteComponent
// ---------------------------------------------------------------------------

func TestDeleteComponent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arn       string
		setupMock func(*mockClients)
		wantErr   bool
	}{
		{
			name: "success",
			arn:  "arn:component",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
					return &imagebuilder.DeleteComponentOutput{}, nil
				}
			},
		},
		{
			name: "error",
			arn:  "arn:component",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
					return nil, fmt.Errorf("delete failed")
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			err := rm.DeleteComponent(context.Background(), tc.arn)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to delete component")
				return
			}
			assert.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// GetComponentVersions
// ---------------------------------------------------------------------------

func TestGetComponentVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		compName  string
		setupMock func(*mockClients)
		wantErr   bool
		wantCount int
	}{
		{
			name:     "versions found",
			compName: "my-component",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{
						ComponentVersionList: []ibtypes.ComponentVersion{
							{Name: aws.String("my-component"), Version: aws.String("1.0.0"), Arn: aws.String("arn:comp:1.0.0")},
							{Name: aws.String("my-component"), Version: aws.String("1.0.1"), Arn: aws.String("arn:comp:1.0.1")},
						},
					}, nil
				}
			},
			wantCount: 2,
		},
		{
			name:     "empty",
			compName: "my-component",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{}, nil
				}
			},
			wantCount: 0,
		},
		{
			name:     "error",
			compName: "my-component",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return nil, fmt.Errorf("API error")
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			result, err := rm.GetComponentVersions(context.Background(), tc.compName)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, result, tc.wantCount)
		})
	}
}

// ---------------------------------------------------------------------------
// GetNextComponentVersion
// ---------------------------------------------------------------------------

func TestGetNextComponentVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		compName    string
		baseVersion string
		setupMock   func(*mockClients)
		want        string
		wantErr     bool
	}{
		{
			name:        "no existing versions",
			compName:    "my-component",
			baseVersion: "1.0.0",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{}, nil
				}
			},
			want: "1.0.0",
		},
		{
			name:        "existing versions with matching prefix",
			compName:    "my-component",
			baseVersion: "1.0.0",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{
						ComponentVersionList: []ibtypes.ComponentVersion{
							{Name: aws.String("my-component"), Version: aws.String("1.0.0")},
							{Name: aws.String("my-component"), Version: aws.String("1.0.3")},
							{Name: aws.String("my-component"), Version: aws.String("1.0.1")},
						},
					}, nil
				}
			},
			want: "1.0.4",
		},
		{
			name:        "short base version padded to three parts",
			compName:    "my-component",
			baseVersion: "2",
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{
						ComponentVersionList: []ibtypes.ComponentVersion{
							{Name: aws.String("my-component"), Version: aws.String("2.0.5")},
						},
					}, nil
				}
			},
			want: "2.0.6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			result, err := rm.GetNextComponentVersion(context.Background(), tc.compName, tc.baseVersion)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}
}

// ---------------------------------------------------------------------------
// CleanupOldComponentVersions
// ---------------------------------------------------------------------------

func TestCleanupOldComponentVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		compName       string
		keepCount      int
		setupMock      func(*mockClients)
		wantDeleted    int
		wantNilDeleted bool
		wantErr        bool
	}{
		{
			name:      "fewer than keepCount",
			compName:  "my-component",
			keepCount: 5,
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{
						ComponentVersionList: []ibtypes.ComponentVersion{
							{Name: aws.String("my-component"), Version: aws.String("1.0.0"), Arn: aws.String("arn:comp:1.0.0")},
							{Name: aws.String("my-component"), Version: aws.String("1.0.1"), Arn: aws.String("arn:comp:1.0.1")},
						},
					}, nil
				}
			},
			wantNilDeleted: true,
		},
		{
			name:      "more than keepCount",
			compName:  "my-component",
			keepCount: 1,
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{
						ComponentVersionList: []ibtypes.ComponentVersion{
							{Name: aws.String("my-component"), Version: aws.String("1.0.0"), Arn: aws.String("arn:comp:1.0.0")},
							{Name: aws.String("my-component"), Version: aws.String("1.0.1"), Arn: aws.String("arn:comp:1.0.1")},
							{Name: aws.String("my-component"), Version: aws.String("1.0.2"), Arn: aws.String("arn:comp:1.0.2")},
						},
					}, nil
				}
				mc.imageBuilder.ListComponentBuildVersionsFunc = func(ctx context.Context, params *imagebuilder.ListComponentBuildVersionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentBuildVersionsOutput, error) {
					return &imagebuilder.ListComponentBuildVersionsOutput{
						ComponentSummaryList: []ibtypes.ComponentSummary{
							{Arn: aws.String(*params.ComponentVersionArn + "/build/1")},
						},
					}, nil
				}
				mc.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
					return &imagebuilder.DeleteComponentOutput{}, nil
				}
			},
			wantDeleted: 2,
		},
		{
			name:      "keepCount less than 1 is clamped to 1",
			compName:  "my-component",
			keepCount: 0,
			setupMock: func(mc *mockClients) {
				mc.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
					return &imagebuilder.ListComponentsOutput{
						ComponentVersionList: []ibtypes.ComponentVersion{
							{Name: aws.String("my-component"), Version: aws.String("1.0.0"), Arn: aws.String("arn:comp:1.0.0")},
							{Name: aws.String("my-component"), Version: aws.String("1.0.1"), Arn: aws.String("arn:comp:1.0.1")},
						},
					}, nil
				}
				mc.imageBuilder.ListComponentBuildVersionsFunc = func(ctx context.Context, params *imagebuilder.ListComponentBuildVersionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentBuildVersionsOutput, error) {
					return &imagebuilder.ListComponentBuildVersionsOutput{
						ComponentSummaryList: []ibtypes.ComponentSummary{
							{Arn: aws.String(*params.ComponentVersionArn + "/build/1")},
						},
					}, nil
				}
				mc.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
					return &imagebuilder.DeleteComponentOutput{}, nil
				}
			},
			wantDeleted: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks)
			rm := NewResourceManager(clients)

			deleted, err := rm.CleanupOldComponentVersions(context.Background(), tc.compName, tc.keepCount)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tc.wantNilDeleted {
				assert.Nil(t, deleted)
			} else {
				assert.Len(t, deleted, tc.wantDeleted)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ListComponentsByPrefix
// ---------------------------------------------------------------------------

func TestListComponentsByPrefix(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{Name: aws.String("warpgate-setup"), Version: aws.String("1.0.0")},
				{Name: aws.String("warpgate-cleanup"), Version: aws.String("1.0.0")},
				{Name: aws.String("other-component"), Version: aws.String("1.0.0")},
			},
		}, nil
	}
	rm := NewResourceManager(clients)

	result, err := rm.ListComponentsByPrefix(context.Background(), "warpgate-")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	for _, v := range result {
		assert.Contains(t, aws.ToString(v.Name), "warpgate-")
	}
}

// ---------------------------------------------------------------------------
// IsResourceExistsError
// ---------------------------------------------------------------------------

func TestIsResourceExistsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "already exists message",
			err:  fmt.Errorf("resource already exists"),
			want: true,
		},
		{
			name: "ResourceAlreadyExistsException",
			err:  fmt.Errorf("ResourceAlreadyExistsException: component foo"),
			want: true,
		},
		{
			name: "unrelated error",
			err:  fmt.Errorf("some other error"),
			want: false,
		},
		{
			name: "wrapped already exists",
			err:  fmt.Errorf("creation failed: %w", fmt.Errorf("resource already exists")),
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsResourceExistsError(tc.err))
		})
	}
}

// ---------------------------------------------------------------------------
// CreatedResources.Cleanup
// ---------------------------------------------------------------------------

func TestCreatedResourcesCleanup(t *testing.T) {
	t.Parallel()

	t.Run("with all ARNs set", func(t *testing.T) {
		t.Parallel()

		clients, mocks := newMockAWSClients()
		var deletedARNs []string

		mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
			deletedARNs = append(deletedARNs, aws.ToString(params.ImagePipelineArn))
			return &imagebuilder.DeleteImagePipelineOutput{}, nil
		}
		mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
			deletedARNs = append(deletedARNs, aws.ToString(params.ImageRecipeArn))
			return &imagebuilder.DeleteImageRecipeOutput{}, nil
		}
		mocks.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
			deletedARNs = append(deletedARNs, aws.ToString(params.DistributionConfigurationArn))
			return &imagebuilder.DeleteDistributionConfigurationOutput{}, nil
		}
		mocks.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
			deletedARNs = append(deletedARNs, aws.ToString(params.InfrastructureConfigurationArn))
			return &imagebuilder.DeleteInfrastructureConfigurationOutput{}, nil
		}
		mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
			deletedARNs = append(deletedARNs, aws.ToString(params.ComponentBuildVersionArn))
			return &imagebuilder.DeleteComponentOutput{}, nil
		}

		rm := NewResourceManager(clients)
		cr := &CreatedResources{
			PipelineARN:   "arn:pipeline:1",
			RecipeARN:     "arn:recipe:1",
			DistARN:       "arn:dist:1",
			InfraARN:      "arn:infra:1",
			ComponentARNs: []string{"arn:comp:1", "arn:comp:2"},
		}

		cr.Cleanup(context.Background(), rm)

		assert.Len(t, deletedARNs, 6)
		assert.Equal(t, "arn:pipeline:1", deletedARNs[0])
		assert.Equal(t, "arn:recipe:1", deletedARNs[1])
		assert.Equal(t, "arn:dist:1", deletedARNs[2])
		assert.Equal(t, "arn:infra:1", deletedARNs[3])
		assert.Equal(t, "arn:comp:1", deletedARNs[4])
		assert.Equal(t, "arn:comp:2", deletedARNs[5])
	})

	t.Run("with some empty ARNs", func(t *testing.T) {
		t.Parallel()

		clients, mocks := newMockAWSClients()
		var deletedARNs []string

		mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
			deletedARNs = append(deletedARNs, aws.ToString(params.ImageRecipeArn))
			return &imagebuilder.DeleteImageRecipeOutput{}, nil
		}
		mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
			deletedARNs = append(deletedARNs, aws.ToString(params.ComponentBuildVersionArn))
			return &imagebuilder.DeleteComponentOutput{}, nil
		}

		rm := NewResourceManager(clients)
		cr := &CreatedResources{
			PipelineARN:   "",
			RecipeARN:     "arn:recipe:1",
			DistARN:       "",
			InfraARN:      "",
			ComponentARNs: []string{"arn:comp:1"},
		}

		cr.Cleanup(context.Background(), rm)

		assert.Len(t, deletedARNs, 2)
		assert.Equal(t, "arn:recipe:1", deletedARNs[0])
		assert.Equal(t, "arn:comp:1", deletedARNs[1])
	})
}

// ---------------------------------------------------------------------------
// ResourceExistsError
// ---------------------------------------------------------------------------

func TestResourceExistsError(t *testing.T) {
	t.Parallel()

	e := &ResourceExistsError{
		ResourceType: "component",
		ResourceName: "my-component",
		ResourceARN:  "arn:comp:1",
	}
	assert.Contains(t, e.Error(), "component")
	assert.Contains(t, e.Error(), "my-component")
	assert.Contains(t, e.Error(), "arn:comp:1")
	assert.Contains(t, e.Error(), "already exists")
}

// ---------------------------------------------------------------------------
// ErrNotFound sentinel
// ---------------------------------------------------------------------------

func TestErrNotFoundSentinel(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("infrastructure configuration %q: %w", "test", ErrNotFound)
	assert.True(t, errors.Is(err, ErrNotFound))
}
