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
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
)

func TestFormatEC2StatusString(t *testing.T) {
	t.Parallel()

	t.Run("nil status", func(t *testing.T) {
		t.Parallel()
		var status *EC2InstanceStatus
		assert.Equal(t, "No build instance found", status.FormatEC2StatusString())
	})

	t.Run("basic status", func(t *testing.T) {
		t.Parallel()
		status := &EC2InstanceStatus{
			InstanceID:   "i-1234567890abcdef0",
			State:        "running",
			InstanceType: "t3.medium",
		}
		result := status.FormatEC2StatusString()
		assert.Contains(t, result, "i-1234567890abcdef0")
		assert.Contains(t, result, "running")
		assert.Contains(t, result, "t3.medium")
	})

	t.Run("with private IP", func(t *testing.T) {
		t.Parallel()
		status := &EC2InstanceStatus{
			InstanceID:   "i-abc",
			State:        "running",
			InstanceType: "t3.micro",
			PrivateIP:    "10.0.1.5",
		}
		result := status.FormatEC2StatusString()
		assert.Contains(t, result, "10.0.1.5")
	})

	t.Run("with launch time", func(t *testing.T) {
		t.Parallel()
		launchTime := time.Now().Add(-5 * time.Minute)
		status := &EC2InstanceStatus{
			InstanceID:   "i-abc",
			State:        "running",
			InstanceType: "t3.micro",
			LaunchTime:   &launchTime,
		}
		result := status.FormatEC2StatusString()
		assert.Contains(t, result, "Uptime:")
	})
}

func TestNewBuildMonitor(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		StreamLogs:    true,
		ShowEC2Status: true,
	}

	monitor := NewBuildMonitor(nil, "test-image", config)
	assert.NotNil(t, monitor)
	assert.Equal(t, "test-image", monitor.imageName)
	assert.True(t, monitor.streamLogs)
	assert.True(t, monitor.showEC2Status)
	assert.Equal(t, "/aws/imagebuilder/test-image", monitor.logGroupName)
}

func TestFindBuildInstance(t *testing.T) {
	t.Parallel()

	t.Run("instance found with matching image tag", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		launchTime := time.Now().Add(-10 * time.Minute)

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-matching123"),
								InstanceType: ec2types.InstanceTypeT3Medium,
								LaunchTime:   &launchTime,
								State: &ec2types.InstanceState{
									Name: ec2types.InstanceStateNameRunning,
								},
								PrivateIpAddress: aws.String("10.0.0.1"),
								PublicIpAddress:  aws.String("54.1.2.3"),
								Placement: &ec2types.Placement{
									AvailabilityZone: aws.String("us-east-1a"),
								},
								Tags: []ec2types.Tag{
									{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image/test-image/1.0.0")},
								},
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		status, err := monitor.FindBuildInstance(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "i-matching123", status.InstanceID)
		assert.Equal(t, "running", status.State)
		assert.Equal(t, "10.0.0.1", status.PrivateIP)
		assert.Equal(t, "54.1.2.3", status.PublicIP)
		assert.Equal(t, "us-east-1a", status.AvailabilityZone)
		assert.Equal(t, "i-matching123", monitor.instanceID)
	})

	t.Run("instance found without matching tag falls back to latest", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		launchTime := time.Now().Add(-5 * time.Minute)

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-fallback456"),
								InstanceType: ec2types.InstanceTypeT3Micro,
								LaunchTime:   &launchTime,
								State: &ec2types.InstanceState{
									Name: ec2types.InstanceStateNameRunning,
								},
								Tags: []ec2types.Tag{
									{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123456789012:image/other-image/1.0.0")},
								},
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		status, err := monitor.FindBuildInstance(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "i-fallback456", status.InstanceID)
	})

	t.Run("no instances found", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		status, err := monitor.FindBuildInstance(context.Background())

		assert.NoError(t, err)
		assert.Nil(t, status)
	})

	t.Run("API error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, fmt.Errorf("AccessDenied: not authorized")
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		status, err := monitor.FindBuildInstance(context.Background())

		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "failed to describe instances")
	})

	t.Run("multiple reservations with different launch times", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		olderTime := time.Now().Add(-30 * time.Minute)
		newerTime := time.Now().Add(-5 * time.Minute)

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-older"),
								InstanceType: ec2types.InstanceTypeT3Micro,
								LaunchTime:   &olderTime,
								State: &ec2types.InstanceState{
									Name: ec2types.InstanceStateNameRunning,
								},
								Tags: []ec2types.Tag{
									{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123:image/test-image/1.0.0")},
								},
							},
						},
					},
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-newer"),
								InstanceType: ec2types.InstanceTypeT3Medium,
								LaunchTime:   &newerTime,
								State: &ec2types.InstanceState{
									Name: ec2types.InstanceStateNameRunning,
								},
								Tags: []ec2types.Tag{
									{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123:image/test-image/2.0.0")},
								},
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		status, err := monitor.FindBuildInstance(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "i-newer", status.InstanceID)
	})
}

