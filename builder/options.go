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

package builder

import (
	"context"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// BuildOptions represents build configuration overrides from CLI or API.
// These options take precedence over values in the template configuration.
type BuildOptions struct {
	// TargetType filters builds to specific target type (container, ami)
	TargetType string

	// Architectures overrides the architectures to build
	Architectures []string

	// Registry overrides the default registry
	Registry string

	// Tags specifies additional tags to apply
	Tags []string

	// Region overrides AWS region for AMI builds
	Region string

	// InstanceType overrides EC2 instance type for AMI builds
	InstanceType string

	// Labels adds image labels in key=value format (parsed externally)
	Labels map[string]string

	// BuildArgs adds build arguments in key=value format (parsed externally)
	BuildArgs map[string]string

	// CacheFrom specifies external cache sources for BuildKit
	CacheFrom []string

	// CacheTo specifies external cache destinations for BuildKit
	CacheTo []string

	// NoCache disables all caching
	NoCache bool

	// Variables contains template variable overrides
	Variables map[string]string

	// Push indicates whether to push after build
	Push bool

	// SaveDigests indicates whether to save image digests
	SaveDigests bool

	// DigestDir specifies directory for saving digests
	DigestDir string

	// ForceRecreate deletes existing AWS resources before creating new ones (AMI builds only)
	ForceRecreate bool
}

// ApplyOverrides applies CLI/API overrides to a build configuration.
// Precedence: BuildOptions > Config > Global Config Defaults
func ApplyOverrides(ctx context.Context, config *Config, opts BuildOptions, globalCfg *config.Config) error {
	if globalCfg == nil {
		logging.WarnContext(ctx, "No global configuration provided, some defaults may not be applied")
	}

	applyTargetTypeFilter(config, opts)
	applyArchitectureOverrides(config, opts, globalCfg)
	applyRegistryOverride(config, opts, globalCfg)
	applyAMITargetOverrides(config, opts)
	applyLabelsAndBuildArgs(ctx, config, opts)
	applyCacheOptions(config, opts)

	return nil
}

// applyTargetTypeFilter filters targets based on the target type override
func applyTargetTypeFilter(config *Config, opts BuildOptions) {
	if opts.TargetType == "" {
		return
	}

	filteredTargets := []Target{}
	for _, target := range config.Targets {
		if target.Type == opts.TargetType {
			filteredTargets = append(filteredTargets, target)
		}
	}
	config.Targets = filteredTargets
}

// applyArchitectureOverrides applies architecture overrides to the build config
func applyArchitectureOverrides(config *Config, opts BuildOptions, globalCfg *config.Config) {
	// CLI option takes highest precedence
	if len(opts.Architectures) > 0 {
		config.Architectures = opts.Architectures
		return
	}

	// Use config's architectures if already set
	if len(config.Architectures) > 0 {
		return
	}

	// Extract architectures from target platforms
	config.Architectures = ExtractArchitecturesFromTargets(config)

	// Fallback to global default architectures if none found
	if len(config.Architectures) == 0 && globalCfg != nil {
		config.Architectures = globalCfg.Build.DefaultArch
	}
}

// applyRegistryOverride applies registry override to the build config
func applyRegistryOverride(config *Config, opts BuildOptions, globalCfg *config.Config) {
	if opts.Registry != "" {
		config.Registry = opts.Registry
	} else if config.Registry == "" && globalCfg != nil {
		config.Registry = globalCfg.Registry.Default
	}
}

// applyAMITargetOverrides applies AMI-specific overrides to the build config
func applyAMITargetOverrides(config *Config, opts BuildOptions) {
	if opts.Region == "" && opts.InstanceType == "" {
		return
	}

	for i := range config.Targets {
		if config.Targets[i].Type != "ami" {
			continue
		}

		if opts.Region != "" {
			config.Targets[i].Region = opts.Region
		}
		if opts.InstanceType != "" {
			config.Targets[i].InstanceType = opts.InstanceType
		}
	}
}

// applyLabelsAndBuildArgs applies labels and build arguments to the build config.
// Note: The labels and build args in opts should already be parsed from key=value format.
func applyLabelsAndBuildArgs(ctx context.Context, config *Config, opts BuildOptions) {
	// Apply labels
	if len(opts.Labels) > 0 {
		if config.Labels == nil {
			config.Labels = make(map[string]string)
		}
		for key, value := range opts.Labels {
			config.Labels[key] = value
			logging.DebugContext(ctx, "Added label: %s=%s", key, logging.RedactSensitiveValue(key, value))
		}
	}

	// Apply build arguments
	if len(opts.BuildArgs) > 0 {
		if config.BuildArgs == nil {
			config.BuildArgs = make(map[string]string)
		}
		for key, value := range opts.BuildArgs {
			config.BuildArgs[key] = value
			logging.DebugContext(ctx, "Added build arg: %s=%s", key, logging.RedactSensitiveValue(key, value))
		}
	}
}

// applyCacheOptions applies cache-related options to the build config
func applyCacheOptions(config *Config, opts BuildOptions) {
	if opts.NoCache {
		config.NoCache = true
	}
}
