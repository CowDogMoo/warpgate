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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
)

func TestValidationResultAddError(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true}
	result.AddError("error: %s", "test")

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "error: test", result.Errors[0])
}

func TestValidationResultAddWarning(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true}
	result.AddWarning("warning: %d", 42)

	assert.True(t, result.Valid)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "warning: 42", result.Warnings[0])
}

func TestValidationResultAddInfo(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true}
	result.AddInfo("info: %s %s", "hello", "world")

	assert.Len(t, result.Info, 1)
	assert.Equal(t, "info: hello world", result.Info[0])
}

func TestValidationResultString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     ValidationResult
		wantParts  []string
		wantAbsent []string
	}{
		{
			name:      "passed with no messages",
			result:    ValidationResult{Valid: true},
			wantParts: []string{"Validation PASSED"},
		},
		{
			name: "failed with errors",
			result: ValidationResult{
				Valid:  false,
				Errors: []string{"missing region", "missing profile"},
			},
			wantParts: []string{"Validation FAILED", "Errors:", "missing region", "missing profile"},
		},
		{
			name: "passed with warnings and info",
			result: ValidationResult{
				Valid:    true,
				Warnings: []string{"instance type not set"},
				Info:     []string{"template: my-template"},
			},
			wantParts:  []string{"Validation PASSED", "Warnings:", "instance type not set", "Info:", "template: my-template"},
			wantAbsent: []string{"Errors:"},
		},
		{
			name: "all sections present",
			result: ValidationResult{
				Valid:    false,
				Errors:   []string{"err1"},
				Warnings: []string{"warn1"},
				Info:     []string{"info1"},
			},
			wantParts: []string{"FAILED", "Errors:", "err1", "Warnings:", "warn1", "Info:", "info1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			output := tc.result.String()
			for _, part := range tc.wantParts {
				assert.Contains(t, output, part)
			}
			for _, absent := range tc.wantAbsent {
				assert.NotContains(t, output, absent)
			}
		})
	}
}

func TestNewValidator(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)
	assert.NotNil(t, v)
	assert.Nil(t, v.clients)
}

