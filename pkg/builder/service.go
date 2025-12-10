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

package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/manifests"
	"github.com/opencontainers/go-digest"
)

// BuildService encapsulates the complete build workflow including building,
// pushing, and manifest creation. It coordinates between builders, orchestrators,
// and configuration to execute complex multi-stage builds.
type BuildService struct {
	globalConfig *globalconfig.Config

	// Platform-specific builder creation functions
	// These are injected to handle platform differences (macOS vs Linux)
	buildKitCreator   BuilderCreatorFunc
	autoSelectCreator BuilderCreatorFunc
}

// NewBuildService creates a new build service with the given configuration.
// The creator functions allow platform-specific builder initialization.
func NewBuildService(cfg *globalconfig.Config, buildKitCreator, autoSelectCreator BuilderCreatorFunc) *BuildService {
	return &BuildService{
		globalConfig:      cfg,
		buildKitCreator:   buildKitCreator,
		autoSelectCreator: autoSelectCreator,
	}
}

// ExecuteContainerBuild performs a complete container build workflow.
// It handles configuration overrides, builder selection, single/multi-arch builds,
// and optionally pushes the results to a registry.
func (s *BuildService) ExecuteContainerBuild(ctx context.Context, config Config, opts BuildOptions) ([]BuildResult, error) {
	logging.InfoContext(ctx, "Executing container build")

	// Apply configuration overrides
	if err := ApplyOverrides(ctx, &config, opts, s.globalConfig); err != nil {
		return nil, fmt.Errorf("failed to apply overrides: %w", err)
	}

	// Select and initialize builder
	bldr, err := s.selectContainerBuilder(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize container builder: %w", err)
	}
	defer func() {
		if err := bldr.Close(); err != nil {
			logging.ErrorContext(ctx, "Failed to close builder", "error", err)
		}
	}()

	// Determine if this is a multi-arch build
	if len(config.Architectures) > 1 {
		return s.executeMultiArchBuild(ctx, &config, bldr, opts)
	}

	// Single architecture build
	result, err := s.executeSingleArchBuild(ctx, &config, bldr, opts)
	if err != nil {
		return nil, err
	}

	return []BuildResult{*result}, nil
}

// Push pushes build results to a registry and optionally saves digests.
// For multi-arch builds, it also creates and pushes a multi-arch manifest.
func (s *BuildService) Push(ctx context.Context, config Config, results []BuildResult, opts BuildOptions) error {
	if opts.Registry == "" {
		return fmt.Errorf("registry must be specified for push")
	}

	// Re-create builder for push operation
	bldr, err := s.selectContainerBuilder(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to initialize builder for push: %w", err)
	}
	defer func() {
		if closeErr := bldr.Close(); closeErr != nil {
			logging.ErrorContext(ctx, "Failed to close builder after push", "error", closeErr)
		}
	}()

	// Single arch push
	if len(results) == 1 {
		return s.pushSingleArch(ctx, &config, results[0], bldr, opts)
	}

	// Multi-arch push
	return s.pushMultiArch(ctx, &config, results, bldr, opts)
}

// DetermineTargetType determines the target type from configuration and options.
func DetermineTargetType(config *Config, opts BuildOptions) string {
	// CLI override takes precedence
	if opts.TargetType != "" {
		return opts.TargetType
	}

	// Check config targets
	if len(config.Targets) > 0 {
		return config.Targets[0].Type
	}

	// Default to container
	return "container"
}

// selectContainerBuilder chooses the appropriate builder based on config and options
func (s *BuildService) selectContainerBuilder(ctx context.Context, opts BuildOptions) (ContainerBuilder, error) {
	// Determine which builder to use
	builderTypeStr := s.globalConfig.Build.BuilderType
	if opts.BuilderType != "" {
		builderTypeStr = opts.BuilderType
	}

	// Validate builder type
	if err := ValidateBuilderType(builderTypeStr); err != nil {
		return nil, err
	}

	// Create factory with platform-specific creator functions
	factory := NewBuilderFactory(
		builderTypeStr,
		s.buildKitCreator,
		s.autoSelectCreator,
	)

	// Create the builder
	bldr, err := factory.CreateContainerBuilder(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}

	// Note: Builder-specific options (like BuildKit cache) should be set
	// in the command layer before passing the builder to the service
	return bldr, nil
}

// executeSingleArchBuild executes a single-architecture build
func (s *BuildService) executeSingleArchBuild(ctx context.Context, config *Config, bldr ContainerBuilder, opts BuildOptions) (*BuildResult, error) {
	// Set platform if not already set
	if config.Base.Platform == "" && len(config.Architectures) > 0 {
		config.Base.Platform = fmt.Sprintf("linux/%s", config.Architectures[0])
	}

	result, err := bldr.Build(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("container build failed: %w", err)
	}

	return result, nil
}

