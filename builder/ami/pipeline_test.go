/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPipelineManager(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)
	assert.NotNil(t, pm)
	assert.Equal(t, clients, pm.clients)
	assert.Nil(t, pm.monitor)
}

func TestNewPipelineManagerWithMonitor(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	cfg := MonitorConfig{StreamLogs: true, ShowEC2Status: true}
	pm := NewPipelineManagerWithMonitor(clients, cfg)
	assert.NotNil(t, pm)
	assert.Equal(t, clients, pm.clients)
	assert.True(t, pm.monitorConfig.StreamLogs)
	assert.True(t, pm.monitorConfig.ShowEC2Status)
}

func TestSetMonitorConfig(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)
	cfg := MonitorConfig{StreamLogs: true}
	pm.SetMonitorConfig(cfg)
	assert.True(t, pm.monitorConfig.StreamLogs)
}

func TestCreatePipeline(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		expectedARN := "arn:aws:imagebuilder:us-east-1:123456:image-pipeline/test"
		mocks.imageBuilder.CreateImagePipelineFunc = func(ctx context.Context, params *imagebuilder.CreateImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImagePipelineOutput, error) {
			return &imagebuilder.CreateImagePipelineOutput{
				ImagePipelineArn: aws.String(expectedARN),
			}, nil
		}

		pm := NewPipelineManager(clients)
		arn, err := pm.CreatePipeline(context.Background(), PipelineConfig{
			Name:           "test-pipeline",
			Description:    "test",
			ImageRecipeARN: "arn:recipe",
			InfraConfigARN: "arn:infra",
			DistConfigARN:  "arn:dist",
			Tags:           map[string]string{"env": "test"},
		})

		require.NoError(t, err)
		assert.Equal(t, expectedARN, *arn)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.CreateImagePipelineFunc = func(ctx context.Context, params *imagebuilder.CreateImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImagePipelineOutput, error) {
			return nil, fmt.Errorf("access denied")
		}

		pm := NewPipelineManager(clients)
		arn, err := pm.CreatePipeline(context.Background(), PipelineConfig{Name: "test"})

		assert.Error(t, err)
		assert.Nil(t, arn)
		assert.Contains(t, err.Error(), "failed to create pipeline")
	})
}

func TestStartPipeline(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		expectedARN := "arn:aws:imagebuilder:us-east-1:123456:image/test/1.0.0/1"
		mocks.imageBuilder.StartImagePipelineExecutionFunc = func(ctx context.Context, params *imagebuilder.StartImagePipelineExecutionInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.StartImagePipelineExecutionOutput, error) {
			return &imagebuilder.StartImagePipelineExecutionOutput{
				ImageBuildVersionArn: aws.String(expectedARN),
			}, nil
		}

		pm := NewPipelineManager(clients)
		arn, err := pm.StartPipeline(context.Background(), "arn:pipeline")

		require.NoError(t, err)
		assert.Equal(t, expectedARN, *arn)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.StartImagePipelineExecutionFunc = func(ctx context.Context, params *imagebuilder.StartImagePipelineExecutionInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.StartImagePipelineExecutionOutput, error) {
			return nil, fmt.Errorf("pipeline not found")
		}

		pm := NewPipelineManager(clients)
		arn, err := pm.StartPipeline(context.Background(), "arn:pipeline")

		assert.Error(t, err)
		assert.Nil(t, arn)
		assert.Contains(t, err.Error(), "failed to start pipeline")
	})
}

func TestGetPipeline(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.GetImagePipelineFunc = func(ctx context.Context, params *imagebuilder.GetImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImagePipelineOutput, error) {
			return &imagebuilder.GetImagePipelineOutput{
				ImagePipeline: &types.ImagePipeline{
					Name: aws.String("test-pipeline"),
				},
			}, nil
		}

		pm := NewPipelineManager(clients)
		pipeline, err := pm.GetPipeline(context.Background(), "arn:pipeline")

		require.NoError(t, err)
		assert.Equal(t, "test-pipeline", *pipeline.Name)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.GetImagePipelineFunc = func(ctx context.Context, params *imagebuilder.GetImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImagePipelineOutput, error) {
			return nil, fmt.Errorf("not found")
		}

		pm := NewPipelineManager(clients)
		pipeline, err := pm.GetPipeline(context.Background(), "arn:pipeline")

		assert.Error(t, err)
		assert.Nil(t, pipeline)
		assert.Contains(t, err.Error(), "failed to get pipeline")
	})
}

