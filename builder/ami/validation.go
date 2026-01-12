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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// ValidationResult contains the results of a dry-run validation
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
	Info     []string
}

// AddError adds an error to the validation result
func (v *ValidationResult) AddError(format string, args ...interface{}) {
	v.Errors = append(v.Errors, fmt.Sprintf(format, args...))
	v.Valid = false
}

// AddWarning adds a warning to the validation result
func (v *ValidationResult) AddWarning(format string, args ...interface{}) {
	v.Warnings = append(v.Warnings, fmt.Sprintf(format, args...))
}

// AddInfo adds an informational message to the validation result
func (v *ValidationResult) AddInfo(format string, args ...interface{}) {
	v.Info = append(v.Info, fmt.Sprintf(format, args...))
}

// String returns a formatted string representation of the validation result
func (v *ValidationResult) String() string {
	var sb strings.Builder

	if v.Valid {
		sb.WriteString("Validation PASSED\n\n")
	} else {
		sb.WriteString("Validation FAILED\n\n")
	}

	if len(v.Errors) > 0 {
		sb.WriteString("Errors:\n")
		for _, e := range v.Errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
		sb.WriteString("\n")
	}

	if len(v.Warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range v.Warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
		sb.WriteString("\n")
	}

	if len(v.Info) > 0 {
		sb.WriteString("Info:\n")
		for _, i := range v.Info {
			sb.WriteString(fmt.Sprintf("  - %s\n", i))
		}
	}

	return sb.String()
}

// Validator performs dry-run validation of AMI build configurations
type Validator struct {
	clients *AWSClients
}

// NewValidator creates a new validator
func NewValidator(clients *AWSClients) *Validator {
	return &Validator{
		clients: clients,
	}
}

// ValidateBuild performs comprehensive validation of a build configuration
func (v *Validator) ValidateBuild(ctx context.Context, config builder.Config, target *builder.Target) *ValidationResult {
	result := &ValidationResult{Valid: true}

	logging.Info("Running dry-run validation...")

	// Validate basic configuration
	v.validateBasicConfig(result, config, target)

	// Validate AWS resources if we have clients
	if v.clients != nil {
		v.validateAWSResources(ctx, result, config, target)
	}

	// Validate provisioners
	v.validateProvisioners(result, config)

	return result
}

// validateBasicConfig validates basic configuration fields
func (v *Validator) validateBasicConfig(result *ValidationResult, config builder.Config, target *builder.Target) {
	// Check template name
	if config.Name == "" {
		result.AddError("Template name is required")
	} else {
		result.AddInfo("Template name: %s", config.Name)
	}

	// Check version
	if config.Version == "" {
		result.AddError("Template version is required")
	} else {
		result.AddInfo("Template version: %s", config.Version)
	}

	// Check region
	region := target.Region
	if region == "" && v.clients != nil {
		region = v.clients.GetRegion()
	}
	if region == "" {
		result.AddError("AWS region must be specified (--region flag, template config, or global config)")
	} else {
		result.AddInfo("Target region: %s", region)
	}

	// Check base image
	if config.Base.Image == "" {
		result.AddError("Base image (parent AMI) must be specified in template config (base.image)")
	} else {
		result.AddInfo("Base image: %s", config.Base.Image)
	}

	// Check instance type
	if target.InstanceType == "" {
		result.AddWarning("Instance type not specified, will use default from global config")
	} else {
		result.AddInfo("Instance type: %s", target.InstanceType)
	}

	// Check instance profile
	if target.InstanceProfileName == "" {
		result.AddWarning("Instance profile not specified in template, will use global config value")
	} else {
		result.AddInfo("Instance profile: %s", target.InstanceProfileName)
	}
}

// validateAWSResources validates AWS resources exist and are accessible
func (v *Validator) validateAWSResources(ctx context.Context, result *ValidationResult, config builder.Config, target *builder.Target) {
	// Validate base AMI exists
	if config.Base.Image != "" && strings.HasPrefix(config.Base.Image, "ami-") {
		v.validateAMI(ctx, result, config.Base.Image)
	}

	// Validate instance profile exists
	if target.InstanceProfileName != "" {
		v.validateInstanceProfile(ctx, result, target.InstanceProfileName)
	}

	// Validate subnet exists if specified
	if target.SubnetID != "" {
		v.validateSubnet(ctx, result, target.SubnetID)
	}

	// Validate security groups exist if specified
	if len(target.SecurityGroupIDs) > 0 {
		v.validateSecurityGroups(ctx, result, target.SecurityGroupIDs)
	}

	// Check for existing resources that would conflict
	v.checkExistingResources(ctx, result, config.Name)
}

