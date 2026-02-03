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

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// EC2API defines the EC2 operations used in this package.
type EC2API interface {
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	DescribeInstanceTypeOfferings(ctx context.Context, params *ec2.DescribeInstanceTypeOfferingsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	ModifyImageAttribute(ctx context.Context, params *ec2.ModifyImageAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyImageAttributeOutput, error)
	CopyImage(ctx context.Context, params *ec2.CopyImageInput, optFns ...func(*ec2.Options)) (*ec2.CopyImageOutput, error)
	DeregisterImage(ctx context.Context, params *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error)
	DeleteSnapshot(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
}

// ImageBuilderAPI defines the Image Builder operations used in this package.
// Method signatures include ...func(*imagebuilder.Options) to satisfy
// the SDK paginator API client interfaces via structural typing.
type ImageBuilderAPI interface {
	// Component operations
	CreateComponent(ctx context.Context, params *imagebuilder.CreateComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateComponentOutput, error)
	GetComponent(ctx context.Context, params *imagebuilder.GetComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetComponentOutput, error)
	DeleteComponent(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error)
	ListComponents(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error)
	ListComponentBuildVersions(ctx context.Context, params *imagebuilder.ListComponentBuildVersionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentBuildVersionsOutput, error)

	// Infrastructure configuration operations
	CreateInfrastructureConfiguration(ctx context.Context, params *imagebuilder.CreateInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateInfrastructureConfigurationOutput, error)
	GetInfrastructureConfiguration(ctx context.Context, params *imagebuilder.GetInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetInfrastructureConfigurationOutput, error)
	DeleteInfrastructureConfiguration(ctx context.Context, params *imagebuilder.DeleteInfrastructureConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteInfrastructureConfigurationOutput, error)
	ListInfrastructureConfigurations(ctx context.Context, params *imagebuilder.ListInfrastructureConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListInfrastructureConfigurationsOutput, error)

	// Distribution configuration operations
	CreateDistributionConfiguration(ctx context.Context, params *imagebuilder.CreateDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateDistributionConfigurationOutput, error)
	GetDistributionConfiguration(ctx context.Context, params *imagebuilder.GetDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetDistributionConfigurationOutput, error)
	DeleteDistributionConfiguration(ctx context.Context, params *imagebuilder.DeleteDistributionConfigurationInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteDistributionConfigurationOutput, error)
	ListDistributionConfigurations(ctx context.Context, params *imagebuilder.ListDistributionConfigurationsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListDistributionConfigurationsOutput, error)

	// Image recipe operations
	CreateImageRecipe(ctx context.Context, params *imagebuilder.CreateImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImageRecipeOutput, error)
	GetImageRecipe(ctx context.Context, params *imagebuilder.GetImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageRecipeOutput, error)
	DeleteImageRecipe(ctx context.Context, params *imagebuilder.DeleteImageRecipeInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImageRecipeOutput, error)
	ListImageRecipes(ctx context.Context, params *imagebuilder.ListImageRecipesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImageRecipesOutput, error)

	// Image pipeline operations
	CreateImagePipeline(ctx context.Context, params *imagebuilder.CreateImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateImagePipelineOutput, error)
	GetImagePipeline(ctx context.Context, params *imagebuilder.GetImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImagePipelineOutput, error)
	DeleteImagePipeline(ctx context.Context, params *imagebuilder.DeleteImagePipelineInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteImagePipelineOutput, error)
	ListImagePipelines(ctx context.Context, params *imagebuilder.ListImagePipelinesInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListImagePipelinesOutput, error)
	StartImagePipelineExecution(ctx context.Context, params *imagebuilder.StartImagePipelineExecutionInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.StartImagePipelineExecutionOutput, error)

	// Image operations
	GetImage(ctx context.Context, params *imagebuilder.GetImageInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.GetImageOutput, error)

	// Workflow operations
	ListWorkflowExecutions(ctx context.Context, params *imagebuilder.ListWorkflowExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowExecutionsOutput, error)
	ListWorkflowStepExecutions(ctx context.Context, params *imagebuilder.ListWorkflowStepExecutionsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListWorkflowStepExecutionsOutput, error)
}

// IAMAPI defines the IAM operations used in this package.
type IAMAPI interface {
	GetInstanceProfile(ctx context.Context, params *iam.GetInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error)
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
	ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
}

// SSMAPI defines the SSM operations used in this package.
type SSMAPI interface {
	ListCommandInvocations(ctx context.Context, params *ssm.ListCommandInvocationsInput, optFns ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error)
}

// CloudWatchLogsAPI defines the CloudWatch Logs operations used in this package.
type CloudWatchLogsAPI interface {
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
}