func TestDeletePipeline(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		var capturedARN string
		mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
			capturedARN = *params.ImagePipelineArn
			return &imagebuilder.DeleteImagePipelineOutput{}, nil
		}

		pm := NewPipelineManager(clients)
		err := pm.DeletePipeline(context.Background(), "arn:pipeline/test")

		require.NoError(t, err)
		assert.Equal(t, "arn:pipeline/test", capturedARN)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.DeleteImagePipelineFunc = func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
			return nil, fmt.Errorf("delete failed")
		}

		pm := NewPipelineManager(clients)
		err := pm.DeletePipeline(context.Background(), "arn:pipeline/test")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete pipeline")
	})
}

func TestCleanupResources(t *testing.T) {
	t.Parallel()

	t.Run("all ARNs present", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		var deletedRecipe, deletedInfra, deletedDist bool
		mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
			deletedRecipe = true
			return &imagebuilder.DeleteImageRecipeOutput{}, nil
		}
		mocks.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
			deletedInfra = true
			return &imagebuilder.DeleteInfrastructureConfigurationOutput{}, nil
		}
		mocks.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
			deletedDist = true
			return &imagebuilder.DeleteDistributionConfigurationOutput{}, nil
		}

		pm := NewPipelineManager(clients)
		err := pm.CleanupResources(context.Background(), "arn:recipe", "arn:infra", "arn:dist")

		require.NoError(t, err)
		assert.True(t, deletedRecipe)
		assert.True(t, deletedInfra)
		assert.True(t, deletedDist)
	})

	t.Run("all empty ARNs skips deletion", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		pm := NewPipelineManager(clients)
		err := pm.CleanupResources(context.Background(), "", "", "")
		require.NoError(t, err)
	})

	t.Run("some empty ARNs", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		var deletedRecipe bool
		mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
			deletedRecipe = true
			return &imagebuilder.DeleteImageRecipeOutput{}, nil
		}

		pm := NewPipelineManager(clients)
		err := pm.CleanupResources(context.Background(), "arn:recipe", "", "")

		require.NoError(t, err)
		assert.True(t, deletedRecipe)
	})

	t.Run("partial failures do not return error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
			return nil, fmt.Errorf("recipe delete failed")
		}
		mocks.imageBuilder.DeleteInfrastructureConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
			return nil, fmt.Errorf("infra delete failed")
		}
		mocks.imageBuilder.DeleteDistributionConfigurationFunc = func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
			return nil, fmt.Errorf("dist delete failed")
		}

		pm := NewPipelineManager(clients)
		err := pm.CleanupResources(context.Background(), "arn:recipe", "arn:infra", "arn:dist")

		// CleanupResources logs warnings but always returns nil
		require.NoError(t, err)
	})
}

func TestEstimateRemainingTime(t *testing.T) {
	t.Parallel()

	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)

	tests := []struct {
		name          string
		status        types.ImageStatus
		elapsed       time.Duration
		expectNonZero bool
	}{
		{"pending stage", types.ImageStatusPending, 1 * time.Minute, true},
		{"creating stage", types.ImageStatusCreating, 2 * time.Minute, true},
		{"building stage", types.ImageStatusBuilding, 5 * time.Minute, true},
		{"testing stage", types.ImageStatusTesting, 1 * time.Minute, true},
		{"distributing stage", types.ImageStatusDistributing, 3 * time.Minute, true},
		{"integrating stage", types.ImageStatusIntegrating, 1 * time.Minute, true},
		{"unknown stage returns zero", types.ImageStatus("UNKNOWN"), 5 * time.Minute, false},
		{"available stage returns zero", types.ImageStatusAvailable, 30 * time.Minute, false},
		{"elapsed exceeds typical still returns nonzero", types.ImageStatusBuilding, 60 * time.Minute, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			remaining := pm.estimateRemainingTime(tt.status, tt.elapsed)
			if tt.expectNonZero {
				assert.Greater(t, remaining, time.Duration(0), "expected nonzero remaining time")
			} else {
				assert.Equal(t, time.Duration(0), remaining)
			}
		})
	}
}

