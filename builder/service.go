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

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/opencontainers/go-digest"
)

// BuildService encapsulates the complete build workflow including building,
// pushing, and manifest creation. It coordinates between builders, orchestrators,
// and configuration to execute complex multi-stage builds.
type BuildService struct {
	globalConfig *config.Config

	// Builder creation function
	buildKitCreator BuilderCreatorFunc
}

// NewBuildService creates a new build service with the given configuration.
// The creator function initializes BuildKit builders.
func NewBuildService(cfg *config.Config, buildKitCreator BuilderCreatorFunc) *BuildService {
	return &BuildService{
		globalConfig:    cfg,
		buildKitCreator: buildKitCreator,
	}
}

// ExecuteContainerBuild performs a complete container build workflow.
func (s *BuildService) ExecuteContainerBuild(ctx context.Context, config Config, opts BuildOptions) ([]BuildResult, error) {
	logging.InfoContext(ctx, "Executing container build")

	// Apply configuration overrides
	if err := ApplyOverrides(ctx, &config, opts, s.globalConfig); err != nil {
		return nil, fmt.Errorf("failed to apply overrides: %w", err)
	}

	// Create builder
	bldr, err := s.buildKitCreator(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
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

// ExecuteAMIBuild performs a complete AMI build workflow with the provided AMI builder.
// It handles region resolution and AMI target selection before delegating to the builder.
// The caller is responsible for creating and closing the AMI builder to avoid import cycles.
func (s *BuildService) ExecuteAMIBuild(ctx context.Context, config Config, opts BuildOptions, amiBuilder AMIBuilder) (*BuildResult, error) {
	logging.InfoContext(ctx, "Executing AMI build")

	// Find AMI target in configuration
	var amiTarget *Target
	for i := range config.Targets {
		if config.Targets[i].Type == "ami" {
			amiTarget = &config.Targets[i]
			break
		}
	}

	if amiTarget == nil {
		return nil, fmt.Errorf("no AMI target found in configuration")
	}

	// Resolve region from multiple sources (CLI flag > target config > global config)
	region := opts.Region
	if region == "" {
		region = amiTarget.Region
	}
	if region == "" && s.globalConfig != nil {
		region = s.globalConfig.AWS.Region
	}
	if region == "" {
		return nil, fmt.Errorf("AWS region must be specified (use --region flag, set in template, or configure in global config)")
	}

	// Update options with resolved region
	opts.Region = region

	// Execute the build
	result, err := amiBuilder.Build(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("AMI build failed: %w", err)
	}

	logging.InfoContext(ctx, "AMI build completed successfully: %s", result.AMIID)
	return result, nil
}

// Push pushes build results to a registry and optionally saves digests.
// For multi-arch builds, it also creates and pushes a multi-arch manifest.
func (s *BuildService) Push(ctx context.Context, config Config, results []BuildResult, opts BuildOptions) error {
	if opts.Registry == "" {
		return fmt.Errorf("registry must be specified for push")
	}

	// Create builder for push operation
	bldr, err := s.buildKitCreator(ctx)
	if err != nil {
		return fmt.Errorf("failed to create builder for push: %w", err)
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

	digest, err := bldr.Push(ctx, result.ImageRef, opts.Registry)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	// Use the digest from Push if available, otherwise fall back to result.Digest
	if digest == "" {
		digest = result.Digest
	}

	// Save digest if requested
	if opts.SaveDigests && digest != "" {
		arch := result.Architecture
		if arch == "" {
			arch = "unknown"
			if len(config.Architectures) > 0 {
				arch = config.Architectures[0]
			}
		}
		if err := manifests.SaveDigestToFile(config.Name, arch, digest, opts.DigestDir); err != nil {
			logging.WarnContext(ctx, "Failed to save digest: %v", err)
		} else {
			logging.InfoContext(ctx, "Saved digest for %s: %s", arch, digest)
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

		// Parse platform information using the consolidated utility
		platformInfo := manifests.ParsePlatform(result.Platform)

		entries = append(entries, manifests.ManifestEntry{
			ImageRef:     result.ImageRef,
			Digest:       imageDigest,
			Platform:     result.Platform,
			Architecture: platformInfo.Architecture,
			OS:           platformInfo.OS,
			Variant:      platformInfo.Variant,
		})
	}

	return entries, nil
}