func TestValidateProvisioners(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)

	tests := []struct {
		name          string
		provisioners  []builder.Provisioner
		wantErrors    int
		wantWarnings  int
		wantInfoCount int
	}{
		{
			name:         "no provisioners warns",
			provisioners: nil,
			wantWarnings: 1,
		},
		{
			name: "valid shell provisioner",
			provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo hello"}},
			},
			wantInfoCount: 2, // count + detail
		},
		{
			name: "shell without inline errors",
			provisioners: []builder.Provisioner{
				{Type: "shell"},
			},
			wantErrors:    1,
			wantInfoCount: 1, // count only
		},
		{
			name: "valid ansible provisioner",
			provisioners: []builder.Provisioner{
				{Type: "ansible", PlaybookPath: "/path/to/playbook.yml"},
			},
			wantInfoCount: 2,
		},
		{
			name: "ansible without playbook errors",
			provisioners: []builder.Provisioner{
				{Type: "ansible"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "valid script provisioner",
			provisioners: []builder.Provisioner{
				{Type: "script", Scripts: []string{"setup.sh"}},
			},
			wantInfoCount: 2,
		},
		{
			name: "script without scripts errors",
			provisioners: []builder.Provisioner{
				{Type: "script"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "valid powershell provisioner",
			provisioners: []builder.Provisioner{
				{Type: "powershell", PSScripts: []string{"setup.ps1"}},
			},
			wantInfoCount: 2,
		},
		{
			name: "powershell without scripts errors",
			provisioners: []builder.Provisioner{
				{Type: "powershell"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "unknown provisioner type errors",
			provisioners: []builder.Provisioner{
				{Type: "terraform"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "multiple provisioners mixed",
			provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo hello"}},
				{Type: "ansible"},
			},
			wantErrors:    1,
			wantInfoCount: 2, // count + 1 valid shell detail
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := &ValidationResult{Valid: true}
			cfg := builder.Config{Provisioners: tc.provisioners}
			v.validateProvisioners(result, cfg)

			assert.Len(t, result.Errors, tc.wantErrors)
			assert.Len(t, result.Warnings, tc.wantWarnings)
			assert.Len(t, result.Info, tc.wantInfoCount)
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateAMI
// ---------------------------------------------------------------------------

func TestValidateAMI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		amiID        string
		setupMock    func(*MockEC2Client)
		wantErrors   int
		wantInfo     int
		wantContains []string
	}{
		{
			name:  "AMI found and available",
			amiID: "ami-12345678",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []ec2types.Image{
							{
								ImageId:  aws.String("ami-12345678"),
								Name:     aws.String("my-base-image"),
								State:    ec2types.ImageStateAvailable,
								Platform: "",
							},
						},
					}, nil
				}
			},
			wantErrors:   0,
			wantInfo:     2, // "Base AMI validated" + "Base AMI platform: Linux"
			wantContains: []string{"Base AMI validated", "my-base-image", "Linux"},
		},
		{
			name:  "AMI found but not available",
			amiID: "ami-pending123",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []ec2types.Image{
							{
								ImageId: aws.String("ami-pending123"),
								Name:    aws.String("pending-image"),
								State:   ec2types.ImageStatePending,
							},
						},
					}, nil
				}
			},
			wantErrors:   1,
			wantInfo:     0,
			wantContains: []string{"not available"},
		},
		{
			name:  "AMI not found empty images",
			amiID: "ami-nonexistent",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []ec2types.Image{},
					}, nil
				}
			},
			wantErrors:   1,
			wantInfo:     0,
			wantContains: []string{"not found"},
		},
		{
			name:  "API error",
			amiID: "ami-error123",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return nil, fmt.Errorf("access denied")
				}
			},
			wantErrors:   1,
			wantInfo:     0,
			wantContains: []string{"Failed to validate base AMI", "access denied"},
		},
		{
			name:  "Windows platform",
			amiID: "ami-windows123",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []ec2types.Image{
							{
								ImageId:  aws.String("ami-windows123"),
								Name:     aws.String("windows-2022-base"),
								State:    ec2types.ImageStateAvailable,
								Platform: ec2types.PlatformValuesWindows,
							},
						},
					}, nil
				}
			},
			wantErrors:   0,
			wantInfo:     2, // "Base AMI validated" + "Base AMI platform: Windows"
			wantContains: []string{"Base AMI validated", "Windows"},
		},
		{
			name:  "unnamed AMI",
			amiID: "ami-unnamed123",
			setupMock: func(m *MockEC2Client) {
				m.DescribeImagesFunc = func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []ec2types.Image{
							{
								ImageId: aws.String("ami-unnamed123"),
								Name:    nil,
								State:   ec2types.ImageStateAvailable,
							},
						},
					}, nil
				}
			},
			wantErrors:   0,
			wantInfo:     2,
			wantContains: []string{"unnamed"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.ec2)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateAMI(context.Background(), result, tc.amiID)

			assert.Len(t, result.Errors, tc.wantErrors)
			assert.Len(t, result.Info, tc.wantInfo)
			for _, s := range tc.wantContains {
				found := false
				for _, e := range result.Errors {
					if assert.ObjectsAreEqual(true, containsStr(e, s)) {
						found = true
						break
					}
				}
				if !found {
					for _, i := range result.Info {
						if containsStr(i, s) {
							found = true
							break
						}
					}
				}
				assert.True(t, found, "expected to find %q in errors or info messages", s)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateInstanceType
// ---------------------------------------------------------------------------

func TestValidateInstanceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		instanceType string
		setupMock    func(*MockEC2Client)
		wantErrors   int
		wantWarnings int
		wantInfo     int
	}{
		{
			name:         "valid instance type with vCPU and memory",
			instanceType: "t3.medium",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypesFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []ec2types.InstanceTypeInfo{
							{
								InstanceType: ec2types.InstanceTypeT3Medium,
								VCpuInfo: &ec2types.VCpuInfo{
									DefaultVCpus: aws.Int32(2),
								},
								MemoryInfo: &ec2types.MemoryInfo{
									SizeInMiB: aws.Int64(4096),
								},
								ProcessorInfo: &ec2types.ProcessorInfo{
									SupportedArchitectures: []ec2types.ArchitectureType{
										ec2types.ArchitectureTypeX8664,
									},
								},
								BurstablePerformanceSupported: aws.Bool(false),
							},
						},
					}, nil
				}
				m.DescribeInstanceTypeOfferingsFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
					return &ec2.DescribeInstanceTypeOfferingsOutput{
						InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
							{Location: aws.String("us-east-1a")},
							{Location: aws.String("us-east-1b")},
						},
					}, nil
				}
			},
			wantErrors: 0,
			wantInfo:   3, // "Instance type validated" + "Supported architectures" + "available in 2 availability zones"
		},
		{
			name:         "instance type not found empty result",
			instanceType: "x99.superlarge",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypesFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []ec2types.InstanceTypeInfo{},
					}, nil
				}
			},
			wantErrors: 1,
			wantInfo:   0,
		},
		{
			name:         "API error with InvalidInstanceType",
			instanceType: "bad.type",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypesFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return nil, fmt.Errorf("InvalidInstanceType: bad.type is not a valid instance type")
				}
			},
			wantErrors:   1,
			wantWarnings: 1, // "Common instance types" suggestion
			wantInfo:     0,
		},
		{
			name:         "API error other",
			instanceType: "t3.micro",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypesFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return nil, fmt.Errorf("connection timeout")
				}
			},
			wantErrors:   0,
			wantWarnings: 1, // "Could not validate instance type"
			wantInfo:     0,
		},
		{
			name:         "burstable instance type",
			instanceType: "t3.micro",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypesFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []ec2types.InstanceTypeInfo{
							{
								InstanceType: ec2types.InstanceTypeT3Micro,
								VCpuInfo: &ec2types.VCpuInfo{
									DefaultVCpus: aws.Int32(2),
								},
								MemoryInfo: &ec2types.MemoryInfo{
									SizeInMiB: aws.Int64(1024),
								},
								BurstablePerformanceSupported: aws.Bool(true),
							},
						},
					}, nil
				}
				m.DescribeInstanceTypeOfferingsFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
					return &ec2.DescribeInstanceTypeOfferingsOutput{
						InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
							{Location: aws.String("us-east-1a")},
						},
					}, nil
				}
			},
			wantErrors: 0,
			wantInfo:   3, // "Instance type validated" + "burstable performance" + "available in 1 availability zones"
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.ec2)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateInstanceType(context.Background(), result, tc.instanceType)

			assert.Len(t, result.Errors, tc.wantErrors, "unexpected error count")
			assert.Len(t, result.Warnings, tc.wantWarnings, "unexpected warning count")
			assert.Len(t, result.Info, tc.wantInfo, "unexpected info count")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateInstanceTypeAvailability
// ---------------------------------------------------------------------------

func TestValidateInstanceTypeAvailability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		instanceType string
		setupMock    func(*MockEC2Client)
		wantErrors   int
		wantWarnings int
		wantInfo     int
	}{
		{
			name:         "available in zones",
			instanceType: "t3.medium",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypeOfferingsFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
					return &ec2.DescribeInstanceTypeOfferingsOutput{
						InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
							{Location: aws.String("us-east-1a")},
							{Location: aws.String("us-east-1b")},
							{Location: aws.String("us-east-1c")},
						},
					}, nil
				}
			},
			wantInfo: 1,
		},
		{
			name:         "not available in any zone",
			instanceType: "p4d.24xlarge",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypeOfferingsFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
					return &ec2.DescribeInstanceTypeOfferingsOutput{
						InstanceTypeOfferings: []ec2types.InstanceTypeOffering{},
					}, nil
				}
			},
			wantErrors: 1,
		},
		{
			name:         "API error",
			instanceType: "t3.medium",
			setupMock: func(m *MockEC2Client) {
				m.DescribeInstanceTypeOfferingsFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
					return nil, fmt.Errorf("throttling exception")
				}
			},
			wantWarnings: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.ec2)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateInstanceTypeAvailability(context.Background(), result, tc.instanceType)

			assert.Len(t, result.Errors, tc.wantErrors, "unexpected error count")
			assert.Len(t, result.Warnings, tc.wantWarnings, "unexpected warning count")
			assert.Len(t, result.Info, tc.wantInfo, "unexpected info count")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateInstanceProfile
// ---------------------------------------------------------------------------

func TestValidateInstanceProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		profileName  string
		setupMock    func(*MockIAMClient)
		wantErrors   int
		wantWarnings int
		wantMinInfo  int
	}{
		{
			name:        "profile found with role",
			profileName: "my-build-profile",
			setupMock: func(m *MockIAMClient) {
				m.GetInstanceProfileFunc = func(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
					return &iam.GetInstanceProfileOutput{
						InstanceProfile: &iamtypes.InstanceProfile{
							InstanceProfileName: aws.String("my-build-profile"),
							Roles: []iamtypes.Role{
								{RoleName: aws.String("my-build-role")},
							},
						},
					}, nil
				}
				m.GetRoleFunc = func(_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
					return &iam.GetRoleOutput{
						Role: &iamtypes.Role{
							RoleName:                 aws.String("my-build-role"),
							AssumeRolePolicyDocument: aws.String(`{"Statement":[{"Principal":{"Service":"ec2.amazonaws.com"}}]}`),
						},
					}, nil
				}
				m.ListAttachedRolePoliciesFunc = func(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
					return &iam.ListAttachedRolePoliciesOutput{
						AttachedPolicies: []iamtypes.AttachedPolicy{
							{PolicyName: aws.String("EC2InstanceProfileForImageBuilder")},
							{PolicyName: aws.String("AmazonSSMManagedInstanceCore")},
						},
					}, nil
				}
			},
			wantErrors:  0,
			wantMinInfo: 3, // "Instance profile validated" + "Associated IAM role" + trust policy info + policy info
		},
		{
			name:        "profile not found API error",
			profileName: "nonexistent-profile",
			setupMock: func(m *MockIAMClient) {
				m.GetInstanceProfileFunc = func(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
					return nil, fmt.Errorf("NoSuchEntity: Instance profile nonexistent-profile not found")
				}
			},
			wantErrors:   1,
			wantWarnings: 1, // "Ensure the instance profile exists"
		},
		{
			name:        "profile nil in response",
			profileName: "nil-profile",
			setupMock: func(m *MockIAMClient) {
				m.GetInstanceProfileFunc = func(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
					return &iam.GetInstanceProfileOutput{
						InstanceProfile: nil,
					}, nil
				}
			},
			wantErrors: 1,
		},
		{
			name:        "no roles attached",
			profileName: "empty-role-profile",
			setupMock: func(m *MockIAMClient) {
				m.GetInstanceProfileFunc = func(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
					return &iam.GetInstanceProfileOutput{
						InstanceProfile: &iamtypes.InstanceProfile{
							InstanceProfileName: aws.String("empty-role-profile"),
							Roles:               []iamtypes.Role{},
						},
					}, nil
				}
			},
			wantErrors:   1, // "has no IAM role attached"
			wantWarnings: 1, // "must have an IAM role"
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.iam)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateInstanceProfile(context.Background(), result, tc.profileName)

			assert.Len(t, result.Errors, tc.wantErrors, "unexpected error count")
			assert.GreaterOrEqual(t, len(result.Warnings), tc.wantWarnings, "unexpected warning count")
			if tc.wantMinInfo > 0 {
				assert.GreaterOrEqual(t, len(result.Info), tc.wantMinInfo, "unexpected info count")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateRoleTrustPolicy
// ---------------------------------------------------------------------------

func TestValidateRoleTrustPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roleName     string
		setupMock    func(*MockIAMClient)
		wantWarnings int
		wantInfo     int
	}{
		{
			name:     "has ec2.amazonaws.com trust",
			roleName: "good-role",
			setupMock: func(m *MockIAMClient) {
				m.GetRoleFunc = func(_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
					return &iam.GetRoleOutput{
						Role: &iamtypes.Role{
							RoleName:                 aws.String("good-role"),
							AssumeRolePolicyDocument: aws.String(`{"Statement":[{"Principal":{"Service":"ec2.amazonaws.com"},"Effect":"Allow","Action":"sts:AssumeRole"}]}`),
						},
					}, nil
				}
			},
			wantInfo: 1,
		},
		{
			name:     "missing ec2 trust",
			roleName: "lambda-role",
			setupMock: func(m *MockIAMClient) {
				m.GetRoleFunc = func(_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
					return &iam.GetRoleOutput{
						Role: &iamtypes.Role{
							RoleName:                 aws.String("lambda-role"),
							AssumeRolePolicyDocument: aws.String(`{"Statement":[{"Principal":{"Service":"lambda.amazonaws.com"}}]}`),
						},
					}, nil
				}
			},
			wantWarnings: 1,
		},
		{
			name:     "API error",
			roleName: "error-role",
			setupMock: func(m *MockIAMClient) {
				m.GetRoleFunc = func(_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
					return nil, fmt.Errorf("access denied")
				}
			},
			wantWarnings: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.iam)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateRoleTrustPolicy(context.Background(), result, tc.roleName)

			assert.Len(t, result.Warnings, tc.wantWarnings, "unexpected warning count")
			assert.Len(t, result.Info, tc.wantInfo, "unexpected info count")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateRolePolicies
// ---------------------------------------------------------------------------

func TestValidateRolePolicies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roleName     string
		setupMock    func(*MockIAMClient)
		wantWarnings int
		wantMinInfo  int
	}{
		{
			name:     "all recommended policies attached",
			roleName: "full-role",
			setupMock: func(m *MockIAMClient) {
				m.ListAttachedRolePoliciesFunc = func(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
					return &iam.ListAttachedRolePoliciesOutput{
						AttachedPolicies: []iamtypes.AttachedPolicy{
							{PolicyName: aws.String("EC2InstanceProfileForImageBuilder")},
							{PolicyName: aws.String("EC2InstanceProfileForImageBuilderECRContainerBuilds")},
							{PolicyName: aws.String("AmazonSSMManagedInstanceCore")},
						},
					}, nil
				}
			},
			wantWarnings: 0,
			wantMinInfo:  3, // one info per recommended policy found
		},
		{
			name:     "none attached",
			roleName: "bare-role",
			setupMock: func(m *MockIAMClient) {
				m.ListAttachedRolePoliciesFunc = func(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
					return &iam.ListAttachedRolePoliciesOutput{
						AttachedPolicies: []iamtypes.AttachedPolicy{},
					}, nil
				}
			},
			wantWarnings: 1, // "Consider attaching"
		},
		{
			name:     "API error",
			roleName: "error-role",
			setupMock: func(m *MockIAMClient) {
				m.ListAttachedRolePoliciesFunc = func(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
					return nil, fmt.Errorf("access denied")
				}
			},
			wantWarnings: 1, // "Could not list role policies"
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.iam)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateRolePolicies(context.Background(), result, tc.roleName)

			assert.Len(t, result.Warnings, tc.wantWarnings, "unexpected warning count")
			if tc.wantMinInfo > 0 {
				assert.GreaterOrEqual(t, len(result.Info), tc.wantMinInfo, "unexpected info count")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateSubnet
// ---------------------------------------------------------------------------

func TestValidateSubnet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		subnetID   string
		setupMock  func(*MockEC2Client)
		wantErrors int
		wantInfo   int
	}{
		{
			name:     "subnet found",
			subnetID: "subnet-abc123",
			setupMock: func(m *MockEC2Client) {
				m.DescribeSubnetsFunc = func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return &ec2.DescribeSubnetsOutput{
						Subnets: []ec2types.Subnet{
							{
								SubnetId:         aws.String("subnet-abc123"),
								VpcId:            aws.String("vpc-xyz789"),
								AvailabilityZone: aws.String("us-east-1a"),
							},
						},
					}, nil
				}
			},
			wantErrors: 0,
			wantInfo:   1,
		},
		{
			name:     "subnet not found empty result",
			subnetID: "subnet-missing",
			setupMock: func(m *MockEC2Client) {
				m.DescribeSubnetsFunc = func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return &ec2.DescribeSubnetsOutput{
						Subnets: []ec2types.Subnet{},
					}, nil
				}
			},
			wantErrors: 1,
			wantInfo:   0,
		},
		{
			name:     "API error",
			subnetID: "subnet-error",
			setupMock: func(m *MockEC2Client) {
				m.DescribeSubnetsFunc = func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return nil, fmt.Errorf("InvalidSubnetID.NotFound")
				}
			},
			wantErrors: 1,
			wantInfo:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.ec2)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateSubnet(context.Background(), result, tc.subnetID)

			assert.Len(t, result.Errors, tc.wantErrors, "unexpected error count")
			assert.Len(t, result.Info, tc.wantInfo, "unexpected info count")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateSecurityGroups
// ---------------------------------------------------------------------------

func TestValidateSecurityGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sgIDs      []string
		setupMock  func(*MockEC2Client)
		wantErrors int
		wantInfo   int
	}{
		{
			name:  "all found",
			sgIDs: []string{"sg-111", "sg-222"},
			setupMock: func(m *MockEC2Client) {
				m.DescribeSecurityGroupsFunc = func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return &ec2.DescribeSecurityGroupsOutput{
						SecurityGroups: []ec2types.SecurityGroup{
							{GroupId: aws.String("sg-111"), GroupName: aws.String("build-sg-1")},
							{GroupId: aws.String("sg-222"), GroupName: aws.String("build-sg-2")},
						},
					}, nil
				}
			},
			wantErrors: 0,
			wantInfo:   2,
		},
		{
			name:  "some missing",
			sgIDs: []string{"sg-111", "sg-222", "sg-333"},
			setupMock: func(m *MockEC2Client) {
				m.DescribeSecurityGroupsFunc = func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return &ec2.DescribeSecurityGroupsOutput{
						SecurityGroups: []ec2types.SecurityGroup{
							{GroupId: aws.String("sg-111"), GroupName: aws.String("build-sg-1")},
						},
					}, nil
				}
			},
			wantErrors: 2, // sg-222 and sg-333 not found
			wantInfo:   1, // only sg-111 validated
		},
		{
			name:  "API error",
			sgIDs: []string{"sg-bad"},
			setupMock: func(m *MockEC2Client) {
				m.DescribeSecurityGroupsFunc = func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return nil, fmt.Errorf("InvalidGroup.NotFound")
				}
			},
			wantErrors: 1,
			wantInfo:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clients, mocks := newMockAWSClients()
			tc.setupMock(mocks.ec2)
			v := NewValidator(clients)
			result := &ValidationResult{Valid: true}

			v.validateSecurityGroups(context.Background(), result, tc.sgIDs)

			assert.Len(t, result.Errors, tc.wantErrors, "unexpected error count")
			assert.Len(t, result.Info, tc.wantInfo, "unexpected info count")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for validateBasicConfig