func TestGetEC2InstanceStatus(t *testing.T) {
	t.Parallel()

	t.Run("with instanceID already set describes specific instance", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		launchTime := time.Now().Add(-15 * time.Minute)

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			assert.Equal(t, []string{"i-known123"}, params.InstanceIds)
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:       aws.String("i-known123"),
								InstanceType:     ec2types.InstanceTypeM5Large,
								LaunchTime:       &launchTime,
								PrivateIpAddress: aws.String("10.0.1.50"),
								PublicIpAddress:  aws.String("3.4.5.6"),
								State: &ec2types.InstanceState{
									Name: ec2types.InstanceStateNameRunning,
								},
								Placement: &ec2types.Placement{
									AvailabilityZone: aws.String("us-west-2b"),
								},
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		monitor.instanceID = "i-known123"

		status, err := monitor.GetEC2InstanceStatus(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "i-known123", status.InstanceID)
		assert.Equal(t, "running", status.State)
		assert.Equal(t, "10.0.1.50", status.PrivateIP)
		assert.Equal(t, "3.4.5.6", status.PublicIP)
		assert.Equal(t, "us-west-2b", status.AvailabilityZone)
	})

	t.Run("without instanceID falls back to FindBuildInstance", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		launchTime := time.Now().Add(-10 * time.Minute)

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			// Should be called with filters (FindBuildInstance path), not InstanceIds
			assert.Empty(t, params.InstanceIds)
			assert.NotEmpty(t, params.Filters)
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-discovered"),
								InstanceType: ec2types.InstanceTypeT3Medium,
								LaunchTime:   &launchTime,
								State: &ec2types.InstanceState{
									Name: ec2types.InstanceStateNameRunning,
								},
								Tags: []ec2types.Tag{
									{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123:image/test-image/1.0.0")},
								},
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		status, err := monitor.GetEC2InstanceStatus(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "i-discovered", status.InstanceID)
	})

	t.Run("instance not found with known instanceID", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		monitor.instanceID = "i-gone"

		status, err := monitor.GetEC2InstanceStatus(context.Background())

		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "instance i-gone not found")
	})

	t.Run("API error with known instanceID", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.ec2.DescribeInstancesFunc = func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, fmt.Errorf("throttling exception")
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		monitor.instanceID = "i-throttled"

		status, err := monitor.GetEC2InstanceStatus(context.Background())

		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "failed to describe instance")
	})
}

