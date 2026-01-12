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

// errorPattern defines a pattern for matching and remediating errors
type errorPattern struct {
	patterns    []string // All patterns must match (AND logic within a pattern set)
	anyPatterns []string // Any of these patterns must match (OR logic)
	msgSuffix   string   // Suffix to add to the context message
	remediation string   // Remediation hint
}

// errorPatterns defines all known error patterns and their remediations
var errorPatterns = []errorPattern{
	{
		anyPatterns: []string{"ResourceAlreadyExistsException", "already exists"},
		msgSuffix:   "resource already exists",
		remediation: "Use --force to delete and recreate existing resources, or manually delete the resource in AWS Console/CLI",
	},
	{
		anyPatterns: []string{"InstanceProfileName", "instance profile"},
		msgSuffix:   "instance profile configuration error",
		remediation: "Ensure instance_profile_name is set in your template or global config (~/.warpgate/config.yaml under aws.ami.instance_profile_name). The profile must have permissions for EC2 Image Builder.",
	},
	{
		anyPatterns: []string{"SecurityGroup", "security group"},
		msgSuffix:   "security group error",
		remediation: "Verify that the specified security group IDs exist in the target VPC and region. Check that your AWS credentials have permission to use these security groups.",
	},
	{
		anyPatterns: []string{"SubnetId", "subnet"},
		msgSuffix:   "subnet configuration error",
		remediation: "Verify that the specified subnet ID exists in the target region. If building Windows AMIs, ensure the subnet has internet access for Windows Update.",
	},
	{
		patterns:    []string{"ami-"},
		anyPatterns: []string{"not found", "does not exist"},
		msgSuffix:   "source AMI not found",
		remediation: "Verify the base AMI ID exists in the target region. AMI IDs are region-specific. Use 'aws ec2 describe-images --image-ids <ami-id> --region <region>' to verify.",
	},
	{
		anyPatterns: []string{"region", "Region"},
		msgSuffix:   "region configuration error",
		remediation: "Specify the region using --region flag, in your template config, or in global config (~/.warpgate/config.yaml under aws.region).",
	},
	{
		anyPatterns: []string{"InstanceType", "instance type"},
		msgSuffix:   "instance type error",
		remediation: "Verify the instance type is available in the target region. For Windows builds, ensure sufficient memory (t3.medium or larger recommended).",
	},
	{
		anyPatterns: []string{"AccessDenied", "not authorized", "UnauthorizedAccess"},
		msgSuffix:   "permission denied",
		remediation: "Verify your AWS credentials have the required permissions for EC2 Image Builder. Required permissions include: imagebuilder:*, ec2:*, iam:PassRole for the instance profile.",
	},
	{
		anyPatterns: []string{"SemanticVersion", "version"},
		msgSuffix:   "component version error",
		remediation: "Component versions must be unique. Use --force to auto-increment versions, or specify a new version in your template.",
	},
	{
		anyPatterns: []string{"LimitExceeded", "quota", "limit"},
		msgSuffix:   "AWS service quota exceeded",
		remediation: "You've hit an AWS service limit. Check your EC2 Image Builder quotas in AWS Console. You may need to request a quota increase or clean up unused resources.",
	},
	{
		patterns:    []string{"pipeline", "failed"},
		msgSuffix:   "pipeline execution failed",
		remediation: "Check the AWS EC2 Image Builder console for detailed build logs. Common issues: provisioner script errors, network connectivity, or missing dependencies.",
	},
}

// WrapWithRemediation wraps an error with a remediation hint based on the error type
func WrapWithRemediation(err error, context string) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	for _, pattern := range errorPatterns {
		if matchesPattern(errMsg, pattern) {
			return &BuildError{
				Message:     fmt.Sprintf("%s: %s", context, pattern.msgSuffix),
				Cause:       err,
				Remediation: pattern.remediation,
			}
		}
	}

	return fmt.Errorf("%s: %w", context, err)
}

// matchesPattern checks if an error message matches the given pattern
func matchesPattern(errMsg string, p errorPattern) bool {
	// All required patterns must match
	for _, pat := range p.patterns {
		if !strings.Contains(errMsg, pat) {
			return false
		}
	}

	// If there are anyPatterns, at least one must match
	if len(p.anyPatterns) > 0 {
		matched := false
		for _, pat := range p.anyPatterns {
			if strings.Contains(errMsg, pat) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
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