// validateAMI checks if an AMI exists and is available
func (v *Validator) validateAMI(ctx context.Context, result *ValidationResult, amiID string) {
	input := &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
	}

	output, err := v.clients.EC2.DescribeImages(ctx, input)
	if err != nil {
		result.AddError("Failed to validate base AMI %s: %v", amiID, err)
		return
	}

	if len(output.Images) == 0 {
		result.AddError("Base AMI %s not found in region %s", amiID, v.clients.GetRegion())
		return
	}

	image := output.Images[0]
	if image.State != ec2types.ImageStateAvailable {
		result.AddError("Base AMI %s is not available (state: %s)", amiID, image.State)
		return
	}

	// Add info about the AMI
	name := "unnamed"
	if image.Name != nil {
		name = *image.Name
	}
	result.AddInfo("Base AMI validated: %s (%s)", amiID, name)

	// Check platform
	if image.Platform == ec2types.PlatformValuesWindows {
		result.AddInfo("Base AMI platform: Windows")
	} else {
		result.AddInfo("Base AMI platform: Linux")
	}
}

// validateInstanceProfile checks if an IAM instance profile exists and has the required permissions
func (v *Validator) validateInstanceProfile(ctx context.Context, result *ValidationResult, profileName string) {
	input := &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	}

	output, err := v.clients.IAM.GetInstanceProfile(ctx, input)
	if err != nil {
		result.AddError("Failed to validate instance profile '%s': %v", profileName, err)
		result.AddWarning("Ensure the instance profile exists and your credentials have iam:GetInstanceProfile permission")
		return
	}

	if output.InstanceProfile == nil {
		result.AddError("Instance profile '%s' not found", profileName)
		return
	}

	result.AddInfo("Instance profile validated: %s", profileName)

	// Check if the instance profile has any roles attached
	if len(output.InstanceProfile.Roles) == 0 {
		result.AddError("Instance profile '%s' has no IAM role attached", profileName)
		result.AddWarning("The instance profile must have an IAM role with permissions for EC2 Image Builder")
		return
	}

	// Get the role name and check its policies
	roleName := aws.ToString(output.InstanceProfile.Roles[0].RoleName)
	result.AddInfo("Associated IAM role: %s", roleName)

	// Check if the role has the required trust policy for EC2
	v.validateRoleTrustPolicy(ctx, result, roleName)

	// Check for recommended managed policies
	v.validateRolePolicies(ctx, result, roleName)
}

// validateRoleTrustPolicy checks if the role can be assumed by EC2
func (v *Validator) validateRoleTrustPolicy(ctx context.Context, result *ValidationResult, roleName string) {
	input := &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	}

	output, err := v.clients.IAM.GetRole(ctx, input)
	if err != nil {
		result.AddWarning("Could not validate role trust policy: %v", err)
		return
	}

	if output.Role != nil && output.Role.AssumeRolePolicyDocument != nil {
		trustPolicy := aws.ToString(output.Role.AssumeRolePolicyDocument)
		// Check if EC2 is in the trust policy
		if !strings.Contains(trustPolicy, "ec2.amazonaws.com") {
			result.AddWarning("Role '%s' may not allow EC2 to assume it. Ensure the trust policy includes ec2.amazonaws.com", roleName)
		} else {
			result.AddInfo("Role trust policy allows EC2 assumption")
		}
	}
}

// validateRolePolicies checks if the role has recommended policies
func (v *Validator) validateRolePolicies(ctx context.Context, result *ValidationResult, roleName string) {
	// List attached managed policies
	input := &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	}

	output, err := v.clients.IAM.ListAttachedRolePolicies(ctx, input)
	if err != nil {
		result.AddWarning("Could not list role policies: %v", err)
		return
	}

	// Check for recommended policies
	recommendedPolicies := map[string]bool{
		"EC2InstanceProfileForImageBuilder":                   false,
		"EC2InstanceProfileForImageBuilderECRContainerBuilds": false,
		"AmazonSSMManagedInstanceCore":                        false,
	}

	for _, policy := range output.AttachedPolicies {
		policyName := aws.ToString(policy.PolicyName)
		if _, ok := recommendedPolicies[policyName]; ok {
			recommendedPolicies[policyName] = true
			result.AddInfo("Recommended policy attached: %s", policyName)
		}
	}

	// Check if any recommended policies are missing
	missingPolicies := []string{}
	for policy, attached := range recommendedPolicies {
		if !attached {
			missingPolicies = append(missingPolicies, policy)
		}
	}

	if len(missingPolicies) > 0 && !recommendedPolicies["EC2InstanceProfileForImageBuilder"] {
		result.AddWarning("Consider attaching 'EC2InstanceProfileForImageBuilder' policy to the role for proper Image Builder permissions")
	}
}

