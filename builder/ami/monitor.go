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

// Package ami provides monitoring capabilities for EC2 Image Builder builds.
//
// This file implements BuildMonitor which tracks EC2 instance status and
// streams build logs in real-time from CloudWatch and SSM. Use MonitorConfig
// to enable specific monitoring features like log streaming and EC2 status display.
//
// Key features:
//   - Real-time EC2 instance status tracking
//   - CloudWatch log streaming
//   - SSM command output retrieval
//   - Build instance discovery by tags
package ami

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// BuildMonitor monitors EC2 Image Builder builds in real-time.
// It provides capabilities for tracking EC2 instance status and streaming
// logs from CloudWatch and SSM during the build process.
type BuildMonitor struct {
	clients       *AWSClients
	imageName     string
	lastLogToken  *string
	lastEC2Status string
	instanceID    string
	logGroupName  string
	streamLogs    bool
	showEC2Status bool
}

// MonitorConfig contains configuration options for the build monitor.
// Use this to enable real-time log streaming and EC2 instance status tracking.
type MonitorConfig struct {
	// StreamLogs enables SSM/CloudWatch log streaming during the build.
	// When enabled, build logs are fetched and displayed in real-time.
	StreamLogs bool

	// ShowEC2Status displays EC2 instance status during build.
	// Includes instance ID, state, type, IP addresses, and uptime.
	ShowEC2Status bool
}

// NewBuildMonitor creates a new build monitor
func NewBuildMonitor(clients *AWSClients, imageName string, config MonitorConfig) *BuildMonitor {
	return &BuildMonitor{
		clients:       clients,
		imageName:     imageName,
		streamLogs:    config.StreamLogs,
		showEC2Status: config.ShowEC2Status,
		logGroupName:  fmt.Sprintf("/aws/imagebuilder/%s", imageName),
	}
}

// EC2InstanceStatus represents the status of an EC2 instance used for Image Builder builds.
// This information is used to track the build instance's lifecycle and connectivity.
type EC2InstanceStatus struct {
	InstanceID       string     // AWS EC2 instance ID (i-xxx)
	State            string     // Instance state (pending, running, stopping, etc.)
	StateReason      string     // Reason for current state
	InstanceType     string     // Instance type (t3.micro, m5.large, etc.)
	LaunchTime       *time.Time // When the instance was launched
	PrivateIP        string     // Private IP address
	PublicIP         string     // Public IP address (if assigned)
	AvailabilityZone string     // AWS availability zone
}

// LogEntry represents a log entry from CloudWatch or SSM.
// Used for real-time build log streaming.
type LogEntry struct {
	Timestamp time.Time // When the log entry was created
	Message   string    // Log message content
	Source    string    // Log source: "cloudwatch" or "ssm"
}

// FindBuildInstance finds the EC2 instance being used for the current Image Builder build
func (m *BuildMonitor) FindBuildInstance(ctx context.Context) (*EC2InstanceStatus, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("tag-key"), Values: []string{"CreatedBy"}},
			{Name: aws.String("tag-value"), Values: []string{"EC2 Image Builder"}},
			{Name: aws.String("instance-state-name"), Values: []string{"pending", "running", "stopping"}},
		},
	}

	result, err := m.clients.EC2.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	latestInstance := m.findLatestMatchingInstance(result.Reservations)
	if latestInstance == nil {
		return nil, nil
	}

	status := m.buildInstanceStatus(latestInstance)
	m.instanceID = status.InstanceID
	return status, nil
}

// findLatestMatchingInstance finds the most recently launched instance matching our image
func (m *BuildMonitor) findLatestMatchingInstance(reservations []ec2types.Reservation) *ec2types.Instance {
	var latestInstance *ec2types.Instance
	var latestLaunchTime time.Time

	for _, reservation := range reservations {
		for i := range reservation.Instances {
			inst := &reservation.Instances[i]
			if inst.LaunchTime == nil || !inst.LaunchTime.After(latestLaunchTime) {
				continue
			}
			if m.instanceMatchesImage(inst) {
				latestInstance = inst
				latestLaunchTime = *inst.LaunchTime
			} else if latestInstance == nil {
				latestInstance = inst
				latestLaunchTime = *inst.LaunchTime
			}
		}
	}
	return latestInstance
}

// instanceMatchesImage checks if an instance is tagged for our image build
func (m *BuildMonitor) instanceMatchesImage(inst *ec2types.Instance) bool {
	for _, tag := range inst.Tags {
		if aws.ToString(tag.Key) == "Ec2ImageBuilderArn" {
			return strings.Contains(aws.ToString(tag.Value), m.imageName)
		}
	}
	return false
}

