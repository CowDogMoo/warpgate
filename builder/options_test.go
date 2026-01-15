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
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
)

func TestApplyOverrides(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		config    *Config
		opts      BuildOptions
		globalCfg *config.Config
		wantErr   bool
		checkFunc func(*testing.T, *Config)
	}{
		{
			name: "apply target type filter",
			config: &Config{
				Targets: []Target{
					{Type: "container"},
					{Type: "ami"},
				},
			},
			opts: BuildOptions{
				TargetType: "container",
			},
			globalCfg: &config.Config{},
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if len(cfg.Targets) != 1 {
					t.Errorf("Expected 1 target after filter, got %d", len(cfg.Targets))
				}
				if len(cfg.Targets) > 0 && cfg.Targets[0].Type != "container" {
					t.Errorf("Expected container target, got %s", cfg.Targets[0].Type)
				}
			},
		},
		{
			name: "apply architecture overrides",
			config: &Config{
				Architectures: []string{},
			},
			opts: BuildOptions{
				Architectures: []string{"amd64", "arm64"},
			},
			globalCfg: &config.Config{},
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if len(cfg.Architectures) != 2 {
					t.Errorf("Expected 2 architectures, got %d", len(cfg.Architectures))
				}
			},
		},
		{
			name: "apply registry override",
			config: &Config{
				Registry: "",
			},
			opts: BuildOptions{
				Registry: "ghcr.io/myorg",
			},
			globalCfg: &config.Config{},
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.Registry != "ghcr.io/myorg" {
					t.Errorf("Expected registry 'ghcr.io/myorg', got %s", cfg.Registry)
				}
			},
		},
		{
			name: "apply labels and build args",
			config: &Config{
				Labels:    map[string]string{},
				BuildArgs: map[string]string{},
			},
			opts: BuildOptions{
				Labels: map[string]string{
					"version": "1.0",
					"author":  "team",
				},
				BuildArgs: map[string]string{
					"GO_VERSION": "1.25",
				},
			},
			globalCfg: &config.Config{},
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if len(cfg.Labels) != 2 {
					t.Errorf("Expected 2 labels, got %d", len(cfg.Labels))
				}
				if len(cfg.BuildArgs) != 1 {
					t.Errorf("Expected 1 build arg, got %d", len(cfg.BuildArgs))
				}
			},
		},
		{
			name: "apply no-cache option",
			config: &Config{
				NoCache: false,
			},
			opts: BuildOptions{
				NoCache: true,
			},
			globalCfg: &config.Config{},
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if !cfg.NoCache {
					t.Error("Expected NoCache to be true")
				}
			},
		},
		{
			name: "apply AMI target overrides",
			config: &Config{
				Targets: []Target{
					{Type: "ami", Region: "us-east-1", InstanceType: "t2.micro"},
				},
			},
			opts: BuildOptions{
				Region:       "us-west-2",
				InstanceType: "t3.large",
			},
			globalCfg: &config.Config{},
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.Targets[0].Region != "us-west-2" {
					t.Errorf("Expected region 'us-west-2', got %s", cfg.Targets[0].Region)
				}
				if cfg.Targets[0].InstanceType != "t3.large" {
					t.Errorf("Expected instance type 't3.large', got %s", cfg.Targets[0].InstanceType)
				}
			},
		},
		{
			name:      "nil global config",
			config:    &Config{},
			opts:      BuildOptions{},
			globalCfg: nil,
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				// Should not error with nil global config
			},
		},
		{
			name: "apply tag override",
			config: &Config{
				Version: "latest",
			},
			opts: BuildOptions{
				Tags: []string{"v1.2.3"},
			},
			globalCfg: &config.Config{},
			wantErr:   false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.Version != "v1.2.3" {
					t.Errorf("Expected version 'v1.2.3', got %s", cfg.Version)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ApplyOverrides(ctx, tt.config, tt.opts, tt.globalCfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyOverrides() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, tt.config)
			}
		})
	}
}

