package ami

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
)

func TestNewAMIOperations(t *testing.T) {
	t.Parallel()

	clients, _ := newMockAWSClients()
	ops := NewAMIOperations(clients, nil)

	assert.NotNil(t, ops)
	assert.Equal(t, clients, ops.clients)
	assert.Nil(t, ops.globalConfig)
}

func TestShareAMI(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		var capturedInput *ec2.ModifyImageAttributeInput
		mocks.ec2.ModifyImageAttributeFunc = func(ctx context.Context, params *ec2.ModifyImageAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error) {
			capturedInput = params
			return &ec2.ModifyImageAttributeOutput{}, nil
		}

		err := ops.ShareAMI(context.Background(), "ami-12345", []string{"111111111111"})
		assert.NoError(t, err)
		assert.NotNil(t, capturedInput)
		assert.Equal(t, "ami-12345", *capturedInput.ImageId)
		assert.Len(t, capturedInput.LaunchPermission.Add, 1)
		assert.Equal(t, "111111111111", *capturedInput.LaunchPermission.Add[0].UserId)
	})

	t.Run("api_error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.ModifyImageAttributeFunc = func(ctx context.Context, params *ec2.ModifyImageAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error) {
			return nil, fmt.Errorf("access denied")
		}

		err := ops.ShareAMI(context.Background(), "ami-12345", []string{"111111111111"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to share AMI")
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("multiple_accounts", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		var capturedInput *ec2.ModifyImageAttributeInput
		mocks.ec2.ModifyImageAttributeFunc = func(ctx context.Context, params *ec2.ModifyImageAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error) {
			capturedInput = params
			return &ec2.ModifyImageAttributeOutput{}, nil
		}

		accounts := []string{"111111111111", "222222222222", "333333333333"}
		err := ops.ShareAMI(context.Background(), "ami-12345", accounts)
		assert.NoError(t, err)
		assert.Len(t, capturedInput.LaunchPermission.Add, 3)
		for i, acct := range accounts {
			assert.Equal(t, acct, *capturedInput.LaunchPermission.Add[i].UserId)
		}
	})
}

func TestDeregisterAMI(t *testing.T) {
	t.Parallel()

	t.Run("success_without_snapshots", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		deregisterCalled := false
		mocks.ec2.DeregisterImageFunc = func(ctx context.Context, params *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
			deregisterCalled = true
			assert.Equal(t, "ami-12345", *params.ImageId)
			return &ec2.DeregisterImageOutput{}, nil
		}

		err := ops.DeregisterAMI(context.Background(), "ami-12345", false)
		assert.NoError(t, err)
		assert.True(t, deregisterCalled)
	})

	t.Run("success_with_snapshots", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId: aws.String("ami-12345"),
						BlockDeviceMappings: []ec2types.BlockDeviceMapping{
							{
								Ebs: &ec2types.EbsBlockDevice{
									SnapshotId: aws.String("snap-aaa"),
								},
							},
							{
								Ebs: &ec2types.EbsBlockDevice{
									SnapshotId: aws.String("snap-bbb"),
								},
							},
						},
					},
				},
			}, nil
		}

		mocks.ec2.DeregisterImageFunc = func(ctx context.Context, params *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
			return &ec2.DeregisterImageOutput{}, nil
		}

		deletedSnapshots := make([]string, 0)
		mocks.ec2.DeleteSnapshotFunc = func(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
			deletedSnapshots = append(deletedSnapshots, *params.SnapshotId)
			return &ec2.DeleteSnapshotOutput{}, nil
		}

		err := ops.DeregisterAMI(context.Background(), "ami-12345", true)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"snap-aaa", "snap-bbb"}, deletedSnapshots)
	})

	t.Run("describe_error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return nil, fmt.Errorf("describe failed")
		}

		err := ops.DeregisterAMI(context.Background(), "ami-12345", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to describe AMI")
	})

	t.Run("deregister_error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{ImageId: aws.String("ami-12345")},
				},
			}, nil
		}

		mocks.ec2.DeregisterImageFunc = func(ctx context.Context, params *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
			return nil, fmt.Errorf("deregister failed")
		}

		err := ops.DeregisterAMI(context.Background(), "ami-12345", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to deregister AMI")
	})

	t.Run("ami_not_found_when_getting_snapshots", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{},
			}, nil
		}

		err := ops.DeregisterAMI(context.Background(), "ami-12345", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AMI not found")
	})

	t.Run("snapshot_delete_failure_warns_but_succeeds", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId: aws.String("ami-12345"),
						BlockDeviceMappings: []ec2types.BlockDeviceMapping{
							{
								Ebs: &ec2types.EbsBlockDevice{
									SnapshotId: aws.String("snap-fail"),
								},
							},
						},
					},
				},
			}, nil
		}

		mocks.ec2.DeregisterImageFunc = func(ctx context.Context, params *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
			return &ec2.DeregisterImageOutput{}, nil
		}

		mocks.ec2.DeleteSnapshotFunc = func(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
			return nil, fmt.Errorf("snapshot delete denied")
		}

		err := ops.DeregisterAMI(context.Background(), "ami-12345", true)
		assert.NoError(t, err)
	})
}