// buildInstanceStatus creates an EC2InstanceStatus from an EC2 instance
func (m *BuildMonitor) buildInstanceStatus(inst *ec2types.Instance) *EC2InstanceStatus {
	status := &EC2InstanceStatus{
		InstanceID:   aws.ToString(inst.InstanceId),
		InstanceType: string(inst.InstanceType),
		LaunchTime:   inst.LaunchTime,
	}
	m.populateInstanceState(status, inst)
	m.populateInstanceNetwork(status, inst)
	return status
}

// populateInstanceState fills in state information for an instance status
func (m *BuildMonitor) populateInstanceState(status *EC2InstanceStatus, inst *ec2types.Instance) {
	if inst.State == nil {
		return
	}
	status.State = string(inst.State.Name)
	if inst.State.Name == ec2types.InstanceStateNameRunning {
		status.StateReason = "Instance is running"
	} else if inst.StateReason != nil {
		status.StateReason = aws.ToString(inst.StateReason.Message)
	}
}

// populateInstanceNetwork fills in network information for an instance status
func (m *BuildMonitor) populateInstanceNetwork(status *EC2InstanceStatus, inst *ec2types.Instance) {
	if inst.PrivateIpAddress != nil {
		status.PrivateIP = *inst.PrivateIpAddress
	}
	if inst.PublicIpAddress != nil {
		status.PublicIP = *inst.PublicIpAddress
	}
	if inst.Placement != nil && inst.Placement.AvailabilityZone != nil {
		status.AvailabilityZone = *inst.Placement.AvailabilityZone
	}
}

// GetEC2InstanceStatus gets the current status of the build instance
func (m *BuildMonitor) GetEC2InstanceStatus(ctx context.Context) (*EC2InstanceStatus, error) {
	if m.instanceID == "" {
		// Try to find the instance first
		return m.FindBuildInstance(ctx)
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{m.instanceID},
	}

	result, err := m.clients.EC2.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", m.instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", m.instanceID)
	}

	inst := result.Reservations[0].Instances[0]
	status := &EC2InstanceStatus{
		InstanceID:   m.instanceID,
		InstanceType: string(inst.InstanceType),
		LaunchTime:   inst.LaunchTime,
	}

	if inst.State != nil {
		status.State = string(inst.State.Name)
	}
	if inst.PrivateIpAddress != nil {
		status.PrivateIP = *inst.PrivateIpAddress
	}
	if inst.PublicIpAddress != nil {
		status.PublicIP = *inst.PublicIpAddress
	}
	if inst.Placement != nil && inst.Placement.AvailabilityZone != nil {
		status.AvailabilityZone = *inst.Placement.AvailabilityZone
	}

	return status, nil
}

