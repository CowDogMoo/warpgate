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

	"github.com/cowdogmoo/warpgate/v3/builder/ami"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
)

// cleanupOptions holds command-line options for the cleanup command
type cleanupOptions struct {
	region       string
	dryRun       bool
	all          bool
	buildName    string
	versions     bool // Clean up old component versions
	keepVersions int  // Number of versions to keep when cleaning up old versions
	yes          bool // Skip confirmation prompts (non-interactive mode)
}

var cleanupCmd *cobra.Command

func init() {
	opts := &cleanupOptions{}

	cleanupCmd = &cobra.Command{
		Use:   "cleanup [build-name]",
		Short: "Clean up AWS Image Builder resources",
		Long: `Clean up AWS Image Builder resources created by warpgate.

This command removes orphaned or leftover resources from failed or interrupted builds.
Resources include: components, infrastructure configurations, distribution configurations,
image recipes, and image pipelines.

Examples:
  # Clean up resources for a specific build
  warpgate cleanup my-template

  # Dry-run to see what would be deleted
  warpgate cleanup my-template --dry-run

  # Clean up resources in a specific region
  warpgate cleanup my-template --region us-west-2

  # List all warpgate-created resources (dry-run)
  warpgate cleanup --all --dry-run

  # Clean up old component versions, keeping 3 most recent
  warpgate cleanup my-template --versions --keep 3

  # Clean up old versions for all warpgate components
  warpgate cleanup --all --versions --keep 5

  # Non-interactive mode for CI/CD pipelines (skip confirmation prompts)
  warpgate cleanup my-template --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.buildName = args[0]
			}
			return runCleanup(cmd, opts)
		},
	}

	cleanupCmd.Flags().StringVar(&opts.region, "region", "", "AWS region (uses config default if not specified)")
	cleanupCmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Show what would be deleted without actually deleting")
	cleanupCmd.Flags().BoolVar(&opts.all, "all", false, "List/clean all warpgate-created resources")
	cleanupCmd.Flags().BoolVar(&opts.versions, "versions", false, "Clean up old component versions instead of all resources")
	cleanupCmd.Flags().IntVar(&opts.keepVersions, "keep", 3, "Number of component versions to keep (used with --versions)")
	cleanupCmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompts (non-interactive mode for CI/CD)")
}

func runCleanup(cmd *cobra.Command, opts *cleanupOptions) error {
	ctx := cmd.Context()
	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	if opts.buildName == "" && !opts.all {
		return fmt.Errorf("either specify a build name or use --all flag")
	}

	region := opts.region
	if region == "" {
		region = cfg.AWS.Region
	}
	if region == "" {
		return fmt.Errorf("AWS region must be specified (--region flag or in config)")
	}

	logging.InfoContext(ctx, "Connecting to AWS in region: %s", region)

	amiConfig := ami.ClientConfig{
		Region:          region,
		Profile:         cfg.AWS.Profile,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		SessionToken:    cfg.AWS.SessionToken,
	}

	clients, err := ami.NewAWSClients(ctx, amiConfig)
	if err != nil {
		return fmt.Errorf("failed to create AWS clients: %w", err)
	}

	// Handle version cleanup mode
	if opts.versions {
		resourceManager := ami.NewResourceManager(clients)
		return runVersionCleanup(ctx, resourceManager, opts)
	}

	cleaner := ami.NewResourceCleaner(clients)

	if opts.all {
		return runCleanupAll(ctx, cleaner, opts.dryRun, opts.yes)
	}

	return runCleanupBuild(ctx, cleaner, opts.buildName, opts.dryRun, opts.yes)
}

func runCleanupAll(ctx context.Context, cleaner *ami.ResourceCleaner, dryRun, skipConfirmation bool) error {
	logging.Info("Scanning for all warpgate-created resources...")

	resources, err := cleaner.ListWarpgateResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	if len(resources) == 0 {
		logging.Info("No warpgate-created resources found")
		return nil
	}

	// Display resources
	fmt.Println("\nWarpgate-created resources found:")
	fmt.Println("================================")

	for _, r := range resources {
		fmt.Printf("  [%s] %s\n", r.Type, r.Name)
		fmt.Printf("       ARN: %s\n", r.ARN)
		if r.BuildName != "" {
			fmt.Printf("       Build: %s\n", r.BuildName)
		}
		fmt.Println()
	}

	fmt.Printf("Total: %d resources\n\n", len(resources))

	if dryRun {
		logging.Info("Dry-run mode - no resources were deleted")
		return nil
	}

	// Skip confirmation if --yes flag is set
	if !skipConfirmation {
		fmt.Print("Delete all these resources? [y/N]: ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			// If Scanln fails (e.g., empty input), treat as 'N'
			logging.Info("Cleanup cancelled")
			return nil
		}
		if response != "y" && response != "Y" {
			logging.Info("Cleanup cancelled")
			return nil
		}
	}

	return cleaner.DeleteResources(ctx, resources)
}

func runCleanupBuild(ctx context.Context, cleaner *ami.ResourceCleaner, buildName string, dryRun, skipConfirmation bool) error {
	logging.Info("Scanning for resources from build: %s", buildName)

	resources, err := cleaner.ListResourcesForBuild(ctx, buildName)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	if len(resources) == 0 {
		logging.Info("No resources found for build: %s", buildName)
		return nil
	}

	// Display resources
	fmt.Printf("\nResources for build '%s':\n", buildName)
	fmt.Println("================================")

	for _, r := range resources {
		fmt.Printf("  [%s] %s\n", r.Type, r.Name)
		fmt.Printf("       ARN: %s\n", r.ARN)
		fmt.Println()
	}

	fmt.Printf("Total: %d resources\n\n", len(resources))

	if dryRun {
		logging.Info("Dry-run mode - no resources were deleted")
		return nil
	}

	// Skip confirmation if --yes flag is set
	if !skipConfirmation {
		fmt.Printf("Delete all resources for build '%s'? [y/N]: ", buildName)
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			// If Scanln fails (e.g., empty input), treat as 'N'
			logging.Info("Cleanup cancelled")
			return nil
		}
		if response != "y" && response != "Y" {
			logging.Info("Cleanup cancelled")
			return nil
		}
	}

	return cleaner.DeleteResources(ctx, resources)
}

// componentInfo holds version information for a component
type componentInfo struct {
	name     string
	versions int
	toDelete int
}

func runVersionCleanup(ctx context.Context, manager *ami.ResourceManager, opts *cleanupOptions) error {
	logging.Info("Scanning for component versions to clean up (keeping %d most recent)...", opts.keepVersions)

	componentNames, err := getComponentNames(ctx, manager, opts)
	if err != nil {
		return err
	}
	if len(componentNames) == 0 {
		logging.Info("No components found")
		return nil
	}

	infos := getComponentInfos(ctx, manager, componentNames, opts.keepVersions)
	totalToDelete := displayComponentInfos(infos, len(componentNames))

	if totalToDelete == 0 {
		logging.Info("No old versions to clean up")
		return nil
	}

	if opts.dryRun {
		logging.Info("Dry-run mode - no versions were deleted")
		return nil
	}

	if !opts.yes && !confirmVersionCleanup() {
		return nil
	}

	return performVersionCleanup(ctx, manager, componentNames, opts.keepVersions)
}

// getComponentNames retrieves component names based on cleanup options
func getComponentNames(ctx context.Context, manager *ami.ResourceManager, opts *cleanupOptions) ([]string, error) {
	prefix := opts.buildName
	if opts.all {
		prefix = ""
	}

	components, err := manager.ListComponentsByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list components: %w", err)
	}

	seen := make(map[string]bool)
	var names []string
	for _, c := range components {
		if c.Name != nil && !seen[*c.Name] {
			names = append(names, *c.Name)
			seen[*c.Name] = true
		}
	}
	return names, nil
}

// getComponentInfos gathers version information for each component
func getComponentInfos(ctx context.Context, manager *ami.ResourceManager, names []string, keepVersions int) []componentInfo {
	var infos []componentInfo
	for _, name := range names {
		versions, err := manager.GetComponentVersions(ctx, name)
		if err != nil {
			logging.Warn("Failed to get versions for %s: %v", name, err)
			continue
		}

		toDelete := 0
		if len(versions) > keepVersions {
			toDelete = len(versions) - keepVersions
		}

		infos = append(infos, componentInfo{
			name:     name,
			versions: len(versions),
			toDelete: toDelete,
		})
	}
	return infos
}

// displayComponentInfos displays component info and returns total versions to delete
func displayComponentInfos(infos []componentInfo, componentCount int) int {
	fmt.Printf("\nComponents found:\n")
	fmt.Println("================")

	totalToDelete := 0
	for _, info := range infos {
		fmt.Printf("  %s: %d versions", info.name, info.versions)
		if info.toDelete > 0 {
			fmt.Printf(" (%d to delete)", info.toDelete)
		}
		fmt.Println()
		totalToDelete += info.toDelete
	}

	if totalToDelete > 0 {
		fmt.Printf("\nTotal: %d versions to delete across %d components\n\n", totalToDelete, componentCount)
	}
	return totalToDelete
}

// confirmVersionCleanup prompts for user confirmation
func confirmVersionCleanup() bool {
	fmt.Print("Delete old component versions? [y/N]: ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		logging.Info("Version cleanup cancelled")
		return false
	}
	if response != "y" && response != "Y" {
		logging.Info("Version cleanup cancelled")
		return false
	}
	return true
}

// performVersionCleanup executes the version cleanup
func performVersionCleanup(ctx context.Context, manager *ami.ResourceManager, names []string, keepVersions int) error {
	totalDeleted := 0
	for _, name := range names {
		deleted, err := manager.CleanupOldComponentVersions(ctx, name, keepVersions)
		if err != nil {
			logging.Warn("Failed to clean up versions for %s: %v", name, err)
			continue
		}
		totalDeleted += len(deleted)
	}

	logging.Info("Successfully deleted %d old component versions", totalDeleted)
	return nil
}
