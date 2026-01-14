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

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/builder/buildkit"
	"github.com/cowdogmoo/warpgate/v3/cli"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/spf13/cobra"
)

// runManifestsCreate executes the manifest create command
func runManifestsCreate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	logging.InfoContext(ctx, "Creating multi-arch manifest for %s", manifestsCreateOpts.name)

	// Discover, validate, and filter digests
	filteredDigests, err := discoverAndValidateDigests(ctx)
	if err != nil {
		return err
	}

	// Perform registry checks
	if err := performRegistryChecks(ctx, cmd, filteredDigests); err != nil {
		return err
	}

	// Handle idempotency check
	if !manifestsCreateOpts.force {
		exists, err := manifests.CheckManifestExists(ctx, filteredDigests)
		if err != nil {
			logging.WarnContext(ctx, "Failed to check existing manifest: %v", err)
		} else if exists {
			logging.InfoContext(ctx, "Manifest already exists and is up-to-date (use --force to recreate)")
			return nil
		}
	}

	// Handle dry-run mode
	if manifestsCreateOpts.dryRun {
		return handleDryRun(ctx, filteredDigests)
	}

	// Parse metadata and create manifests
	annotations, labels, err := parseMetadata(ctx)
	if err != nil {
		return err
	}

	return createAndPushManifests(ctx, filteredDigests, annotations, labels)
}

// discoverAndValidateDigests discovers, validates, and filters digest files
func discoverAndValidateDigests(ctx context.Context) ([]manifests.DigestFile, error) {
	// Discover digest files
	digestFiles, err := manifests.DiscoverDigestFiles(ctx, manifests.DiscoveryOptions{
		ImageName: manifestsCreateOpts.name,
		Directory: manifestsCreateOpts.digestDir,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to discover digest files: %w", err)
	}

	if len(digestFiles) == 0 {
		return nil, fmt.Errorf("no digest files found in %s (expected files matching pattern: digest-%s-*.txt)", manifestsCreateOpts.digestDir, manifestsCreateOpts.name)
	}

	logging.InfoContext(ctx, "Found %d digest file(s):", len(digestFiles))
	for _, df := range digestFiles {
		logging.InfoContext(ctx, "  - %s: %s", df.Architecture, df.Digest.String())
	}

	// Validate digest files
	if err := manifests.ValidateDigestFiles(ctx, digestFiles, manifests.ValidationOptions{
		ImageName: manifestsCreateOpts.name,
		MaxAge:    manifestsCreateOpts.maxAge,
	}); err != nil {
		return nil, fmt.Errorf("digest validation failed: %w", err)
	}

	// Filter architectures based on requirements
	filteredDigests, err := manifests.FilterArchitectures(ctx, digestFiles, manifests.FilterOptions{
		RequiredArchitectures: manifestsCreateOpts.requireArch,
		BestEffort:            manifestsCreateOpts.bestEffort,
	})
	if err != nil {
		return nil, fmt.Errorf("architecture filtering failed: %w", err)
	}

	if len(filteredDigests) == 0 {
		return nil, fmt.Errorf("no valid architectures found after filtering")
	}

	return filteredDigests, nil
}

// performRegistryChecks performs health check and digest verification if requested
func performRegistryChecks(ctx context.Context, cmd *cobra.Command, filteredDigests []manifests.DigestFile) error {
	// Perform registry health check if requested
	if manifestsCreateOpts.healthCheck {
		if err := manifests.HealthCheckRegistry(ctx, manifests.VerificationOptions{
			Registry:  manifestsSharedOpts.registry,
			Namespace: manifestsSharedOpts.namespace,
			AuthFile:  manifestsSharedOpts.authFile,
		}); err != nil {
			return fmt.Errorf("registry health check failed: %w", err)
		}
	}

	// Verify digests exist in registry if requested
	if manifestsCreateOpts.verifyRegistry {
		if err := verifyDigestsInRegistry(ctx, cmd, filteredDigests); err != nil {
			return err
		}
	}

	return nil
}

// verifyDigestsInRegistry verifies that digests exist in the registry
func verifyDigestsInRegistry(ctx context.Context, cmd *cobra.Command, filteredDigests []manifests.DigestFile) error {
	// Use first tag for verification
	tag := "latest"
	if len(manifestsCreateOpts.tags) > 0 {
		tag = manifestsCreateOpts.tags[0]
	}

	// Get concurrency from config if not set via CLI
	concurrency := manifestsCreateOpts.verifyConcurrency
	if concurrency == 0 {
		// Try to load from config
		if cfg := configFromContext(cmd); cfg != nil && cfg.Manifests.VerifyConcurrency > 0 {
			concurrency = cfg.Manifests.VerifyConcurrency
		} else {
			concurrency = 5 // Built-in default
		}
	}

	if err := manifests.VerifyDigestsInRegistry(ctx, filteredDigests, manifests.VerificationOptions{
		Registry:      manifestsSharedOpts.registry,
		Namespace:     manifestsSharedOpts.namespace,
		Tag:           tag,
		AuthFile:      manifestsSharedOpts.authFile,
		MaxConcurrent: concurrency,
	}); err != nil {
		return fmt.Errorf("registry verification failed: %w", err)
	}

	return nil
}

// parseMetadata parses annotations and labels from command line options
func parseMetadata(_ context.Context) (map[string]string, map[string]string, error) {
	parser := cli.NewParser()

	annotations, err := parser.ParseKeyValuePairs(manifestsCreateOpts.annotations)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse annotations: %w", err)
	}

	labels, err := parser.ParseKeyValuePairs(manifestsCreateOpts.labels)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse labels: %w", err)
	}

	return annotations, labels, nil
}