// StreamCloudWatchLogs streams logs from CloudWatch for the Image Builder build
func (m *BuildMonitor) StreamCloudWatchLogs(ctx context.Context) ([]LogEntry, error) {
	var entries []LogEntry

	streamsInput := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(m.logGroupName),
		OrderBy:      "LastEventTime",
		Descending:   aws.Bool(true),
		Limit:        aws.Int32(5),
	}

	streamsResult, err := m.clients.CloudWatchLogs.DescribeLogStreams(ctx, streamsInput)
	if err != nil {
		// Log group might not exist yet
		if strings.Contains(err.Error(), "ResourceNotFoundException") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to describe log streams: %w", err)
	}

	if len(streamsResult.LogStreams) == 0 {
		return nil, nil
	}

	logStream := streamsResult.LogStreams[0]
	logsInput := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(m.logGroupName),
		LogStreamName: logStream.LogStreamName,
		StartFromHead: aws.Bool(false),
		Limit:         aws.Int32(50),
	}

	if m.lastLogToken != nil {
		logsInput.NextToken = m.lastLogToken
	}

	logsResult, err := m.clients.CloudWatchLogs.GetLogEvents(ctx, logsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get log events: %w", err)
	}

	m.lastLogToken = logsResult.NextForwardToken

	for _, event := range logsResult.Events {
		entry := LogEntry{
			Timestamp: time.UnixMilli(*event.Timestamp),
			Message:   aws.ToString(event.Message),
			Source:    "cloudwatch",
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetSSMCommandOutput gets the output of SSM commands run on the build instance
func (m *BuildMonitor) GetSSMCommandOutput(ctx context.Context) ([]LogEntry, error) {
	if m.instanceID == "" {
		return nil, nil
	}

	var entries []LogEntry

	// List recent command invocations for this instance
	input := &ssm.ListCommandInvocationsInput{
		InstanceId: aws.String(m.instanceID),
		MaxResults: aws.Int32(10),
		Details:    true,
	}

	result, err := m.clients.SSM.ListCommandInvocations(ctx, input)
	if err != nil {
		// Instance might not have SSM agent or no commands yet
		if strings.Contains(err.Error(), "InvalidInstanceId") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list SSM command invocations: %w", err)
	}

	for _, invocation := range result.CommandInvocations {
		// Get output for each command plugin
		for _, plugin := range invocation.CommandPlugins {
			if plugin.Output != nil && *plugin.Output != "" {
				entry := LogEntry{
					Timestamp: aws.ToTime(invocation.RequestedDateTime),
					Message:   fmt.Sprintf("[%s] %s", aws.ToString(plugin.Name), aws.ToString(plugin.Output)),
					Source:    "ssm",
				}
				entries = append(entries, entry)
			}
		}
	}

	return entries, nil
}

// PollAndDisplay polls for updates and displays them
func (m *BuildMonitor) PollAndDisplay(ctx context.Context) {
	// Check EC2 instance status if enabled
	if m.showEC2Status {
		status, err := m.GetEC2InstanceStatus(ctx)
		if err != nil {
			logging.Debug("Failed to get EC2 status: %v", err)
		} else if status != nil && status.State != m.lastEC2Status {
			m.lastEC2Status = status.State
			m.displayEC2Status(status)
		}
	}

	// Stream logs if enabled
	if m.streamLogs {
		// Try CloudWatch logs first
		cwLogs, err := m.StreamCloudWatchLogs(ctx)
		if err != nil {
			logging.Debug("Failed to get CloudWatch logs: %v", err)
		}

		// Also try SSM output
		ssmLogs, err := m.GetSSMCommandOutput(ctx)
		if err != nil {
			logging.Debug("Failed to get SSM output: %v", err)
		}

		// Combine and sort logs
		allLogs := cwLogs
		allLogs = append(allLogs, ssmLogs...)
		sort.Slice(allLogs, func(i, j int) bool {
			return allLogs[i].Timestamp.Before(allLogs[j].Timestamp)
		})

		// Display new logs
		for _, entry := range allLogs {
			m.displayLogEntry(entry)
		}
	}
}

// displayEC2Status displays EC2 instance status information
func (m *BuildMonitor) displayEC2Status(status *EC2InstanceStatus) {
	var details []string
	details = append(details, fmt.Sprintf("Instance: %s", status.InstanceID))
	details = append(details, fmt.Sprintf("State: %s", status.State))
	details = append(details, fmt.Sprintf("Type: %s", status.InstanceType))

	if status.AvailabilityZone != "" {
		details = append(details, fmt.Sprintf("AZ: %s", status.AvailabilityZone))
	}
	if status.PrivateIP != "" {
		details = append(details, fmt.Sprintf("Private IP: %s", status.PrivateIP))
	}
	if status.LaunchTime != nil {
		uptime := time.Since(*status.LaunchTime).Round(time.Second)
		details = append(details, fmt.Sprintf("Uptime: %s", uptime))
	}

	logging.Info("EC2 Build Instance: %s", strings.Join(details, " | "))
}

// displayLogEntry displays a single log entry
func (m *BuildMonitor) displayLogEntry(entry LogEntry) {
	// Truncate very long messages
	message := entry.Message
	if len(message) > 500 {
		message = message[:500] + "..."
	}

	// Clean up the message (remove excessive newlines)
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	timestamp := entry.Timestamp.Format("15:04:05")
	logging.Info("[%s] [%s] %s", timestamp, entry.Source, message)
}

// GetBuildInstanceLogs returns all available logs for the build instance
func (m *BuildMonitor) GetBuildInstanceLogs(ctx context.Context) ([]LogEntry, error) {
	var allLogs []LogEntry

	// Get CloudWatch logs
	cwLogs, err := m.StreamCloudWatchLogs(ctx)
	if err == nil {
		allLogs = append(allLogs, cwLogs...)
	}

	// Get SSM command output
	ssmLogs, err := m.GetSSMCommandOutput(ctx)
	if err == nil {
		allLogs = append(allLogs, ssmLogs...)
	}

	// Sort by timestamp
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Timestamp.Before(allLogs[j].Timestamp)
	})

	return allLogs, nil
}

// FormatEC2StatusString returns a formatted string of EC2 instance status
func (status *EC2InstanceStatus) FormatEC2StatusString() string {
	if status == nil {
		return "No build instance found"
	}

	parts := []string{
		fmt.Sprintf("ID: %s", status.InstanceID),
		fmt.Sprintf("State: %s", status.State),
		fmt.Sprintf("Type: %s", status.InstanceType),
	}

	if status.PrivateIP != "" {
		parts = append(parts, fmt.Sprintf("IP: %s", status.PrivateIP))
	}

	if status.LaunchTime != nil {
		uptime := time.Since(*status.LaunchTime).Round(time.Second)
		parts = append(parts, fmt.Sprintf("Uptime: %s", uptime))
	}

	return strings.Join(parts, " | ")
}
