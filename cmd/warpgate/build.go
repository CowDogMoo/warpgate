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
	"sync"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/builder/ami"
	"github.com/cowdogmoo/warpgate/v3/cli"
	"github.com/cowdogmoo/warpgate/v3/config"
	wgit "github.com/cowdogmoo/warpgate/v3/git"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// buildOptions holds command-line options for the build command
type buildOptions struct {
	template        string
	fromGit         string
	targetType      string
	push            bool
	registry        string
	arch            []string
	tags            []string
	saveDigests     bool
	digestDir       string
	region          string
	regions         []string // Multi-region AMI builds
	instanceType    string
	vars            []string
	varFiles        []string
	cacheFrom       []string
	cacheTo         []string
	labels          []string
	buildArgs       []string
	noCache         bool
	forceRecreate   bool
	dryRun          bool
	parallelRegions bool     // Build in all regions in parallel
	copyToRegions   []string // Copy built AMI to additional regions
	streamLogs      bool     // Stream CloudWatch/SSM logs during build
	showEC2Status   bool     // Show EC2 instance status during build
}

var buildCmd *cobra.Command

func init() {
	opts := &buildOptions{}

	buildCmd = &cobra.Command{
		Use:   "build [config|template]",
		Short: "Build image from config or template",
		Long: `Build a container image or AMI from a template configuration.

Examples:
  # Build from local config file
  warpgate build warpgate.yaml

  # Build with variable overrides
  warpgate build warpgate.yaml --var PROVISION_REPO_PATH=/path/to/arsenal

  # Build with multiple variables
  warpgate build warpgate.yaml --var KEY1=value1 --var KEY2=value2

  # Build with variables from file
  warpgate build warpgate.yaml --var-file vars.yaml

  # Build from official template
  warpgate build --template attack-box

  # Build specific version
  warpgate build --template attack-box@v1.2.0

  # Build from git repository
  warpgate build --from-git https://github.com/user/templates.git//my-template

  # Build multiple architectures and push
  warpgate build --template attack-box --arch amd64,arm64 --push

  # Build AMI in different region with custom instance type
  warpgate build --template attack-box --target ami --region us-west-2 --instance-type t3.large

  # Build AMI in multiple regions
  warpgate build --template attack-box --target ami --regions us-east-1,us-west-2,eu-west-1

  # Build AMI in multiple regions in parallel
  warpgate build --template attack-box --target ami --regions us-east-1,us-west-2 --parallel-regions

  # Build AMI and copy to additional regions
  warpgate build --template attack-box --target ami --region us-east-1 --copy-to-regions us-west-2,eu-west-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, args, opts)
		},
	}

	buildCmd.Flags().StringVar(&opts.template, "template", "", "Use named template from registry")
	buildCmd.Flags().StringVar(&opts.fromGit, "from-git", "", "Load template from git URL")
	buildCmd.Flags().StringVar(&opts.targetType, "target", "", "Override target type (container, ami)")
	buildCmd.Flags().BoolVar(&opts.push, "push", false, "Push image to registry after build")
	buildCmd.Flags().StringVar(&opts.registry, "registry", "", "Registry to push to")
	buildCmd.Flags().StringSliceVar(&opts.arch, "arch", nil, "Architectures to build (comma-separated)")
	buildCmd.Flags().StringSliceVarP(&opts.tags, "tag", "t", []string{}, "Additional tags to apply")
	buildCmd.Flags().BoolVar(&opts.saveDigests, "save-digests", false, "Save image digests to files after push")
	buildCmd.Flags().StringVar(&opts.digestDir, "digest-dir", ".", "Directory to save digest files")
	buildCmd.Flags().StringVar(&opts.region, "region", "", "AWS region for AMI builds (overrides config)")
	buildCmd.Flags().StringVar(&opts.instanceType, "instance-type", "", "EC2 instance type for AMI builds (overrides config)")
	buildCmd.Flags().StringArrayVar(&opts.vars, "var", []string{}, "Set template variables (key=value)")
	buildCmd.Flags().StringArrayVar(&opts.varFiles, "var-file", []string{}, "Load variables from YAML file")
	buildCmd.Flags().StringArrayVar(&opts.cacheFrom, "cache-from", []string{}, "External cache sources for BuildKit (e.g., type=registry,ref=user/app:cache)")
	buildCmd.Flags().StringArrayVar(&opts.cacheTo, "cache-to", []string{}, "External cache destinations for BuildKit (e.g., type=registry,ref=user/app:cache,mode=max)")
	buildCmd.Flags().StringArrayVar(&opts.labels, "label", []string{}, "Set image labels (key=value)")
	buildCmd.Flags().StringArrayVar(&opts.buildArgs, "build-arg", []string{}, "Set build arguments (key=value)")
	buildCmd.Flags().BoolVar(&opts.noCache, "no-cache", false, "Disable all caching")
	buildCmd.Flags().BoolVar(&opts.forceRecreate, "force", false, "Force recreation of existing AWS resources (AMI builds only)")
	buildCmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Validate configuration without creating resources (AMI builds only)")
	buildCmd.Flags().StringSliceVar(&opts.regions, "regions", nil, "Build AMI in multiple regions (comma-separated)")
	buildCmd.Flags().BoolVar(&opts.parallelRegions, "parallel-regions", false, "Build in all regions in parallel (default: sequential)")
	buildCmd.Flags().StringSliceVar(&opts.copyToRegions, "copy-to-regions", nil, "Copy AMI to additional regions after build (comma-separated)")
	buildCmd.Flags().BoolVar(&opts.streamLogs, "stream-logs", false, "Stream CloudWatch/SSM logs from build instance (AMI builds only)")
	buildCmd.Flags().BoolVar(&opts.showEC2Status, "show-ec2-status", false, "Show EC2 instance status during build (AMI builds only)")
}

func runBuild(cmd *cobra.Command, args []string, opts *buildOptions) error {
	ctx := cmd.Context()
	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	logging.InfoContext(ctx, "Starting build process")

	parser := cli.NewParser()
	labels, buildArgs, err := parser.ParseLabelsAndBuildArgs(opts.labels, opts.buildArgs)
	if err != nil {
		return err
	}

	buildConfig, err := loadAndValidateBuildConfig(ctx, args, opts)
	if err != nil {
		return err
	}

	var configFilePath string
	if len(args) > 0 {
		configFilePath = args[0]
	}
	cleanup, err := wgit.FetchSourcesWithCleanup(ctx, buildConfig.Sources, configFilePath)
	if err != nil {
		return fmt.Errorf("failed to fetch sources: %w", err)
	}
	defer cleanup()

	if len(buildConfig.Sources) > 0 {
		updateProvisionerSourcePaths(buildConfig)
	}

	service := builder.NewBuildService(cfg, newBuildKitBuilderFunc)

	builderOpts := builder.BuildOptions{
		TargetType:    opts.targetType,
		Architectures: opts.arch,
		Registry:      opts.registry,
		Tags:          opts.tags,
		Region:        opts.region,
		InstanceType:  opts.instanceType,
		Labels:        labels,
		BuildArgs:     buildArgs,
		CacheFrom:     opts.cacheFrom,
		CacheTo:       opts.cacheTo,
		NoCache:       opts.noCache,
		Push:          opts.push,
		SaveDigests:   opts.saveDigests,
		DigestDir:     opts.digestDir,
		ForceRecreate: opts.forceRecreate,
	}

	targetType := builder.DetermineTargetType(buildConfig, builderOpts)

	results, err := executeBuild(ctx, targetType, service, cfg, buildConfig, builderOpts, opts)
	if err != nil {
		return err
	}
	if results == nil {
		return nil // dry-run completed successfully
	}

	formatter := cli.NewOutputFormatter("text")
	formatter.DisplayBuildResults(ctx, results)

	if opts.push && opts.registry != "" {
		return service.Push(ctx, *buildConfig, results, builderOpts)
	}

	return nil
}

// loadAndValidateBuildConfig validates CLI options and loads the build configuration
func loadAndValidateBuildConfig(ctx context.Context, args []string, opts *buildOptions) (*builder.Config, error) {
	validator := cli.NewValidator()
	cliOpts := buildOptsToCliOpts(args, opts)
	if err := validator.ValidateBuildOptions(cliOpts); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	buildConfig, err := loadBuildConfig(ctx, args, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load build configuration: %w", err)
	}
	return buildConfig, nil
}

// executeBuild dispatches to the appropriate build handler based on target type
func executeBuild(ctx context.Context, targetType string, service *builder.BuildService, cfg *config.Config, buildConfig *builder.Config, builderOpts builder.BuildOptions, opts *buildOptions) ([]builder.BuildResult, error) {
	switch targetType {
	case "container":
		results, err := service.ExecuteContainerBuild(ctx, *buildConfig, builderOpts)
		if err != nil {
			return nil, fmt.Errorf("container build failed: %w", err)
		}
		return results, nil
	case "ami":
		return executeAMIBuildTarget(ctx, service, cfg, buildConfig, builderOpts, opts)
	default:
		return nil, fmt.Errorf("unsupported target type: %s", targetType)
	}
}

// executeAMIBuildTarget handles the AMI-specific build logic
func executeAMIBuildTarget(ctx context.Context, service *builder.BuildService, cfg *config.Config, buildConfig *builder.Config, builderOpts builder.BuildOptions, opts *buildOptions) ([]builder.BuildResult, error) {
	// Determine regions to build in
	regions := determineTargetRegions(cfg, opts)

	if len(regions) > 1 {
		return executeMultiRegionAMIBuild(ctx, service, cfg, buildConfig, builderOpts, opts, regions)
	}

	// Single region build
	region := cfg.AWS.Region
	if opts.region != "" {
		region = opts.region
	}
	if len(regions) == 1 {
		region = regions[0]
	}

	amiConfig := ami.ClientConfig{
		Region:          region,
		Profile:         cfg.AWS.Profile,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		SessionToken:    cfg.AWS.SessionToken,
	}

	monitorConfig := ami.MonitorConfig{
		StreamLogs:    opts.streamLogs,
		ShowEC2Status: opts.showEC2Status,
	}

	amiBuilder, err := ami.NewImageBuilderWithAllOptions(ctx, amiConfig, opts.forceRecreate, monitorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AMI builder: %w", err)
	}
	defer func() {
		if closeErr := amiBuilder.Close(); closeErr != nil {
			logging.WarnContext(ctx, "Failed to close AMI builder: %v", closeErr)
		}
	}()

	if opts.dryRun {
		return handleAMIDryRun(ctx, amiBuilder, buildConfig)
	}

	result, err := service.ExecuteAMIBuild(ctx, *buildConfig, builderOpts, amiBuilder)
	if err != nil {
		return nil, fmt.Errorf("AMI build failed: %w", err)
	}

	results := []builder.BuildResult{*result}

	if len(opts.copyToRegions) > 0 {
		copyResults, err := copyAMIToRegions(ctx, cfg, result.AMIID, region, opts.copyToRegions)
		if err != nil {
			logging.WarnContext(ctx, "Some cross-region copies failed: %v", err)
		}
		results = append(results, copyResults...)
	}

	return results, nil
}

// determineTargetRegions determines which regions to build in
func determineTargetRegions(cfg *config.Config, opts *buildOptions) []string {
	// Priority: --regions > --region > config
	if len(opts.regions) > 0 {
		return opts.regions
	}
	if opts.region != "" {
		return []string{opts.region}
	}
	if cfg.AWS.Region != "" {
		return []string{cfg.AWS.Region}
	}
	return []string{}
}

// executeMultiRegionAMIBuild builds AMIs in multiple regions
func executeMultiRegionAMIBuild(ctx context.Context, service *builder.BuildService, cfg *config.Config, buildConfig *builder.Config, builderOpts builder.BuildOptions, opts *buildOptions, regions []string) ([]builder.BuildResult, error) {
	logging.InfoContext(ctx, "Building AMI in %d regions: %v", len(regions), regions)

	if opts.parallelRegions {
		return executeParallelRegionBuilds(ctx, service, cfg, buildConfig, builderOpts, opts, regions)
	}

	return executeSequentialRegionBuilds(ctx, service, cfg, buildConfig, builderOpts, opts, regions)
}

// executeSequentialRegionBuilds builds AMIs in regions one at a time
func executeSequentialRegionBuilds(ctx context.Context, service *builder.BuildService, cfg *config.Config, buildConfig *builder.Config, builderOpts builder.BuildOptions, opts *buildOptions, regions []string) ([]builder.BuildResult, error) {
	var allResults []builder.BuildResult

	for i, region := range regions {
		logging.InfoContext(ctx, "Building in region %d/%d: %s", i+1, len(regions), region)

		amiConfig := ami.ClientConfig{
			Region:          region,
			Profile:         cfg.AWS.Profile,
			AccessKeyID:     cfg.AWS.AccessKeyID,
			SecretAccessKey: cfg.AWS.SecretAccessKey,
			SessionToken:    cfg.AWS.SessionToken,
		}

		monitorConfig := ami.MonitorConfig{
			StreamLogs:    opts.streamLogs,
			ShowEC2Status: opts.showEC2Status,
		}

		amiBuilder, err := ami.NewImageBuilderWithAllOptions(ctx, amiConfig, opts.forceRecreate, monitorConfig)
		if err != nil {
			logging.ErrorContext(ctx, "Failed to create AMI builder for region %s: %v", region, err)
			continue
		}

		if opts.dryRun {
			dryRunResult, err := handleAMIDryRun(ctx, amiBuilder, buildConfig)
			if closeErr := amiBuilder.Close(); closeErr != nil {
				logging.WarnContext(ctx, "Failed to close AMI builder: %v", closeErr)
			}
			if err != nil {
				logging.ErrorContext(ctx, "Dry-run failed for region %s: %v", region, err)
				continue
			}
			if dryRunResult != nil {
				allResults = append(allResults, dryRunResult...)
			}
			continue
		}

		regionOpts := builderOpts
		regionOpts.Region = region

		result, err := service.ExecuteAMIBuild(ctx, *buildConfig, regionOpts, amiBuilder)
		if closeErr := amiBuilder.Close(); closeErr != nil {
			logging.WarnContext(ctx, "Failed to close AMI builder: %v", closeErr)
		}
		if err != nil {
			logging.ErrorContext(ctx, "Build failed in region %s: %v", region, err)
			continue
		}

		logging.InfoContext(ctx, "Built AMI in %s: %s", region, result.AMIID)
		allResults = append(allResults, *result)
	}

	if len(allResults) == 0 {
		return nil, fmt.Errorf("all regional builds failed")
	}

	logging.InfoContext(ctx, "Successfully built AMIs in %d/%d regions", len(allResults), len(regions))
	return allResults, nil
}

// executeParallelRegionBuilds builds AMIs in all regions simultaneously
func executeParallelRegionBuilds(ctx context.Context, service *builder.BuildService, cfg *config.Config, buildConfig *builder.Config, builderOpts builder.BuildOptions, opts *buildOptions, regions []string) ([]builder.BuildResult, error) {
	logging.InfoContext(ctx, "Building in parallel across %d regions", len(regions))

	// Pre-allocate results slice with mutex for thread-safe access
	var mu sync.Mutex
	allResults := make([]builder.BuildResult, 0, len(regions))

	g, ctx := errgroup.WithContext(ctx)

	for _, region := range regions {
		region := region // Capture loop variable

		g.Go(func() error {
			logging.InfoContext(ctx, "Starting build in region: %s", region)

			amiConfig := ami.ClientConfig{
				Region:          region,
				Profile:         cfg.AWS.Profile,
				AccessKeyID:     cfg.AWS.AccessKeyID,
				SecretAccessKey: cfg.AWS.SecretAccessKey,
				SessionToken:    cfg.AWS.SessionToken,
			}

			monitorConfig := ami.MonitorConfig{
				StreamLogs:    opts.streamLogs,
				ShowEC2Status: opts.showEC2Status,
			}

			amiBuilder, err := ami.NewImageBuilderWithAllOptions(ctx, amiConfig, opts.forceRecreate, monitorConfig)
			if err != nil {
				return fmt.Errorf("region %s: failed to create AMI builder: %w", region, err)
			}
			defer func() {
				if closeErr := amiBuilder.Close(); closeErr != nil {
					logging.WarnContext(ctx, "Failed to close AMI builder for %s: %v", region, closeErr)
				}
			}()

			if opts.dryRun {
				_, err := handleAMIDryRun(ctx, amiBuilder, buildConfig)
				if err != nil {
					return fmt.Errorf("region %s: dry-run failed: %w", region, err)
				}
				return nil
			}

			regionOpts := builderOpts
			regionOpts.Region = region

			result, err := service.ExecuteAMIBuild(ctx, *buildConfig, regionOpts, amiBuilder)
			if err != nil {
				return fmt.Errorf("region %s: build failed: %w", region, err)
			}

			mu.Lock()
			allResults = append(allResults, *result)
			mu.Unlock()

			logging.InfoContext(ctx, "Completed build in %s: %s", region, result.AMIID)
			return nil
		})
	}

	// Wait for all builds to complete
	if err := g.Wait(); err != nil {
		logging.WarnContext(ctx, "Some parallel builds failed: %v", err)
		// Return partial results if we have any
		if len(allResults) > 0 {
			logging.InfoContext(ctx, "Successfully built AMIs in %d/%d regions", len(allResults), len(regions))
			return allResults, nil
		}
		return nil, err
	}

	logging.InfoContext(ctx, "Successfully built AMIs in all %d regions", len(regions))
	return allResults, nil
}

// copyAMIToRegions copies an AMI to multiple target regions
func copyAMIToRegions(ctx context.Context, cfg *config.Config, amiID, sourceRegion string, targetRegions []string) ([]builder.BuildResult, error) {
	logging.InfoContext(ctx, "Copying AMI %s from %s to %d regions", amiID, sourceRegion, len(targetRegions))

	var results []builder.BuildResult
	var copyErrors []error

	for _, targetRegion := range targetRegions {
		if targetRegion == sourceRegion {
			logging.InfoContext(ctx, "Skipping copy to source region %s", targetRegion)
			continue
		}

		logging.InfoContext(ctx, "Copying AMI to region: %s", targetRegion)

		targetConfig := ami.ClientConfig{
			Region:          targetRegion,
			Profile:         cfg.AWS.Profile,
			AccessKeyID:     cfg.AWS.AccessKeyID,
			SecretAccessKey: cfg.AWS.SecretAccessKey,
			SessionToken:    cfg.AWS.SessionToken,
		}

		amiBuilder, err := ami.NewImageBuilder(ctx, targetConfig)
		if err != nil {
			copyErrors = append(copyErrors, fmt.Errorf("region %s: failed to create client: %w", targetRegion, err))
			continue
		}

		copiedAMIID, err := amiBuilder.Copy(ctx, amiID, sourceRegion, targetRegion)
		if closeErr := amiBuilder.Close(); closeErr != nil {
			logging.WarnContext(ctx, "Failed to close AMI builder for %s: %v", targetRegion, closeErr)
		}
		if err != nil {
			copyErrors = append(copyErrors, fmt.Errorf("region %s: copy failed: %w", targetRegion, err))
			continue
		}

		logging.InfoContext(ctx, "Copied AMI to %s: %s", targetRegion, copiedAMIID)
		results = append(results, builder.BuildResult{
			AMIID:  copiedAMIID,
			Region: targetRegion,
			Notes:  []string{fmt.Sprintf("Copied from %s:%s", sourceRegion, amiID)},
		})
	}

	if len(copyErrors) > 0 {
		return results, fmt.Errorf("some copies failed: %v", copyErrors)
	}

	return results, nil
}

// handleAMIDryRun performs dry-run validation for AMI builds
func handleAMIDryRun(ctx context.Context, amiBuilder *ami.ImageBuilder, buildConfig *builder.Config) ([]builder.BuildResult, error) {
	logging.InfoContext(ctx, "Running dry-run validation...")
	validationResult, err := amiBuilder.DryRun(ctx, *buildConfig)
	if err != nil {
		return nil, fmt.Errorf("dry-run validation failed: %w", err)
	}

	fmt.Println(validationResult.String())

	if !validationResult.Valid {
		return nil, fmt.Errorf("dry-run validation failed with %d errors", len(validationResult.Errors))
	}
	logging.InfoContext(ctx, "Dry-run validation completed successfully")
	return nil, nil
}

// updateProvisionerSourcePaths updates file provisioner source paths
// to use fetched sources when they reference ${sources.<name>}
func updateProvisionerSourcePaths(config *builder.Config) {
	sourceMap := make(map[string]string)
	for _, src := range config.Sources {
		sourceMap[src.Name] = src.Path
		logging.Info("[UPDATE PATHS] Source %s -> %s", src.Name, src.Path)
	}

	for i := range config.Provisioners {
		prov := &config.Provisioners[i]
		logging.Info("[UPDATE PATHS] Checking provisioner %d: type=%s, source=%s", i, prov.Type, prov.Source)
		if prov.Type == "file" && prov.Source != "" {
			oldSource := prov.Source
			prov.Source = resolveSourceReference(prov.Source, sourceMap)
			logging.Info("[UPDATE PATHS] Provisioner %d: %s -> %s", i, oldSource, prov.Source)
		}
	}
}

// resolveSourceReference resolves ${sources.<name>} references to actual paths
func resolveSourceReference(source string, sourceMap map[string]string) string {
	// Check for ${sources.<name>} pattern
	if len(source) > 11 && source[:10] == "${sources." && source[len(source)-1] == '}' {
		name := source[10 : len(source)-1]
		if path, ok := sourceMap[name]; ok {
			return path
		}
	}
	return source
}
