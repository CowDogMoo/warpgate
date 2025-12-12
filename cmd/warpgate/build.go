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
	"github.com/cowdogmoo/warpgate/pkg/cli"
	"github.com/cowdogmoo/warpgate/pkg/config"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/templates"
	"github.com/spf13/cobra"
)

// buildOptions holds command-line options for the build command
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
	vars         []string
	varFiles     []string
	cacheFrom    []string
	cacheTo      []string
	labels       []string
	buildArgs    []string
	noCache      bool
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
  warpgate build --template attack-box --target ami --region us-west-2 --instance-type t3.large`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, args, opts)
		},
	}

	// Build command flags - bind to local opts
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
}

func runBuild(cmd *cobra.Command, args []string, opts *buildOptions) error {
	ctx := cmd.Context()
	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	logging.InfoContext(ctx, "Starting build process")

	validator := cli.NewValidator()
	cliOpts := buildOptsToCliOpts(args, opts)
	if err := validator.ValidateBuildOptions(cliOpts); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	parser := cli.NewParser()
	labels, err := parser.ParseLabels(opts.labels)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}
	buildArgs, err := parser.ParseBuildArgs(opts.buildArgs)
	if err != nil {
		return fmt.Errorf("failed to parse build-args: %w", err)
	}

	buildConfig, err := loadBuildConfig(ctx, args, opts)
	if err != nil {
		return err
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
	}

	targetType := builder.DetermineTargetType(buildConfig, builderOpts)

	var results []builder.BuildResult
	switch targetType {
	case "container":
		results, err = service.ExecuteContainerBuild(ctx, *buildConfig, builderOpts)
		if err != nil {
			return err
		}
	case "ami":
		result, err := executeAMIBuildInCmd(ctx, cfg, buildConfig, builderOpts)
		if err != nil {
			return err
		}
		results = []builder.BuildResult{*result}
	default:
		return fmt.Errorf("unsupported target type: %s", targetType)
	}

	formatter := cli.NewOutputFormatter("text")
	formatter.DisplayBuildResults(ctx, results)

	if opts.push && opts.registry != "" {
		return service.Push(ctx, *buildConfig, results, builderOpts)
	}

	return nil
}

// executeAMIBuildInCmd handles AMI builds in the command layer to avoid import cycles.
func executeAMIBuildInCmd(ctx context.Context, cfg *globalconfig.Config, buildConfig *builder.Config, builderOpts builder.BuildOptions) (*builder.BuildResult, error) {
	logging.InfoContext(ctx, "Executing AMI build")

	var amiTarget *builder.Target
	for i := range buildConfig.Targets {
		if buildConfig.Targets[i].Type == "ami" {
			amiTarget = &buildConfig.Targets[i]
			break
		}
	}

	if amiTarget == nil {
		return nil, fmt.Errorf("no AMI target found in configuration")
	}

	region := builderOpts.Region
	if region == "" {
		region = amiTarget.Region
	}
	if region == "" && cfg != nil {
		region = cfg.AWS.Region
	}
	if region == "" {
		return nil, fmt.Errorf("AWS region must be specified (use --region flag, set in template, or configure in global config)")
	}

	amiConfig := struct {
		Region          string
		Profile         string
		AccessKeyID     string
		SecretAccessKey string
		SessionToken    string
	}{
		Region:          region,
		Profile:         cfg.AWS.Profile,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		SessionToken:    cfg.AWS.SessionToken,
	}

	amiBuilder, err := createAMIBuilder(ctx, amiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AMI builder: %w", err)
	}
	defer func() {
		if closeErr := amiBuilder.Close(); closeErr != nil {
			logging.WarnContext(ctx, "Failed to close AMI builder: %v", closeErr)
		}
	}()

	result, err := amiBuilder.Build(ctx, *buildConfig)
	if err != nil {
		return nil, fmt.Errorf("AMI build failed: %w", err)
	}

	logging.InfoContext(ctx, "AMI build completed successfully: %s", result.AMIID)
	return result, nil
}

// createAMIBuilder creates an AMI builder with the given configuration.
func createAMIBuilder(ctx context.Context, config interface{}) (builder.AMIBuilder, error) {
	type clientConfig struct {
		Region          string
		Profile         string
		AccessKeyID     string
		SecretAccessKey string
		SessionToken    string
	}

	cfg := config.(clientConfig)
	amiCfg := ami.ClientConfig{
		Region:          cfg.Region,
		Profile:         cfg.Profile,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		SessionToken:    cfg.SessionToken,
	}

	return ami.NewImageBuilder(ctx, amiCfg)
}

// buildOptsToCliOpts converts buildOptions to CLI validation options
func buildOptsToCliOpts(args []string, opts *buildOptions) cli.BuildCLIOptions {
	configFile := ""
	if len(args) > 0 {
		configFile = args[0]
	}

	return cli.BuildCLIOptions{
		ConfigFile:    configFile,
		Template:      opts.template,
		FromGit:       opts.fromGit,
		TargetType:    opts.targetType,
		Architectures: opts.arch,
		Registry:      opts.registry,
		Tags:          opts.tags,
		Region:        opts.region,
		InstanceType:  opts.instanceType,
		Labels:        opts.labels,
		BuildArgs:     opts.buildArgs,
		Variables:     opts.vars,
		VarFiles:      opts.varFiles,
		CacheFrom:     opts.cacheFrom,
		CacheTo:       opts.cacheTo,
		NoCache:       opts.noCache,
		Push:          opts.push,
		SaveDigests:   opts.saveDigests,
		DigestDir:     opts.digestDir,
	}
}

// loadBuildConfig loads configuration from template, git, or file
func loadBuildConfig(ctx context.Context, args []string, opts *buildOptions) (*builder.Config, error) {
	variables, err := config.ParseVariables(opts.vars, opts.varFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to parse variables: %w", err)
	}

	if opts.template != "" {
		logging.InfoContext(ctx, "Building from template: %s", opts.template)
		return loadFromTemplate(opts.template, variables)
	}
	if opts.fromGit != "" {
		logging.InfoContext(ctx, "Building from git: %s", opts.fromGit)
		return loadFromGit(opts.fromGit, variables)
	}
	if len(args) > 0 {
		logging.InfoContext(ctx, "Building from config file: %s", args[0])
		return loadFromFile(args[0], variables)
	}
	return nil, fmt.Errorf("specify config file, --template, or --from-git")
}

// newBuildKitBuilderFunc creates a new BuildKit builder
func newBuildKitBuilderFunc(ctx context.Context) (builder.ContainerBuilder, error) {
	logging.Info("Creating BuildKit builder")
	bldr, err := buildkit.NewBuildKitBuilder(ctx)
	if err != nil {
		return nil, enhanceBuildKitError(err)
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

// loadFromFile loads config from a local file with variable substitution
func loadFromFile(configPath string, variables map[string]string) (*builder.Config, error) {
	loader := config.NewLoader()
	cfg, err := loader.LoadFromFileWithVars(configPath, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	cfg.IsLocalTemplate = true

	return cfg, nil
}

// loadFromTemplate loads config from a template (official registry or cached) with variable substitution
func loadFromTemplate(templateName string, variables map[string]string) (*builder.Config, error) {
	loader, err := templates.NewTemplateLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template loader: %w", err)
	}

	cfg, err := loader.LoadTemplateWithVars(templateName, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}

	return cfg, nil
}

// loadFromGit loads config from a git repository with variable substitution
func loadFromGit(gitURL string, variables map[string]string) (*builder.Config, error) {
	loader, err := templates.NewTemplateLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template loader: %w", err)
	}

	cfg, err := loader.LoadTemplateWithVars(gitURL, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to load template from git: %w", err)
	}

	return cfg, nil
}
