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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/cowdogmoo/warpgate/pkg/config"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// AMIOperations provides operations for managing AMIs
type AMIOperations struct {
	clients      *AWSClients
	globalConfig *config.Config
}

// NewAMIOperations creates a new AMI operations handler
func NewAMIOperations(clients *AWSClients, cfg *config.Config) *AMIOperations {
	return &AMIOperations{
		clients:      clients,
		globalConfig: cfg,
	}
}

// ShareAMI shares an AMI with specified AWS accounts
func (o *AMIOperations) ShareAMI(ctx context.Context, amiID string, accountIDs []string) error {
	logging.Info("Sharing AMI %s with accounts: %v", amiID, accountIDs)

	// Add launch permissions for each account
	launchPermissions := make([]types.LaunchPermission, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		launchPermissions = append(launchPermissions, types.LaunchPermission{
			UserId: aws.String(accountID),
		})
	}

	input := &ec2.ModifyImageAttributeInput{
		ImageId: aws.String(amiID),
		LaunchPermission: &types.LaunchPermissionModifications{
			Add: launchPermissions,
		},
	}

	_, err := o.clients.EC2.ModifyImageAttribute(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to share AMI: %w", err)
	}

	logging.Info("AMI shared successfully: %s", amiID)
	return nil
}

// CopyAMI copies an AMI to another region
func (o *AMIOperations) CopyAMI(ctx context.Context, amiID, sourceRegion, destRegion, name string) (string, error) {
	logging.Info("Copying AMI %s from %s to %s", amiID, sourceRegion, destRegion)

	// Create EC2 client for destination region
	destCfg := o.clients.Config.Copy()
	destCfg.Region = destRegion
	destEC2 := ec2.NewFromConfig(destCfg)

	input := &ec2.CopyImageInput{
		SourceImageId: aws.String(amiID),
		SourceRegion:  aws.String(sourceRegion),
		Name:          aws.String(name),
		Description:   aws.String(fmt.Sprintf("Copy of %s from %s", amiID, sourceRegion)),
	}

	result, err := destEC2.CopyImage(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to copy AMI: %w", err)
	}

	newAMIID := *result.ImageId
	logging.Info("AMI copy initiated: %s (region: %s)", newAMIID, destRegion)

	// Wait for AMI to be available
	if err := o.waitForAMIAvailable(ctx, destEC2, newAMIID); err != nil {
		return "", fmt.Errorf("failed waiting for AMI copy: %w", err)
	}

	logging.Info("AMI copied successfully: %s (region: %s)", newAMIID, destRegion)
	return newAMIID, nil
}

// DeregisterAMI deregisters an AMI and optionally deletes associated snapshots
func (o *AMIOperations) DeregisterAMI(ctx context.Context, amiID string, deleteSnapshots bool) error {
	logging.Info("Deregistering AMI %s (delete_snapshots: %v)", amiID, deleteSnapshots)

	// Get AMI details to find snapshots if we need to delete them
	var snapshotIDs []string
	if deleteSnapshots {
		describeInput := &ec2.DescribeImagesInput{
			ImageIds: []string{amiID},
		}

		describeResult, err := o.clients.EC2.DescribeImages(ctx, describeInput)
		if err != nil {
			return fmt.Errorf("failed to describe AMI: %w", err)
		}

		if len(describeResult.Images) == 0 {
			return fmt.Errorf("AMI not found: %s", amiID)
		}

		// Collect snapshot IDs from block device mappings
		for _, bdm := range describeResult.Images[0].BlockDeviceMappings {
			if bdm.Ebs != nil && bdm.Ebs.SnapshotId != nil {
				snapshotIDs = append(snapshotIDs, *bdm.Ebs.SnapshotId)
			}
		}
	}

	// Deregister the AMI
	deregisterInput := &ec2.DeregisterImageInput{
		ImageId: aws.String(amiID),
	}

	_, err := o.clients.EC2.DeregisterImage(ctx, deregisterInput)
	if err != nil {
		return fmt.Errorf("failed to deregister AMI: %w", err)
	}

	logging.Info("AMI deregistered: %s", amiID)

	// Delete snapshots if requested
	if deleteSnapshots {
		for _, snapshotID := range snapshotIDs {
			if err := o.deleteSnapshot(ctx, snapshotID); err != nil {
				logging.Warn("Failed to delete snapshot %s: %v", snapshotID, err)
			}
		}
	}

	logging.Info("AMI deregistration complete: %s", amiID)
	return nil
}