// validateSubnet checks if a subnet exists
func (v *Validator) validateSubnet(ctx context.Context, result *ValidationResult, subnetID string) {
	input := &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetID},
	}

	output, err := v.clients.EC2.DescribeSubnets(ctx, input)
	if err != nil {
		result.AddError("Failed to validate subnet %s: %v", subnetID, err)
		return
	}

	if len(output.Subnets) == 0 {
		result.AddError("Subnet %s not found", subnetID)
		return
	}

	subnet := output.Subnets[0]
	result.AddInfo("Subnet validated: %s (VPC: %s, AZ: %s)",
		subnetID,
		aws.ToString(subnet.VpcId),
		aws.ToString(subnet.AvailabilityZone))
}

// validateSecurityGroups checks if security groups exist
func (v *Validator) validateSecurityGroups(ctx context.Context, result *ValidationResult, sgIDs []string) {
	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: sgIDs,
	}

	output, err := v.clients.EC2.DescribeSecurityGroups(ctx, input)
	if err != nil {
		result.AddError("Failed to validate security groups: %v", err)
		return
	}

	foundIDs := make(map[string]bool)
	for _, sg := range output.SecurityGroups {
		if sg.GroupId != nil {
			foundIDs[*sg.GroupId] = true
			result.AddInfo("Security group validated: %s (%s)",
				*sg.GroupId,
				aws.ToString(sg.GroupName))
		}
	}

	for _, id := range sgIDs {
		if !foundIDs[id] {
			result.AddError("Security group %s not found", id)
		}
	}
}

// checkExistingResources checks for existing Image Builder resources that would conflict
func (v *Validator) checkExistingResources(ctx context.Context, result *ValidationResult, buildName string) {
	resourceManager := NewResourceManager(v.clients)

	// Check for existing infrastructure config
	infraName := fmt.Sprintf("%s-infra", buildName)
	if infra, err := resourceManager.GetInfrastructureConfig(ctx, infraName); err == nil && infra != nil {
		result.AddWarning("Infrastructure configuration '%s' already exists (will be reused unless --force is specified)", infraName)
	}

	// Check for existing distribution config
	distName := fmt.Sprintf("%s-dist", buildName)
	if dist, err := resourceManager.GetDistributionConfig(ctx, distName); err == nil && dist != nil {
		result.AddWarning("Distribution configuration '%s' already exists (will be reused unless --force is specified)", distName)
	}

	// Check for existing image recipe
	recipeName := fmt.Sprintf("%s-recipe", buildName)
	if recipe, err := resourceManager.GetImageRecipe(ctx, recipeName, ""); err == nil && recipe != nil {
		result.AddWarning("Image recipe '%s' already exists (will be reused unless --force is specified)", recipeName)
	}

	// Check for existing pipeline
	pipelineName := fmt.Sprintf("%s-pipeline", buildName)
	if pipeline, err := resourceManager.GetImagePipeline(ctx, pipelineName); err == nil && pipeline != nil {
		result.AddWarning("Image pipeline '%s' already exists (will be reused unless --force is specified)", pipelineName)
	}
}

// validateProvisioners validates the provisioner configurations
func (v *Validator) validateProvisioners(result *ValidationResult, config builder.Config) {
	if len(config.Provisioners) == 0 {
		result.AddWarning("No provisioners specified - the AMI will be built with only the base image")
		return
	}

	result.AddInfo("Provisioners: %d configured", len(config.Provisioners))

	for i, p := range config.Provisioners {
		switch p.Type {
		case "shell":
			if len(p.Inline) == 0 {
				result.AddError("Provisioner %d (shell): no inline commands specified", i)
			} else {
				result.AddInfo("Provisioner %d: shell with %d commands", i, len(p.Inline))
			}
		case "script":
			if len(p.Scripts) == 0 {
				result.AddError("Provisioner %d (script): no scripts specified", i)
			} else {
				result.AddInfo("Provisioner %d: script with %d files", i, len(p.Scripts))
			}
		case "ansible":
			if p.PlaybookPath == "" {
				result.AddError("Provisioner %d (ansible): no playbook path specified", i)
			} else {
				result.AddInfo("Provisioner %d: ansible with playbook %s", i, p.PlaybookPath)
			}
		case "powershell":
			if len(p.PSScripts) == 0 {
				result.AddError("Provisioner %d (powershell): no PowerShell scripts specified", i)
			} else {
				result.AddInfo("Provisioner %d: powershell with %d scripts", i, len(p.PSScripts))
			}
		default:
			result.AddError("Provisioner %d: unknown type '%s'", i, p.Type)
		}
	}
}

// DryRun performs a dry-run validation and returns the result
func (b *ImageBuilder) DryRun(ctx context.Context, config builder.Config) (*ValidationResult, error) {
	// Find AMI target
	var amiTarget *builder.Target
	for i := range config.Targets {
		if config.Targets[i].Type == "ami" {
			amiTarget = &config.Targets[i]
			break
		}
	}

	if amiTarget == nil {
		return nil, fmt.Errorf("no AMI target found in configuration")
	}

	validator := NewValidator(b.clients)
	result := validator.ValidateBuild(ctx, config, amiTarget)

	return result, nil
}