// createAndPushManifests creates and pushes manifests for all tags using the appropriate builder
func createAndPushManifests(ctx context.Context, filteredDigests []manifests.DigestFile, annotations, labels map[string]string) error {
	// Create the appropriate builder for manifest operations
	bldr, err := createBuilderForManifests(ctx)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}
	defer func() {
		if err := bldr.Close(); err != nil {
			logging.WarnContext(ctx, "Failed to close builder: %v", err)
		}
	}()

	// Note: This is NOT atomic - if a later tag fails, earlier tags are already pushed
	// This is intentional for idempotency and partial success scenarios
	var failedTags []string
	var successTags []string

	for _, tag := range manifestsCreateOpts.tags {
		logging.InfoContext(ctx, "Creating manifest for tag: %s", tag)

		// Convert DigestFiles to ManifestEntries
		entries := convertDigestFilesToManifestEntries(filteredDigests, manifestsSharedOpts.registry, manifestsSharedOpts.namespace, tag)

		// Build the manifest name/destination
		manifestName := manifests.BuildManifestReference(manifestsSharedOpts.registry, manifestsSharedOpts.namespace, manifestsCreateOpts.name, tag)

		// Create and push the manifest using the builder
		if err := createManifestWithBuilder(ctx, bldr, manifestName, entries); err != nil {
			logging.ErrorContext(ctx, "Failed to create/push manifest for tag %s: %v", tag, err)
			failedTags = append(failedTags, tag)
			continue
		}

		logging.InfoContext(ctx, "Successfully created and pushed manifest to %s", manifestName)
		successTags = append(successTags, tag)
	}

	// Report results
	if len(failedTags) > 0 {
		logging.ErrorContext(ctx, "Failed to push %d of %d tag(s): %v", len(failedTags), len(manifestsCreateOpts.tags), failedTags)
		logging.InfoContext(ctx, "Successfully pushed %d tag(s): %v", len(successTags), successTags)
		return fmt.Errorf("failed to push %d of %d tag(s)", len(failedTags), len(manifestsCreateOpts.tags))
	}

	logging.InfoContext(ctx, "Successfully pushed all %d tag(s)", len(successTags))
	return nil
}

// handleDryRun handles dry-run mode by previewing what would be created
func handleDryRun(ctx context.Context, filteredDigests []manifests.DigestFile) error {
	logging.InfoContext(ctx, "Dry run: would create manifest with %d architecture(s):", len(filteredDigests))
	for _, df := range filteredDigests {
		logging.InfoContext(ctx, "  - %s: %s", df.Architecture, df.Digest.String())
	}

	// Show all tags that would be created
	logging.InfoContext(ctx, "Would push with %d tag(s):", len(manifestsCreateOpts.tags))
	for _, tag := range manifestsCreateOpts.tags {
		manifestRef := manifests.BuildManifestReference(manifestsSharedOpts.registry, manifestsSharedOpts.namespace, manifestsCreateOpts.name, tag)
		logging.InfoContext(ctx, "  - %s", manifestRef)
	}

	return nil
}

// convertDigestFilesToManifestEntries converts DigestFiles to ManifestEntries
func convertDigestFilesToManifestEntries(digestFiles []manifests.DigestFile, registry, namespace, tag string) []manifests.ManifestEntry {
	entries := make([]manifests.ManifestEntry, 0, len(digestFiles))

	for _, df := range digestFiles {
		// Build the digest-based image reference (registry/namespace/image@sha256:...)
		// instead of tag-based reference with architecture suffix
		imageRef := manifests.BuildManifestReference(registry, namespace, df.ImageName, "")
		// Remove trailing colon if tag was empty
		imageRef = strings.TrimSuffix(imageRef, ":")
		// Add digest
		imageRef = fmt.Sprintf("%s@%s", imageRef, df.Digest.String())

		// Parse platform information using the consolidated utility
		// The architecture field in DigestFile may contain variants (e.g., "arm/v7")
		platformInfo := manifests.ParsePlatform(df.Architecture)

		// Format the platform string properly
		platform := manifests.FormatPlatform(platformInfo)

		entry := manifests.ManifestEntry{
			ImageRef:     imageRef,
			Digest:       df.Digest,
			Platform:     platform,
			Architecture: platformInfo.Architecture,
			OS:           platformInfo.OS,
			Variant:      platformInfo.Variant,
		}

		entries = append(entries, entry)
	}

	return entries
}

// manifestBuilder is an interface for builders that support manifest operations.
// This interface is defined here (in the consumer) following Go best practices:
// "Accept interfaces, return structs" and "Define interfaces where they're consumed."
// This allows for easy mocking in tests and avoids coupling the builder package to manifests.
type manifestBuilder interface {
	builder.ContainerBuilder
	CreateAndPushManifest(ctx context.Context, manifestName string, entries []manifests.ManifestEntry) error
}

// Verify at compile time that BuildKitBuilder implements manifestBuilder
var _ manifestBuilder = (*buildkit.BuildKitBuilder)(nil)

// createBuilderForManifests creates a BuildKit builder for manifest operations
func createBuilderForManifests(ctx context.Context) (manifestBuilder, error) {
	return buildkit.NewBuildKitBuilder(ctx)
}

// createManifestWithBuilder creates and pushes a manifest using the builder
func createManifestWithBuilder(ctx context.Context, bldr manifestBuilder, manifestName string, entries []manifests.ManifestEntry) error {
	if err := bldr.CreateAndPushManifest(ctx, manifestName, entries); err != nil {
		return fmt.Errorf("failed to create and push manifest: %w", err)
	}
	return nil
}