// ---------------------------------------------------------------------------

func TestValidateBasicConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       builder.Config
		target       builder.Target
		useClients   bool
		wantErrors   int
		wantWarnings int
		wantMinInfo  int
	}{
		{
			name: "all fields set",
			config: builder.Config{
				Name:    "my-template",
				Version: "1.0.0",
				Base:    builder.BaseImage{Image: "ami-12345678"},
			},
			target: builder.Target{
				Region:              "us-west-2",
				InstanceType:        "t3.medium",
				InstanceProfileName: "my-profile",
			},
			wantErrors:   0,
			wantWarnings: 0,
			wantMinInfo:  6, // name, version, region, base image, instance type, instance profile
		},
		{
			name:       "missing name",
			config:     builder.Config{Version: "1.0.0", Base: builder.BaseImage{Image: "ami-123"}},
			target:     builder.Target{Region: "us-east-1", InstanceType: "t3.micro", InstanceProfileName: "prof"},
			wantErrors: 1,
		},
		{
			name:       "missing version",
			config:     builder.Config{Name: "test", Base: builder.BaseImage{Image: "ami-123"}},
			target:     builder.Target{Region: "us-east-1", InstanceType: "t3.micro", InstanceProfileName: "prof"},
			wantErrors: 1,
		},
		{
			name:       "missing region without clients",
			config:     builder.Config{Name: "test", Version: "1.0.0", Base: builder.BaseImage{Image: "ami-123"}},
			target:     builder.Target{InstanceType: "t3.micro", InstanceProfileName: "prof"},
			wantErrors: 1,
		},
		{
			name:       "missing region with clients provides region from client",
			config:     builder.Config{Name: "test", Version: "1.0.0", Base: builder.BaseImage{Image: "ami-123"}},
			target:     builder.Target{InstanceType: "t3.micro", InstanceProfileName: "prof"},
			useClients: true,
			wantErrors: 0, // clients.GetRegion() returns "us-east-1"
		},
		{
			name:       "missing base image",
			config:     builder.Config{Name: "test", Version: "1.0.0"},
			target:     builder.Target{Region: "us-east-1", InstanceType: "t3.micro", InstanceProfileName: "prof"},
			wantErrors: 1,
		},
		{
			name:         "instance type warning when missing",
			config:       builder.Config{Name: "test", Version: "1.0.0", Base: builder.BaseImage{Image: "ami-123"}},
			target:       builder.Target{Region: "us-east-1", InstanceProfileName: "prof"},
			wantWarnings: 1,
		},
		{
			name:         "instance profile warning when missing",
			config:       builder.Config{Name: "test", Version: "1.0.0", Base: builder.BaseImage{Image: "ami-123"}},
			target:       builder.Target{Region: "us-east-1", InstanceType: "t3.micro"},
			wantWarnings: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var v *Validator
			if tc.useClients {
				clients, _ := newMockAWSClients()
				v = NewValidator(clients)
			} else {
				v = NewValidator(nil)
			}
			result := &ValidationResult{Valid: true}

			v.validateBasicConfig(result, tc.config, &tc.target)

			assert.Len(t, result.Errors, tc.wantErrors, "unexpected error count")
			assert.GreaterOrEqual(t, len(result.Warnings), tc.wantWarnings, "unexpected warning count")
			if tc.wantMinInfo > 0 {
				assert.GreaterOrEqual(t, len(result.Info), tc.wantMinInfo, "unexpected info count")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for ValidateBuild (integration with mocks)
// ---------------------------------------------------------------------------

func TestValidateBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     builder.Config
		target     builder.Target
		setupMock  func(*mockClients)
		useClients bool
		wantValid  bool
		wantErrors int
	}{
		{
			name: "full valid configuration",
			config: builder.Config{
				Name:    "my-template",
				Version: "1.0.0",
				Base:    builder.BaseImage{Image: "ami-12345678"},
				Provisioners: []builder.Provisioner{
					{Type: "shell", Inline: []string{"echo hello"}},
				},
			},
			target: builder.Target{
				Type:                "ami",
				Region:              "us-east-1",
				InstanceType:        "t3.medium",
				InstanceProfileName: "build-profile",
				SubnetID:            "subnet-abc",
				SecurityGroupIDs:    []string{"sg-111"},
			},
			useClients: true,
			setupMock: func(mc *mockClients) {
				mc.ec2.DescribeImagesFunc = func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
					return &ec2.DescribeImagesOutput{
						Images: []ec2types.Image{
							{
								ImageId: aws.String("ami-12345678"),
								Name:    aws.String("base-image"),
								State:   ec2types.ImageStateAvailable,
							},
						},
					}, nil
				}
				mc.ec2.DescribeInstanceTypesFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
					return &ec2.DescribeInstanceTypesOutput{
						InstanceTypes: []ec2types.InstanceTypeInfo{
							{
								InstanceType: ec2types.InstanceTypeT3Medium,
								VCpuInfo:     &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(2)},
								MemoryInfo:   &ec2types.MemoryInfo{SizeInMiB: aws.Int64(4096)},
							},
						},
					}, nil
				}
				mc.ec2.DescribeInstanceTypeOfferingsFunc = func(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
					return &ec2.DescribeInstanceTypeOfferingsOutput{
						InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
							{Location: aws.String("us-east-1a")},
						},
					}, nil
				}
				mc.iam.GetInstanceProfileFunc = func(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
					return &iam.GetInstanceProfileOutput{
						InstanceProfile: &iamtypes.InstanceProfile{
							InstanceProfileName: aws.String("build-profile"),
							Roles: []iamtypes.Role{
								{RoleName: aws.String("build-role")},
							},
						},
					}, nil
				}
				mc.iam.GetRoleFunc = func(_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
					return &iam.GetRoleOutput{
						Role: &iamtypes.Role{
							RoleName:                 aws.String("build-role"),
							AssumeRolePolicyDocument: aws.String(`{"Statement":[{"Principal":{"Service":"ec2.amazonaws.com"}}]}`),
						},
					}, nil
				}
				mc.iam.ListAttachedRolePoliciesFunc = func(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
					return &iam.ListAttachedRolePoliciesOutput{
						AttachedPolicies: []iamtypes.AttachedPolicy{
							{PolicyName: aws.String("EC2InstanceProfileForImageBuilder")},
							{PolicyName: aws.String("AmazonSSMManagedInstanceCore")},
						},
					}, nil
				}
				mc.ec2.DescribeSubnetsFunc = func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
					return &ec2.DescribeSubnetsOutput{
						Subnets: []ec2types.Subnet{
							{SubnetId: aws.String("subnet-abc"), VpcId: aws.String("vpc-xyz"), AvailabilityZone: aws.String("us-east-1a")},
						},
					}, nil
				}
				mc.ec2.DescribeSecurityGroupsFunc = func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
					return &ec2.DescribeSecurityGroupsOutput{
						SecurityGroups: []ec2types.SecurityGroup{
							{GroupId: aws.String("sg-111"), GroupName: aws.String("build-sg")},
						},
					}, nil
				}
				// checkExistingResources calls - return errors so no warnings about existing resources
				mc.imageBuilder.GetInfrastructureConfigurationFunc = func(_ context.Context, _ *imagebuilder.GetInfrastructureConfigurationInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
					return nil, fmt.Errorf("not found")
				}
				mc.imageBuilder.ListInfrastructureConfigurationsFunc = func(_ context.Context, _ *imagebuilder.ListInfrastructureConfigurationsInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
					return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
				}
				mc.imageBuilder.GetDistributionConfigurationFunc = func(_ context.Context, _ *imagebuilder.GetDistributionConfigurationInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error) {
					return nil, fmt.Errorf("not found")
				}
				mc.imageBuilder.ListDistributionConfigurationsFunc = func(_ context.Context, _ *imagebuilder.ListDistributionConfigurationsInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
					return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
				}
				mc.imageBuilder.GetImageRecipeFunc = func(_ context.Context, _ *imagebuilder.GetImageRecipeInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error) {
					return nil, fmt.Errorf("not found")
				}
				mc.imageBuilder.ListImageRecipesFunc = func(_ context.Context, _ *imagebuilder.ListImageRecipesInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
					return &imagebuilder.ListImageRecipesOutput{}, nil
				}
				mc.imageBuilder.GetImagePipelineFunc = func(_ context.Context, _ *imagebuilder.GetImagePipelineInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.GetImagePipelineOutput, error) {
					return nil, fmt.Errorf("not found")
				}
				mc.imageBuilder.ListImagePipelinesFunc = func(_ context.Context, _ *imagebuilder.ListImagePipelinesInput, _ ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
					return &imagebuilder.ListImagePipelinesOutput{}, nil
				}
			},
			wantValid:  true,
			wantErrors: 0,
		},
		{
			name: "no clients skips AWS validation",
			config: builder.Config{
				Name:    "my-template",
				Version: "1.0.0",
				Base:    builder.BaseImage{Image: "ami-12345678"},
				Provisioners: []builder.Provisioner{
					{Type: "shell", Inline: []string{"echo hello"}},
				},
			},
			target: builder.Target{
				Region:              "us-east-1",
				InstanceType:        "t3.medium",
				InstanceProfileName: "build-profile",
			},
			useClients: false,
			wantValid:  true,
			wantErrors: 0,
		},
		{
			name:   "missing required fields fails",
			config: builder.Config{
				// Missing Name, Version, Base.Image
			},
			target: builder.Target{
				// Missing Region
			},
			useClients: false,
			wantValid:  false,
			wantErrors: 3, // name, version, base image (region error too but via separate path when no clients)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var v *Validator
			if tc.useClients {
				clients, mocks := newMockAWSClients()
				if tc.setupMock != nil {
					tc.setupMock(mocks)
				}
				v = NewValidator(clients)
			} else {
				v = NewValidator(nil)
			}

			result := v.ValidateBuild(context.Background(), tc.config, &tc.target)

			assert.Equal(t, tc.wantValid, result.Valid, "unexpected validation result")
			if tc.wantErrors > 0 {
				assert.GreaterOrEqual(t, len(result.Errors), tc.wantErrors, "expected at least %d errors, got %d", tc.wantErrors, len(result.Errors))
			} else {
				assert.Empty(t, result.Errors, "expected no errors but got: %v", result.Errors)
			}
		})
	}
}

// containsStr is a helper that checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
