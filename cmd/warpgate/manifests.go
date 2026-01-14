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
	"time"

	"github.com/spf13/cobra"
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
	// Required flags for create command
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.name, "name", "", "Image name (required)")
	manifestsCreateCmd.Flags().StringVar(&manifestsOpts.registry, "registry", "", "Registry to push to (required)")
	_ = manifestsCreateCmd.MarkFlagRequired("name")
	_ = manifestsCreateCmd.MarkFlagRequired("registry")

	// Optional flags for create command
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

	manifestsCmd.AddCommand(manifestsCreateCmd)
	manifestsCmd.AddCommand(manifestsInspectCmd)
	manifestsCmd.AddCommand(manifestsListCmd)

	registerManifestsCompletions(manifestsCreateCmd)
	registerManifestsCompletions(manifestsInspectCmd)
	registerManifestsCompletions(manifestsListCmd)
}

// registerManifestsCompletions registers dynamic shell completion functions for manifests command flags.
func registerManifestsCompletions(cmd *cobra.Command) {
	// Registry completion with common registries
	_ = cmd.RegisterFlagCompletionFunc("registry", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"ghcr.io\tGitHub Container Registry",
			"docker.io\tDocker Hub",
			"gcr.io\tGoogle Container Registry",
			"quay.io\tRed Hat Quay",
			"registry.gitlab.com\tGitLab Container Registry",
		}, cobra.ShellCompDirectiveNoFileComp
	})

	// Architecture completion for require-arch flag
	_ = cmd.RegisterFlagCompletionFunc("require-arch", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"amd64\tLinux/macOS x86_64",
			"arm64\tLinux/macOS ARM64 (Apple Silicon, AWS Graviton)",
			"arm/v7\tARM 32-bit (Raspberry Pi)",
		}, cobra.ShellCompDirectiveNoFileComp
	})
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
