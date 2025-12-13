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
	"os"
	"strings"
	"time"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/builder/buildkit"
	"github.com/cowdogmoo/warpgate/pkg/cli"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/manifests"
	"github.com/spf13/cobra"
)

// Exit codes for manifest creation
const (
	ExitSuccess         = 0
	ExitValidationError = 1
	ExitRegistryError   = 2
	ExitDigestNotFound  = 3
)

// manifestsOptions holds command-line options for the manifests command
type manifestsOptions struct {
	// Required flags
	name     string
	registry string

	// Optional flags
	tags           []string
	digestDir      string
	namespace      string
	verifyRegistry bool
	maxAge         time.Duration
	requireArch    []string
	bestEffort     bool
	authFile       string
	force          bool
	dryRun         bool
	quiet          bool
	verbose        bool

	annotations []string // OCI annotations in key=value format
	labels      []string // OCI labels in key=value format
	healthCheck bool     // Perform registry health check before operations
	showDiff    bool     // Show manifest comparison/diff
	noProgress  bool     // Disable progress indicators

	verifyConcurrency int // Number of concurrent verification requests
}

var manifestsOpts = &manifestsOptions{}

var manifestsCmd = &cobra.Command{
	Use:   "manifests",
	Short: "Manage multi-architecture manifests",
	Long: `Manage multi-architecture container image manifests.

This command provides tools for creating and pushing multi-architecture
manifests from digest files generated during separate architecture builds.`,
}

var manifestsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and push a multi-architecture manifest",
	Long: `Create a multi-architecture manifest from digest files and push to registry.

This command reads digest files from previous 'warpgate build' invocations,
creates a multi-architecture manifest list, and pushes it to the registry.

Digest files should follow the naming convention:
  digest-{IMAGE_NAME}-{ARCHITECTURE}.txt

Each digest file should contain a single line with the format:
  sha256:<64-character-hex-digest>

Examples:
  # Create manifest with auto-detected architectures (from digest files)
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo

  # Create manifest with multiple tags simultaneously
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --tag latest,v1.0.0,stable

  # Create manifest with specific architectures
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --require-arch amd64,arm64

  # Add OCI annotations and labels
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --annotation "org.opencontainers.image.version=1.0.0" \
    --label "maintainer=security-team"

  # Tune verification concurrency (default: 5, max: 20)
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --verify-concurrency 10

  # Health check registry before operations
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --health-check

  # Dry run to preview manifest without pushing
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --dry-run`,
	RunE: runManifestsCreate,
}

func init() {
	// Required flags
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.name, "name", "", "Image name (required)")
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.registry, "registry", "", "Registry to push to (required)")
	_ = manifestsCreateCmd.MarkFlagRequired("name")
	_ = manifestsCreateCmd.MarkFlagRequired("registry")

	// Optional flags
	manifestsCreateCmd.Flags().StringSliceVarP(&manifestsOpts.tags, "tag", "t", []string{"latest"}, "Image tags (comma-separated, can specify multiple)")
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.digestDir, "digest-dir", ".", "Directory containing digest files")
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.namespace, "namespace", "", "Image namespace/organization")

	// Security & Validation
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.verifyRegistry, "verify-registry", true, "Verify digests exist in registry")
	manifestsCreateCmd.Flags().IntVar(&manifestsOpts.verifyConcurrency, "verify-concurrency", 0, "Number of concurrent digest verifications (default from config: 5)")
	manifestsCreateCmd.Flags().DurationVar(&manifestsOpts.maxAge, "max-age", 0, "Maximum age of digest files (e.g., 1h, 30m)")
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.healthCheck, "health-check", false, "Perform registry health check before operations")

	// Architecture Control
	manifestsCreateCmd.Flags().StringSliceVar(&manifestsOpts.requireArch, "require-arch", nil, "Required architectures (comma-separated)")
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.bestEffort, "best-effort", false, "Create manifest with available architectures")

	// Authentication
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.authFile, "auth-file", "", "Path to authentication file")

	manifestsCreateCmd.Flags().StringSliceVar(&manifestsOpts.annotations, "annotation", nil, "OCI annotations (key=value, can specify multiple)")
	manifestsCreateCmd.Flags().StringSliceVar(&manifestsOpts.labels, "label", nil, "OCI labels (key=value, can specify multiple)")
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.showDiff, "show-diff", false, "Show manifest comparison/diff if manifest exists")
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.noProgress, "no-progress", false, "Disable progress indicators")

	// Behavior
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.force, "force", false, "Force recreation even if manifest exists")
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.dryRun, "dry-run", false, "Preview manifest without pushing")
	manifestsCreateCmd.Flags().BoolVarP(&manifestsOpts.quiet, "quiet", "q", false, "Only output errors")
	manifestsCreateCmd.Flags().BoolVarP(&manifestsOpts.verbose, "verbose", "v", false, "Verbose output with detailed progress")

	// Add create subcommand
	manifestsCmd.AddCommand(manifestsCreateCmd)
	manifestsCmd.AddCommand(manifestsInspectCmd)
	manifestsCmd.AddCommand(manifestsListCmd)
}