func TestFormatBuildStage(t *testing.T) {
	t.Parallel()

	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)

	tests := []struct {
		status   types.ImageStatus
		contains string
	}{
		{types.ImageStatusPending, "PENDING"},
		{types.ImageStatusCreating, "CREATING"},
		{types.ImageStatusBuilding, "BUILDING"},
		{types.ImageStatusTesting, "TESTING"},
		{types.ImageStatusDistributing, "DISTRIBUTING"},
		{types.ImageStatusIntegrating, "INTEGRATING"},
		{types.ImageStatusAvailable, "AVAILABLE"},
		{types.ImageStatusFailed, "FAILED"},
		{types.ImageStatusCancelled, "CANCELLED"},
		{types.ImageStatus("CUSTOM"), "CUSTOM"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			result := pm.formatBuildStage(tt.status)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestBuildFailureError(t *testing.T) {
	t.Parallel()

	t.Run("nil details", func(t *testing.T) {
		t.Parallel()
		err := &BuildFailureError{
			Status:   "FAILED",
			Duration: "10m",
		}
		msg := err.Error()
		assert.Contains(t, msg, "FAILED")
		assert.Contains(t, msg, "10m")
	})

	t.Run("full details with workflow logs", func(t *testing.T) {
		t.Parallel()
		err := &BuildFailureError{
			Status:   "FAILED",
			Duration: "25m",
			Details: &FailureDetails{
				Reason:          "Script execution failed",
				FailedStep:      "RunBuildScript",
				FailedComponent: "my-component",
				ErrorMessage:    "exit code 1",
				LogsURL:         "https://console.aws.amazon.com/cloudwatch/test",
				WorkflowStepLogs: []WorkflowStepLog{
					{StepName: "step-1", Status: "COMPLETED"},
					{StepName: "step-2", Status: "FAILED", Message: "script error"},
				},
			},
			Remediation: "Check your script",
		}
		msg := err.Error()
		assert.Contains(t, msg, "Script execution failed")
		assert.Contains(t, msg, "RunBuildScript")
		assert.Contains(t, msg, "my-component")
		assert.Contains(t, msg, "exit code 1")
		assert.Contains(t, msg, "cloudwatch")
		assert.Contains(t, msg, "step-1: COMPLETED")
		assert.Contains(t, msg, "step-2: FAILED")
		assert.Contains(t, msg, "script error")
		assert.Contains(t, msg, "Check your script")
	})

	t.Run("details with empty fields", func(t *testing.T) {
		t.Parallel()
		err := &BuildFailureError{
			Status:   "FAILED",
			Duration: "5m",
			Details:  &FailureDetails{},
		}
		msg := err.Error()
		assert.Contains(t, msg, "FAILED")
		assert.NotContains(t, msg, "Reason:")
		assert.NotContains(t, msg, "Failed Step:")
	})
}

func TestGetFailureDetails(t *testing.T) {
	t.Parallel()

	t.Run("image with state reason and name", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{}, nil
		}

		pm := NewPipelineManager(clients)
		image := &types.Image{
			State: &types.ImageState{
				Reason: aws.String("Build timed out"),
			},
			Name: aws.String("test-image"),
		}

		details := pm.getFailureDetails(context.Background(), image, "arn:image")

		assert.Equal(t, "Build timed out", details.Reason)
		assert.Contains(t, details.LogsURL, "us-east-1")
		assert.Contains(t, details.LogsURL, "test-image")
	})

	t.Run("image with workflow executions", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		startTime := "2025-01-01T00:00:00Z"
		endTime := "2025-01-01T00:10:00Z"
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{
				WorkflowExecutions: []types.WorkflowExecutionMetadata{
					{
						WorkflowExecutionId:     aws.String("exec-1"),
						WorkflowBuildVersionArn: aws.String("arn:workflow/build"),
						Status:                  "FAILED",
						Message:                 aws.String("build failed"),
						StartTime:               &startTime,
						EndTime:                 &endTime,
					},
				},
			}, nil
		}
		mocks.imageBuilder.ListWorkflowStepExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowStepExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowStepExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowStepExecutionsOutput{
				Steps: []types.WorkflowStepMetadata{
					{
						Name:      aws.String("RunScript"),
						Status:    "FAILED",
						Message:   aws.String("exit code 1"),
						StartTime: &startTime,
						EndTime:   &endTime,
					},
				},
			}, nil
		}

		pm := NewPipelineManager(clients)
		image := &types.Image{
			State: &types.ImageState{},
		}

		details := pm.getFailureDetails(context.Background(), image, "arn:image")

		// The first FAILED entry found is the workflow execution itself
		assert.Equal(t, "arn:workflow/build", details.FailedStep)
		assert.Equal(t, "build failed", details.ErrorMessage)
		assert.Len(t, details.WorkflowStepLogs, 2) // 1 exec + 1 step
	})

	t.Run("image with recipe components", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{}, nil
		}

		pm := NewPipelineManager(clients)
		image := &types.Image{
			State: &types.ImageState{},
			ImageRecipe: &types.ImageRecipe{
				Components: []types.ComponentConfiguration{
					{ComponentArn: aws.String("arn:aws:imagebuilder:us-east-1:123456:component/my-component/1.0.0")},
				},
			},
		}

		details := pm.getFailureDetails(context.Background(), image, "arn:image")

		assert.Equal(t, "1.0.0", details.FailedComponent)
	})

	t.Run("workflow execution error is handled gracefully", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return nil, fmt.Errorf("API error")
		}

		pm := NewPipelineManager(clients)
		image := &types.Image{
			State: &types.ImageState{
				Reason: aws.String("failed"),
			},
		}

		details := pm.getFailureDetails(context.Background(), image, "arn:image")

		assert.Equal(t, "failed", details.Reason)
		assert.Empty(t, details.WorkflowStepLogs)
	})
}

