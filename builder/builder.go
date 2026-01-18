/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

// Package builder provides the core abstractions and services for building container images and AWS AMIs.
//
// This package implements the domain logic for multi-platform builds, coordinating between
// BuildKit for container builds and target types (containers, AMIs).
//
// # Architecture
//
// The package is organized into several layers:
//
//   - Interfaces (builder.go): Core abstractions for ContainerBuilder and AMIBuilder
//   - Service Layer (service.go): High-level build orchestration and workflow
//   - Factory (factory.go): Builder selection and instantiation
//   - Orchestrator (orchestrator.go): Parallel multi-arch build execution
//   - Configuration (config.go, options.go): Build configuration and overrides
//
// # Key Concepts
//
// BuildService: The main entry point for executing builds. It coordinates configuration
// overrides, builder selection, and multi-arch builds:
//
//	service := builder.NewBuildService(cfg, buildKitCreator)
//	results, err := service.ExecuteContainerBuild(ctx, config, opts)
//
// ContainerBuilder Interface: Abstracts container build operations across different backends:
//
//	type ContainerBuilder interface {
//	    Build(ctx context.Context, config Config) (*BuildResult, error)
//	    Push(ctx context.Context, imageRef, registry string) error
//	    Close() error
//	}
//
// AMIBuilder Interface: Abstracts AWS AMI build operations:
//
//	type AMIBuilder interface {
//	    Build(ctx context.Context, config Config) (*BuildResult, error)
//	    Share(ctx context.Context, amiID string, accountIDs []string) error
//	    Close() error
//	}
//
// BuildOrchestrator: Manages parallel execution of multi-arch builds with concurrency control:
//
//	orchestrator := builder.NewBuildOrchestrator(maxConcurrency)
//	results, err := orchestrator.BuildMultiArch(ctx, requests, builder)
//
// # Configuration Precedence
//
// Build options follow a clear precedence hierarchy:
//
//	CLI Flags > BuildOptions > Template Config > Global Config > Defaults
//
// This allows flexible override of configuration at different levels.
//
// # Design Principles
//
//   - Interface Segregation: Separate interfaces for container and AMI builders
//   - Dependency Inversion: Service layer depends on interfaces, not concrete implementations
//   - Single Responsibility: Each component has a focused, well-defined purpose
//   - Platform Abstraction: Builder backends are abstracted behind common interfaces
//   - Concurrency Control: Built-in support for parallel multi-arch builds
//
// # Platform Support
//
// The package uses BuildKit for container image builds across all platforms:
//
//   - BuildKit: Docker BuildKit for modern, efficient container builds (Linux/macOS/Windows)
//   - Auto: Automatically selects BuildKit (same as explicit BuildKit selection)
//
// # Import Cycles
//
// To avoid circular dependencies:
//   - The service layer (service.go) cannot import concrete builder implementations
//   - Builder factories use function injection for platform-specific creation
//   - AMI builds are executed in the command layer, not the service layer
package builder

import (
	"context"
)

// BuilderCreatorFunc creates a ContainerBuilder instance.
// The context can be used for initialization and resource cleanup.
// This function type enables dependency injection of builder implementations
// without creating import cycles between the service layer and concrete builders.
type BuilderCreatorFunc func(ctx context.Context) (ContainerBuilder, error)

// Builder is the base interface for all image builders (containers and AMIs).
// It provides the common operations shared across all builder types.
//
// Implementations must be safe for concurrent use from multiple goroutines.
// The Build method should respect context cancellation for graceful shutdown.
//
// Resource Management:
// Callers must call Close() when done with the builder to release resources.
// Using defer is recommended:
//
//	builder, err := createBuilder(ctx)
//	if err != nil {
//	    return err
//	}
//	defer func() {
//	    if err := builder.Close(); err != nil {
//	        log.Warn("Failed to close builder: %v", err)
//	    }
//	}()
type Builder interface {
	// Build creates an image from the given configuration.
	// It returns a BuildResult containing the image reference and digest on success.
	// The context should be used for cancellation; long-running builds should
	// periodically check ctx.Done() and return ctx.Err() if cancelled.
	Build(ctx context.Context, config Config) (*BuildResult, error)

	// Close releases any resources held by the builder.
	// This includes network connections, temporary files, and cleanup of
	// any intermediate build artifacts. Close should be idempotent.
	Close() error
}