// manifestsInspectCmd inspects a manifest from a registry
var manifestsInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect a multi-architecture manifest",
	Long: `Inspect a multi-architecture manifest from a registry.

Shows detailed information about the manifest including all architectures,
digests, sizes, and annotations.

Examples:
  # Inspect a manifest
  warpgate manifests inspect --name attack-box --registry ghcr.io/cowdogmoo

  # Inspect specific tag
  warpgate manifests inspect --name attack-box --registry ghcr.io/cowdogmoo --tag v1.0.0

  # Show in JSON format
  warpgate manifests inspect --name attack-box --registry ghcr.io/cowdogmoo --format json`,
	RunE: runManifestsInspect,
}

// manifestsListCmd lists manifests
var manifestsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available manifest tags",
	Long: `List available manifest tags for an image in the registry.

Examples:
  # List all tags
  warpgate manifests list --name attack-box --registry ghcr.io/cowdogmoo

  # List with details
  warpgate manifests list --name attack-box --registry ghcr.io/cowdogmoo --detailed`,
	RunE: runManifestsList,
}

func init() {
	// Inspect command flags
	manifestsInspectCmd.Flags().StringVar(&manifestsOpts.name, "name", "", "Image name (required)")
	manifestsInspectCmd.Flags().StringVar(&manifestsOpts.registry, "registry", "", "Registry to inspect (required)")
	manifestsInspectCmd.Flags().StringSliceVarP(&manifestsOpts.tags, "tag", "t", []string{"latest"}, "Image tag to inspect")
	manifestsInspectCmd.Flags().StringVar(&manifestsOpts.namespace, "namespace", "", "Image namespace/organization")
	manifestsInspectCmd.Flags().StringVar(&manifestsOpts.authFile, "auth-file", "", "Path to authentication file")
	_ = manifestsInspectCmd.MarkFlagRequired("name")
	_ = manifestsInspectCmd.MarkFlagRequired("registry")

	// List command flags
	manifestsListCmd.Flags().StringVar(&manifestsOpts.name, "name", "", "Image name (required)")
	manifestsListCmd.Flags().StringVar(&manifestsOpts.registry, "registry", "", "Registry to list from (required)")
	manifestsListCmd.Flags().StringVar(&manifestsOpts.namespace, "namespace", "", "Image namespace/organization")
	manifestsListCmd.Flags().StringVar(&manifestsOpts.authFile, "auth-file", "", "Path to authentication file")
	_ = manifestsListCmd.MarkFlagRequired("name")
	_ = manifestsListCmd.MarkFlagRequired("registry")
}

// runManifestsCreate executes the manifest create command
func runManifestsCreate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	logging.InfoContext(ctx, "Creating multi-arch manifest for %s", manifestsOpts.name)

	// Discover, validate, and filter digests
	filteredDigests := discoverAndValidateDigests(ctx)

	// Perform registry checks
	performRegistryChecks(ctx, cmd, filteredDigests)

	// Handle idempotency check
	if !manifestsOpts.force {
		exists, err := manifests.CheckManifestExists(ctx, filteredDigests)
		if err != nil {
			logging.WarnContext(ctx, "Failed to check existing manifest: %v", err)
		} else if exists {
			logging.InfoContext(ctx, "Manifest already exists and is up-to-date (use --force to recreate)")
			return nil
		}
	}

	// Handle dry-run mode
	if manifestsOpts.dryRun {
		return handleDryRun(ctx, filteredDigests)
	}

	// Parse metadata and create manifests
	annotations, labels := parseMetadata(ctx)
	if err := createAndPushManifests(ctx, filteredDigests, annotations, labels); err != nil {
		return err
	}

	return nil
}