func TestGetWorkflowExecutionLogs(t *testing.T) {
	t.Parallel()

	t.Run("nil workflow execution ID skipped", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{
				WorkflowExecutions: []types.WorkflowExecutionMetadata{
					{WorkflowExecutionId: nil},
				},
			}, nil
		}

		pm := NewPipelineManager(clients)
		logs := pm.getWorkflowExecutionLogs(context.Background(), "arn:image")
		assert.Empty(t, logs)
	})

	t.Run("step execution error handled gracefully", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{
				WorkflowExecutions: []types.WorkflowExecutionMetadata{
					{
						WorkflowExecutionId:     aws.String("exec-1"),
						WorkflowBuildVersionArn: aws.String("arn:workflow"),
						Status:                  "FAILED",
					},
				},
			}, nil
		}
		mocks.imageBuilder.ListWorkflowStepExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowStepExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowStepExecutionsOutput, error) {
			return nil, fmt.Errorf("step API error")
		}

		pm := NewPipelineManager(clients)
		logs := pm.getWorkflowExecutionLogs(context.Background(), "arn:image")
		// Should still have the workflow execution entry
		assert.Len(t, logs, 1)
		assert.Equal(t, "FAILED", logs[0].Status)
	})
}

func TestAddRemediationHints(t *testing.T) {
	t.Parallel()

	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)

	tests := []struct {
		name           string
		reason         string
		errorMessage   string
		expectContains string
	}{
		{"timeout hint", "Build timeout occurred", "", "timed out"},
		{"script failed hint", "script execution failed", "", "provisioner script failed"},
		{"network hint", "network connectivity error", "", "Network connectivity"},
		{"connection hint", "connection refused", "", "Network connectivity"},
		{"permission hint", "permission denied", "", "Permission denied"},
		{"access denied hint", "access denied to resource", "", "Permission denied"},
		{"disk space hint", "disk space full", "", "Disk space"},
		{"ami not found hint", "ami not found in region", "", "AMI not found"},
		{"component hint", "component execution error", "", "Component execution"},
		{"no match returns unmodified", "something random", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			details := &FailureDetails{
				Reason:       tt.reason,
				ErrorMessage: tt.errorMessage,
			}
			result := pm.addRemediationHints(details)
			if tt.expectContains != "" {
				assert.Contains(t, result.ErrorMessage, tt.expectContains)
			} else {
				assert.Equal(t, tt.errorMessage, result.ErrorMessage)
			}
		})
	}
}