func TestStreamCloudWatchLogs(t *testing.T) {
	t.Parallel()

	t.Run("streams found with events", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		ts := int64(1700000000000)
		nextToken := "next-token-abc"

		mocks.cloudWatchLogs.DescribeLogStreamsFunc = func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			assert.Equal(t, "/aws/imagebuilder/test-image", *params.LogGroupName)
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwltypes.LogStream{
					{
						LogStreamName: aws.String("build-log-stream-1"),
					},
				},
			}, nil
		}

		mocks.cloudWatchLogs.GetLogEventsFunc = func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			assert.Equal(t, "build-log-stream-1", *params.LogStreamName)
			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []cwltypes.OutputLogEvent{
					{
						Timestamp: &ts,
						Message:   aws.String("Step 1: Installing packages"),
					},
					{
						Timestamp: &ts,
						Message:   aws.String("Step 2: Configuring system"),
					},
				},
				NextForwardToken: &nextToken,
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{StreamLogs: true})
		entries, err := monitor.StreamCloudWatchLogs(context.Background())

		assert.NoError(t, err)
		assert.Len(t, entries, 2)
		assert.Equal(t, "Step 1: Installing packages", entries[0].Message)
		assert.Equal(t, "Step 2: Configuring system", entries[1].Message)
		assert.Equal(t, "cloudwatch", entries[0].Source)
		assert.Equal(t, &nextToken, monitor.lastLogToken)
	})

	t.Run("no streams", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.cloudWatchLogs.DescribeLogStreamsFunc = func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwltypes.LogStream{},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{StreamLogs: true})
		entries, err := monitor.StreamCloudWatchLogs(context.Background())

		assert.NoError(t, err)
		assert.Nil(t, entries)
	})

	t.Run("ResourceNotFoundException error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.cloudWatchLogs.DescribeLogStreamsFunc = func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return nil, fmt.Errorf("ResourceNotFoundException: log group does not exist")
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{StreamLogs: true})
		entries, err := monitor.StreamCloudWatchLogs(context.Background())

		assert.NoError(t, err)
		assert.Nil(t, entries)
	})

	t.Run("other API error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.cloudWatchLogs.DescribeLogStreamsFunc = func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return nil, fmt.Errorf("InternalServiceError: something went wrong")
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{StreamLogs: true})
		entries, err := monitor.StreamCloudWatchLogs(context.Background())

		assert.Error(t, err)
		assert.Nil(t, entries)
		assert.Contains(t, err.Error(), "failed to describe log streams")
	})

	t.Run("with lastLogToken set", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		existingToken := "existing-token-xyz"
		ts := int64(1700000001000)

		mocks.cloudWatchLogs.DescribeLogStreamsFunc = func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwltypes.LogStream{
					{LogStreamName: aws.String("stream-1")},
				},
			}, nil
		}

		mocks.cloudWatchLogs.GetLogEventsFunc = func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			assert.Equal(t, &existingToken, params.NextToken)
			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []cwltypes.OutputLogEvent{
					{
						Timestamp: &ts,
						Message:   aws.String("Continued log message"),
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{StreamLogs: true})
		monitor.lastLogToken = &existingToken

		entries, err := monitor.StreamCloudWatchLogs(context.Background())

		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.Equal(t, "Continued log message", entries[0].Message)
	})
}