// discoverAndValidateDigests discovers, validates, and filters digest files
func discoverAndValidateDigests(ctx context.Context) []manifests.DigestFile {
	// Discover digest files
	digestFiles, err := manifests.DiscoverDigestFiles(manifests.DiscoveryOptions{
		ImageName: manifestsOpts.name,
		Directory: manifestsOpts.digestDir,
	})
	if err != nil {
		logging.ErrorContext(ctx, "Failed to discover digest files: %v", err)
		os.Exit(ExitValidationError)
	}

	if len(digestFiles) == 0 {
		logging.ErrorContext(ctx, "No digest files found in %s", manifestsOpts.digestDir)
		logging.ErrorContext(ctx, "Expected files matching pattern: digest-%s-*.txt", manifestsOpts.name)
		os.Exit(ExitValidationError)
	}

	logging.InfoContext(ctx, "Found %d digest file(s):", len(digestFiles))
	for _, df := range digestFiles {
		logging.InfoContext(ctx, "  - %s: %s", df.Architecture, df.Digest.String())
	}

	// Validate digest files
	if err := manifests.ValidateDigestFiles(digestFiles, manifests.ValidationOptions{
		ImageName: manifestsOpts.name,
		MaxAge:    manifestsOpts.maxAge,
	}); err != nil {
		logging.ErrorContext(ctx, "Digest validation failed: %v", err)
		os.Exit(ExitValidationError)
	}

	// Filter architectures based on requirements
	filteredDigests, err := manifests.FilterArchitectures(digestFiles, manifests.FilterOptions{
		RequiredArchitectures: manifestsOpts.requireArch,
		BestEffort:            manifestsOpts.bestEffort,
	})
	if err != nil {
		logging.ErrorContext(ctx, "Architecture filtering failed: %v", err)
		os.Exit(ExitValidationError)
	}

	if len(filteredDigests) == 0 {
		logging.ErrorContext(ctx, "No valid architectures found after filtering")
		os.Exit(ExitValidationError)
	}

	return filteredDigests
}

// performRegistryChecks performs health check and digest verification if requested
func performRegistryChecks(ctx context.Context, cmd *cobra.Command, filteredDigests []manifests.DigestFile) {
	// Perform registry health check if requested
	if manifestsOpts.healthCheck {
		if err := manifests.HealthCheckRegistry(ctx, manifests.VerificationOptions{
			Registry:  manifestsOpts.registry,
			Namespace: manifestsOpts.namespace,
			AuthFile:  manifestsOpts.authFile,
		}); err != nil {
			logging.ErrorContext(ctx, "Registry health check failed: %v", err)
			os.Exit(ExitRegistryError)
		}
	}

	// Verify digests exist in registry if requested
	if manifestsOpts.verifyRegistry {
		verifyDigestsInRegistry(ctx, cmd, filteredDigests)
	}
}

// verifyDigestsInRegistry verifies that digests exist in the registry
func verifyDigestsInRegistry(ctx context.Context, cmd *cobra.Command, filteredDigests []manifests.DigestFile) {
	// Use first tag for verification
	tag := "latest"
	if len(manifestsOpts.tags) > 0 {
		tag = manifestsOpts.tags[0]
	}

	// Get concurrency from config if not set via CLI
	concurrency := manifestsOpts.verifyConcurrency
	if concurrency == 0 {
		// Try to load from config
		if cfg := configFromContext(cmd); cfg != nil && cfg.Manifests.VerifyConcurrency > 0 {
			concurrency = cfg.Manifests.VerifyConcurrency
		} else {
			concurrency = 5 // Built-in default
		}
	}

	if err := manifests.VerifyDigestsInRegistry(ctx, filteredDigests, manifests.VerificationOptions{
		Registry:      manifestsOpts.registry,
		Namespace:     manifestsOpts.namespace,
		Tag:           tag,
		AuthFile:      manifestsOpts.authFile,
		MaxConcurrent: concurrency,
	}); err != nil {
		logging.ErrorContext(ctx, "Registry verification failed: %v", err)
		os.Exit(ExitDigestNotFound)
	}
}

// parseMetadata parses annotations and labels from command line options
func parseMetadata(ctx context.Context) (map[string]string, map[string]string) {
	parser := cli.NewParser()

	annotations, err := parser.ParseKeyValuePairs(manifestsOpts.annotations)
	if err != nil {
		logging.ErrorContext(ctx, "Failed to parse annotations: %v", err)
		os.Exit(ExitValidationError)
	}

	labels, err := parser.ParseKeyValuePairs(manifestsOpts.labels)
	if err != nil {
		logging.ErrorContext(ctx, "Failed to parse labels: %v", err)
		os.Exit(ExitValidationError)
	}

	return annotations, labels
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

	for _, tag := range manifestsOpts.tags {
		logging.InfoContext(ctx, "Creating manifest for tag: %s", tag)

		// Convert DigestFiles to ManifestEntries
		entries := convertDigestFilesToManifestEntries(filteredDigests, manifestsOpts.registry, manifestsOpts.namespace, tag)

		// Build the manifest name/destination
		manifestName := manifests.BuildManifestReference(manifestsOpts.registry, manifestsOpts.namespace, manifestsOpts.name, tag)

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
		logging.ErrorContext(ctx, "Failed to push %d of %d tag(s): %v", len(failedTags), len(manifestsOpts.tags), failedTags)
		logging.InfoContext(ctx, "Successfully pushed %d tag(s): %v", len(successTags), successTags)
		return fmt.Errorf("failed to push %d of %d tag(s)", len(failedTags), len(manifestsOpts.tags))
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
	logging.InfoContext(ctx, "Would push with %d tag(s):", len(manifestsOpts.tags))
	for _, tag := range manifestsOpts.tags {
		manifestRef := manifests.BuildManifestReference(manifestsOpts.registry, manifestsOpts.namespace, manifestsOpts.name, tag)
		logging.InfoContext(ctx, "  - %s", manifestRef)
	}

	return nil
}

// runManifestsInspect inspects a manifest from the registry
func runManifestsInspect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Use first tag
	tag := "latest"
	if len(manifestsOpts.tags) > 0 {
		tag = manifestsOpts.tags[0]
	}

	manifestRef := manifests.BuildManifestReference(manifestsOpts.registry, manifestsOpts.namespace, manifestsOpts.name, tag)
	logging.InfoContext(ctx, "Inspecting manifest: %s", manifestRef)

	// Inspect the manifest
	manifestInfo, err := manifests.InspectManifest(ctx, manifests.InspectOptions{
		Registry:  manifestsOpts.registry,
		Namespace: manifestsOpts.namespace,
		ImageName: manifestsOpts.name,
		Tag:       tag,
		AuthFile:  manifestsOpts.authFile,
	})
	if err != nil {
		logging.ErrorContext(ctx, "Failed to inspect manifest: %v", err)
		os.Exit(ExitRegistryError)
	}

	// Display manifest information
	displayManifestInfo(manifestInfo)

	return nil
}

