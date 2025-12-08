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
	"os"
	"time"

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
	tag            string
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
  # Create manifest with all found architectures
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo

  # Create manifest with specific architectures
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --require-arch amd64,arm64

  # Create manifest with best-effort (include available architectures)
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --require-arch amd64,arm64 --best-effort

  # Verify digests exist in registry before creating manifest
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --verify-registry

  # Only use digests less than 1 hour old
  warpgate manifests create --name attack-box --registry ghcr.io/cowdogmoo \
    --max-age 1h

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
	manifestsCreateCmd.Flags().StringVarP(&manifestsOpts.tag, "tag", "t", "latest", "Image tag")
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.digestDir, "digest-dir", ".", "Directory containing digest files")
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.namespace, "namespace", "", "Image namespace/organization")

	// Security & Validation
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.verifyRegistry, "verify-registry", true, "Verify digests exist in registry")
	manifestsCreateCmd.Flags().DurationVar(&manifestsOpts.maxAge, "max-age", 0, "Maximum age of digest files (e.g., 1h, 30m)")

	// Architecture Control
	manifestsCreateCmd.Flags().StringSliceVar(&manifestsOpts.requireArch, "require-arch", nil, "Required architectures (comma-separated)")
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.bestEffort, "best-effort", false, "Create manifest with available architectures")

	// Authentication
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.authFile, "auth-file", "", "Path to authentication file")

	// Behavior
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.force, "force", false, "Force recreation even if manifest exists")
	manifestsCreateCmd.Flags().BoolVar(&manifestsOpts.dryRun, "dry-run", false, "Preview manifest without pushing")
	manifestsCreateCmd.Flags().BoolVarP(&manifestsOpts.quiet, "quiet", "q", false, "Only output errors")
	manifestsCreateCmd.Flags().BoolVarP(&manifestsOpts.verbose, "verbose", "v", false, "Verbose output with detailed progress")

	// Add create subcommand
	manifestsCmd.AddCommand(manifestsCreateCmd)
}

// runManifestsCreate executes the manifest create command
func runManifestsCreate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize logging with command-specific options
	if err := initManifestLogging(); err != nil {
		return err
	}

	logging.Info("Creating multi-arch manifest for %s", manifestsOpts.name)

	// Discover digest files
	digestFiles, err := manifests.DiscoverDigestFiles(manifests.DiscoveryOptions{
		ImageName: manifestsOpts.name,
		Directory: manifestsOpts.digestDir,
	})
	if err != nil {
		logging.Error("Failed to discover digest files: %v", err)
		os.Exit(ExitValidationError)
	}

	if len(digestFiles) == 0 {
		logging.Error("No digest files found in %s", manifestsOpts.digestDir)
		logging.Error("Expected files matching pattern: digest-%s-*.txt", manifestsOpts.name)
		os.Exit(ExitValidationError)
	}

	logging.Info("Found %d digest file(s):", len(digestFiles))
	for _, df := range digestFiles {
		logging.Info("  - %s: %s", df.Architecture, df.Digest.String())
	}

	// Validate digest files
	if err := manifests.ValidateDigestFiles(digestFiles, manifests.ValidationOptions{
		ImageName: manifestsOpts.name,
		MaxAge:    manifestsOpts.maxAge,
	}); err != nil {
		logging.Error("Digest validation failed: %v", err)
		os.Exit(ExitValidationError)
	}

	// Filter architectures based on requirements
	filteredDigests, err := manifests.FilterArchitectures(digestFiles, manifests.FilterOptions{
		RequiredArchitectures: manifestsOpts.requireArch,
		BestEffort:            manifestsOpts.bestEffort,
	})
	if err != nil {
		logging.Error("Architecture filtering failed: %v", err)
		os.Exit(ExitValidationError)
	}

	if len(filteredDigests) == 0 {
		logging.Error("No valid architectures found after filtering")
		os.Exit(ExitValidationError)
	}

	// Verify digests exist in registry if requested
	if manifestsOpts.verifyRegistry {
		if err := manifests.VerifyDigestsInRegistry(ctx, filteredDigests, manifests.VerificationOptions{
			Registry:  manifestsOpts.registry,
			Namespace: manifestsOpts.namespace,
			Tag:       manifestsOpts.tag,
			AuthFile:  manifestsOpts.authFile,
		}); err != nil {
			logging.Error("Registry verification failed: %v", err)
			os.Exit(ExitDigestNotFound)
		}
	}

	// Handle idempotency check
	if !manifestsOpts.force {
		exists, err := manifests.CheckManifestExists(ctx, filteredDigests)
		if err != nil {
			logging.Warn("Failed to check existing manifest: %v", err)
		} else if exists {
			logging.Info("Manifest already exists and is up-to-date (use --force to recreate)")
			return nil
		}
	}

	// Handle dry-run mode
	if manifestsOpts.dryRun {
		return handleDryRun(filteredDigests)
	}

	// Create and push the manifest
	if err := manifests.CreateAndPushManifest(ctx, filteredDigests, manifests.CreationOptions{
		Registry:  manifestsOpts.registry,
		Namespace: manifestsOpts.namespace,
		ImageName: manifestsOpts.name,
		Tag:       manifestsOpts.tag,
	}); err != nil {
		logging.Error("Failed to create/push manifest: %v", err)
		os.Exit(ExitRegistryError)
	}

	manifestRef := manifests.BuildManifestReference(manifestsOpts.registry, manifestsOpts.namespace, manifestsOpts.name, manifestsOpts.tag)
	logging.Info("Successfully created and pushed manifest to %s", manifestRef)

	return nil
}

// handleDryRun handles dry-run mode by previewing what would be created
func handleDryRun(filteredDigests []manifests.DigestFile) error {
	logging.Info("Dry run: would create manifest with %d architecture(s):", len(filteredDigests))
	for _, df := range filteredDigests {
		logging.Info("  - %s: %s", df.Architecture, df.Digest.String())
	}
	manifestRef := manifests.BuildManifestReference(manifestsOpts.registry, manifestsOpts.namespace, manifestsOpts.name, manifestsOpts.tag)
	logging.Info("Would push to: %s", manifestRef)
	return nil
}

// initManifestLogging initializes logging for manifest commands
func initManifestLogging() error {
	logLevel := "info"
	if manifestsOpts.verbose {
		logLevel = "debug"
	}
	if manifestsOpts.quiet {
		logLevel = "error"
	}

	return logging.Initialize(logLevel, "color", manifestsOpts.quiet, manifestsOpts.verbose)
}
