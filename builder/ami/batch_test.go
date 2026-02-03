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
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	ibtypes "github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/stretchr/testify/assert"
)

func TestNewBatchOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		clients *AWSClients
	}{
		{
			name:    "nil clients",
			clients: nil,
		},
		{
			name:    "empty clients",
			clients: &AWSClients{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bo := NewBatchOperations(tt.clients)
			assert.NotNil(t, bo)
			assert.Equal(t, tt.clients, bo.clients)
		})
	}
}

func TestBatchTagResources_EmptyInput(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(nil)

	tests := []struct {
		name        string
		resourceIDs []string
		tags        map[string]string
		wantErr     bool
	}{
		{
			name:        "empty resource IDs",
			resourceIDs: []string{},
			tags:        map[string]string{"key": "value"},
			wantErr:     false,
		},
		{
			name:        "empty tags",
			resourceIDs: []string{"i-12345"},
			tags:        map[string]string{},
			wantErr:     false,
		},
		{
			name:        "both empty",
			resourceIDs: []string{},
			tags:        map[string]string{},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := bo.BatchTagResources(context.Background(), tt.resourceIDs, tt.tags)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBatchDeleteComponents_EmptyInput(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(nil)
	err := bo.BatchDeleteComponents(context.Background(), []string{})
	assert.NoError(t, err)
}

func TestBatchDescribeImages_EmptyInput(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(nil)
	images, err := bo.BatchDescribeImages(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Nil(t, images)
}

func TestBatchGetComponentVersions_EmptyInput(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(nil)
	versions, err := bo.BatchGetComponentVersions(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Nil(t, versions)
}

func TestBatchCheckResourceExistence_EmptyInput(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(nil)
	results := bo.BatchCheckResourceExistence(context.Background(), []ResourceCheck{})
	assert.NotNil(t, results)
	assert.Empty(t, results)
}

func TestResourceCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		check    ResourceCheck
		wantType string
		wantName string
	}{
		{
			name:     "pipeline resource check",
			check:    ResourceCheck{Type: "pipeline", Name: "test-pipeline"},
			wantType: "pipeline",
			wantName: "test-pipeline",
		},
		{
			name:     "recipe resource check",
			check:    ResourceCheck{Type: "recipe", Name: "test-recipe"},
			wantType: "recipe",
			wantName: "test-recipe",
		},
		{
			name:     "infra resource check",
			check:    ResourceCheck{Type: "infra", Name: "test-infra"},
			wantType: "infra",
			wantName: "test-infra",
		},
		{
			name:     "dist resource check",
			check:    ResourceCheck{Type: "dist", Name: "test-dist"},
			wantType: "dist",
			wantName: "test-dist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantType, tt.check.Type)
			assert.Equal(t, tt.wantName, tt.check.Name)
		})
	}
}

func TestBatchDeleteComponents_CancelledContext(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(&AWSClients{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := bo.BatchDeleteComponents(ctx, []string{"arn:aws:imagebuilder:us-east-1:123456789012:component/test/1.0.0/1"})
	assert.Error(t, err)
}

func TestBatchGetComponentVersions_CancelledContext(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(&AWSClients{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := bo.BatchGetComponentVersions(ctx, []string{"test-component"})
	assert.Error(t, err)
}

func TestBatchCheckResourceExistence_CancelledContext(t *testing.T) {
	t.Parallel()

	bo := NewBatchOperations(&AWSClients{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	checks := []ResourceCheck{
		{Type: "pipeline", Name: "test"},
	}

	// Should return empty results since context is cancelled
	results := bo.BatchCheckResourceExistence(ctx, checks)
	assert.NotNil(t, results)
}

func TestBatchOpsInterfaceCompliance(t *testing.T) {
	t.Parallel()

	// Compile-time check that BatchOperations implements BatchOps
	var _ BatchOps = (*BatchOperations)(nil)

	// Runtime check - verify assignment works
	bo := NewBatchOperations(nil)
	var ops BatchOps = bo
	_ = ops
}

func TestErrNotFound(t *testing.T) {
	t.Parallel()

	// Test that ErrNotFound can be wrapped and unwrapped correctly
	wrappedErr := errors.New("resource 'test': " + ErrNotFound.Error())
	assert.False(t, errors.Is(wrappedErr, ErrNotFound), "String concatenation should not create wrapped error")

	// Test proper wrapping
	properlyWrapped := errors.Join(errors.New("context"), ErrNotFound)
	assert.True(t, errors.Is(properlyWrapped, ErrNotFound), "errors.Join should preserve ErrNotFound for errors.Is")
}

// --- BatchTagResources with mock tests ---

func TestBatchTagResources_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	var capturedInput *ec2.CreateTagsInput
	mocks.ec2.CreateTagsFunc = func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
		capturedInput = params
		return &ec2.CreateTagsOutput{}, nil
	}

	resourceIDs := []string{"i-111", "i-222", "i-333"}
	tags := map[string]string{
		"warpgate:name": "my-build",
		"env":           "test",
	}

	err := bo.BatchTagResources(context.Background(), resourceIDs, tags)
	assert.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.Equal(t, resourceIDs, capturedInput.Resources)
	assert.Len(t, capturedInput.Tags, 2)
}

func TestBatchTagResources_APIError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	mocks.ec2.CreateTagsFunc = func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
		return nil, fmt.Errorf("unauthorized")
	}

	err := bo.BatchTagResources(context.Background(), []string{"i-111"}, map[string]string{"key": "val"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to batch tag resources")
}

// --- BatchDeleteComponents with mock tests ---

func TestBatchDeleteComponents_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	var deletedARNs []string
	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		deletedARNs = append(deletedARNs, aws.ToString(params.ComponentBuildVersionArn))
		return &imagebuilder.DeleteComponentOutput{}, nil
	}

	arns := []string{
		"arn:aws:imagebuilder:us-east-1:123:component/comp-a/1.0.0/1",
		"arn:aws:imagebuilder:us-east-1:123:component/comp-b/1.0.0/1",
	}

	err := bo.BatchDeleteComponents(context.Background(), arns)
	assert.NoError(t, err)
	assert.Len(t, deletedARNs, 2)
}

func TestBatchDeleteComponents_PartialFailure(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		if aws.ToString(params.ComponentBuildVersionArn) == "arn:bad" {
			return nil, fmt.Errorf("component in use")
		}
		return &imagebuilder.DeleteComponentOutput{}, nil
	}

	arns := []string{"arn:good-1", "arn:bad", "arn:good-2"}

	err := bo.BatchDeleteComponents(context.Background(), arns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete component arn:bad")
}

// --- BatchDescribeImages with mock tests ---

func TestBatchDescribeImages_SingleBatch(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		images := make([]ec2types.Image, len(params.ImageIds))
		for i, id := range params.ImageIds {
			images[i] = ec2types.Image{
				ImageId: aws.String(id),
				State:   ec2types.ImageStateAvailable,
			}
		}
		return &ec2.DescribeImagesOutput{Images: images}, nil
	}

	imageIDs := []string{"ami-111", "ami-222", "ami-333"}
	images, err := bo.BatchDescribeImages(context.Background(), imageIDs)
	assert.NoError(t, err)
	assert.Len(t, images, 3)
	assert.Equal(t, "ami-111", aws.ToString(images[0].ImageId))
}

func TestBatchDescribeImages_MultipleBatches(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	callCount := 0
	mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		callCount++
		images := make([]ec2types.Image, len(params.ImageIds))
		for i, id := range params.ImageIds {
			images[i] = ec2types.Image{ImageId: aws.String(id)}
		}
		return &ec2.DescribeImagesOutput{Images: images}, nil
	}

	// Create 150 image IDs to force 2 batches (max 100 per batch)
	imageIDs := make([]string, 150)
	for i := 0; i < 150; i++ {
		imageIDs[i] = fmt.Sprintf("ami-%05d", i)
	}

	images, err := bo.BatchDescribeImages(context.Background(), imageIDs)
	assert.NoError(t, err)
	assert.Len(t, images, 150)
	assert.Equal(t, 2, callCount)
}

func TestBatchDescribeImages_PartialError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	callCount := 0
	mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
		callCount++
		if callCount == 2 {
			return nil, fmt.Errorf("rate limit exceeded")
		}
		images := make([]ec2types.Image, len(params.ImageIds))
		for i, id := range params.ImageIds {
			images[i] = ec2types.Image{ImageId: aws.String(id)}
		}
		return &ec2.DescribeImagesOutput{Images: images}, nil
	}

	// Create 150 IDs so we get 2 batches, second one fails
	imageIDs := make([]string, 150)
	for i := 0; i < 150; i++ {
		imageIDs[i] = fmt.Sprintf("ami-%05d", i)
	}

	images, err := bo.BatchDescribeImages(context.Background(), imageIDs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to describe images batch")
	// The first batch of 100 should have been returned
	assert.Len(t, images, 100)
}

// --- BatchGetComponentVersions with mock tests ---

func TestBatchGetComponentVersions_Success(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		// Determine which component we are listing by the filter
		var name string
		for _, f := range params.Filters {
			if aws.ToString(f.Name) == "name" && len(f.Values) > 0 {
				name = f.Values[0]
			}
		}
		switch name {
		case "comp-a":
			return &imagebuilder.ListComponentsOutput{
				ComponentVersionList: []ibtypes.ComponentVersion{
					{Name: aws.String("comp-a"), Version: aws.String("1.0.0")},
					{Name: aws.String("comp-a"), Version: aws.String("2.0.0")},
				},
			}, nil
		case "comp-b":
			return &imagebuilder.ListComponentsOutput{
				ComponentVersionList: []ibtypes.ComponentVersion{
					{Name: aws.String("comp-b"), Version: aws.String("3.0.0")},
				},
			}, nil
		default:
			return &imagebuilder.ListComponentsOutput{}, nil
		}
	}

	versions, err := bo.BatchGetComponentVersions(context.Background(), []string{"comp-a", "comp-b"})
	assert.NoError(t, err)
	assert.Len(t, versions, 2)
	assert.Equal(t, []string{"1.0.0", "2.0.0"}, versions["comp-a"])
	assert.Equal(t, []string{"3.0.0"}, versions["comp-b"])
}