func TestTagAMI(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		var capturedInput *ec2.CreateTagsInput
		mocks.ec2.CreateTagsFunc = func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
			capturedInput = params
			return &ec2.CreateTagsOutput{}, nil
		}

		tags := map[string]string{"Name": "test-ami", "Env": "dev"}
		err := ops.TagAMI(context.Background(), "ami-12345", tags)
		assert.NoError(t, err)
		assert.NotNil(t, capturedInput)
		assert.Equal(t, []string{"ami-12345"}, capturedInput.Resources)
		assert.Len(t, capturedInput.Tags, 2)
	})

	t.Run("empty_tags_returns_nil", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		err := ops.TagAMI(context.Background(), "ami-12345", map[string]string{})
		assert.NoError(t, err)
	})

	t.Run("api_error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.CreateTagsFunc = func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
			return nil, fmt.Errorf("tag creation failed")
		}

		err := ops.TagAMI(context.Background(), "ami-12345", map[string]string{"Key": "Value"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to tag AMI")
	})
}

func TestGetAMI(t *testing.T) {
	t.Parallel()

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId: aws.String("ami-12345"),
						Name:    aws.String("test-image"),
						State:   ec2types.ImageStateAvailable,
					},
				},
			}, nil
		}

		image, err := ops.GetAMI(context.Background(), "ami-12345")
		assert.NoError(t, err)
		assert.NotNil(t, image)
		assert.Equal(t, "ami-12345", *image.ImageId)
		assert.Equal(t, "test-image", *image.Name)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{},
			}, nil
		}

		image, err := ops.GetAMI(context.Background(), "ami-nonexistent")
		assert.Error(t, err)
		assert.Nil(t, image)
		assert.Contains(t, err.Error(), "AMI not found")
	})

	t.Run("api_error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return nil, fmt.Errorf("api failure")
		}

		image, err := ops.GetAMI(context.Background(), "ami-12345")
		assert.Error(t, err)
		assert.Nil(t, image)
		assert.Contains(t, err.Error(), "failed to describe AMI")
	})
}

func TestListAMIs(t *testing.T) {
	t.Parallel()

	t.Run("no_filters", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			assert.Equal(t, []string{"self"}, params.Owners)
			assert.Nil(t, params.Filters)
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{ImageId: aws.String("ami-111")},
					{ImageId: aws.String("ami-222")},
				},
			}, nil
		}

		images, err := ops.ListAMIs(context.Background(), nil)
		assert.NoError(t, err)
		assert.Len(t, images, 2)
	})

	t.Run("with_filters", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			assert.NotNil(t, params.Filters)
			assert.Len(t, params.Filters, 1)
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{ImageId: aws.String("ami-filtered")},
				},
			}, nil
		}

		filters := map[string]string{"name": "my-ami-*"}
		images, err := ops.ListAMIs(context.Background(), filters)
		assert.NoError(t, err)
		assert.Len(t, images, 1)
		assert.Equal(t, "ami-filtered", *images[0].ImageId)
	})

	t.Run("api_error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ops := NewAMIOperations(clients, nil)

		mocks.ec2.DescribeImagesFunc = func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return nil, fmt.Errorf("list failed")
		}

		images, err := ops.ListAMIs(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, images)
		assert.Contains(t, err.Error(), "failed to list AMIs")
	})
}
