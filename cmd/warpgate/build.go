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
	"github.com/cowdogmoo/warpgate/pkg/builder/container"
	"github.com/cowdogmoo/warpgate/pkg/config"
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
  warpgate build warpgate.yaml --var ARSENAL_REPO_PATH=/path/to/arsenal

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
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logging.Info("Starting build process")

	// Load and configure build
	buildConfig, err := loadBuildConfig(args)
	if err != nil {
		return err
	}

	applyConfigOverrides(buildConfig)

	// Validate configuration
	if err := validateConfig(buildConfig); err != nil {
		return err
	}

	// Execute build based on target type
	return executeBuild(ctx, buildConfig)
}

// loadBuildConfig loads configuration from template, git, or file
func loadBuildConfig(args []string) (*builder.Config, error) {
	if buildOpts.template != "" {
		logging.Info("Building from template: %s", buildOpts.template)
		return loadFromTemplate(buildOpts.template)
	}
	if buildOpts.fromGit != "" {
		logging.Info("Building from git: %s", buildOpts.fromGit)
		return loadFromGit(buildOpts.fromGit)
	}
	if len(args) > 0 {
		logging.Info("Building from config file: %s", args[0])
		return loadFromFile(args[0])
	}
	return nil, fmt.Errorf("specify config file, --template, or --from-git")
}

// applyConfigOverrides applies CLI flag overrides to build config
func applyConfigOverrides(buildConfig *builder.Config) {
	// Filter targets if target type override is specified
	if buildOpts.targetType != "" {
		filteredTargets := []builder.Target{}
		for _, target := range buildConfig.Targets {
			if target.Type == buildOpts.targetType {
				filteredTargets = append(filteredTargets, target)
			}
		}
		buildConfig.Targets = filteredTargets
	}

	// Override architectures if specified
	if len(buildOpts.arch) > 0 {
		buildConfig.Architectures = buildOpts.arch
	} else if len(buildConfig.Architectures) == 0 {
		// Extract architectures from target platforms
		buildConfig.Architectures = builder.ExtractArchitecturesFromTargets(buildConfig)
		// Fallback to default architectures if none found
		if len(buildConfig.Architectures) == 0 {
			buildConfig.Architectures = cfg.Build.DefaultArch
		}
	}

	// Override registry if specified
	if buildOpts.registry != "" {
		buildConfig.Registry = buildOpts.registry
	} else if buildConfig.Registry == "" {
		buildConfig.Registry = cfg.Registry.Default
	}

	// Override region and instance type for AMI targets
	if buildOpts.region != "" || buildOpts.instanceType != "" {
		for i := range buildConfig.Targets {
			if buildConfig.Targets[i].Type == "ami" {
				if buildOpts.region != "" {
					buildConfig.Targets[i].Region = buildOpts.region
				}
				if buildOpts.instanceType != "" {
					buildConfig.Targets[i].InstanceType = buildOpts.instanceType
				}
			}
		}
	}
}

// validateConfig validates the build configuration
func validateConfig(buildConfig *builder.Config) error {
	validator := config.NewValidator()
	if err := validator.Validate(buildConfig); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	return nil
}

// executeBuild executes the build based on target type
func executeBuild(ctx context.Context, buildConfig *builder.Config) error {
	// Determine target type
	targetType := determineTargetType(buildConfig)

	switch targetType {
	case "container":
		return executeContainerBuild(ctx, buildConfig)
	case "ami":
		return executeAMIBuild(ctx, buildConfig)
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

// executeContainerBuild executes a container build
func executeContainerBuild(ctx context.Context, buildConfig *builder.Config) error {
	logging.Info("Executing container build")

	builderCfg := container.GetDefaultConfig()
	bldr, err := container.NewBuildahBuilder(builderCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container builder: %w", err)
	}
	defer bldr.Close()

	// Check if multi-arch build is required
	if len(buildConfig.Architectures) > 1 {
		return executeMultiArchBuild(ctx, buildConfig, bldr)
	}

	// Single architecture build - set platform if not already set
	if buildConfig.Base.Platform == "" && len(buildConfig.Architectures) > 0 {
		buildConfig.Base.Platform = fmt.Sprintf("linux/%s", buildConfig.Architectures[0])
	}

	result, err := bldr.Build(ctx, *buildConfig)
	if err != nil {
		return fmt.Errorf("container build failed: %w", err)
	}

	displayBuildResults(result)

	// Push if requested
	if buildOpts.push && buildOpts.registry != "" {
		logging.Info("Pushing to registry: %s", buildOpts.registry)
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
				logging.Warn("Failed to save digest: %v", err)
			}
		}

		logging.Info("Successfully pushed to %s", buildOpts.registry)
	}

	return nil
}

