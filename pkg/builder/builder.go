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
type BuilderCreatorFunc func(ctx context.Context) (ContainerBuilder, error)

// Builder is the main interface for image builders
type Builder interface {
	// Build creates an image from the given configuration
	Build(ctx context.Context, config Config) (*BuildResult, error)

	// Close cleans up any resources used by the builder
	Close() error
}

// ContainerBuilder builds container images
type ContainerBuilder interface {
	Builder

	// Push pushes the built image to a registry
	Push(ctx context.Context, imageRef, registry string) error

	// Tag adds additional tags to an image
	Tag(ctx context.Context, imageRef, newTag string) error

	// Remove removes an image from local storage
	Remove(ctx context.Context, imageRef string) error
}

// AMIBuilder builds AWS AMIs
type AMIBuilder interface {
	Builder

	// Share shares the AMI with other AWS accounts
	Share(ctx context.Context, amiID string, accountIDs []string) error

	// Copy copies an AMI to another region
	Copy(ctx context.Context, amiID, sourceRegion, destRegion string) (string, error)

	// Deregister deregisters an AMI
	Deregister(ctx context.Context, amiID, region string) error
}
