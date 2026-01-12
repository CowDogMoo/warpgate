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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// PipelineManager manages EC2 Image Builder pipelines
type PipelineManager struct {
	clients       *AWSClients
	monitor       *BuildMonitor
	monitorConfig MonitorConfig
}

// BuildFailureError represents a detailed build failure with root cause analysis
type BuildFailureError struct {
	Status      string
	Duration    string
	Details     *FailureDetails
	Remediation string
}

// FailureDetails contains detailed information about a build failure
type FailureDetails struct {
	Reason           string
	FailedStep       string
	FailedComponent  string
	ErrorMessage     string
	LogsURL          string
	WorkflowStepLogs []WorkflowStepLog
}

// WorkflowStepLog represents a single workflow step execution log
type WorkflowStepLog struct {
	StepName  string
	Status    string
	Message   string
	StartTime string
	EndTime   string
}

func (e *BuildFailureError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Build failed with status %s after %s\n", e.Status, e.Duration))

	if e.Details != nil {
		if e.Details.Reason != "" {
			sb.WriteString(fmt.Sprintf("\nReason: %s\n", e.Details.Reason))
		}
		if e.Details.FailedStep != "" {
			sb.WriteString(fmt.Sprintf("Failed Step: %s\n", e.Details.FailedStep))
		}
		if e.Details.FailedComponent != "" {
			sb.WriteString(fmt.Sprintf("Failed Component: %s\n", e.Details.FailedComponent))
		}
		if e.Details.ErrorMessage != "" {
			sb.WriteString(fmt.Sprintf("Error Message: %s\n", e.Details.ErrorMessage))
		}
		if len(e.Details.WorkflowStepLogs) > 0 {
			sb.WriteString("\nWorkflow Step Details:\n")
			for _, step := range e.Details.WorkflowStepLogs {
				sb.WriteString(fmt.Sprintf("  - %s: %s\n", step.StepName, step.Status))
				if step.Message != "" {
					sb.WriteString(fmt.Sprintf("    Message: %s\n", step.Message))
				}
			}
		}
		if e.Details.LogsURL != "" {
			sb.WriteString(fmt.Sprintf("\nView full logs: %s\n", e.Details.LogsURL))
		}
	}

	if e.Remediation != "" {
		sb.WriteString(fmt.Sprintf("\nRemediation: %s\n", e.Remediation))
	}

	return sb.String()
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

// NewPipelineManagerWithMonitor creates a new pipeline manager with monitoring enabled
func NewPipelineManagerWithMonitor(clients *AWSClients, config MonitorConfig) *PipelineManager {
	return &PipelineManager{
		clients:       clients,
		monitorConfig: config,
	}
}

// SetMonitorConfig sets the monitor configuration
func (m *PipelineManager) SetMonitorConfig(config MonitorConfig) {
	m.monitorConfig = config
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
	return m.WaitForPipelineCompletionWithImageName(ctx, imageARN, pollInterval, "")
}

// WaitForPipelineCompletionWithImageName waits for completion with optional monitoring
func (m *PipelineManager) WaitForPipelineCompletionWithImageName(ctx context.Context, imageARN string, pollInterval time.Duration, imageName string) (*types.Image, error) {
	logging.Info("Waiting for pipeline completion: %s", imageARN)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	state := &pipelineWaitState{
		startTime:       time.Now(),
		stageStartTimes: make(map[types.ImageStatus]time.Time),
	}

	m.initMonitorIfEnabled(imageName)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for pipeline: %w", ctx.Err())
		case <-ticker.C:
			result, err := m.processPipelineTick(ctx, imageARN, state)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}
		}
	}
}

// pipelineWaitState holds state during pipeline wait loop
type pipelineWaitState struct {
	startTime       time.Time
	lastStatus      types.ImageStatus
	stageStartTimes map[types.ImageStatus]time.Time
}

// initMonitorIfEnabled initializes the build monitor if monitoring is configured
func (m *PipelineManager) initMonitorIfEnabled(imageName string) {
	if (m.monitorConfig.StreamLogs || m.monitorConfig.ShowEC2Status) && imageName != "" {
		m.monitor = NewBuildMonitor(m.clients, imageName, m.monitorConfig)
		logging.Info("Build monitoring enabled (logs: %v, EC2 status: %v)",
			m.monitorConfig.StreamLogs, m.monitorConfig.ShowEC2Status)
	}
}