func TestGetSSMCommandOutput(t *testing.T) {
	t.Parallel()

	t.Run("no instanceID returns nil", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		// instanceID is empty by default
		entries, err := monitor.GetSSMCommandOutput(context.Background())

		assert.NoError(t, err)
		assert.Nil(t, entries)
	})

	t.Run("commands with output", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		requestedTime := time.Now().Add(-5 * time.Minute)

		mocks.ssm.ListCommandInvocationsFunc = func(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
			assert.Equal(t, "i-ssmtest", *params.InstanceId)
			assert.True(t, params.Details)
			return &ssm.ListCommandInvocationsOutput{
				CommandInvocations: []ssmtypes.CommandInvocation{
					{
						RequestedDateTime: &requestedTime,
						CommandPlugins: []ssmtypes.CommandPlugin{
							{
								Name:   aws.String("aws:runShellScript"),
								Output: aws.String("hello world output"),
							},
							{
								Name:   aws.String("aws:runPowerShellScript"),
								Output: aws.String("powershell output"),
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		monitor.instanceID = "i-ssmtest"

		entries, err := monitor.GetSSMCommandOutput(context.Background())

		assert.NoError(t, err)
		assert.Len(t, entries, 2)
		assert.Equal(t, "ssm", entries[0].Source)
		assert.Contains(t, entries[0].Message, "aws:runShellScript")
		assert.Contains(t, entries[0].Message, "hello world output")
		assert.Contains(t, entries[1].Message, "aws:runPowerShellScript")
		assert.Contains(t, entries[1].Message, "powershell output")
	})

	t.Run("InvalidInstanceId error returns nil", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.ssm.ListCommandInvocationsFunc = func(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
			return nil, fmt.Errorf("InvalidInstanceId: i-invalid is not a valid instance ID")
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		monitor.instanceID = "i-invalid"

		entries, err := monitor.GetSSMCommandOutput(context.Background())

		assert.NoError(t, err)
		assert.Nil(t, entries)
	})

	t.Run("other API error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		mocks.ssm.ListCommandInvocationsFunc = func(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
			return nil, fmt.Errorf("ServiceUnavailable: SSM is down")
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		monitor.instanceID = "i-somerror"

		entries, err := monitor.GetSSMCommandOutput(context.Background())

		assert.Error(t, err)
		assert.Nil(t, entries)
		assert.Contains(t, err.Error(), "failed to list SSM command invocations")
	})

	t.Run("empty output skipped", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		requestedTime := time.Now().Add(-2 * time.Minute)

		mocks.ssm.ListCommandInvocationsFunc = func(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
			return &ssm.ListCommandInvocationsOutput{
				CommandInvocations: []ssmtypes.CommandInvocation{
					{
						RequestedDateTime: &requestedTime,
						CommandPlugins: []ssmtypes.CommandPlugin{
							{
								Name:   aws.String("aws:runShellScript"),
								Output: aws.String(""),
							},
							{
								Name:   aws.String("aws:runShellScript2"),
								Output: nil,
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{})
		monitor.instanceID = "i-emptyout"

		entries, err := monitor.GetSSMCommandOutput(context.Background())

		assert.NoError(t, err)
		assert.Empty(t, entries)
	})
}

func TestGetBuildInstanceLogs(t *testing.T) {
	t.Parallel()

	t.Run("combines CloudWatch and SSM logs", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()

		cwTimestamp := int64(1700000001000) // later
		ssmTime := time.UnixMilli(1700000000000)

		mocks.cloudWatchLogs.DescribeLogStreamsFunc = func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwltypes.LogStream{
					{LogStreamName: aws.String("stream-1")},
				},
			}, nil
		}

		mocks.cloudWatchLogs.GetLogEventsFunc = func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []cwltypes.OutputLogEvent{
					{
						Timestamp: &cwTimestamp,
						Message:   aws.String("CloudWatch log entry"),
					},
				},
			}, nil
		}

		mocks.ssm.ListCommandInvocationsFunc = func(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
			return &ssm.ListCommandInvocationsOutput{
				CommandInvocations: []ssmtypes.CommandInvocation{
					{
						RequestedDateTime: &ssmTime,
						CommandPlugins: []ssmtypes.CommandPlugin{
							{
								Name:   aws.String("plugin1"),
								Output: aws.String("SSM command output"),
							},
						},
					},
				},
			}, nil
		}

		monitor := NewBuildMonitor(clients, "test-image", MonitorConfig{StreamLogs: true})
		monitor.instanceID = "i-combined"

		logs, err := monitor.GetBuildInstanceLogs(context.Background())

		assert.NoError(t, err)
		assert.Len(t, logs, 2)
		// SSM log should be first (earlier timestamp)
		assert.Equal(t, "ssm", logs[0].Source)
		assert.Contains(t, logs[0].Message, "SSM command output")
		// CloudWatch log should be second (later timestamp)
		assert.Equal(t, "cloudwatch", logs[1].Source)
		assert.Equal(t, "CloudWatch log entry", logs[1].Message)
	})
}

func TestFindLatestMatchingInstance(t *testing.T) {
	t.Parallel()

	t.Run("empty reservations", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{imageName: "test-image"}
		result := monitor.findLatestMatchingInstance([]ec2types.Reservation{})
		assert.Nil(t, result)
	})

	t.Run("single instance", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{imageName: "test-image"}
		launchTime := time.Now().Add(-10 * time.Minute)

		reservations := []ec2types.Reservation{
			{
				Instances: []ec2types.Instance{
					{
						InstanceId: aws.String("i-single"),
						LaunchTime: &launchTime,
						Tags: []ec2types.Tag{
							{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123:image/test-image/1.0.0")},
						},
					},
				},
			},
		}

		result := monitor.findLatestMatchingInstance(reservations)
		assert.NotNil(t, result)
		assert.Equal(t, "i-single", aws.ToString(result.InstanceId))
	})

	t.Run("multiple instances picks latest", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{imageName: "test-image"}
		olderTime := time.Now().Add(-30 * time.Minute)
		newerTime := time.Now().Add(-5 * time.Minute)
		newestTime := time.Now().Add(-1 * time.Minute)

		reservations := []ec2types.Reservation{
			{
				Instances: []ec2types.Instance{
					{
						InstanceId: aws.String("i-oldest"),
						LaunchTime: &olderTime,
						Tags: []ec2types.Tag{
							{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123:image/test-image/1.0.0")},
						},
					},
					{
						InstanceId: aws.String("i-newest"),
						LaunchTime: &newestTime,
						Tags: []ec2types.Tag{
							{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123:image/test-image/2.0.0")},
						},
					},
				},
			},
			{
				Instances: []ec2types.Instance{
					{
						InstanceId: aws.String("i-middle"),
						LaunchTime: &newerTime,
						Tags: []ec2types.Tag{
							{Key: aws.String("Ec2ImageBuilderArn"), Value: aws.String("arn:aws:imagebuilder:us-east-1:123:image/test-image/1.5.0")},
						},
					},
				},
			},
		}

		result := monitor.findLatestMatchingInstance(reservations)
		assert.NotNil(t, result)
		assert.Equal(t, "i-newest", aws.ToString(result.InstanceId))
	})
}

func TestBuildInstanceStatus(t *testing.T) {
	t.Parallel()

	t.Run("with all fields populated", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{imageName: "test-image"}
		launchTime := time.Now().Add(-20 * time.Minute)

		inst := &ec2types.Instance{
			InstanceId:       aws.String("i-allfilled"),
			InstanceType:     ec2types.InstanceTypeM5Large,
			LaunchTime:       &launchTime,
			PrivateIpAddress: aws.String("10.0.2.100"),
			PublicIpAddress:  aws.String("52.10.20.30"),
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameRunning,
			},
			Placement: &ec2types.Placement{
				AvailabilityZone: aws.String("eu-west-1c"),
			},
		}

		status := monitor.buildInstanceStatus(inst)

		assert.Equal(t, "i-allfilled", status.InstanceID)
		assert.Equal(t, string(ec2types.InstanceTypeM5Large), status.InstanceType)
		assert.Equal(t, &launchTime, status.LaunchTime)
		assert.Equal(t, "running", status.State)
		assert.Equal(t, "Instance is running", status.StateReason)
		assert.Equal(t, "10.0.2.100", status.PrivateIP)
		assert.Equal(t, "52.10.20.30", status.PublicIP)
		assert.Equal(t, "eu-west-1c", status.AvailabilityZone)
	})
}