// runManifestsList lists available manifest tags
func runManifestsList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	imageRef := manifests.BuildManifestReference(manifestsOpts.registry, manifestsOpts.namespace, manifestsOpts.name, "")
	logging.InfoContext(ctx, "Listing tags for: %s", strings.TrimSuffix(imageRef, ":"))

	// List tags
	tags, err := manifests.ListTags(ctx, manifests.ListOptions{
		Registry:  manifestsOpts.registry,
		Namespace: manifestsOpts.namespace,
		ImageName: manifestsOpts.name,
		AuthFile:  manifestsOpts.authFile,
	})
	if err != nil {
		logging.ErrorContext(ctx, "Failed to list tags: %v", err)
		os.Exit(ExitRegistryError)
	}

	if len(tags) == 0 {
		logging.InfoContext(ctx, "No tags found")
		return nil
	}

	logging.InfoContext(ctx, "Found %d tag(s):", len(tags))
	for _, tag := range tags {
		fmt.Printf("  - %s\n", tag)
	}

	return nil
}

// displayManifestInfo displays detailed manifest information
func displayManifestInfo(info *manifests.ManifestInfo) {
	fmt.Println("\n=== Manifest Information ===")
	fmt.Printf("Name:         %s\n", info.Name)
	fmt.Printf("Tag:          %s\n", info.Tag)
	fmt.Printf("Digest:       %s\n", info.Digest)

	isMultiArch := len(info.Architectures) > 1
	if isMultiArch {
		fmt.Printf("Media Type:   %s (multi-architecture manifest)\n", info.MediaType)
	} else {
		fmt.Printf("Media Type:   %s (single-architecture manifest)\n", info.MediaType)
	}

	fmt.Printf("Size:         %d bytes\n", info.Size)

	if len(info.Annotations) > 0 {
		fmt.Println("\nAnnotations:")
		for k, v := range info.Annotations {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	if isMultiArch {
		fmt.Printf("\n=== Architectures (%d) ===\n", len(info.Architectures))
		for i, arch := range info.Architectures {
			fmt.Printf("\n[%d] %s/%s", i+1, arch.OS, arch.Architecture)
			if arch.Variant != "" {
				fmt.Printf("/%s", arch.Variant)
			}
			fmt.Println()
			fmt.Printf("    Manifest Digest: %s\n", arch.Digest)
			fmt.Printf("    Size:            %d bytes\n", arch.Size)
			if arch.MediaType != "" {
				fmt.Printf("    Media Type:      %s\n", arch.MediaType)
			}
		}
	} else {
		fmt.Println("\n=== Platform ===")
		arch := info.Architectures[0]
		fmt.Printf("\nOS/Architecture: %s/%s", arch.OS, arch.Architecture)
		if arch.Variant != "" {
			fmt.Printf("/%s", arch.Variant)
		}
		fmt.Println()
		fmt.Printf("Config Digest:   %s\n", arch.Digest)
		fmt.Printf("Config Size:     %d bytes\n", arch.Size)
		if arch.MediaType != "" {
			fmt.Printf("Config Media:    %s\n", arch.MediaType)
		}
	}
	fmt.Println()
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

// manifestBuilder is an interface for builders that support manifest operations
type manifestBuilder interface {
	builder.ContainerBuilder
	CreateAndPushManifest(ctx context.Context, manifestName string, entries []manifests.ManifestEntry) error
}

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