// processPipelineTick handles a single poll iteration, returns image if complete or nil to continue
func (m *PipelineManager) processPipelineTick(ctx context.Context, imageARN string, state *pipelineWaitState) (*types.Image, error) {
	image, err := m.getImageStatus(ctx, imageARN)
	if err != nil {
		return nil, fmt.Errorf("failed to get image status: %w", err)
	}

	status := image.State.Status
	elapsed := time.Since(state.startTime).Round(time.Second)

	if _, exists := state.stageStartTimes[status]; !exists {
		state.stageStartTimes[status] = time.Now()
	}

	m.logBuildProgress(status, elapsed, status != state.lastStatus)
	state.lastStatus = status

	if m.monitor != nil && (status == types.ImageStatusBuilding || status == types.ImageStatusCreating) {
		m.monitor.PollAndDisplay(ctx)
	}

	return m.handlePipelineStatus(ctx, image, imageARN, status, elapsed)
}

// logBuildProgress logs build stage or periodic progress updates
func (m *PipelineManager) logBuildProgress(status types.ImageStatus, elapsed time.Duration, statusChanged bool) {
	estimatedRemaining := m.estimateRemainingTime(status, elapsed)

	if statusChanged {
		stageInfo := m.formatBuildStage(status)
		if estimatedRemaining > 0 {
			logging.Info("Build stage: %s (elapsed: %s, estimated remaining: ~%s)",
				stageInfo, elapsed, estimatedRemaining.Round(time.Minute))
		} else {
			logging.Info("Build stage: %s (elapsed: %s)", stageInfo, elapsed)
		}
	} else {
		if estimatedRemaining > 0 {
			logging.Debug("Build status: %s (elapsed: %s, ~%s remaining)",
				status, elapsed, estimatedRemaining.Round(time.Minute))
		} else {
			logging.Debug("Build status: %s (elapsed: %s)", status, elapsed)
		}
	}
}

// handlePipelineStatus processes the current pipeline status and returns result or nil to continue
func (m *PipelineManager) handlePipelineStatus(ctx context.Context, image *types.Image, imageARN string, status types.ImageStatus, elapsed time.Duration) (*types.Image, error) {
	switch status {
	case types.ImageStatusAvailable:
		logging.Info("Pipeline completed successfully in %s", elapsed)
		return image, nil
	case types.ImageStatusFailed, types.ImageStatusCancelled, types.ImageStatusDeprecated:
		failureDetails := m.getFailureDetails(ctx, image, imageARN)
		return nil, &BuildFailureError{
			Status:   string(status),
			Duration: elapsed.String(),
			Details:  failureDetails,
		}
	case types.ImageStatusBuilding, types.ImageStatusCreating, types.ImageStatusPending, types.ImageStatusTesting, types.ImageStatusDistributing, types.ImageStatusIntegrating:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected pipeline status: %s", status)
	}
}

// estimateRemainingTime estimates the remaining build time based on current stage and elapsed time
// These are typical times based on common AMI build patterns
func (m *PipelineManager) estimateRemainingTime(currentStatus types.ImageStatus, elapsed time.Duration) time.Duration {
	// Typical durations for each stage (based on general AMI build patterns)
	// These are estimates and actual times vary based on instance type, provisioners, etc.
	typicalStageDurations := map[types.ImageStatus]time.Duration{
		types.ImageStatusPending:      2 * time.Minute,  // Waiting for resources
		types.ImageStatusCreating:     5 * time.Minute,  // Initializing EC2 instance
		types.ImageStatusBuilding:     20 * time.Minute, // Running provisioners (highly variable)
		types.ImageStatusTesting:      5 * time.Minute,  // Running image tests
		types.ImageStatusDistributing: 10 * time.Minute, // Creating AMI snapshot
		types.ImageStatusIntegrating:  2 * time.Minute,  // Finalizing
	}

	// Order of stages
	stageOrder := []types.ImageStatus{
		types.ImageStatusPending,
		types.ImageStatusCreating,
		types.ImageStatusBuilding,
		types.ImageStatusTesting,
		types.ImageStatusDistributing,
		types.ImageStatusIntegrating,
		types.ImageStatusAvailable,
	}

	// Find current stage index
	currentIndex := -1
	for i, stage := range stageOrder {
		if stage == currentStatus {
			currentIndex = i
			break
		}
	}

	if currentIndex < 0 {
		return 0 // Unknown stage
	}

	// Estimate remaining time based on elapsed time in current stage and future stages
	var remaining time.Duration

	// Estimate time remaining in current stage (assume we're halfway through if no better info)
	if typicalDuration, ok := typicalStageDurations[currentStatus]; ok {
		// Use a simple heuristic: if we've been in this stage for less than typical,
		// estimate we have the difference remaining; otherwise, add a buffer
		elapsedInStage := elapsed
		if elapsedInStage < typicalDuration {
			remaining += typicalDuration - elapsedInStage
		} else {
			// We're over the typical time, add a small buffer
			remaining += 2 * time.Minute
		}
	}

	// Add typical times for remaining stages
	for i := currentIndex + 1; i < len(stageOrder)-1; i++ { // -1 to exclude AVAILABLE
		if duration, ok := typicalStageDurations[stageOrder[i]]; ok {
			remaining += duration
		}
	}

	return remaining
}