func TestHandlePipelineStatus(t *testing.T) {
	t.Parallel()

	t.Run("available returns image", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{}, nil
		}
		pm := NewPipelineManager(clients)

		image := &types.Image{State: &types.ImageState{Status: types.ImageStatusAvailable}}
		result, err := pm.handlePipelineStatus(context.Background(), image, "arn:image", types.ImageStatusAvailable, 10*time.Minute)

		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("failed returns BuildFailureError", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{}, nil
		}
		pm := NewPipelineManager(clients)

		image := &types.Image{State: &types.ImageState{Status: types.ImageStatusFailed}}
		result, err := pm.handlePipelineStatus(context.Background(), image, "arn:image", types.ImageStatusFailed, 10*time.Minute)

		assert.Nil(t, result)
		assert.Error(t, err)
		var bfe *BuildFailureError
		assert.ErrorAs(t, err, &bfe)
		assert.Equal(t, "FAILED", bfe.Status)
	})

	t.Run("cancelled returns BuildFailureError", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
			return &imagebuilder.ListWorkflowExecutionsOutput{}, nil
		}
		pm := NewPipelineManager(clients)

		image := &types.Image{State: &types.ImageState{Status: types.ImageStatusCancelled}}
		result, err := pm.handlePipelineStatus(context.Background(), image, "arn:image", types.ImageStatusCancelled, 5*time.Minute)

		assert.Nil(t, result)
		assert.Error(t, err)
	})

	inProgressStatuses := []types.ImageStatus{
		types.ImageStatusBuilding,
		types.ImageStatusCreating,
		types.ImageStatusPending,
		types.ImageStatusTesting,
		types.ImageStatusDistributing,
		types.ImageStatusIntegrating,
	}
	for _, status := range inProgressStatuses {
		t.Run("in-progress status "+string(status), func(t *testing.T) {
			t.Parallel()
			clients, _ := newMockAWSClients()
			pm := NewPipelineManager(clients)

			image := &types.Image{State: &types.ImageState{Status: status}}
			result, err := pm.handlePipelineStatus(context.Background(), image, "arn:image", status, 5*time.Minute)

			assert.Nil(t, result)
			assert.NoError(t, err)
		})
	}

	t.Run("unknown status returns error", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		pm := NewPipelineManager(clients)

		image := &types.Image{State: &types.ImageState{Status: types.ImageStatus("WEIRD")}}
		result, err := pm.handlePipelineStatus(context.Background(), image, "arn:image", types.ImageStatus("WEIRD"), 5*time.Minute)

		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected pipeline status")
	})
}

func TestWaitForPipelineCompletion_CancelledContext(t *testing.T) {
	t.Parallel()

	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := pm.WaitForPipelineCompletion(ctx, "arn:image", 100*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestWaitForPipelineCompletion_ImmediateSuccess(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
		return &imagebuilder.GetImageOutput{
			Image: &types.Image{
				State: &types.ImageState{
					Status: types.ImageStatusAvailable,
				},
			},
		}, nil
	}

	pm := NewPipelineManager(clients)
	image, err := pm.WaitForPipelineCompletion(context.Background(), "arn:image", 10*time.Millisecond)

	require.NoError(t, err)
	assert.NotNil(t, image)
	assert.Equal(t, types.ImageStatusAvailable, image.State.Status)
}

func TestWaitForPipelineCompletion_FailureAfterPolling(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	callCount := 0
	mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
		callCount++
		if callCount < 3 {
			return &imagebuilder.GetImageOutput{
				Image: &types.Image{
					State: &types.ImageState{Status: types.ImageStatusBuilding},
				},
			}, nil
		}
		return &imagebuilder.GetImageOutput{
			Image: &types.Image{
				State: &types.ImageState{Status: types.ImageStatusFailed, Reason: aws.String("build error")},
			},
		}, nil
	}
	mocks.imageBuilder.ListWorkflowExecutionsFunc = func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
		return &imagebuilder.ListWorkflowExecutionsOutput{}, nil
	}

	pm := NewPipelineManager(clients)
	image, err := pm.WaitForPipelineCompletion(context.Background(), "arn:image", 10*time.Millisecond)

	assert.Nil(t, image)
	assert.Error(t, err)
	var bfe *BuildFailureError
	assert.ErrorAs(t, err, &bfe)
}

func TestWaitForPipelineCompletion_GetImageError(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
		return nil, fmt.Errorf("API error")
	}

	pm := NewPipelineManager(clients)
	image, err := pm.WaitForPipelineCompletion(context.Background(), "arn:image", 10*time.Millisecond)

	assert.Nil(t, image)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get image status")
}

