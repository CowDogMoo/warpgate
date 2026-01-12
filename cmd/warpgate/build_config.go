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

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/builder/buildkit"
	"github.com/cowdogmoo/warpgate/v3/cli"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/templates"
)

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
	variables, err := templates.ParseVariables(opts.vars, opts.varFiles)
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
	loader := templates.NewLoader()
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