// executeMultiArchBuild executes a multi-architecture container build
func executeMultiArchBuild(ctx context.Context, buildConfig *builder.Config, bldr *container.BuildahBuilder) error {
	logging.Info("Executing multi-arch build for %d architectures: %v", len(buildConfig.Architectures), buildConfig.Architectures)

	// Create build orchestrator
	orchestrator := builder.NewBuildOrchestrator(2) // Allow 2 concurrent builds

	// Create build requests for each architecture
	requests := builder.CreateBuildRequests(buildConfig)

	// Build all architectures in parallel
	results, err := orchestrator.BuildMultiArch(ctx, requests, bldr)
	if err != nil {
		return fmt.Errorf("multi-arch build failed: %w", err)
	}

	// Display results for each architecture
	for _, result := range results {
		displayBuildResults(&result)
	}

	// Push if requested
	if buildOpts.push && buildOpts.registry != "" {
		return pushMultiArchImages(ctx, buildConfig, results, bldr, orchestrator)
	}

	return nil
}

// pushMultiArchImages pushes multi-arch images and creates manifest
func pushMultiArchImages(ctx context.Context, buildConfig *builder.Config, results []builder.BuildResult, bldr *container.BuildahBuilder, orchestrator *builder.BuildOrchestrator) error {
	logging.Info("Pushing multi-arch images to registry: %s", buildOpts.registry)

	// Push individual architecture images
	if err := orchestrator.PushMultiArch(ctx, results, buildOpts.registry, bldr); err != nil {
		return fmt.Errorf("failed to push multi-arch images: %w", err)
	}

	// Save digests if requested
	if buildOpts.saveDigests {
		saveDigests(buildConfig.Name, results)
	}

	// Create and push multi-arch manifest
	if err := createAndPushManifest(ctx, buildConfig, results, bldr); err != nil {
		return fmt.Errorf("failed to create multi-arch manifest: %w", err)
	}

	logging.Info("Successfully pushed multi-arch build to %s", buildOpts.registry)
	return nil
}

// saveDigests saves digests for all architectures
func saveDigests(imageName string, results []builder.BuildResult) {
	logging.Info("Saving image digests to %s", buildOpts.digestDir)
	for _, result := range results {
		if result.Digest != "" {
			if err := manifests.SaveDigestToFile(imageName, result.Architecture, result.Digest, buildOpts.digestDir); err != nil {
				logging.Warn("Failed to save digest for %s: %v", result.Architecture, err)
			}
		}
	}
}

// createAndPushManifest creates and pushes a multi-arch manifest
func createAndPushManifest(ctx context.Context, buildConfig *builder.Config, results []builder.BuildResult, bldr *container.BuildahBuilder) error {
	manifestName := fmt.Sprintf("%s:%s", buildConfig.Name, buildConfig.Version)
	logging.Info("Creating multi-arch manifest: %s", manifestName)

	// Get manifest manager
	manifestMgr := bldr.GetManifestManager()

	// Create manifest entries from build results
	entries := make([]container.ManifestEntry, 0, len(results))
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

		entries = append(entries, container.ManifestEntry{
			ImageRef:     result.ImageRef,
			Digest:       imageDigest,
			Platform:     result.Platform,
			Architecture: result.Architecture,
			OS:           os,
			Variant:      variant,
		})
	}

	// Create the manifest
	manifestList, err := manifestMgr.CreateManifest(ctx, manifestName, entries)
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// Push the manifest to the registry
	destination := fmt.Sprintf("%s/%s", buildOpts.registry, manifestName)
	if err := manifestMgr.PushManifest(ctx, manifestList, destination); err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	logging.Info("Successfully created and pushed multi-arch manifest to %s", destination)
	return nil
}

// executeAMIBuild executes an AMI build
func executeAMIBuild(ctx context.Context, buildConfig *builder.Config) error {
	logging.Info("Executing AMI build")

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
	defer bldr.Close()

	result, err := bldr.Build(ctx, *buildConfig)
	if err != nil {
		return fmt.Errorf("AMI build failed: %w", err)
	}

	displayBuildResults(result)

	return nil
}

// displayBuildResults logs the build results
func displayBuildResults(result *builder.BuildResult) {
	logging.Info("Build completed successfully!")

	if result.ImageRef != "" {
		logging.Info("Image: %s", result.ImageRef)
	}

	if result.AMIID != "" {
		logging.Info("AMI ID: %s", result.AMIID)
		logging.Info("Region: %s", result.Region)
	}

	logging.Info("Duration: %s", result.Duration)

	for _, note := range result.Notes {
		logging.Info("Note: %s", note)
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
