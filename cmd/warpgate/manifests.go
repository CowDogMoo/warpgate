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
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// manifestsSharedOptions holds flags shared across all manifests subcommands.
// These are defined as PersistentFlags on the parent manifestsCmd.
type manifestsSharedOptions struct {
	registry  string
	namespace string
	authFile  string
}

// manifestsCreateOptions holds command-line options for the manifests create command
type manifestsCreateOptions struct {
	// Required flags (name is command-specific)
	name string

	// Optional flags
	tags           []string
	digestDir      string
	verifyRegistry bool
	maxAge         time.Duration
	requireArch    []string
	bestEffort     bool
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

// manifestsInspectOptions holds command-line options for the manifests inspect command
type manifestsInspectOptions struct {
	name string
	tags []string
}

// manifestsListOptions holds command-line options for the manifests list command
type manifestsListOptions struct {
	name string
}

var (
	manifestsCmd        *cobra.Command
	manifestsCreateCmd  *cobra.Command
	manifestsInspectCmd *cobra.Command
	manifestsListCmd    *cobra.Command

	// Shared options for all manifests subcommands (PersistentFlags)
	manifestsSharedOpts *manifestsSharedOptions

	// Command-local options
	manifestsCreateOpts  *manifestsCreateOptions
	manifestsInspectOpts *manifestsInspectOptions
	manifestsListOpts    *manifestsListOptions
)

func init() {
	manifestsSharedOpts = &manifestsSharedOptions{}
	manifestsCreateOpts = &manifestsCreateOptions{}
	manifestsInspectOpts = &manifestsInspectOptions{}
	manifestsListOpts = &manifestsListOptions{}

	manifestsCmd = &cobra.Command{
		Use:   "manifests",
		Short: "Manage multi-architecture manifests",
		Long: `Manage multi-architecture container image manifests.

This command provides tools for creating and pushing multi-architecture
manifests from digest files generated during separate architecture builds.`,
		Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if manifestsSharedOpts.registry == "" {
				return fmt.Errorf("required flag \"registry\" not set")
			}
			return nil
		},
	}

	// --- Shared PersistentFlags for all manifests subcommands ---
	manifestsCmd.PersistentFlags().StringVar(&manifestsSharedOpts.registry, "registry", "", "Container registry (required)")
	manifestsCmd.PersistentFlags().StringVar(&manifestsSharedOpts.namespace, "namespace", "", "Image namespace/organization")
	manifestsCmd.PersistentFlags().StringVar(&manifestsSharedOpts.authFile, "auth-file", "", "Path to authentication file")

	manifestsCreateCmd = &cobra.Command{
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
		Args: cobra.NoArgs,
		RunE: runManifestsCreate,
	}

	manifestsInspectCmd = &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a multi-architecture manifest",
		Long: `Inspect a multi-architecture manifest from a registry.

Shows detailed information about the manifest including all architectures,
digests, sizes, and annotations.

Examples:
  # Inspect a manifest
  warpgate manifests inspect --name attack-box --registry ghcr.io/cowdogmoo

  # Inspect specific tag
  warpgate manifests inspect --name attack-box --registry ghcr.io/cowdogmoo --tag v1.0.0`,
		Args: cobra.NoArgs,
		RunE: runManifestsInspect,
	}

	manifestsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List available manifest tags",
		Long: `List available manifest tags for an image in the registry.

Examples:
  # List all tags
  warpgate manifests list --name attack-box --registry ghcr.io/cowdogmoo

  # List with details
  warpgate manifests list --name attack-box --registry ghcr.io/cowdogmoo --detailed`,
		Args: cobra.NoArgs,
		RunE: runManifestsList,
	}

	// --- Create command flags ---
	// Required flags (--registry is inherited from parent PersistentFlags)
	manifestsCreateCmd.Flags().StringVar(&manifestsCreateOpts.name, "name", "", "Image name (required)")
	_ = manifestsCreateCmd.MarkFlagRequired("name")

	// Optional flags
	manifestsCreateCmd.Flags().StringSliceVarP(&manifestsCreateOpts.tags, "tag", "t", []string{"latest"}, "Image tags (comma-separated, can specify multiple)")
	manifestsCreateCmd.Flags().StringVar(&manifestsCreateOpts.digestDir, "digest-dir", ".", "Directory containing digest files")

	// Security & Validation
	manifestsCreateCmd.Flags().BoolVar(&manifestsCreateOpts.verifyRegistry, "verify-registry", true, "Verify digests exist in registry")
	manifestsCreateCmd.Flags().IntVar(&manifestsCreateOpts.verifyConcurrency, "verify-concurrency", 0, "Number of concurrent digest verifications (default from config: 5)")
	manifestsCreateCmd.Flags().DurationVar(&manifestsCreateOpts.maxAge, "max-age", 0, "Maximum age of digest files (e.g., 1h, 30m)")
	manifestsCreateCmd.Flags().BoolVar(&manifestsCreateOpts.healthCheck, "health-check", false, "Perform registry health check before operations")

	// Architecture Control
	manifestsCreateCmd.Flags().StringSliceVar(&manifestsCreateOpts.requireArch, "require-arch", nil, "Required architectures (comma-separated)")
	manifestsCreateCmd.Flags().BoolVar(&manifestsCreateOpts.bestEffort, "best-effort", false, "Create manifest with available architectures")

	// OCI metadata
	manifestsCreateCmd.Flags().StringSliceVar(&manifestsCreateOpts.annotations, "annotation", nil, "OCI annotations (key=value, can specify multiple)")
	manifestsCreateCmd.Flags().StringSliceVar(&manifestsCreateOpts.labels, "label", nil, "OCI labels (key=value, can specify multiple)")
	manifestsCreateCmd.Flags().BoolVar(&manifestsCreateOpts.showDiff, "show-diff", false, "Show manifest comparison/diff if manifest exists")
	manifestsCreateCmd.Flags().BoolVar(&manifestsCreateOpts.noProgress, "no-progress", false, "Disable progress indicators")

	// Behavior
	manifestsCreateCmd.Flags().BoolVar(&manifestsCreateOpts.force, "force", false, "Force recreation even if manifest exists")
	manifestsCreateCmd.Flags().BoolVar(&manifestsCreateOpts.dryRun, "dry-run", false, "Preview manifest without pushing")
	manifestsCreateCmd.Flags().BoolVarP(&manifestsCreateOpts.quiet, "quiet", "q", false, "Only output errors")
	manifestsCreateCmd.Flags().BoolVarP(&manifestsCreateOpts.verbose, "verbose", "v", false, "Verbose output with detailed progress")

	// --- Inspect command flags ---
	// (--registry, --namespace, --auth-file inherited from parent PersistentFlags)
	manifestsInspectCmd.Flags().StringVar(&manifestsInspectOpts.name, "name", "", "Image name (required)")
	manifestsInspectCmd.Flags().StringSliceVarP(&manifestsInspectOpts.tags, "tag", "t", []string{"latest"}, "Image tag to inspect")
	_ = manifestsInspectCmd.MarkFlagRequired("name")

	// --- List command flags ---
	// (--registry, --namespace, --auth-file inherited from parent PersistentFlags)
	manifestsListCmd.Flags().StringVar(&manifestsListOpts.name, "name", "", "Image name (required)")
	_ = manifestsListCmd.MarkFlagRequired("name")

	manifestsCmd.AddCommand(manifestsCreateCmd)
	manifestsCmd.AddCommand(manifestsInspectCmd)
	manifestsCmd.AddCommand(manifestsListCmd)

	// Register shell completions
	// Register on parent for persistent flags (--registry)
	registerManifestsParentCompletions(manifestsCmd)
	// Register on create command for command-specific flags
	registerManifestsCreateCompletions(manifestsCreateCmd)
}

// registerManifestsParentCompletions registers completions for shared persistent flags.
func registerManifestsParentCompletions(cmd *cobra.Command) {
	// Registry completion with common registries (persistent flag)
	_ = cmd.RegisterFlagCompletionFunc("registry", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"ghcr.io\tGitHub Container Registry",
			"docker.io\tDocker Hub",
			"gcr.io\tGoogle Container Registry",
			"quay.io\tRed Hat Quay",
			"registry.gitlab.com\tGitLab Container Registry",
		}, cobra.ShellCompDirectiveNoFileComp
	})
}

// registerManifestsCreateCompletions registers completions for create command flags.
func registerManifestsCreateCompletions(cmd *cobra.Command) {
	// Architecture completion for require-arch flag
	_ = cmd.RegisterFlagCompletionFunc("require-arch", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"amd64\tLinux/macOS x86_64",
			"arm64\tLinux/macOS ARM64 (Apple Silicon, AWS Graviton)",
			"arm/v7\tARM 32-bit (Raspberry Pi)",
		}, cobra.ShellCompDirectiveNoFileComp
	})
}