func TestBatchGetComponentVersions_ErrorFromOneComponent(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		var name string
		for _, f := range params.Filters {
			if aws.ToString(f.Name) == "name" && len(f.Values) > 0 {
				name = f.Values[0]
			}
		}
		if name == "bad-comp" {
			return nil, fmt.Errorf("access denied for bad-comp")
		}
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{Name: aws.String(name), Version: aws.String("1.0.0")},
			},
		}, nil
	}

	versions, err := bo.BatchGetComponentVersions(context.Background(), []string{"good-comp", "bad-comp"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list components for bad-comp")
	// The good component might still have results (depending on goroutine ordering)
	// but the error is propagated via errgroup
	assert.NotNil(t, versions)
}

// --- BatchCheckResourceExistence with mock tests ---

func TestBatchCheckResourceExistence_WithMocks(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	bo := NewBatchOperations(clients)

	// Pipeline exists
	mocks.imageBuilder.ListImagePipelinesFunc = func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
		return &imagebuilder.ListImagePipelinesOutput{
			ImagePipelineList: []ibtypes.ImagePipeline{
				{
					Name: aws.String("my-build-pipeline"),
					Arn:  aws.String("arn:aws:imagebuilder:us-east-1:123:image-pipeline/my-build-pipeline"),
				},
			},
		}, nil
	}

	// Recipe does not exist (empty list)
	mocks.imageBuilder.ListImageRecipesFunc = func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
		return &imagebuilder.ListImageRecipesOutput{}, nil
	}

	// Infra does not exist
	mocks.imageBuilder.ListInfrastructureConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
		return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
	}

	// Dist does not exist
	mocks.imageBuilder.ListDistributionConfigurationsFunc = func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
		return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
	}

	checks := []ResourceCheck{
		{Type: "pipeline", Name: "my-build-pipeline"},
		{Type: "recipe", Name: "nonexistent-recipe"},
		{Type: "unknown", Name: "unknown-type"},
	}

	results := bo.BatchCheckResourceExistence(context.Background(), checks)
	assert.NotNil(t, results)
	// Unknown type should return false
	assert.False(t, results["unknown-type"])
}
