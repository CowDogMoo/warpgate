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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Compile-time interface checks
var (
	_ EC2API            = (*MockEC2Client)(nil)
	_ ImageBuilderAPI   = (*MockImageBuilderClient)(nil)
	_ IAMAPI            = (*MockIAMClient)(nil)
	_ SSMAPI            = (*MockSSMClient)(nil)
	_ CloudWatchLogsAPI = (*MockCloudWatchLogsClient)(nil)
)

// MockEC2Client implements EC2API for testing.
type MockEC2Client struct {
	DescribeImagesFunc                func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeInstancesFunc             func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceTypesFunc         func(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	DescribeInstanceTypeOfferingsFunc func(ctx context.Context, params *ec2.DescribeInstanceTypeOfferingsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error)
	DescribeSubnetsFunc               func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroupsFunc        func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	ModifyImageAttributeFunc          func(ctx context.Context, params *ec2.ModifyImageAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error)
	CopyImageFunc                     func(ctx context.Context, params *ec2.CopyImageInput, optFns ...func(*ec2.Options)) (*ec2.CopyImageOutput, error)
	DeregisterImageFunc               func(ctx context.Context, params *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error)
	DeleteSnapshotFunc                func(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	CreateTagsFunc                    func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
}

func (m *MockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	if m.DescribeImagesFunc != nil {
		return m.DescribeImagesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeImagesOutput{}, nil
}

func (m *MockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.DescribeInstancesFunc != nil {
		return m.DescribeInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (m *MockEC2Client) DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	if m.DescribeInstanceTypesFunc != nil {
		return m.DescribeInstanceTypesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstanceTypesOutput{}, nil
}

func (m *MockEC2Client) DescribeInstanceTypeOfferings(ctx context.Context, params *ec2.DescribeInstanceTypeOfferingsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
	if m.DescribeInstanceTypeOfferingsFunc != nil {
		return m.DescribeInstanceTypeOfferingsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstanceTypeOfferingsOutput{}, nil
}

func (m *MockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if m.DescribeSubnetsFunc != nil {
		return m.DescribeSubnetsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSubnetsOutput{}, nil
}

func (m *MockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if m.DescribeSecurityGroupsFunc != nil {
		return m.DescribeSecurityGroupsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

func (m *MockEC2Client) ModifyImageAttribute(ctx context.Context, params *ec2.ModifyImageAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error) {
	if m.ModifyImageAttributeFunc != nil {
		return m.ModifyImageAttributeFunc(ctx, params, optFns...)
	}
	return &ec2.ModifyImageAttributeOutput{}, nil
}

func (m *MockEC2Client) CopyImage(ctx context.Context, params *ec2.CopyImageInput, optFns ...func(*ec2.Options)) (*ec2.CopyImageOutput, error) {
	if m.CopyImageFunc != nil {
		return m.CopyImageFunc(ctx, params, optFns...)
	}
	return &ec2.CopyImageOutput{}, nil
}

func (m *MockEC2Client) DeregisterImage(ctx context.Context, params *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
	if m.DeregisterImageFunc != nil {
		return m.DeregisterImageFunc(ctx, params, optFns...)
	}
	return &ec2.DeregisterImageOutput{}, nil
}

func (m *MockEC2Client) DeleteSnapshot(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
	if m.DeleteSnapshotFunc != nil {
		return m.DeleteSnapshotFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteSnapshotOutput{}, nil
}

func (m *MockEC2Client) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	if m.CreateTagsFunc != nil {
		return m.CreateTagsFunc(ctx, params, optFns...)
	}
	return &ec2.CreateTagsOutput{}, nil
}

// MockImageBuilderClient implements ImageBuilderAPI for testing.
type MockImageBuilderClient struct {
	CreateComponentFunc                   func(ctx context.Context, params *imagebuilder.CreateComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateComponentOutput, error)
	GetComponentFunc                      func(ctx context.Context, params *imagebuilder.GetComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetComponentOutput, error)
	DeleteComponentFunc                   func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error)
	ListComponentsFunc                    func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error)
	ListComponentBuildVersionsFunc        func(ctx context.Context, params *imagebuilder.ListComponentBuildVersionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentBuildVersionsOutput, error)
	CreateInfrastructureConfigurationFunc func(ctx context.Context, params *imagebuilder.CreateInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateInfrastructureConfigurationOutput, error)
	GetInfrastructureConfigurationFunc    func(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error)
	DeleteInfrastructureConfigurationFunc func(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error)
	ListInfrastructureConfigurationsFunc  func(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error)
	CreateDistributionConfigurationFunc   func(ctx context.Context, params *imagebuilder.CreateDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateDistributionConfigurationOutput, error)
	GetDistributionConfigurationFunc      func(ctx context.Context, params *imagebuilder.GetDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error)
	DeleteDistributionConfigurationFunc   func(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error)
	ListDistributionConfigurationsFunc    func(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error)
	CreateImageRecipeFunc                 func(ctx context.Context, params *imagebuilder.CreateImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImageRecipeOutput, error)
	GetImageRecipeFunc                    func(ctx context.Context, params *imagebuilder.GetImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error)
	DeleteImageRecipeFunc                 func(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error)
	ListImageRecipesFunc                  func(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error)
	CreateImagePipelineFunc               func(ctx context.Context, params *imagebuilder.CreateImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImagePipelineOutput, error)
	GetImagePipelineFunc                  func(ctx context.Context, params *imagebuilder.GetImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImagePipelineOutput, error)
	DeleteImagePipelineFunc               func(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error)
	ListImagePipelinesFunc                func(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error)
	StartImagePipelineExecutionFunc       func(ctx context.Context, params *imagebuilder.StartImagePipelineExecutionInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.StartImagePipelineExecutionOutput, error)
	GetImageFunc                          func(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error)
	ListWorkflowExecutionsFunc            func(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error)
	ListWorkflowStepExecutionsFunc        func(ctx context.Context, params *imagebuilder.ListWorkflowStepExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowStepExecutionsOutput, error)
}

func (m *MockImageBuilderClient) CreateComponent(ctx context.Context, params *imagebuilder.CreateComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateComponentOutput, error) {
	if m.CreateComponentFunc != nil {
		return m.CreateComponentFunc(ctx, params, optFns...)
	}
	return &imagebuilder.CreateComponentOutput{}, nil
}

func (m *MockImageBuilderClient) GetComponent(ctx context.Context, params *imagebuilder.GetComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetComponentOutput, error) {
	if m.GetComponentFunc != nil {
		return m.GetComponentFunc(ctx, params, optFns...)
	}
	return &imagebuilder.GetComponentOutput{}, nil
}

func (m *MockImageBuilderClient) DeleteComponent(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
	if m.DeleteComponentFunc != nil {
		return m.DeleteComponentFunc(ctx, params, optFns...)
	}
	return &imagebuilder.DeleteComponentOutput{}, nil
}

func (m *MockImageBuilderClient) ListComponents(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
	if m.ListComponentsFunc != nil {
		return m.ListComponentsFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListComponentsOutput{}, nil
}

func (m *MockImageBuilderClient) ListComponentBuildVersions(ctx context.Context, params *imagebuilder.ListComponentBuildVersionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentBuildVersionsOutput, error) {
	if m.ListComponentBuildVersionsFunc != nil {
		return m.ListComponentBuildVersionsFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListComponentBuildVersionsOutput{}, nil
}

func (m *MockImageBuilderClient) CreateInfrastructureConfiguration(ctx context.Context, params *imagebuilder.CreateInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateInfrastructureConfigurationOutput, error) {
	if m.CreateInfrastructureConfigurationFunc != nil {
		return m.CreateInfrastructureConfigurationFunc(ctx, params, optFns...)
	}
	return &imagebuilder.CreateInfrastructureConfigurationOutput{}, nil
}

func (m *MockImageBuilderClient) GetInfrastructureConfiguration(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error) {
	if m.GetInfrastructureConfigurationFunc != nil {
		return m.GetInfrastructureConfigurationFunc(ctx, params, optFns...)
	}
	return &imagebuilder.GetInfrastructureConfigurationOutput{}, nil
}

func (m *MockImageBuilderClient) DeleteInfrastructureConfiguration(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error) {
	if m.DeleteInfrastructureConfigurationFunc != nil {
		return m.DeleteInfrastructureConfigurationFunc(ctx, params, optFns...)
	}
	return &imagebuilder.DeleteInfrastructureConfigurationOutput{}, nil
}

func (m *MockImageBuilderClient) ListInfrastructureConfigurations(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error) {
	if m.ListInfrastructureConfigurationsFunc != nil {
		return m.ListInfrastructureConfigurationsFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListInfrastructureConfigurationsOutput{}, nil
}

func (m *MockImageBuilderClient) CreateDistributionConfiguration(ctx context.Context, params *imagebuilder.CreateDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateDistributionConfigurationOutput, error) {
	if m.CreateDistributionConfigurationFunc != nil {
		return m.CreateDistributionConfigurationFunc(ctx, params, optFns...)
	}
	return &imagebuilder.CreateDistributionConfigurationOutput{}, nil
}

func (m *MockImageBuilderClient) GetDistributionConfiguration(ctx context.Context, params *imagebuilder.GetDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error) {
	if m.GetDistributionConfigurationFunc != nil {
		return m.GetDistributionConfigurationFunc(ctx, params, optFns...)
	}
	return &imagebuilder.GetDistributionConfigurationOutput{}, nil
}

func (m *MockImageBuilderClient) DeleteDistributionConfiguration(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error) {
	if m.DeleteDistributionConfigurationFunc != nil {
		return m.DeleteDistributionConfigurationFunc(ctx, params, optFns...)
	}
	return &imagebuilder.DeleteDistributionConfigurationOutput{}, nil
}

func (m *MockImageBuilderClient) ListDistributionConfigurations(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error) {
	if m.ListDistributionConfigurationsFunc != nil {
		return m.ListDistributionConfigurationsFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListDistributionConfigurationsOutput{}, nil
}

func (m *MockImageBuilderClient) CreateImageRecipe(ctx context.Context, params *imagebuilder.CreateImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImageRecipeOutput, error) {
	if m.CreateImageRecipeFunc != nil {
		return m.CreateImageRecipeFunc(ctx, params, optFns...)
	}
	return &imagebuilder.CreateImageRecipeOutput{}, nil
}

func (m *MockImageBuilderClient) GetImageRecipe(ctx context.Context, params *imagebuilder.GetImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error) {
	if m.GetImageRecipeFunc != nil {
		return m.GetImageRecipeFunc(ctx, params, optFns...)
	}
	return &imagebuilder.GetImageRecipeOutput{}, nil
}

func (m *MockImageBuilderClient) DeleteImageRecipe(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error) {
	if m.DeleteImageRecipeFunc != nil {
		return m.DeleteImageRecipeFunc(ctx, params, optFns...)
	}
	return &imagebuilder.DeleteImageRecipeOutput{}, nil
}

func (m *MockImageBuilderClient) ListImageRecipes(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error) {
	if m.ListImageRecipesFunc != nil {
		return m.ListImageRecipesFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListImageRecipesOutput{}, nil
}

func (m *MockImageBuilderClient) CreateImagePipeline(ctx context.Context, params *imagebuilder.CreateImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImagePipelineOutput, error) {
	if m.CreateImagePipelineFunc != nil {
		return m.CreateImagePipelineFunc(ctx, params, optFns...)
	}
	return &imagebuilder.CreateImagePipelineOutput{}, nil
}

func (m *MockImageBuilderClient) GetImagePipeline(ctx context.Context, params *imagebuilder.GetImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImagePipelineOutput, error) {
	if m.GetImagePipelineFunc != nil {
		return m.GetImagePipelineFunc(ctx, params, optFns...)
	}
	return &imagebuilder.GetImagePipelineOutput{}, nil
}

func (m *MockImageBuilderClient) DeleteImagePipeline(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error) {
	if m.DeleteImagePipelineFunc != nil {
		return m.DeleteImagePipelineFunc(ctx, params, optFns...)
	}
	return &imagebuilder.DeleteImagePipelineOutput{}, nil
}

func (m *MockImageBuilderClient) ListImagePipelines(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error) {
	if m.ListImagePipelinesFunc != nil {
		return m.ListImagePipelinesFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListImagePipelinesOutput{}, nil
}

func (m *MockImageBuilderClient) StartImagePipelineExecution(ctx context.Context, params *imagebuilder.StartImagePipelineExecutionInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.StartImagePipelineExecutionOutput, error) {
	if m.StartImagePipelineExecutionFunc != nil {
		return m.StartImagePipelineExecutionFunc(ctx, params, optFns...)
	}
	return &imagebuilder.StartImagePipelineExecutionOutput{}, nil
}

func (m *MockImageBuilderClient) GetImage(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error) {
	if m.GetImageFunc != nil {
		return m.GetImageFunc(ctx, params, optFns...)
	}
	return &imagebuilder.GetImageOutput{}, nil
}

func (m *MockImageBuilderClient) ListWorkflowExecutions(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error) {
	if m.ListWorkflowExecutionsFunc != nil {
		return m.ListWorkflowExecutionsFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListWorkflowExecutionsOutput{}, nil
}

func (m *MockImageBuilderClient) ListWorkflowStepExecutions(ctx context.Context, params *imagebuilder.ListWorkflowStepExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowStepExecutionsOutput, error) {
	if m.ListWorkflowStepExecutionsFunc != nil {
		return m.ListWorkflowStepExecutionsFunc(ctx, params, optFns...)
	}
	return &imagebuilder.ListWorkflowStepExecutionsOutput{}, nil
}

// MockIAMClient implements IAMAPI for testing.
type MockIAMClient struct {
	GetInstanceProfileFunc       func(ctx context.Context, params *iam.GetInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error)
	GetRoleFunc                  func(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
	ListAttachedRolePoliciesFunc func(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
}

func (m *MockIAMClient) GetInstanceProfile(ctx context.Context, params *iam.GetInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	if m.GetInstanceProfileFunc != nil {
		return m.GetInstanceProfileFunc(ctx, params, optFns...)
	}
	return &iam.GetInstanceProfileOutput{}, nil
}

func (m *MockIAMClient) GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	if m.GetRoleFunc != nil {
		return m.GetRoleFunc(ctx, params, optFns...)
	}
	return &iam.GetRoleOutput{}, nil
}

func (m *MockIAMClient) ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	if m.ListAttachedRolePoliciesFunc != nil {
		return m.ListAttachedRolePoliciesFunc(ctx, params, optFns...)
	}
	return &iam.ListAttachedRolePoliciesOutput{}, nil
}

// MockSSMClient implements SSMAPI for testing.
type MockSSMClient struct {
	ListCommandInvocationsFunc func(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error)
}

func (m *MockSSMClient) ListCommandInvocations(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
	if m.ListCommandInvocationsFunc != nil {
		return m.ListCommandInvocationsFunc(ctx, params, optFns...)
	}
	return &ssm.ListCommandInvocationsOutput{}, nil
}

// MockCloudWatchLogsClient implements CloudWatchLogsAPI for testing.
type MockCloudWatchLogsClient struct {
	DescribeLogStreamsFunc func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	GetLogEventsFunc       func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
}

func (m *MockCloudWatchLogsClient) DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	if m.DescribeLogStreamsFunc != nil {
		return m.DescribeLogStreamsFunc(ctx, params, optFns...)
	}
	return &cloudwatchlogs.DescribeLogStreamsOutput{}, nil
}

func (m *MockCloudWatchLogsClient) GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	if m.GetLogEventsFunc != nil {
		return m.GetLogEventsFunc(ctx, params, optFns...)
	}
	return &cloudwatchlogs.GetLogEventsOutput{}, nil
}

// mockClients holds references to all mock clients for easy configuration in tests.
type mockClients struct {
	ec2            *MockEC2Client
	imageBuilder   *MockImageBuilderClient
	iam            *MockIAMClient
	ssm            *MockSSMClient
	cloudWatchLogs *MockCloudWatchLogsClient
}

// newMockAWSClients creates an AWSClients with all mock implementations and returns
// both the AWSClients and the individual mock references for test configuration.
func newMockAWSClients() (*AWSClients, *mockClients) {
	mocks := &mockClients{
		ec2:            &MockEC2Client{},
		imageBuilder:   &MockImageBuilderClient{},
		iam:            &MockIAMClient{},
		ssm:            &MockSSMClient{},
		cloudWatchLogs: &MockCloudWatchLogsClient{},
	}

	clients := &AWSClients{
		EC2:            mocks.ec2,
		ImageBuilder:   mocks.imageBuilder,
		IAM:            mocks.iam,
		SSM:            mocks.ssm,
		CloudWatchLogs: mocks.cloudWatchLogs,
		Config:         aws.Config{Region: "us-east-1"},
	}

	return clients, mocks
}