// formatBuildStage returns a human-readable description of the build stage
func (m *PipelineManager) formatBuildStage(status types.ImageStatus) string {
	switch status {
	case types.ImageStatusPending:
		return "PENDING - Waiting for resources"
	case types.ImageStatusCreating:
		return "CREATING - Initializing build environment"
	case types.ImageStatusBuilding:
		return "BUILDING - Running provisioners on EC2 instance"
	case types.ImageStatusTesting:
		return "TESTING - Running image tests"
	case types.ImageStatusDistributing:
		return "DISTRIBUTING - Creating AMI in target regions"
	case types.ImageStatusIntegrating:
		return "INTEGRATING - Finalizing image configuration"
	case types.ImageStatusAvailable:
		return "AVAILABLE - Build complete"
	case types.ImageStatusFailed:
		return "FAILED - Build failed"
	case types.ImageStatusCancelled:
		return "CANCELLED - Build was cancelled"
	default:
		return string(status)
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

// getFailureDetails extracts detailed failure information from an image build
func (m *PipelineManager) getFailureDetails(ctx context.Context, image *types.Image, imageARN string) *FailureDetails {
	details := &FailureDetails{}

	// Get the basic reason from the image state
	if image.State != nil && image.State.Reason != nil {
		details.Reason = *image.State.Reason
	}

	// Try to get workflow execution details
	workflowLogs := m.getWorkflowExecutionLogs(ctx, imageARN)
	if len(workflowLogs) > 0 {
		details.WorkflowStepLogs = workflowLogs

		// Find the failed step
		for _, log := range workflowLogs {
			if log.Status == "FAILED" || log.Status == "ERROR" {
				details.FailedStep = log.StepName
				details.ErrorMessage = log.Message
				break
			}
		}
	}

	// Extract component information if available
	if image.ImageRecipe != nil && image.ImageRecipe.Components != nil {
		for _, comp := range image.ImageRecipe.Components {
			if comp.ComponentArn != nil {
				// Store the last component as potentially the failed one
				// (Image Builder processes components in order)
				parts := strings.Split(*comp.ComponentArn, "/")
				if len(parts) > 0 {
					details.FailedComponent = parts[len(parts)-1]
				}
			}
		}
	}

	// Generate CloudWatch Logs URL if we can determine the log group
	// Format: /aws/imagebuilder/{imageName}
	if image.Name != nil {
		region := m.clients.GetRegion()
		logGroup := fmt.Sprintf("/aws/imagebuilder/%s", *image.Name)
		details.LogsURL = fmt.Sprintf(
			"https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#logsV2:log-groups/log-group/%s",
			region, region, strings.ReplaceAll(logGroup, "/", "$252F"),
		)
	}

	// Add remediation hints based on the error
	details = m.addRemediationHints(details)

	return details
}

// getWorkflowExecutionLogs retrieves workflow execution logs
func (m *PipelineManager) getWorkflowExecutionLogs(ctx context.Context, imageARN string) []WorkflowStepLog {
	var logs []WorkflowStepLog

	// List workflow executions for this image
	listExecInput := &imagebuilder.ListWorkflowExecutionsInput{
		ImageBuildVersionArn: aws.String(imageARN),
	}

	listExecResult, err := m.clients.ImageBuilder.ListWorkflowExecutions(ctx, listExecInput)
	if err != nil {
		logging.Debug("Failed to get workflow executions: %v", err)
		return logs
	}

	// Get step details for each workflow execution
	for _, exec := range listExecResult.WorkflowExecutions {
		if exec.WorkflowExecutionId == nil {
			continue
		}

		// Add the workflow execution as a log entry
		log := WorkflowStepLog{
			StepName: aws.ToString(exec.WorkflowBuildVersionArn),
			Status:   string(exec.Status),
			Message:  aws.ToString(exec.Message),
		}

		if exec.StartTime != nil {
			log.StartTime = *exec.StartTime
		}
		if exec.EndTime != nil {
			log.EndTime = *exec.EndTime
		}

		logs = append(logs, log)

		// If this execution failed, try to get step details
		if exec.Status == "FAILED" {
			stepInput := &imagebuilder.ListWorkflowStepExecutionsInput{
				WorkflowExecutionId: exec.WorkflowExecutionId,
			}

			stepResult, stepErr := m.clients.ImageBuilder.ListWorkflowStepExecutions(ctx, stepInput)
			if stepErr != nil {
				logging.Debug("Failed to get workflow step executions: %v", stepErr)
				continue
			}

			for _, step := range stepResult.Steps {
				stepLog := WorkflowStepLog{
					StepName: aws.ToString(step.Name),
					Status:   string(step.Status),
					Message:  aws.ToString(step.Message),
				}

				if step.StartTime != nil {
					stepLog.StartTime = *step.StartTime
				}
				if step.EndTime != nil {
					stepLog.EndTime = *step.EndTime
				}

				logs = append(logs, stepLog)
			}
		}
	}

	return logs
}

// addRemediationHints analyzes the failure details and adds remediation suggestions
func (m *PipelineManager) addRemediationHints(details *FailureDetails) *FailureDetails {
	reason := strings.ToLower(details.Reason + " " + details.ErrorMessage)

	// Check for common failure patterns and add remediation hints
	switch {
	case strings.Contains(reason, "timeout"):
		details.ErrorMessage += "\n\nRemediation: The build timed out. Consider:\n" +
			"  - Using a larger instance type for faster builds\n" +
			"  - Reducing the number of provisioners\n" +
			"  - Checking if provisioners are waiting for user input"

	case strings.Contains(reason, "script") && strings.Contains(reason, "failed"):
		details.ErrorMessage += "\n\nRemediation: A provisioner script failed. Check:\n" +
			"  - Script syntax errors (run locally first)\n" +
			"  - Missing dependencies or packages\n" +
			"  - Network connectivity from the build instance\n" +
			"  - CloudWatch logs for detailed output"

	case strings.Contains(reason, "network") || strings.Contains(reason, "connection"):
		details.ErrorMessage += "\n\nRemediation: Network connectivity issue. Check:\n" +
			"  - Security group allows outbound traffic\n" +
			"  - Subnet has internet access (NAT gateway or internet gateway)\n" +
			"  - VPC endpoints if using private subnets"

	case strings.Contains(reason, "permission") || strings.Contains(reason, "access denied"):
		details.ErrorMessage += "\n\nRemediation: Permission denied. Check:\n" +
			"  - Instance profile has required permissions\n" +
			"  - IAM role trust policy allows EC2 Image Builder\n" +
			"  - S3 bucket policies if uploading/downloading files"

	case strings.Contains(reason, "disk") || strings.Contains(reason, "space"):
		details.ErrorMessage += "\n\nRemediation: Disk space issue. Consider:\n" +
			"  - Increasing volume_size in target configuration\n" +
			"  - Cleaning up temporary files in provisioner scripts"

	case strings.Contains(reason, "ami") && strings.Contains(reason, "not found"):
		details.ErrorMessage += "\n\nRemediation: AMI not found. Check:\n" +
			"  - Base AMI ID exists in the target region\n" +
			"  - Base AMI is not deprecated or deregistered\n" +
			"  - You have permission to use the AMI"

	case strings.Contains(reason, "component"):
		details.ErrorMessage += "\n\nRemediation: Component execution failed. Check:\n" +
			"  - Component script syntax\n" +
			"  - Required packages are available\n" +
			"  - CloudWatch logs for detailed component output"
	}

	return details
}