func TestGetImageStatus(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
			assert.Equal(t, "arn:image/test", *params.ImageBuildVersionArn)
			return &imagebuilder.GetImageOutput{
				Image: &types.Image{
					State: &types.ImageState{Status: types.ImageStatusBuilding},
				},
			}, nil
		}

		pm := NewPipelineManager(clients)
		image, err := pm.getImageStatus(context.Background(), "arn:image/test")

		require.NoError(t, err)
		assert.Equal(t, types.ImageStatusBuilding, image.State.Status)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
			return nil, fmt.Errorf("not found")
		}

		pm := NewPipelineManager(clients)
		image, err := pm.getImageStatus(context.Background(), "arn:image/test")

		assert.Error(t, err)
		assert.Nil(t, image)
		assert.Contains(t, err.Error(), "failed to get image")
	})
}

func TestLogBuildProgress(t *testing.T) {
	t.Parallel()

	// logBuildProgress only logs, so we just ensure it doesn't panic
	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)

	pm.logBuildProgress(types.ImageStatusBuilding, 5*time.Minute, true)
	pm.logBuildProgress(types.ImageStatusBuilding, 5*time.Minute, false)
	pm.logBuildProgress(types.ImageStatus("UNKNOWN"), 5*time.Minute, true)
}

func TestInitMonitorIfEnabled(t *testing.T) {
	t.Parallel()

	t.Run("disabled config does not create monitor", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		pm := NewPipelineManager(clients)
		pm.initMonitorIfEnabled("test-image")
		assert.Nil(t, pm.monitor)
	})

	t.Run("enabled config without image name does not create monitor", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		pm := NewPipelineManagerWithMonitor(clients, MonitorConfig{StreamLogs: true})
		pm.initMonitorIfEnabled("")
		assert.Nil(t, pm.monitor)
	})

	t.Run("enabled config with image name creates monitor", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		pm := NewPipelineManagerWithMonitor(clients, MonitorConfig{StreamLogs: true})
		pm.initMonitorIfEnabled("test-image")
		assert.NotNil(t, pm.monitor)
	})
}

func TestProcessPipelineTick(t *testing.T) {
	t.Parallel()

	t.Run("returns image on available status", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
			return &imagebuilder.GetImageOutput{
				Image: &types.Image{
					State: &types.ImageState{Status: types.ImageStatusAvailable},
				},
			}, nil
		}

		pm := NewPipelineManager(clients)
		state := &pipelineWaitState{
			startTime:       time.Now(),
			stageStartTimes: make(map[types.ImageStatus]time.Time),
		}

		result, err := pm.processPipelineTick(context.Background(), "arn:image", state)

		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("returns nil for in-progress status", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
			return &imagebuilder.GetImageOutput{
				Image: &types.Image{
					State: &types.ImageState{Status: types.ImageStatusBuilding},
				},
			}, nil
		}

		pm := NewPipelineManager(clients)
		state := &pipelineWaitState{
			startTime:       time.Now(),
			stageStartTimes: make(map[types.ImageStatus]time.Time),
		}

		result, err := pm.processPipelineTick(context.Background(), "arn:image", state)

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("updates stage start times", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.GetImageFunc = func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
			return &imagebuilder.GetImageOutput{
				Image: &types.Image{
					State: &types.ImageState{Status: types.ImageStatusCreating},
				},
			}, nil
		}

		pm := NewPipelineManager(clients)
		state := &pipelineWaitState{
			startTime:       time.Now(),
			stageStartTimes: make(map[types.ImageStatus]time.Time),
		}

		_, _ = pm.processPipelineTick(context.Background(), "arn:image", state)

		_, exists := state.stageStartTimes[types.ImageStatusCreating]
		assert.True(t, exists)
		assert.Equal(t, types.ImageStatusCreating, state.lastStatus)
	})
}

func TestDeleteHelpers(t *testing.T) {
	t.Parallel()

	t.Run("deleteImageRecipe success", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
			assert.Equal(t, "arn:recipe", *params.ImageRecipeArn)
			return &imagebuilder.DeleteImageRecipeOutput{}, nil
		}

		pm := NewPipelineManager(clients)
		err := pm.deleteImageRecipe(context.Background(), "arn:recipe")
		assert.NoError(t, err)
	})

	t.Run("deleteInfrastructureConfig success", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		pm := NewPipelineManager(clients)
		err := pm.deleteInfrastructureConfig(context.Background(), "arn:infra")
		assert.NoError(t, err)
	})

	t.Run("deleteDistributionConfig success", func(t *testing.T) {
		t.Parallel()
		clients, _ := newMockAWSClients()
		pm := NewPipelineManager(clients)
		err := pm.deleteDistributionConfig(context.Background(), "arn:dist")
		assert.NoError(t, err)
	})

	t.Run("deleteImageRecipe error", func(t *testing.T) {
		t.Parallel()
		clients, mocks := newMockAWSClients()
		mocks.imageBuilder.DeleteImageRecipeFunc = func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
			return nil, fmt.Errorf("recipe error")
		}

		pm := NewPipelineManager(clients)
		err := pm.deleteImageRecipe(context.Background(), "arn:recipe")
		assert.Error(t, err)
	})
}