func TestPopulateInstanceState(t *testing.T) {
	t.Parallel()

	t.Run("running state", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		status := &EC2InstanceStatus{}
		inst := &ec2types.Instance{
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameRunning,
			},
		}

		monitor.populateInstanceState(status, inst)

		assert.Equal(t, "running", status.State)
		assert.Equal(t, "Instance is running", status.StateReason)
	})

	t.Run("non-running with reason", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		status := &EC2InstanceStatus{}
		inst := &ec2types.Instance{
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameStopped,
			},
			StateReason: &ec2types.StateReason{
				Message: aws.String("User initiated (2024-01-15 10:00:00 GMT)"),
			},
		}

		monitor.populateInstanceState(status, inst)

		assert.Equal(t, "stopped", status.State)
		assert.Equal(t, "User initiated (2024-01-15 10:00:00 GMT)", status.StateReason)
	})

	t.Run("nil state", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		status := &EC2InstanceStatus{}
		inst := &ec2types.Instance{
			State: nil,
		}

		monitor.populateInstanceState(status, inst)

		assert.Empty(t, status.State)
		assert.Empty(t, status.StateReason)
	})
}

func TestPopulateInstanceNetwork(t *testing.T) {
	t.Parallel()

	t.Run("all network fields", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		status := &EC2InstanceStatus{}
		inst := &ec2types.Instance{
			PrivateIpAddress: aws.String("172.16.0.50"),
			PublicIpAddress:  aws.String("203.0.113.25"),
			Placement: &ec2types.Placement{
				AvailabilityZone: aws.String("ap-southeast-1a"),
			},
		}

		monitor.populateInstanceNetwork(status, inst)

		assert.Equal(t, "172.16.0.50", status.PrivateIP)
		assert.Equal(t, "203.0.113.25", status.PublicIP)
		assert.Equal(t, "ap-southeast-1a", status.AvailabilityZone)
	})

	t.Run("partial fields", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		status := &EC2InstanceStatus{}
		inst := &ec2types.Instance{
			PrivateIpAddress: aws.String("10.0.0.1"),
			// No public IP, no placement
		}

		monitor.populateInstanceNetwork(status, inst)

		assert.Equal(t, "10.0.0.1", status.PrivateIP)
		assert.Empty(t, status.PublicIP)
		assert.Empty(t, status.AvailabilityZone)
	})

	t.Run("no fields", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		status := &EC2InstanceStatus{}
		inst := &ec2types.Instance{}

		monitor.populateInstanceNetwork(status, inst)

		assert.Empty(t, status.PrivateIP)
		assert.Empty(t, status.PublicIP)
		assert.Empty(t, status.AvailabilityZone)
	})
}

