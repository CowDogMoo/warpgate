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

// Package ami provides functionality for building Amazon Machine Images (AMIs)
// using AWS EC2 Image Builder.
//
// This package implements the [github.com/cowdogmoo/warpgate/v3/builder.AMIBuilder]
// interface to create custom AMIs from warpgate template configurations. It manages
// the full lifecycle of AWS Image Builder resources including components,
// infrastructure configurations, distribution configurations, image recipes, and
// pipelines.
//
// # Architecture Overview
//
// The package is organized into several key components:
//
//   - [ImageBuilder]: Main entry point implementing the build workflow
//   - [ComponentGenerator]: Converts warpgate provisioners to Image Builder components
//   - [PipelineManager]: Manages Image Builder pipelines and execution
//   - [ResourceManager]: Handles idempotent resource creation and lookup
//   - [ResourceCleaner]: Provides cleanup capabilities for warpgate-created resources
//   - [AWSClients]: AWS SDK client wrapper for Image Builder, EC2, and STS services
//
// # Usage
//
// Create a new [ImageBuilder] and execute a build:
//
//	ctx := context.Background()
//	clientConfig := ami.ClientConfig{
//	    Region: "us-east-1",
//	}
//
//	builder, err := ami.NewImageBuilder(ctx, clientConfig)
//	if err != nil {
//	    return err
//	}
//	defer builder.Close()
//
//	result, err := builder.Build(ctx, config)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Built AMI: %s\n", result.AMIID)
//
// # Resource Naming
//
// Resources created by this package are tagged with "warpgate:name" to enable
// identification and cleanup. Resource names follow the pattern:
//
//	{build-name}-{resource-type}
//
// For example: "my-template-infra", "my-template-pipeline", "my-template-recipe"
//
// # Error Handling
//
// The package provides enhanced error handling through the [WrapWithRemediation]
// function, which adds contextual information and remediation suggestions to
// common AWS errors. This helps users diagnose and fix issues like permission
// problems, resource conflicts, and quota limits.
//
// Errors returned by this package follow Go error wrapping conventions:
//
//	if err != nil {
//	    return fmt.Errorf("failed to create component: %w", err)
//	}
//
// Use [errors.Is] and [errors.As] from the standard library to inspect wrapped errors.
//
// # Cleanup
//
// Use [ResourceCleaner] to clean up resources created by warpgate:
//
//	cleaner := ami.NewResourceCleaner(clients)
//	resources, err := cleaner.ListResourcesForBuild(ctx, "my-template")
//	if err != nil {
//	    return err
//	}
//	err = cleaner.DeleteResources(ctx, resources)
//
// Resources are deleted in dependency order (pipelines → recipes → configs → components)
// to avoid dependency conflicts.
//
// # Concurrency
//
// The [ImageBuilder.Build] method creates multiple AWS resources concurrently using
// [golang.org/x/sync/errgroup] for efficient parallel execution. The implementation
// handles context cancellation and cleans up partially-created resources on failure.
//
// # Windows Support
//
// The package supports building Windows AMIs with optional Fast Launch configuration
// to reduce cold start times. Enable Fast Launch by setting the appropriate target
// configuration options.
package ami
