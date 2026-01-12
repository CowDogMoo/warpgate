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
	"fmt"
	"strings"
)

// BuildError represents an AMI build error with remediation hints
type BuildError struct {
	Message     string
	Cause       error
	Remediation string
}

func (e *BuildError) Error() string {
	if e.Remediation != "" {
		return fmt.Sprintf("%s\n\nRemediation: %s", e.Message, e.Remediation)
	}
	return e.Message
}

func (e *BuildError) Unwrap() error {
	return e.Cause
}

// WrapWithRemediation wraps an error with a remediation hint based on the error type
func WrapWithRemediation(err error, context string) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Resource already exists errors
	if strings.Contains(errMsg, "ResourceAlreadyExistsException") || strings.Contains(errMsg, "already exists") {
		resourceType := extractResourceType(errMsg)
		return &BuildError{
			Message:     fmt.Sprintf("%s: %s already exists", context, resourceType),
			Cause:       err,
			Remediation: fmt.Sprintf("Use --force to delete and recreate existing resources, or manually delete the %s in AWS Console/CLI", resourceType),
		}
	}

	// IAM role/instance profile errors
	if strings.Contains(errMsg, "InstanceProfileName") || strings.Contains(errMsg, "instance profile") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: instance profile configuration error", context),
			Cause:       err,
			Remediation: "Ensure instance_profile_name is set in your template or global config (~/.warpgate/config.yaml under aws.ami.instance_profile_name). The profile must have permissions for EC2 Image Builder.",
		}
	}

	// Security group errors
	if strings.Contains(errMsg, "SecurityGroup") || strings.Contains(errMsg, "security group") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: security group error", context),
			Cause:       err,
			Remediation: "Verify that the specified security group IDs exist in the target VPC and region. Check that your AWS credentials have permission to use these security groups.",
		}
	}

	// Subnet errors
	if strings.Contains(errMsg, "SubnetId") || strings.Contains(errMsg, "subnet") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: subnet configuration error", context),
			Cause:       err,
			Remediation: "Verify that the specified subnet ID exists in the target region. If building Windows AMIs, ensure the subnet has internet access for Windows Update.",
		}
	}

	// AMI not found errors
	if strings.Contains(errMsg, "ami-") && (strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "does not exist")) {
		return &BuildError{
			Message:     fmt.Sprintf("%s: source AMI not found", context),
			Cause:       err,
			Remediation: "Verify the base AMI ID exists in the target region. AMI IDs are region-specific. Use 'aws ec2 describe-images --image-ids <ami-id> --region <region>' to verify.",
		}
	}

	// Region errors
	if strings.Contains(errMsg, "region") || strings.Contains(errMsg, "Region") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: region configuration error", context),
			Cause:       err,
			Remediation: "Specify the region using --region flag, in your template config, or in global config (~/.warpgate/config.yaml under aws.region).",
		}
	}

	// Instance type errors
	if strings.Contains(errMsg, "InstanceType") || strings.Contains(errMsg, "instance type") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: instance type error", context),
			Cause:       err,
			Remediation: "Verify the instance type is available in the target region. For Windows builds, ensure sufficient memory (t3.medium or larger recommended).",
		}
	}

	// Permission/authorization errors
	if strings.Contains(errMsg, "AccessDenied") || strings.Contains(errMsg, "not authorized") || strings.Contains(errMsg, "UnauthorizedAccess") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: permission denied", context),
			Cause:       err,
			Remediation: "Verify your AWS credentials have the required permissions for EC2 Image Builder. Required permissions include: imagebuilder:*, ec2:*, iam:PassRole for the instance profile.",
		}
	}

	// Component version errors
	if strings.Contains(errMsg, "SemanticVersion") || strings.Contains(errMsg, "version") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: component version error", context),
			Cause:       err,
			Remediation: "Component versions must be unique. Use --force to auto-increment versions, or specify a new version in your template.",
		}
	}

	// Quota/limit errors
	if strings.Contains(errMsg, "LimitExceeded") || strings.Contains(errMsg, "quota") || strings.Contains(errMsg, "limit") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: AWS service quota exceeded", context),
			Cause:       err,
			Remediation: "You've hit an AWS service limit. Check your EC2 Image Builder quotas in AWS Console. You may need to request a quota increase or clean up unused resources.",
		}
	}

	// Pipeline execution errors
	if strings.Contains(errMsg, "pipeline") && strings.Contains(errMsg, "failed") {
		return &BuildError{
			Message:     fmt.Sprintf("%s: pipeline execution failed", context),
			Cause:       err,
			Remediation: "Check the AWS EC2 Image Builder console for detailed build logs. Common issues: provisioner script errors, network connectivity, or missing dependencies.",
		}
	}

	// Default: return original error wrapped with context
	return fmt.Errorf("%s: %w", context, err)
}

// extractResourceType attempts to extract the resource type from an error message
func extractResourceType(errMsg string) string {
	resourceTypes := []string{
		"InfrastructureConfiguration",
		"DistributionConfiguration",
		"ImageRecipe",
		"ImagePipeline",
		"Component",
	}

	for _, rt := range resourceTypes {
		if strings.Contains(errMsg, rt) {
			return rt
		}
	}

	return "resource"
}

// ValidatePrerequisites checks common prerequisites before starting a build
func ValidatePrerequisites(region, instanceProfile, parentImage string) error {
	var errors []string

	if region == "" {
		errors = append(errors, "AWS region is not specified. Use --region flag or set in config")
	}

	if instanceProfile == "" {
		errors = append(errors, "Instance profile is not specified. Set instance_profile_name in template or global config")
	}

	if parentImage == "" {
		errors = append(errors, "Parent image (base AMI) is not specified. Set base.image in template or aws.ami.default_parent_image in global config")
	}

	if len(errors) > 0 {
		return &BuildError{
			Message:     "Build prerequisites not met",
			Remediation: strings.Join(errors, "\n  - "),
		}
	}

	return nil
}
