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

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/builder/ami"
	"github.com/cowdogmoo/warpgate/pkg/builder/buildkit"
	"github.com/cowdogmoo/warpgate/pkg/config"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/manifests"
	"github.com/cowdogmoo/warpgate/pkg/templates"
	"github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
)

// Build command options
type buildOptions struct {
	template     string
	fromGit      string
	targetType   string
	push         bool
	registry     string
	arch         []string
	tags         []string
	saveDigests  bool
	digestDir    string
	region       string
	instanceType string
	vars         []string // Variable overrides in key=value format
	varFiles     []string // Files containing variable definitions
	cacheFrom    []string // Cache sources for BuildKit (e.g., "type=registry,ref=...")
	cacheTo      []string // Cache destinations for BuildKit (e.g., "type=registry,ref=...")
	builderType  string   // Builder type override: auto, buildkit, or buildah
	labels       []string // Image labels in key=value format
	buildArgs    []string // Build arguments in key=value format
	noCache      bool     // Disable all caching
}

var buildOpts = &buildOptions{}

var buildCmd = &cobra.Command{
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
  warpgate build --template attack-box --target ami --region us-west-2 --instance-type t3.large`,
	RunE: runBuild,
}

func init() {
	// Build command flags
	buildCmd.Flags().StringVar(&buildOpts.template, "template", "", "Use named template from registry")
	buildCmd.Flags().StringVar(&buildOpts.fromGit, "from-git", "", "Load template from git URL")
	buildCmd.Flags().StringVar(&buildOpts.targetType, "target", "", "Override target type (container, ami)")
	buildCmd.Flags().BoolVar(&buildOpts.push, "push", false, "Push image to registry after build")
	buildCmd.Flags().StringVar(&buildOpts.registry, "registry", "", "Registry to push to")
	buildCmd.Flags().StringSliceVar(&buildOpts.arch, "arch", nil, "Architectures to build (comma-separated)")
	buildCmd.Flags().StringSliceVarP(&buildOpts.tags, "tag", "t", []string{}, "Additional tags to apply")
	buildCmd.Flags().BoolVar(&buildOpts.saveDigests, "save-digests", false, "Save image digests to files after push")
	buildCmd.Flags().StringVar(&buildOpts.digestDir, "digest-dir", ".", "Directory to save digest files")
	buildCmd.Flags().StringVar(&buildOpts.region, "region", "", "AWS region for AMI builds (overrides config)")
	buildCmd.Flags().StringVar(&buildOpts.instanceType, "instance-type", "", "EC2 instance type for AMI builds (overrides config)")
	buildCmd.Flags().StringArrayVar(&buildOpts.vars, "var", []string{}, "Set template variables (key=value)")
	buildCmd.Flags().StringArrayVar(&buildOpts.varFiles, "var-file", []string{}, "Load variables from YAML file")
	buildCmd.Flags().StringArrayVar(&buildOpts.cacheFrom, "cache-from", []string{}, "External cache sources for BuildKit (e.g., type=registry,ref=user/app:cache)")
	buildCmd.Flags().StringArrayVar(&buildOpts.cacheTo, "cache-to", []string{}, "External cache destinations for BuildKit (e.g., type=registry,ref=user/app:cache,mode=max)")
	buildCmd.Flags().StringVar(&buildOpts.builderType, "builder", "", "Builder to use: auto, buildkit, or buildah (overrides config)")
	buildCmd.Flags().StringArrayVar(&buildOpts.labels, "label", []string{}, "Set image labels (key=value)")
	buildCmd.Flags().StringArrayVar(&buildOpts.buildArgs, "build-arg", []string{}, "Set build arguments (key=value)")
	buildCmd.Flags().BoolVar(&buildOpts.noCache, "no-cache", false, "Disable all caching")
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	logging.InfoContext(ctx, "Starting build process")

	// Load and configure build
	buildConfig, err := loadBuildConfig(ctx, args)
	if err != nil {
		return err
	}

	applyConfigOverrides(cmd, buildConfig)

	// Validate configuration
	if err := validateConfig(buildConfig); err != nil {
		return err
	}

	// Execute build based on target type
	return executeBuild(cmd, ctx, buildConfig)
}

// loadBuildConfig loads configuration from template, git, or file
func loadBuildConfig(ctx context.Context, args []string) (*builder.Config, error) {
	if buildOpts.template != "" {
		logging.InfoContext(ctx, "Building from template: %s", buildOpts.template)
		return loadFromTemplate(buildOpts.template)
	}
	if buildOpts.fromGit != "" {
		logging.InfoContext(ctx, "Building from git: %s", buildOpts.fromGit)
		return loadFromGit(buildOpts.fromGit)
	}
	if len(args) > 0 {
		logging.InfoContext(ctx, "Building from config file: %s", args[0])
		return loadFromFile(args[0])
	}
	return nil, fmt.Errorf("specify config file, --template, or --from-git")
}

// applyConfigOverrides applies CLI flag overrides to build config
func applyConfigOverrides(cmd *cobra.Command, buildConfig *builder.Config) {
	ctx := cmd.Context()
	cfg := configFromContext(cmd)
	if cfg == nil {
		logging.WarnContext(ctx, "configuration not available, skipping defaults")
		return
	}

	// Apply various overrides
	applyTargetTypeFilter(buildConfig)
	applyArchitectureOverrides(buildConfig, cfg)
	applyRegistryOverride(buildConfig, cfg)
	applyAMITargetOverrides(buildConfig)
	applyLabelsAndBuildArgs(ctx, buildConfig)
	applyCacheOptions(buildConfig)
}

// applyLabelsAndBuildArgs parses and applies labels and build arguments from CLI flags
func applyLabelsAndBuildArgs(ctx context.Context, buildConfig *builder.Config) {
	// Parse labels
	if len(buildOpts.labels) > 0 {
		if buildConfig.Labels == nil {
			buildConfig.Labels = make(map[string]string)
		}
		for _, label := range buildOpts.labels {
			parts := strings.SplitN(label, "=", 2)
			if len(parts) == 2 {
				buildConfig.Labels[parts[0]] = parts[1]
				logging.DebugContext(ctx, "Added label: %s=%s", parts[0], parts[1])
			} else {
				logging.WarnContext(ctx, "Invalid label format (expected key=value): %s", label)
			}
		}
	}

	// Parse build arguments
	if len(buildOpts.buildArgs) > 0 {
		if buildConfig.BuildArgs == nil {
			buildConfig.BuildArgs = make(map[string]string)
		}
		for _, arg := range buildOpts.buildArgs {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				buildConfig.BuildArgs[parts[0]] = parts[1]
				logging.DebugContext(ctx, "Added build arg: %s=%s", parts[0], parts[1])
			} else {
				logging.WarnContext(ctx, "Invalid build-arg format (expected key=value): %s", arg)
			}
		}
	}
}

// applyCacheOptions applies cache-related options from CLI flags
func applyCacheOptions(buildConfig *builder.Config) {
	if buildOpts.noCache {
		buildConfig.NoCache = true
	}
}

// applyTargetTypeFilter filters targets based on the target type override
func applyTargetTypeFilter(buildConfig *builder.Config) {
	if buildOpts.targetType == "" {
		return
	}

	filteredTargets := []builder.Target{}
	for _, target := range buildConfig.Targets {
		if target.Type == buildOpts.targetType {
			filteredTargets = append(filteredTargets, target)
		}
	}
	buildConfig.Targets = filteredTargets
}

// applyArchitectureOverrides applies architecture overrides to the build config
func applyArchitectureOverrides(buildConfig *builder.Config, cfg *globalconfig.Config) {
	if len(buildOpts.arch) > 0 {
		buildConfig.Architectures = buildOpts.arch
		return
	}

	if len(buildConfig.Architectures) > 0 {
		return
	}

	// Extract architectures from target platforms
	buildConfig.Architectures = builder.ExtractArchitecturesFromTargets(buildConfig)

	// Fallback to default architectures if none found
	if len(buildConfig.Architectures) == 0 {
		buildConfig.Architectures = cfg.Build.DefaultArch
	}
}

// applyRegistryOverride applies registry override to the build config
func applyRegistryOverride(buildConfig *builder.Config, cfg *globalconfig.Config) {
	if buildOpts.registry != "" {
		buildConfig.Registry = buildOpts.registry
	} else if buildConfig.Registry == "" {
		buildConfig.Registry = cfg.Registry.Default
	}
}

// applyAMITargetOverrides applies AMI-specific overrides to the build config
func applyAMITargetOverrides(buildConfig *builder.Config) {
	if buildOpts.region == "" && buildOpts.instanceType == "" {
		return
	}

	for i := range buildConfig.Targets {
		if buildConfig.Targets[i].Type != "ami" {
			continue
		}

		if buildOpts.region != "" {
			buildConfig.Targets[i].Region = buildOpts.region
		}
		if buildOpts.instanceType != "" {
			buildConfig.Targets[i].InstanceType = buildOpts.instanceType
		}
	}
}

// validateConfig validates the build configuration
func validateConfig(buildConfig *builder.Config) error {
	validator := config.NewValidator()
	if err := validator.Validate(buildConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Validate builder type if specified
	if buildOpts.builderType != "" {
		builderType := strings.ToLower(buildOpts.builderType)
		if builderType != "auto" && builderType != "buildkit" && builderType != "buildah" {
			return fmt.Errorf("invalid builder type: %s (supported: auto, buildkit, buildah)", buildOpts.builderType)
		}
	}

	// Validate labels format
	for _, label := range buildOpts.labels {
		if !strings.Contains(label, "=") {
			return fmt.Errorf("invalid label format: %s (expected key=value)", label)
		}
	}

	// Validate build-args format
	for _, arg := range buildOpts.buildArgs {
		if !strings.Contains(arg, "=") {
			return fmt.Errorf("invalid build-arg format: %s (expected key=value)", arg)
		}
	}

	return nil
}

// executeBuild executes the build based on target type
func executeBuild(cmd *cobra.Command, ctx context.Context, buildConfig *builder.Config) error {
	// Determine target type
	targetType := determineTargetType(buildConfig)

	// Get global config for build settings
	cfg := configFromContext(cmd)

	switch targetType {
	case "container":
		return executeContainerBuild(ctx, buildConfig, cfg)
	case "ami":
		return executeAMIBuild(cmd, ctx, buildConfig)
	default:
		return fmt.Errorf("unsupported target type: %s", targetType)
	}
}

// determineTargetType determines the target type from configuration
func determineTargetType(buildConfig *builder.Config) string {
	// Check CLI override
	if buildOpts.targetType != "" {
		return buildOpts.targetType
	}

	// Check config targets
	if len(buildConfig.Targets) > 0 {
		return buildConfig.Targets[0].Type
	}

	// Default to container
	return "container"
}

// selectContainerBuilder chooses the appropriate builder based on config and platform
func selectContainerBuilder(ctx context.Context, cfg *globalconfig.Config) (builder.ContainerBuilder, error) {
	// Determine which builder to use
	builderTypeStr := cfg.Build.BuilderType
	if buildOpts.builderType != "" {
		builderTypeStr = buildOpts.builderType
	}

	// Normalize builder type
	builderTypeStr = strings.ToLower(builderTypeStr)

	// Create the appropriate builder
	switch builderTypeStr {
	case "buildkit":
		return newBuildKitBuilder(ctx)
	case "buildah":
		return newBuildahBuilder()
	case "auto", "":
		// Auto-detect based on platform
		return autoSelectBuilder(ctx)
	default:
		return nil, fmt.Errorf("unsupported builder type: %s (supported: auto, buildkit, buildah)", builderTypeStr)
	}
}

// autoSelectBuilder and newBuildahBuilder are defined in build_linux.go and build_nonlinux.go

// newBuildKitBuilder creates a new BuildKit builder
func newBuildKitBuilder(ctx context.Context) (builder.ContainerBuilder, error) {
	logging.Info("Using BuildKit builder")
	bldr, err := buildkit.NewBuildKitBuilder(ctx)
	if err != nil {
		return nil, enhanceBuildKitError(err)
	}

	// Set cache options if provided via flags
	if buildOpts.noCache {
		logging.Info("Caching disabled via --no-cache flag")
	} else if len(buildOpts.cacheFrom) > 0 || len(buildOpts.cacheTo) > 0 {
		bldr.SetCacheOptions(buildOpts.cacheFrom, buildOpts.cacheTo)
	}

	return bldr, nil
}

// enhanceBuildKitError provides better error messages for BuildKit-related errors
func enhanceBuildKitError(err error) error {
	errMsg := err.Error()

	if strings.Contains(errMsg, "no active buildx builder") {
		return fmt.Errorf("BuildKit builder not available: %w\n\nTo fix this, create a buildx builder:\n  docker buildx create --name warpgate --driver docker-container --bootstrap", err)
	}

	if strings.Contains(errMsg, "Cannot connect to the Docker daemon") ||
		strings.Contains(errMsg, "docker daemon") ||
		strings.Contains(errMsg, "connection refused") {
		return fmt.Errorf("docker is not running: %w\n\nPlease start Docker Desktop or the Docker daemon before building", err)
	}

	if strings.Contains(errMsg, "docker buildx") {
		return fmt.Errorf("docker buildx not available: %w\n\nBuildKit requires docker buildx. Please ensure Docker Desktop is installed and up to date", err)
	}

	return err
}

// executeContainerBuild executes a container build
func executeContainerBuild(ctx context.Context, buildConfig *builder.Config, cfg *globalconfig.Config) error {
	logging.InfoContext(ctx, "Executing container build")

	// Select builder based on config and platform
	bldr, err := selectContainerBuilder(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container builder: %w", err)
	}
	defer func() {
		if err := bldr.Close(); err != nil {
			logging.ErrorContext(ctx, "Failed to close builder", "error", err)
		}
	}()

	// Check if multi-arch build is required
	if len(buildConfig.Architectures) > 1 {
		return executeMultiArchBuild(ctx, buildConfig, bldr, cfg)
	}

	// Single architecture build - set platform if not already set
	if buildConfig.Base.Platform == "" && len(buildConfig.Architectures) > 0 {
		buildConfig.Base.Platform = fmt.Sprintf("linux/%s", buildConfig.Architectures[0])
	}

	result, err := bldr.Build(ctx, *buildConfig)
	if err != nil {
		return fmt.Errorf("container build failed: %w", err)
	}

	displayBuildResults(ctx, result)

	// Push if requested
	if buildOpts.push && buildOpts.registry != "" {
		logging.InfoContext(ctx, "Pushing to registry: %s", buildOpts.registry)
		if err := bldr.Push(ctx, result.ImageRef, buildOpts.registry); err != nil {
			return fmt.Errorf("failed to push image: %w", err)
		}

		// Save digest if requested
		if buildOpts.saveDigests && result.Digest != "" {
			arch := "unknown"
			if len(buildConfig.Architectures) > 0 {
				arch = buildConfig.Architectures[0]
			}
			if err := manifests.SaveDigestToFile(buildConfig.Name, arch, result.Digest, buildOpts.digestDir); err != nil {
				logging.WarnContext(ctx, "Failed to save digest: %v", err)
			}
		}

		logging.InfoContext(ctx, "Successfully pushed to %s", buildOpts.registry)
	}

	return nil
}

// executeMultiArchBuild executes a multi-architecture container build
func executeMultiArchBuild(ctx context.Context, buildConfig *builder.Config, bldr builder.ContainerBuilder, cfg *globalconfig.Config) error {
	logging.InfoContext(ctx, "Executing multi-arch build for %d architectures: %v", len(buildConfig.Architectures), buildConfig.Architectures)

	// Use configured concurrency, or default if not set
	concurrency := builder.DefaultMaxConcurrency
	if cfg != nil && cfg.Build.Concurrency > 0 {
		concurrency = cfg.Build.Concurrency
	}

	// Create build orchestrator
	orchestrator := builder.NewBuildOrchestrator(concurrency)

	// Create build requests for each architecture
	requests := builder.CreateBuildRequests(buildConfig)

	// Build all architectures in parallel
	results, err := orchestrator.BuildMultiArch(ctx, requests, bldr)
	if err != nil {
		return fmt.Errorf("multi-arch build failed: %w", err)
	}

	// Display results for each architecture
	for _, result := range results {
		displayBuildResults(ctx, &result)
	}

	// Push if requested
	if buildOpts.push && buildOpts.registry != "" {
		return pushMultiArchImages(ctx, buildConfig, results, bldr, orchestrator)
	}

	return nil
}

// pushMultiArchImages pushes multi-arch images and creates manifest
func pushMultiArchImages(ctx context.Context, buildConfig *builder.Config, results []builder.BuildResult, bldr builder.ContainerBuilder, orchestrator *builder.BuildOrchestrator) error {
	logging.InfoContext(ctx, "Pushing multi-arch images to registry: %s", buildOpts.registry)

	// Push individual architecture images
	if err := orchestrator.PushMultiArch(ctx, results, buildOpts.registry, bldr); err != nil {
		return fmt.Errorf("failed to push multi-arch images: %w", err)
	}

	// Save digests if requested
	if buildOpts.saveDigests {
		saveDigests(ctx, buildConfig.Name, results)
	}

	// Create and push multi-arch manifest
	if err := createAndPushManifest(ctx, buildConfig, results, bldr); err != nil {
		return fmt.Errorf("failed to create multi-arch manifest: %w", err)
	}

	logging.InfoContext(ctx, "Successfully pushed multi-arch build to %s", buildOpts.registry)
	return nil
}

// saveDigests saves digests for all architectures
func saveDigests(ctx context.Context, imageName string, results []builder.BuildResult) {
	logging.InfoContext(ctx, "Saving image digests to %s", buildOpts.digestDir)
	for _, result := range results {
		if result.Digest != "" {
			if err := manifests.SaveDigestToFile(imageName, result.Architecture, result.Digest, buildOpts.digestDir); err != nil {
				logging.WarnContext(ctx, "Failed to save digest for %s: %v", result.Architecture, err)
			}
		}
	}
}

// createAndPushManifest creates and pushes a multi-arch manifest
func createAndPushManifest(ctx context.Context, buildConfig *builder.Config, results []builder.BuildResult, bldr builder.ContainerBuilder) error {
	manifestName := fmt.Sprintf("%s:%s", buildConfig.Name, buildConfig.Version)
	logging.InfoContext(ctx, "Creating multi-arch manifest: %s", manifestName)

	// Create manifest entries from build results
	entries := make([]builder.ManifestEntry, 0, len(results))
	for _, result := range results {
		// Parse digest
		var imageDigest digest.Digest
		if result.Digest != "" {
			var err error
			imageDigest, err = digest.Parse(result.Digest)
			if err != nil {
				logging.WarnContext(ctx, "Failed to parse digest for %s: %v", result.Architecture, err)
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

		entries = append(entries, builder.ManifestEntry{
			ImageRef:     result.ImageRef,
			Digest:       imageDigest,
			Platform:     result.Platform,
			Architecture: result.Architecture,
			OS:           os,
			Variant:      variant,
		})
	}

	// Push manifest to registry
	destination := fmt.Sprintf("%s/%s", buildOpts.registry, manifestName)

	// Try to create manifest using BuildKit (which supports multi-arch manifests)
	if bk, ok := bldr.(*buildkit.BuildKitBuilder); ok {
		// BuildKit uses docker buildx imagetools create
		if err := bk.CreateAndPushManifest(ctx, destination, entries); err != nil {
			return fmt.Errorf("failed to create/push manifest with BuildKit: %w", err)
		}
		return nil
	}

	// For other builder types (Buildah on Linux), use platform-specific implementation
	return createAndPushManifestPlatformSpecific(ctx, manifestName, destination, entries, bldr)
}

// executeAMIBuild executes an AMI build
func executeAMIBuild(cmd *cobra.Command, ctx context.Context, buildConfig *builder.Config) error {
	logging.InfoContext(ctx, "Executing AMI build")

	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	// Get AWS configuration from environment or config
	clientConfig := ami.ClientConfig{
		Region:          cfg.AWS.Region,
		Profile:         cfg.AWS.Profile,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		SessionToken:    cfg.AWS.SessionToken,
	}

	// Create AMI builder
	bldr, err := ami.NewImageBuilder(ctx, clientConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize AMI builder: %w", err)
	}
	defer func() {
		if err := bldr.Close(); err != nil {
			logging.ErrorContext(ctx, "Failed to close AMI builder", "error", err)
		}
	}()

	result, err := bldr.Build(ctx, *buildConfig)
	if err != nil {
		return fmt.Errorf("AMI build failed: %w", err)
	}

	displayBuildResults(ctx, result)

	return nil
}

// displayBuildResults logs the build results
func displayBuildResults(ctx context.Context, result *builder.BuildResult) {
	logging.InfoContext(ctx, "Build completed successfully!")

	if result.ImageRef != "" {
		logging.InfoContext(ctx, "Image: %s", result.ImageRef)
	}

	if result.AMIID != "" {
		logging.InfoContext(ctx, "AMI ID: %s", result.AMIID)
		logging.InfoContext(ctx, "Region: %s", result.Region)
	}

	logging.InfoContext(ctx, "Duration: %s", result.Duration)

	for _, note := range result.Notes {
		logging.InfoContext(ctx, "Note: %s", note)
	}
}

// loadFromFile loads config from a local file
func loadFromFile(configPath string) (*builder.Config, error) {
	// Parse variables from CLI flags and var files
	variables, err := config.ParseVariables(buildOpts.vars, buildOpts.varFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to parse variables: %w", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.LoadFromFileWithVars(configPath, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

// loadFromTemplate loads config from a template (official registry or cached)
func loadFromTemplate(templateName string) (*builder.Config, error) {
	loader, err := templates.NewTemplateLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template loader: %w", err)
	}

	cfg, err := loader.LoadTemplate(templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}

	return cfg, nil
}

// loadFromGit loads config from a git repository
func loadFromGit(gitURL string) (*builder.Config, error) {
	loader, err := templates.NewTemplateLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template loader: %w", err)
	}

	// The LoadTemplate method already handles git URLs
	cfg, err := loader.LoadTemplate(gitURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load template from git: %w", err)
	}

	return cfg, nil
}