// deleteSnapshot deletes an EBS snapshot
func (o *AMIOperations) deleteSnapshot(ctx context.Context, snapshotID string) error {
	logging.Debug("Deleting snapshot: %s", snapshotID)

	input := &ec2.DeleteSnapshotInput{
		SnapshotId: aws.String(snapshotID),
	}

	_, err := o.clients.EC2.DeleteSnapshot(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	logging.Debug("Snapshot deleted: %s", snapshotID)
	return nil
}

// waitForAMIAvailable waits for an AMI to become available
func (o *AMIOperations) waitForAMIAvailable(ctx context.Context, ec2Client *ec2.Client, amiID string) error {
	logging.Debug("Waiting for AMI to be available: %s", amiID)

	pollingInterval := time.Duration(o.globalConfig.AWS.AMI.PollingIntervalSec) * time.Second
	buildTimeout := time.Duration(o.globalConfig.AWS.AMI.BuildTimeoutMin) * time.Minute

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	timeout := time.After(buildTimeout)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		case <-timeout:
			return fmt.Errorf("timeout waiting for AMI to become available")
		case <-ticker.C:
			input := &ec2.DescribeImagesInput{
				ImageIds: []string{amiID},
			}

			result, err := ec2Client.DescribeImages(ctx, input)
			if err != nil {
				return fmt.Errorf("failed to describe AMI: %w", err)
			}

			if len(result.Images) == 0 {
				return fmt.Errorf("AMI not found: %s", amiID)
			}

			state := result.Images[0].State
			logging.Debug("AMI state: %s (ami: %s)", state, amiID)

			switch state {
			case types.ImageStateAvailable:
				return nil
			case types.ImageStateFailed, types.ImageStateInvalid, types.ImageStateDeregistered:
				return fmt.Errorf("AMI entered failed state: %s", state)
			case types.ImageStatePending:
				// Continue waiting
				continue
			default:
				return fmt.Errorf("unexpected AMI state: %s", state)
			}
		}
	}
}

// TagAMI adds tags to an AMI
func (o *AMIOperations) TagAMI(ctx context.Context, amiID string, tags map[string]string) error {
	if len(tags) == 0 {
		return nil
	}

	logging.Info("Tagging AMI %s with tags: %v", amiID, tags)

	ec2Tags := make([]types.Tag, 0, len(tags))
	for key, value := range tags {
		ec2Tags = append(ec2Tags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	input := &ec2.CreateTagsInput{
		Resources: []string{amiID},
		Tags:      ec2Tags,
	}

	_, err := o.clients.EC2.CreateTags(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to tag AMI: %w", err)
	}

	logging.Info("AMI tagged successfully: %s", amiID)
	return nil
}

// GetAMI retrieves AMI details
func (o *AMIOperations) GetAMI(ctx context.Context, amiID string) (*types.Image, error) {
	input := &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
	}

	result, err := o.clients.EC2.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe AMI: %w", err)
	}

	if len(result.Images) == 0 {
		return nil, fmt.Errorf("AMI not found: %s", amiID)
	}

	return &result.Images[0], nil
}

// ListAMIs lists AMIs owned by the caller with optional filters
func (o *AMIOperations) ListAMIs(ctx context.Context, filters map[string]string) ([]types.Image, error) {
	input := &ec2.DescribeImagesInput{
		Owners: []string{"self"},
	}

	// Add filters if provided
	if len(filters) > 0 {
		var ec2Filters []types.Filter
		for key, value := range filters {
			ec2Filters = append(ec2Filters, types.Filter{
				Name:   aws.String(key),
				Values: []string{value},
			})
		}
		input.Filters = ec2Filters
	}

	result, err := o.clients.EC2.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list AMIs: %w", err)
	}

	return result.Images, nil
}