func TestApplyTargetTypeFilter(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		opts    BuildOptions
		wantLen int
	}{
		{
			name: "filter container targets",
			config: &Config{
				Targets: []Target{
					{Type: "container"},
					{Type: "ami"},
					{Type: "container"},
				},
			},
			opts: BuildOptions{
				TargetType: "container",
			},
			wantLen: 2,
		},
		{
			name: "filter ami targets",
			config: &Config{
				Targets: []Target{
					{Type: "container"},
					{Type: "ami"},
				},
			},
			opts: BuildOptions{
				TargetType: "ami",
			},
			wantLen: 1,
		},
		{
			name: "no filter with empty target type",
			config: &Config{
				Targets: []Target{
					{Type: "container"},
					{Type: "ami"},
				},
			},
			opts: BuildOptions{
				TargetType: "",
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyTargetTypeFilter(tt.config, tt.opts)
			if len(tt.config.Targets) != tt.wantLen {
				t.Errorf("applyTargetTypeFilter() got %d targets, want %d", len(tt.config.Targets), tt.wantLen)
			}
		})
	}
}

func TestApplyArchitectureOverrides(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		opts      BuildOptions
		globalCfg *config.Config
		wantLen   int
		wantArchs []string
	}{
		{
			name: "CLI override takes precedence",
			config: &Config{
				Architectures: []string{"amd64"},
			},
			opts: BuildOptions{
				Architectures: []string{"arm64", "amd64"},
			},
			globalCfg: &config.Config{},
			wantLen:   2,
			wantArchs: []string{"arm64", "amd64"},
		},
		{
			name: "use config architectures if set",
			config: &Config{
				Architectures: []string{"amd64"},
			},
			opts:      BuildOptions{},
			globalCfg: &config.Config{},
			wantLen:   1,
			wantArchs: []string{"amd64"},
		},
		{
			name: "fallback to global config default",
			config: &Config{
				Architectures: []string{},
			},
			opts: BuildOptions{},
			globalCfg: &config.Config{
				Build: config.BuildConfig{
					DefaultArch: []string{"amd64", "arm64"},
				},
			},
			wantLen:   2,
			wantArchs: []string{"amd64", "arm64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyArchitectureOverrides(tt.config, tt.opts, tt.globalCfg)
			if len(tt.config.Architectures) != tt.wantLen {
				t.Errorf("applyArchitectureOverrides() got %d architectures, want %d", len(tt.config.Architectures), tt.wantLen)
			}
			for i, wantArch := range tt.wantArchs {
				if i >= len(tt.config.Architectures) || tt.config.Architectures[i] != wantArch {
					t.Errorf("applyArchitectureOverrides() architecture[%d] = %v, want %v", i, tt.config.Architectures[i], wantArch)
				}
			}
		})
	}
}

func TestApplyRegistryOverride(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		opts         BuildOptions
		globalCfg    *config.Config
		wantRegistry string
	}{
		{
			name:   "CLI override takes precedence",
			config: &Config{Registry: "old-registry"},
			opts: BuildOptions{
				Registry: "ghcr.io/myorg",
			},
			globalCfg:    &config.Config{},
			wantRegistry: "ghcr.io/myorg",
		},
		{
			name:   "use config registry if set",
			config: &Config{Registry: "ghcr.io/myorg"},
			opts:   BuildOptions{},
			globalCfg: &config.Config{
				Registry: config.RegistryConfig{
					Default: "docker.io",
				},
			},
			wantRegistry: "ghcr.io/myorg",
		},
		{
			name:   "fallback to global config default",
			config: &Config{Registry: ""},
			opts:   BuildOptions{},
			globalCfg: &config.Config{
				Registry: config.RegistryConfig{
					Default: "docker.io",
				},
			},
			wantRegistry: "docker.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyRegistryOverride(tt.config, tt.opts, tt.globalCfg)
			if tt.config.Registry != tt.wantRegistry {
				t.Errorf("applyRegistryOverride() registry = %s, want %s", tt.config.Registry, tt.wantRegistry)
			}
		})
	}
}

func TestApplyAMITargetOverrides(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		opts         BuildOptions
		wantRegion   string
		wantInstance string
	}{
		{
			name: "override AMI target region and instance",
			config: &Config{
				Targets: []Target{
					{Type: "ami", Region: "us-east-1", InstanceType: "t2.micro"},
				},
			},
			opts: BuildOptions{
				Region:       "us-west-2",
				InstanceType: "t3.large",
			},
			wantRegion:   "us-west-2",
			wantInstance: "t3.large",
		},
		{
			name: "skip non-AMI targets",
			config: &Config{
				Targets: []Target{
					{Type: "container", Region: "us-east-1"},
				},
			},
			opts: BuildOptions{
				Region: "us-west-2",
			},
			wantRegion: "us-east-1", // Should not change
		},
		{
			name: "no overrides provided",
			config: &Config{
				Targets: []Target{
					{Type: "ami", Region: "us-east-1", InstanceType: "t2.micro"},
				},
			},
			opts:         BuildOptions{},
			wantRegion:   "us-east-1",
			wantInstance: "t2.micro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyAMITargetOverrides(tt.config, tt.opts)
			if len(tt.config.Targets) > 0 {
				if tt.config.Targets[0].Region != tt.wantRegion {
					t.Errorf("applyAMITargetOverrides() region = %s, want %s", tt.config.Targets[0].Region, tt.wantRegion)
				}
				if tt.config.Targets[0].InstanceType != tt.wantInstance {
					t.Errorf("applyAMITargetOverrides() instance = %s, want %s", tt.config.Targets[0].InstanceType, tt.wantInstance)
				}
			}
		})
	}
}