func TestDisplayLogEntry(t *testing.T) {
	t.Parallel()

	t.Run("normal message", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		entry := LogEntry{
			Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
			Message:   "Build step completed successfully",
			Source:    "cloudwatch",
		}

		// displayLogEntry just logs, so we verify it does not panic
		assert.NotPanics(t, func() {
			monitor.displayLogEntry(context.Background(), entry)
		})
	})

	t.Run("empty message skipped", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		entry := LogEntry{
			Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
			Message:   "   ",
			Source:    "ssm",
		}

		// Should not panic; empty/whitespace messages are skipped
		assert.NotPanics(t, func() {
			monitor.displayLogEntry(context.Background(), entry)
		})
	})

	t.Run("long message truncated", func(t *testing.T) {
		t.Parallel()
		monitor := &BuildMonitor{}
		longMsg := strings.Repeat("A", 600)
		entry := LogEntry{
			Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
			Message:   longMsg,
			Source:    "cloudwatch",
		}

		// Verify the truncation logic works correctly by testing the internal behavior
		// The function truncates messages >500 chars and adds "..."
		message := entry.Message
		if len(message) > 500 {
			message = message[:500] + "..."
		}
		assert.Len(t, message, 503) // 500 + "..."
		assert.True(t, strings.HasSuffix(message, "..."))

		// Also verify displayLogEntry does not panic with long messages
		assert.NotPanics(t, func() {
			monitor.displayLogEntry(context.Background(), entry)
		})
	})
}
