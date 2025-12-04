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

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/builder/ami"
	"github.com/cowdogmoo/warpgate/pkg/builder/container"
	"github.com/cowdogmoo/warpgate/pkg/config"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/templates"
	"github.com/spf13/cobra"
)

// Build command options
type buildOptions struct {
	template   string
	fromGit    string
	targetType string
	push       bool
	registry   string
	arch       []string
	tags       []string
}

var buildOpts = &buildOptions{}

var buildCmd = &cobra.Command{
	Use:   "build [config|template]",
	Short: "Build image from config or template",
	Long: `Build a container image or AMI from a template configuration.

Examples:
  # Build from local config file
  warpgate build warpgate.yaml

  # Build from official template
  warpgate build --template attack-box

  # Build specific version
  warpgate build --template attack-box@v1.2.0

  # Build from git repository
  warpgate build --from-git https://github.com/user/templates.git//my-template

  # Build multiple architectures and push
  warpgate build --template attack-box --arch amd64,arm64 --push`,
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
	// Override architectures if specified
	if len(buildOpts.arch) > 0 {
		buildConfig.Architectures = buildOpts.arch
	} else if len(buildConfig.Architectures) == 0 {
		buildConfig.Architectures = cfg.Build.DefaultArch
	}

	// Override registry if specified
	if buildOpts.registry != "" {
		buildConfig.Registry = buildOpts.registry
	} else if buildConfig.Registry == "" {
		buildConfig.Registry = cfg.Registry.Default
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
		logging.Info("Successfully pushed to %s", buildOpts.registry)
	}

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
	loader := config.NewLoader()
	cfg, err := loader.LoadFromFile(configPath)
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