func TestApplyLabelsAndBuildArgs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		config          *Config
		opts            BuildOptions
		wantLabelsLen   int
		wantBuildArgLen int
	}{
		{
			name: "add labels and build args",
			config: &Config{
				Labels:    map[string]string{},
				BuildArgs: map[string]string{},
			},
			opts: BuildOptions{
				Labels: map[string]string{
					"version": "1.0",
					"author":  "team",
				},
				BuildArgs: map[string]string{
					"GO_VERSION": "1.25",
					"BUILD_DATE": "2025-12-10",
				},
			},
			wantLabelsLen:   2,
			wantBuildArgLen: 2,
		},
		{
			name: "merge with existing labels and build args",
			config: &Config{
				Labels: map[string]string{
					"existing": "value",
				},
				BuildArgs: map[string]string{
					"EXISTING_ARG": "value",
				},
			},
			opts: BuildOptions{
				Labels: map[string]string{
					"version": "1.0",
				},
				BuildArgs: map[string]string{
					"GO_VERSION": "1.25",
				},
			},
			wantLabelsLen:   2,
			wantBuildArgLen: 2,
		},
		{
			name: "no labels or build args to add",
			config: &Config{
				Labels:    map[string]string{},
				BuildArgs: map[string]string{},
			},
			opts:            BuildOptions{},
			wantLabelsLen:   0,
			wantBuildArgLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyLabelsAndBuildArgs(ctx, tt.config, tt.opts)
			if len(tt.config.Labels) != tt.wantLabelsLen {
				t.Errorf("applyLabelsAndBuildArgs() labels count = %d, want %d", len(tt.config.Labels), tt.wantLabelsLen)
			}
			if len(tt.config.BuildArgs) != tt.wantBuildArgLen {
				t.Errorf("applyLabelsAndBuildArgs() build args count = %d, want %d", len(tt.config.BuildArgs), tt.wantBuildArgLen)
			}
		})
	}
}

func TestApplyCacheOptions(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		opts        BuildOptions
		wantNoCache bool
	}{
		{
			name: "enable no-cache",
			config: &Config{
				NoCache: false,
			},
			opts: BuildOptions{
				NoCache: true,
			},
			wantNoCache: true,
		},
		{
			name: "keep cache enabled",
			config: &Config{
				NoCache: false,
			},
			opts: BuildOptions{
				NoCache: false,
			},
			wantNoCache: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyCacheOptions(tt.config, tt.opts)
			if tt.config.NoCache != tt.wantNoCache {
				t.Errorf("applyCacheOptions() NoCache = %v, want %v", tt.config.NoCache, tt.wantNoCache)
			}
		})
	}
}

func TestApplyTagOverride(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		config      *Config
		opts        BuildOptions
		wantVersion string
	}{
		{
			name: "override version with first tag",
			config: &Config{
				Version: "latest",
			},
			opts: BuildOptions{
				Tags: []string{"v1.2.3"},
			},
			wantVersion: "v1.2.3",
		},
		{
			name: "use first tag when multiple provided",
			config: &Config{
				Version: "latest",
			},
			opts: BuildOptions{
				Tags: []string{"latest-amd64", "v1.0.0", "stable"},
			},
			wantVersion: "latest-amd64",
		},
		{
			name: "no override when no tags provided",
			config: &Config{
				Version: "latest",
			},
			opts:        BuildOptions{},
			wantVersion: "latest",
		},
		{
			name: "empty tags slice does not override",
			config: &Config{
				Version: "v2.0.0",
			},
			opts: BuildOptions{
				Tags: []string{},
			},
			wantVersion: "v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyTagOverride(ctx, tt.config, tt.opts)
			if tt.config.Version != tt.wantVersion {
				t.Errorf("applyTagOverride() version = %s, want %s", tt.config.Version, tt.wantVersion)
			}
		})
	}
}