func TestPipelineConfig(t *testing.T) {
	t.Parallel()

	cfg := PipelineConfig{
		Name:             "test-pipeline",
		Description:      "A test pipeline",
		ImageRecipeARN:   "arn:recipe",
		InfraConfigARN:   "arn:infra",
		DistConfigARN:    "arn:dist",
		Tags:             map[string]string{"env": "test"},
		EnhancedMetadata: true,
	}

	assert.Equal(t, "test-pipeline", cfg.Name)
	assert.Equal(t, "arn:recipe", cfg.ImageRecipeARN)
	assert.True(t, cfg.EnhancedMetadata)
	assert.Equal(t, "test", cfg.Tags["env"])
}

func TestWorkflowStepLog(t *testing.T) {
	t.Parallel()

	log := WorkflowStepLog{
		StepName:  "BuildImage",
		Status:    "COMPLETED",
		Message:   "Build completed",
		StartTime: "2025-01-01T00:00:00Z",
		EndTime:   "2025-01-01T00:05:00Z",
	}

	assert.Equal(t, "BuildImage", log.StepName)
	assert.Equal(t, "COMPLETED", log.Status)
	assert.Equal(t, "Build completed", log.Message)
}

func TestCreatePipeline_VerifiesInput(t *testing.T) {
	t.Parallel()

	clients, mocks := newMockAWSClients()
	var capturedInput *imagebuilder.CreateImagePipelineInput
	mocks.imageBuilder.CreateImagePipelineFunc = func(ctx context.Context, params *imagebuilder.CreateImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImagePipelineOutput, error) {
		capturedInput = params
		return &imagebuilder.CreateImagePipelineOutput{
			ImagePipelineArn: aws.String("arn:pipeline"),
		}, nil
	}

	pm := NewPipelineManager(clients)
	_, err := pm.CreatePipeline(context.Background(), PipelineConfig{
		Name:             "my-pipeline",
		Description:      "my description",
		ImageRecipeARN:   "arn:recipe/test",
		InfraConfigARN:   "arn:infra/test",
		DistConfigARN:    "arn:dist/test",
		EnhancedMetadata: true,
		Tags:             map[string]string{"project": "warpgate"},
	})

	require.NoError(t, err)
	assert.Equal(t, "my-pipeline", *capturedInput.Name)
	assert.Equal(t, "my description", *capturedInput.Description)
	assert.Equal(t, "arn:recipe/test", *capturedInput.ImageRecipeArn)
	assert.Equal(t, "arn:infra/test", *capturedInput.InfrastructureConfigurationArn)
	assert.Equal(t, "arn:dist/test", *capturedInput.DistributionConfigurationArn)
	assert.True(t, *capturedInput.EnhancedImageMetadataEnabled)
	assert.Equal(t, types.PipelineStatusEnabled, capturedInput.Status)
	assert.Equal(t, "warpgate", capturedInput.Tags["project"])
}

func TestFailureDetailsRemediationHintsFromErrorMessage(t *testing.T) {
	t.Parallel()

	clients, _ := newMockAWSClients()
	pm := NewPipelineManager(clients)

	// Test that remediation hints can be triggered by ErrorMessage field
	details := &FailureDetails{
		Reason:       "unknown",
		ErrorMessage: "timeout exceeded while waiting",
	}
	result := pm.addRemediationHints(details)
	assert.Contains(t, result.ErrorMessage, "timed out")
}

func TestBuildFailureErrorImplementsError(t *testing.T) {
	t.Parallel()
	var err error = &BuildFailureError{Status: "FAILED", Duration: "1m"}
	assert.NotEmpty(t, err.Error())
	assert.True(t, strings.Contains(err.Error(), "FAILED"))
}
