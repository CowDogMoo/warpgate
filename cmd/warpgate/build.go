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

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/builder/ami"
	"github.com/cowdogmoo/warpgate/v3/cli"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
)

// buildOptions holds command-line options for the build command
type buildOptions struct {
	template      string
	fromGit       string
	targetType    string
	push          bool
	registry      string
	arch          []string
	tags          []string
	saveDigests   bool
	digestDir     string
	region        string
	instanceType  string
	vars          []string
	varFiles      []string
	cacheFrom     []string
	cacheTo       []string
	labels        []string
	buildArgs     []string
	noCache       bool
	forceRecreate bool
	dryRun        bool
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
	buildCmd.Flags().BoolVar(&opts.forceRecreate, "force", false, "Force recreation of existing AWS resources (AMI builds only)")
	buildCmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Validate configuration without creating resources (AMI builds only)")
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
		return fmt.Errorf("failed to load build configuration: %w", err)
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

	var results []builder.BuildResult
	switch targetType {
	case "container":
		results, err = service.ExecuteContainerBuild(ctx, *buildConfig, builderOpts)
		if err != nil {
			return fmt.Errorf("container build failed: %w", err)
		}
	case "ami":
		// Create AMI client configuration
		amiConfig := ami.ClientConfig{
			Region:          cfg.AWS.Region,
			Profile:         cfg.AWS.Profile,
			AccessKeyID:     cfg.AWS.AccessKeyID,
			SecretAccessKey: cfg.AWS.SecretAccessKey,
			SessionToken:    cfg.AWS.SessionToken,
		}

		// Create AMI builder with force recreate option
		amiBuilder, err := ami.NewImageBuilderWithOptions(ctx, amiConfig, opts.forceRecreate)
		if err != nil {
			return fmt.Errorf("failed to create AMI builder: %w", err)
		}
		defer func() {
			if closeErr := amiBuilder.Close(); closeErr != nil {
				logging.WarnContext(ctx, "Failed to close AMI builder: %v", closeErr)
			}
		}()

		// Handle dry-run mode
		if opts.dryRun {
			logging.InfoContext(ctx, "Running dry-run validation...")
			validationResult, err := amiBuilder.DryRun(ctx, *buildConfig)
			if err != nil {
				return fmt.Errorf("dry-run validation failed: %w", err)
			}

			// Display validation results
			fmt.Println(validationResult.String())

			if !validationResult.Valid {
				return fmt.Errorf("dry-run validation failed with %d errors", len(validationResult.Errors))
			}
			logging.InfoContext(ctx, "Dry-run validation completed successfully")
			return nil
		}

		// Execute AMI build with service
		result, err := service.ExecuteAMIBuild(ctx, *buildConfig, builderOpts, amiBuilder)
		if err != nil {
			return fmt.Errorf("AMI build failed: %w", err)
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