// ContainerBuilder defines the interface for building and managing
// container images.
//
// ContainerBuilder implementations must be safe for concurrent use
// by multiple goroutines.
//
// # Methods
//
//   - Build: Builds a container image from the provided [Config].
//   - Push: Pushes the image to a registry with a tag.
//   - PushDigest: Pushes the image by digest without creating a tag.
//   - Tag: Adds an additional tag to an existing image.
//   - Remove: Removes a local image.
//   - Close: Cleans up any resources held by the builder.
//
// Implementations:
//   - buildkit.BuildKitBuilder: Uses Docker BuildKit for efficient builds
//
// Example usage:
//
//	builder, _ := buildkit.NewBuildKitBuilder(ctx)
//	defer builder.Close()
//
//	result, err := builder.Build(ctx, config)
//	if err != nil {
//	    return err
//	}
//
//	digest, err := builder.Push(ctx, result.ImageRef, "ghcr.io/myorg")
type ContainerBuilder interface {
	Builder

	// Push pushes the built image to the specified registry with a tag.
	//
	// imageRef should be a valid image reference (e.g., "myimage:latest").
	// registry specifies the target registry (e.g., "ghcr.io/myorg").
	//
	// Returns the pushed image digest (e.g., "sha256:abc123...") on success.
	// Returns an error if the push fails.
	Push(ctx context.Context, imageRef, registry string) (string, error)

	// PushDigest pushes the built image by digest without creating a registry tag.
	//
	// imageRef should be a fully qualified image reference (e.g., "myimage:latest").
	// registry specifies the target registry (e.g., "ghcr.io/myorg").
	//
	// If imageRef is not fully qualified (does not contain a '/'), this method
	// will locally tag the image with the registry prefix (registry/imageRef)
	// before pushing. This temporary tag will persist in the local Docker daemon
	// after the push completes.
	//
	// Returns the pushed image digest (e.g., "sha256:abc123...") on success.
	// Returns an error if the push fails.
	PushDigest(ctx context.Context, imageRef, registry string) (string, error)

	// Tag adds an additional tag to an existing image.
	//
	// Both imageRef and newTag should be valid image references.
	// Returns an error if the tagging operation fails.
	Tag(ctx context.Context, imageRef, newTag string) error

	// Remove deletes a local image specified by imageRef.
	//
	// Returns an error if the image cannot be removed.
	Remove(ctx context.Context, imageRef string) error
}

// AMIBuilder extends Builder with AWS AMI-specific operations.
// It provides operations for building, sharing, copying, and managing
// Amazon Machine Images (AMIs).
//
// Implementations:
//   - ami.ImageBuilder: Uses AWS EC2 Image Builder for AMI creation
//
// Example usage:
//
//	builder, _ := ami.NewImageBuilder(ctx, clientConfig)
//	defer builder.Close()
//
//	result, err := builder.Build(ctx, config)
//	if err != nil {
//	    return err
//	}
//
//	// Share with another AWS account
//	err = builder.Share(ctx, result.ImageRef, []string{"123456789012"})
type AMIBuilder interface {
	Builder

	// Share shares the AMI with other AWS accounts by modifying launch permissions.
	// The amiID should be a valid AMI ID (e.g., "ami-12345678").
	// The accountIDs are 12-digit AWS account IDs to share with.
	Share(ctx context.Context, amiID string, accountIDs []string) error

	// Copy copies an AMI to another AWS region.
	// Returns the new AMI ID in the destination region.
	// The source AMI must exist in sourceRegion.
	Copy(ctx context.Context, amiID, sourceRegion, destRegion string) (string, error)

	// Deregister deregisters an AMI, making it unavailable for launching new instances.
	// This does not delete associated snapshots; those must be cleaned up separately.
	Deregister(ctx context.Context, amiID, region string) error
}
