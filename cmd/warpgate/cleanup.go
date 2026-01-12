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
	region    string
	dryRun    bool
	all       bool
	buildName string
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
  warpgate cleanup --all --dry-run`,
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
}

func runCleanup(cmd *cobra.Command, opts *cleanupOptions) error {
	ctx := cmd.Context()
	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	// Require either a build name or --all flag
	if opts.buildName == "" && !opts.all {
		return fmt.Errorf("either specify a build name or use --all flag")
	}

	// Determine region
	region := opts.region
	if region == "" {
		region = cfg.AWS.Region
	}
	if region == "" {
		return fmt.Errorf("AWS region must be specified (--region flag or in config)")
	}

	logging.InfoContext(ctx, "Connecting to AWS in region: %s", region)

	// Create AWS clients
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

	cleaner := ami.NewResourceCleaner(clients)

	if opts.all {
		return runCleanupAll(ctx, cleaner, opts.dryRun)
	}

	return runCleanupBuild(ctx, cleaner, opts.buildName, opts.dryRun)
}

func runCleanupAll(ctx context.Context, cleaner *ami.ResourceCleaner, dryRun bool) error {
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

	// Confirm deletion
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

	return cleaner.DeleteResources(ctx, resources)
}

func runCleanupBuild(ctx context.Context, cleaner *ami.ResourceCleaner, buildName string, dryRun bool) error {
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

	// Confirm deletion
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

	return cleaner.DeleteResources(ctx, resources)
}