// executeMultiArchBuild executes a multi-architecture build
func (s *BuildService) executeMultiArchBuild(ctx context.Context, config *Config, bldr ContainerBuilder, opts BuildOptions) ([]BuildResult, error) {
	logging.InfoContext(ctx, "Executing multi-arch build for %d architectures: %v", len(config.Architectures), config.Architectures)

	// Use configured concurrency
	concurrency := DefaultMaxConcurrency
	if s.globalConfig != nil && s.globalConfig.Build.Concurrency > 0 {
		concurrency = s.globalConfig.Build.Concurrency
	}

	// Create build orchestrator
	orchestrator := NewBuildOrchestrator(concurrency)

	// Create build requests for each architecture
	requests := CreateBuildRequests(config)

	// Build all architectures in parallel
	results, err := orchestrator.BuildMultiArch(ctx, requests, bldr)
	if err != nil {
		return nil, fmt.Errorf("multi-arch build failed: %w", err)
	}

	return results, nil
}

// pushSingleArch pushes a single architecture image
func (s *BuildService) pushSingleArch(ctx context.Context, config *Config, result BuildResult, bldr ContainerBuilder, opts BuildOptions) error {
	logging.InfoContext(ctx, "Pushing to registry: %s", opts.Registry)

	if err := bldr.Push(ctx, result.ImageRef, opts.Registry); err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	// Save digest if requested
	if opts.SaveDigests && result.Digest != "" {
		arch := "unknown"
		if len(config.Architectures) > 0 {
			arch = config.Architectures[0]
		}
		if err := manifests.SaveDigestToFile(config.Name, arch, result.Digest, opts.DigestDir); err != nil {
			logging.WarnContext(ctx, "Failed to save digest: %v", err)
		}
	}

	logging.InfoContext(ctx, "Successfully pushed to %s", opts.Registry)
	return nil
}

// pushMultiArch pushes multi-architecture images and creates a manifest
func (s *BuildService) pushMultiArch(ctx context.Context, config *Config, results []BuildResult, bldr ContainerBuilder, opts BuildOptions) error {
	logging.InfoContext(ctx, "Pushing multi-arch images to registry: %s", opts.Registry)

	// Use configured concurrency
	concurrency := DefaultMaxConcurrency
	if s.globalConfig != nil && s.globalConfig.Build.Concurrency > 0 {
		concurrency = s.globalConfig.Build.Concurrency
	}

	// Create orchestrator for parallel push
	orchestrator := NewBuildOrchestrator(concurrency)

	// Push individual architecture images
	if err := orchestrator.PushMultiArch(ctx, results, opts.Registry, bldr); err != nil {
		return fmt.Errorf("failed to push multi-arch images: %w", err)
	}

	// Save digests if requested
	if opts.SaveDigests {
		s.saveDigests(ctx, config.Name, results, opts.DigestDir)
	}

	// Note: Multi-arch manifest creation and push should be done in the command layer
	// since it requires platform-specific handling for BuildKit

	logging.InfoContext(ctx, "Successfully pushed multi-arch images to %s", opts.Registry)
	logging.InfoContext(ctx, "Note: Manifest creation must be done separately in the command layer")
	return nil
}

// saveDigests saves digests for all architectures
func (s *BuildService) saveDigests(ctx context.Context, imageName string, results []BuildResult, digestDir string) {
	logging.InfoContext(ctx, "Saving image digests to %s", digestDir)
	for _, result := range results {
		if result.Digest != "" {
			if err := manifests.SaveDigestToFile(imageName, result.Architecture, result.Digest, digestDir); err != nil {
				logging.WarnContext(ctx, "Failed to save digest for %s: %v", result.Architecture, err)
			}
		}
	}
}

// CreateManifestEntries creates manifest entries from build results.
// The actual manifest creation and push should be done in the command layer
// since it requires platform-specific handling.
func CreateManifestEntries(results []BuildResult) ([]manifests.ManifestEntry, error) {
	entries := make([]manifests.ManifestEntry, 0, len(results))
	for _, result := range results {
		// Parse digest
		var imageDigest digest.Digest
		if result.Digest != "" {
			var err error
			imageDigest, err = digest.Parse(result.Digest)
			if err != nil {
				logging.Warn("Failed to parse digest for %s: %v", result.Architecture, err)
				continue
			}
		}

		os := "linux"
		variant := ""
		if strings.Contains(result.Platform, "/") {
			parts := strings.Split(result.Platform, "/")
			if len(parts) >= 2 {
				os = parts[0]
			}
			if len(parts) >= 3 {
				variant = parts[2]
			}
		}

		entries = append(entries, manifests.ManifestEntry{
			ImageRef:     result.ImageRef,
			Digest:       imageDigest,
			Platform:     result.Platform,
			Architecture: result.Architecture,
			OS:           os,
			Variant:      variant,
		})
	}

	return entries, nil
}
