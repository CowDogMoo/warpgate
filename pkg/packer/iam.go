package packer

import (
	"fmt"
	"time"

	"github.com/l50/awsutils/iam"
)

// createIAMName generates a unique IAM role or instance profile name based on the blueprint name and timestamp.
//
// **Parameters:**
//
// baseName: The base name for the IAM role or instance profile.
//
// **Returns:**
//
// string: A unique IAM name.
func createIAMName(baseName string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%d", baseName, timestamp)
}

// CreateIAMResources creates an IAM role and instance profile with the necessary policies for Packer to build an AMI.
//
// **Parameters:**
//
// bucketName: The name of the S3 bucket to use for Packer builds.
//
// **Returns:**
//
// string: The name of the instance profile.
// string: The name of the IAM role.
// error: An error if the IAM resources could not be created.
func CreateIAMResources(bucketName string) (string, string, error) {
	// Initialize AWSService
	s, err := iam.NewAWSService()
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize AWS service: %v", err)
	}

	profileName := createIAMName("PackerInstanceProfile")
	roleName := createIAMName("PackerInstanceRole")

	// Create IAM role
	assumeRolePolicy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "ec2.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`

	_, err = s.CreateRole(roleName, assumeRolePolicy)
	if err != nil {
		return "", "", fmt.Errorf("failed to create IAM role: %v", err)
	}

	// Attach AmazonSSMManagedInstanceCore managed policy
	_, err = s.AttachRolePolicy(roleName, "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore")
	if err != nil {
		return "", "", fmt.Errorf("failed to attach AmazonSSMManagedInstanceCore policy: %v", err)
	}

	// Create custom policy for S3 access
	s3AccessPolicy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"cloudtrail:LookupEvents"
				],
				"Resource": "*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"s3:GetBucketLocation",
					"s3:CreateBucket",
					"s3:DeleteBucket"
				],
				"Resource": [
					"arn:aws:s3:::%[1]s",
					"arn:aws:s3:::%[1]s/*"
				]
			},
			{
				"Effect": "Allow",
				"Action": [
					"ec2:DescribeImages",
					"ec2:DescribeInstanceStatus",
					"ec2:DeleteSecurityGroup",
					"ec2:CreateKeyPair",
					"ec2:RunInstances",
					"ec2:DescribeSecurityGroups",
					"ec2:CreateSecurityGroup",
					"ec2:CreateTags",
					"ec2:DeleteKeyPair",
					"ec2:TerminateInstances",
					"ec2:DescribeSubnets",
					"ec2:DescribeInstances",
					"ec2:DescribeVpcs",
					"ec2:StopInstances",
					"ec2:CreateImage",
					"ec2:DescribeVolumes",
					"ec2:DescribeRegions",
					"ec2:DescribeInstanceTypeOfferings"
				],
				"Resource": "*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"ssm:ResumeSession",
					"ssm:TerminateSession",
					"ssm:StartSession"
				],
				"Resource": "*"
			}
		]
	}`, bucketName)
	_, err = s.PutRolePolicy(roleName, "S3AccessPolicy", s3AccessPolicy)
	if err != nil {
		return "", "", fmt.Errorf("failed to put S3 access policy: %v", err)
	}

	// Create instance profile
	_, err = s.CreateInstanceProfile(profileName)
	if err != nil {
		return "", "", fmt.Errorf("failed to create instance profile: %v", err)
	}

	// Add role to instance profile
	_, err = s.AddRoleToInstanceProfile(profileName, roleName)
	if err != nil {
		return "", "", fmt.Errorf("failed to add role to instance profile: %v", err)
	}

	return profileName, roleName, nil
}

// DestroyIAMResources removes the IAM role and instance profile created for Packer builds.
//
// **Parameters:**
//
// profileName: The name of the instance profile.
// roleName: The name of the IAM role.
//
// **Returns:**
//
// error: An error if the IAM resources could not be destroyed.
func DestroyIAMResources(profileName, roleName string) error {
	// Initialize AWSService
	s, err := iam.NewAWSService()
	if err != nil {
		return fmt.Errorf("failed to initialize AWS service: %v", err)
	}

	// Detach AmazonSSMManagedInstanceCore managed policy
	_, err = s.DetachRolePolicy(roleName, "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore")
	if err != nil {
		return fmt.Errorf("failed to detach AmazonSSMManagedInstanceCore policy: %v", err)
	}

	// Remove inline policy
	_, err = s.DeleteRolePolicy(roleName, "S3AccessPolicy")
	if err != nil {
		return fmt.Errorf("failed to delete S3 access policy: %v", err)
	}

	// Remove role from instance profile
	_, err = s.RemoveRoleFromInstanceProfile(profileName, roleName)
	if err != nil {
		return fmt.Errorf("failed to remove role from instance profile: %v", err)
	}

	// Delete instance profile
	_, err = s.DeleteInstanceProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to delete instance profile: %v", err)
	}

	// Delete IAM role
	_, err = s.DeleteRole(roleName)
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %v", err)
	}

	return nil
}
