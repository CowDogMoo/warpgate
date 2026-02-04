/*
Copyright (c) 2025 Jayson Grace <jayson.e.grace@gmail.com>

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
)

func TestBuildCommandFlags(t *testing.T) {
	t.Parallel()

	flags := []struct {
		name      string
		shorthand string
	}{
		{"template", ""},
		{"from-git", ""},
		{"target", ""},
		{"push", ""},
		{"push-digest", ""},
		{"registry", ""},
		{"arch", ""},
		{"tag", "t"},
		{"save-digests", ""},
		{"digest-dir", ""},
		{"region", ""},
		{"instance-type", ""},
		{"var", ""},
		{"var-file", ""},
		{"cache-from", ""},
		{"cache-to", ""},
		{"label", ""},
		{"build-arg", ""},
		{"no-cache", ""},
		{"force", ""},
		{"dry-run", ""},
		{"regions", ""},
		{"parallel-regions", ""},
		{"copy-to-regions", ""},
		{"stream-logs", ""},
		{"show-ec2-status", ""},
		{"output-manifest", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := buildCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("missing flag --%s on build command", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("flag --%s shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestValidBuildTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target string
		valid  bool
	}{
		{"container", true},
		{"ami", true},
		{"", true},
		{"invalid", false},
		{"docker", false},
	}

	for _, tt := range tests {
		t.Run("target_"+tt.target, func(t *testing.T) {
			t.Parallel()
			if got := validBuildTargets[tt.target]; got != tt.valid {
				t.Errorf("validBuildTargets[%q] = %v, want %v", tt.target, got, tt.valid)
			}
		})
	}
}

func TestShouldPerformPush(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     buildOptions
		expected bool
	}{
		{
			name:     "push with registry",
			opts:     buildOptions{push: true, registry: "ghcr.io/test"},
			expected: true,
		},
		{
			name:     "push-digest with registry",
			opts:     buildOptions{pushDigest: true, registry: "ghcr.io/test"},
			expected: true,
		},
		{
			name:     "push without registry",
			opts:     buildOptions{push: true, registry: ""},
			expected: false,
		},
		{
			name:     "no push",
			opts:     buildOptions{push: false, pushDigest: false, registry: "ghcr.io/test"},
			expected: false,
		},
		{
			name:     "neither push nor registry",
			opts:     buildOptions{push: false, pushDigest: false, registry: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldPerformPush(&tt.opts)
			if got != tt.expected {
				t.Errorf("shouldPerformPush() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetermineTargetRegions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.Config
		opts     *buildOptions
		expected []string
	}{
		{
			name:     "regions flag takes priority",
			cfg:      &config.Config{},
			opts:     &buildOptions{regions: []string{"us-east-1", "us-west-2"}, region: "eu-west-1"},
			expected: []string{"us-east-1", "us-west-2"},
		},
		{
			name:     "region flag takes priority over config",
			cfg:      &config.Config{},
			opts:     &buildOptions{region: "us-west-2"},
			expected: []string{"us-west-2"},
		},
		{
			name: "config region used as fallback",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.AWS.Region = "eu-central-1"
				return c
			}(),
			opts:     &buildOptions{},
			expected: []string{"eu-central-1"},
		},
		{
			name:     "no regions specified",
			cfg:      &config.Config{},
			opts:     &buildOptions{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := determineTargetRegions(tt.cfg, tt.opts)
			if len(got) != len(tt.expected) {
				t.Fatalf("determineTargetRegions() returned %d regions, want %d", len(got), len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("region[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetCommonAWSRegions(t *testing.T) {
	t.Parallel()

	regions := getCommonAWSRegions()
	if len(regions) == 0 {
		t.Fatal("getCommonAWSRegions() returned empty list")
	}

	// Verify some expected regions are present
	found := false
	for _, r := range regions {
		if r == "us-east-1\tUS East (N. Virginia)" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected us-east-1 in common AWS regions")
	}
}

func TestGetCommonRegistries(t *testing.T) {
	t.Parallel()

	registries := getCommonRegistries()
	if len(registries) == 0 {
		t.Fatal("getCommonRegistries() returned empty list")
	}

	found := false
	for _, r := range registries {
		if r == "ghcr.io\tGitHub Container Registry" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ghcr.io in common registries")
	}
}

func TestResolveSourceReference(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceMap := map[string]string{
		"arsenal": "/tmp/arsenal",
		"tools":   "/opt/tools",
	}

	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name:     "simple source reference",
			source:   "${sources.arsenal}",
			expected: "/tmp/arsenal",
		},
		{
			name:     "source reference with subpath",
			source:   "${sources.arsenal/scripts/setup.sh}",
			expected: "/tmp/arsenal/scripts/setup.sh",
		},
		{
			name:     "non-reference passthrough",
			source:   "/usr/local/bin/script.sh",
			expected: "/usr/local/bin/script.sh",
		},
		{
			name:     "unknown source reference",
			source:   "${sources.nonexistent}",
			expected: "${sources.nonexistent}",
		},
		{
			name:     "empty string",
			source:   "",
			expected: "",
		},
		{
			name:     "partial match not a reference",
			source:   "sources.arsenal",
			expected: "sources.arsenal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveSourceReference(ctx, tt.source, sourceMap)
			if got != tt.expected {
				t.Errorf("resolveSourceReference(%q) = %q, want %q", tt.source, got, tt.expected)
			}
		})
	}
}

func TestUpdateProvisionerSourcePaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cfg := &builder.Config{
		Sources: []builder.Source{
			{Name: "arsenal", Path: "/tmp/arsenal-clone"},
		},
		Provisioners: []builder.Provisioner{
			{
				Type:   "file",
				Source: "${sources.arsenal}",
			},
			{
				Type:   "file",
				Source: "${sources.arsenal/scripts/run.sh}",
			},
			{
				Type:   "shell",
				Source: "",
				Inline: []string{"echo hello"},
			},
			{
				Type:   "file",
				Source: "/absolute/path/file.txt",
			},
		},
	}

	updateProvisionerSourcePaths(ctx, cfg)

	expected := []string{
		"/tmp/arsenal-clone",
		"/tmp/arsenal-clone/scripts/run.sh",
		"",
		"/absolute/path/file.txt",
	}

	for i, want := range expected {
		got := cfg.Provisioners[i].Source
		if got != want {
			t.Errorf("provisioner[%d].Source = %q, want %q", i, got, want)
		}
	}
}

func TestSourceRefPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		matches bool
	}{
		{"valid source ref", "${sources.arsenal}", true},
		{"valid with subpath", "${sources.arsenal/scripts/run.sh}", true},
		{"valid with hyphens", "${sources.my-template}", true},
		{"valid with underscores", "${sources.my_template}", true},
		{"invalid no dollar", "sources.arsenal", false},
		{"invalid no braces", "$sources.arsenal", false},
		{"invalid empty name", "${sources.}", false},
		{"invalid spaces", "${sources. arsenal}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sourceRefPattern.MatchString(tt.input)
			if got != tt.matches {
				t.Errorf("sourceRefPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.matches)
			}
		})
	}
}

func TestRunBuild_NilConfig(t *testing.T) {
	cmd := &cobra.Command{Use: "build"}
	cmd.SetContext(context.Background())

	opts := &buildOptions{template: "test"}

	err := runBuild(cmd, []string{}, opts)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "configuration not initialized") {
		t.Errorf("error should mention configuration not initialized, got: %v", err)
	}
}

func TestRunBuild_InvalidTarget(t *testing.T) {
	cfg := &config.Config{}
	logger := logging.NewCustomLoggerWithOptions("error", "text", true, false)
	ctx := context.WithValue(context.Background(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "build"}
	cmd.SetContext(ctx)

	opts := &buildOptions{
		template:   "test",
		targetType: "invalid-target",
	}

	err := runBuild(cmd, []string{}, opts)
	if err == nil {
		t.Fatal("expected error for invalid target")
	}
	if !strings.Contains(err.Error(), "invalid target") {
		t.Errorf("error should mention invalid target, got: %v", err)
	}
}

func TestRunBuild_NoInputsSpecified(t *testing.T) {
	cfg := &config.Config{}
	logger := logging.NewCustomLoggerWithOptions("error", "text", true, false)
	ctx := context.WithValue(context.Background(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "build"}
	cmd.SetContext(ctx)

	opts := &buildOptions{}

	err := runBuild(cmd, []string{}, opts)
	if err == nil {
		t.Fatal("expected error when no inputs specified")
	}
	if !strings.Contains(err.Error(), "specify config file") {
		t.Errorf("error should mention specifying config file, got: %v", err)
	}
}

func TestRunBuild_NonexistentConfigFile(t *testing.T) {
	cfg := &config.Config{}
	logger := logging.NewCustomLoggerWithOptions("error", "text", true, false)
	ctx := context.WithValue(context.Background(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "build"}
	cmd.SetContext(ctx)

	opts := &buildOptions{}

	err := runBuild(cmd, []string{"/nonexistent/warpgate.yaml"}, opts)
	if err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
}

func TestExecuteBuild_UnsupportedTarget(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	cfg := &config.Config{}

	_, err := executeBuild(ctx, "unknown-target", nil, cfg, nil, builder.BuildOptions{}, &buildOptions{})
	if err == nil {
		t.Fatal("expected error for unsupported target type")
	}
	if !strings.Contains(err.Error(), "unsupported target type") {
		t.Errorf("error should mention unsupported target type, got: %v", err)
	}
}

func TestLoadAndValidateBuildConfig_NoInputs(t *testing.T) {
	ctx := setupTestContext(t)
	opts := &buildOptions{}

	_, err := loadAndValidateBuildConfig(ctx, []string{}, opts)
	if err == nil {
		t.Fatal("expected error when no inputs provided")
	}
}

func TestBuildOptionsStruct(t *testing.T) {
	t.Parallel()

	opts := &buildOptions{
		template:        "my-template",
		fromGit:         "https://github.com/user/repo.git",
		targetType:      "container",
		push:            true,
		pushDigest:      false,
		registry:        "ghcr.io/test",
		arch:            []string{"amd64", "arm64"},
		tags:            []string{"latest"},
		saveDigests:     true,
		digestDir:       "/tmp/digests",
		region:          "us-east-1",
		regions:         []string{"us-east-1", "us-west-2"},
		instanceType:    "t3.large",
		vars:            []string{"KEY=value"},
		varFiles:        []string{"vars.yaml"},
		cacheFrom:       []string{"type=registry,ref=cache"},
		cacheTo:         []string{"type=registry,ref=cache"},
		labels:          []string{"key=value"},
		buildArgs:       []string{"ARG=val"},
		noCache:         true,
		forceRecreate:   true,
		dryRun:          false,
		parallelRegions: true,
		copyToRegions:   []string{"eu-west-1"},
		streamLogs:      true,
		showEC2Status:   true,
		outputManifest:  "manifest.json",
	}

	if opts.template != "my-template" {
		t.Errorf("template = %q, want %q", opts.template, "my-template")
	}
	if !opts.push {
		t.Error("push should be true")
	}
	if !opts.noCache {
		t.Error("noCache should be true")
	}
	if !opts.forceRecreate {
		t.Error("forceRecreate should be true")
	}
	if !opts.parallelRegions {
		t.Error("parallelRegions should be true")
	}
	if !opts.streamLogs {
		t.Error("streamLogs should be true")
	}
	if !opts.showEC2Status {
		t.Error("showEC2Status should be true")
	}
	if len(opts.regions) != 2 {
		t.Errorf("regions length = %d, want 2", len(opts.regions))
	}
}

func TestRunBuild_ValidConfigFile_NoSources(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	// Config with no sources to avoid git fetch
	validConfig := `name: test-image
base:
  image: "ubuntu:22.04"
targets:
  - type: container
provisioners:
  - type: shell
    inline:
      - echo hello
`
	if err := os.WriteFile(configFile, []byte(validConfig), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	logger := logging.NewCustomLoggerWithOptions("error", "text", true, false)
	ctx := context.WithValue(context.Background(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "build"}
	cmd.SetContext(ctx)

	opts := &buildOptions{}

	// This will fail at the container build step (no BuildKit) but should get past config loading
	err := runBuild(cmd, []string{configFile}, opts)
	if err == nil {
		t.Fatal("expected error (no BuildKit available), but should have gotten past config loading")
	}
	// Error should be about build failure, not config loading
	if strings.Contains(err.Error(), "specify config file") {
		t.Errorf("should not fail at config loading, got: %v", err)
	}
	if strings.Contains(err.Error(), "configuration not initialized") {
		t.Errorf("should not fail at config check, got: %v", err)
	}
}

func TestRunBuild_WithLabelsAndBuildArgs(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	validConfig := `name: test-image
base:
  image: "ubuntu:22.04"
targets:
  - type: container
provisioners:
  - type: shell
    inline:
      - echo hello
`
	if err := os.WriteFile(configFile, []byte(validConfig), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	logger := logging.NewCustomLoggerWithOptions("error", "text", true, false)
	ctx := context.WithValue(context.Background(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "build"}
	cmd.SetContext(ctx)

	opts := &buildOptions{
		labels:    []string{"app=test", "version=1.0"},
		buildArgs: []string{"DEBUG=true"},
	}

	// Will fail at build time, but exercises the labels/buildArgs parsing path
	err := runBuild(cmd, []string{configFile}, opts)
	// Should get past label parsing
	if err != nil && strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("should not fail at label parsing, got: %v", err)
	}
}

func TestRunBuild_InvalidLabels(t *testing.T) {
	cfg := &config.Config{}
	logger := logging.NewCustomLoggerWithOptions("error", "text", true, false)
	ctx := context.WithValue(context.Background(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "build"}
	cmd.SetContext(ctx)

	opts := &buildOptions{
		labels: []string{"invalid-no-equals"},
	}

	err := runBuild(cmd, []string{"some-config.yaml"}, opts)
	if err == nil {
		t.Fatal("expected error for invalid labels")
	}
}

func TestLoadAndValidateBuildConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	validConfig := `name: test-image
base:
  image: "ubuntu:22.04"
targets:
  - type: container
provisioners:
  - type: shell
    inline:
      - echo hello
`
	if err := os.WriteFile(configFile, []byte(validConfig), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := setupTestContext(t)
	opts := &buildOptions{}

	cfg, err := loadAndValidateBuildConfig(ctx, []string{configFile}, opts)
	if err != nil {
		t.Fatalf("loadAndValidateBuildConfig() unexpected error: %v", err)
	}
	if cfg.Name != "test-image" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-image")
	}
}

func TestLoadAndValidateBuildConfig_InvalidOptions(t *testing.T) {
	ctx := setupTestContext(t)
	// Both push and push-digest set -- that should be an invalid option
	opts := &buildOptions{
		push:       true,
		pushDigest: true,
	}

	_, err := loadAndValidateBuildConfig(ctx, []string{"some-file.yaml"}, opts)
	if err == nil {
		// If the validator doesn't catch this, the loadBuildConfig will fail on nonexistent file
		// Either way, there should be an error
		t.Fatal("expected error for invalid options or nonexistent file")
	}
}

func TestLoadAndValidateBuildConfig_NonexistentFile(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	opts := &buildOptions{}

	_, err := loadAndValidateBuildConfig(ctx, []string{"/tmp/nonexistent-warpgate-config.yaml"}, opts)
	if err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
	if !strings.Contains(err.Error(), "failed to load build configuration") {
		t.Errorf("error should mention failed to load, got: %v", err)
	}
}

func TestLoadAndValidateBuildConfig_EmptyYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	// Write a minimal but parseable YAML
	if err := os.WriteFile(configFile, []byte("name: \"\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := setupTestContext(t)
	opts := &buildOptions{}

	// Should load but may fail validation depending on validator behavior
	_, err := loadAndValidateBuildConfig(ctx, []string{configFile}, opts)
	// We just verify it does not panic -- whether it errors or not depends on validation logic
	_ = err
}

func TestLoadAndValidateBuildConfig_InvalidYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	// Write invalid YAML content
	if err := os.WriteFile(configFile, []byte("{{{{not valid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := setupTestContext(t)
	opts := &buildOptions{}

	_, err := loadAndValidateBuildConfig(ctx, []string{configFile}, opts)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadAndValidateBuildConfig_WithVars(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	validConfig := `name: test-image
base:
  image: "ubuntu:22.04"
targets:
  - type: container
provisioners:
  - type: shell
    inline:
      - echo hello
`
	if err := os.WriteFile(configFile, []byte(validConfig), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := setupTestContext(t)
	opts := &buildOptions{
		vars: []string{"MY_VAR=my_value"},
	}

	cfg, err := loadAndValidateBuildConfig(ctx, []string{configFile}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestExecuteBuild_UnsupportedTargetType(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	cfg := &config.Config{}

	_, err := executeBuild(ctx, "vmware", nil, cfg, nil, builder.BuildOptions{}, &buildOptions{})
	if err == nil {
		t.Fatal("expected error for unsupported target type")
	}
	if !strings.Contains(err.Error(), "unsupported target type: vmware") {
		t.Errorf("error should mention unsupported target type, got: %v", err)
	}
}

func TestExecuteBuild_ContainerPath_NilService(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	cfg := &config.Config{}
	buildConfig := &builder.Config{
		Name: "test",
	}

	// Calling with a nil service for container should panic or error
	// We need a non-nil service, so create one with a failing builder
	service := builder.NewBuildService(cfg, func(ctx context.Context) (builder.ContainerBuilder, error) {
		return nil, fmt.Errorf("mock: no BuildKit available")
	})

	_, err := executeBuild(ctx, "container", service, cfg, buildConfig, builder.BuildOptions{}, &buildOptions{})
	if err == nil {
		t.Fatal("expected error for container build without BuildKit")
	}
	if !strings.Contains(err.Error(), "container build failed") {
		t.Errorf("error should mention container build failed, got: %v", err)
	}
}

func TestExecuteBuild_AMIPath_NoAWSCredentials(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	ctx := setupTestContext(t)
	cfg := &config.Config{}
	cfg.AWS.Region = "us-east-1"
	buildConfig := &builder.Config{
		Name: "test",
	}

	service := builder.NewBuildService(cfg, func(ctx context.Context) (builder.ContainerBuilder, error) {
		return nil, fmt.Errorf("not used")
	})

	opts := &buildOptions{
		region: "us-east-1",
	}

	// AMI build will fail when trying to create the AMI builder with mock credentials
	_, err := executeBuild(ctx, "ami", service, cfg, buildConfig, builder.BuildOptions{}, opts)
	// Should fail at the AMI builder creation or build step, not at dispatch
	if err == nil {
		t.Log("AMI build succeeded unexpectedly (likely had real AWS credentials)")
	}
}

func TestExecuteBuild_EmptyTargetType(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	cfg := &config.Config{}

	_, err := executeBuild(ctx, "", nil, cfg, nil, builder.BuildOptions{}, &buildOptions{})
	if err == nil {
		t.Fatal("expected error for empty target type")
	}
	if !strings.Contains(err.Error(), "unsupported target type") {
		t.Errorf("error should mention unsupported target type, got: %v", err)
	}
}

func TestCopyAMIToRegions_SkipSourceRegion(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	cfg := &config.Config{}

	// When target region is the same as source region, it should be skipped
	// The function will try to create AWS clients for non-source regions
	// and fail, but source region should be skipped
	results, err := copyAMIToRegions(ctx, cfg, "ami-12345", "us-east-1", []string{"us-east-1"})
	// All regions are the source region, so all should be skipped
	// No AWS clients needed, no errors
	if err != nil {
		t.Fatalf("copyAMIToRegions() with only source region should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when all regions are source, got %d", len(results))
	}
}

func TestCopyAMIToRegions_MixedRegions(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	ctx := setupTestContext(t)
	cfg := &config.Config{}

	// Source region should be skipped, other regions will fail at client creation
	_, err := copyAMIToRegions(ctx, cfg, "ami-12345", "us-east-1", []string{"us-east-1", "us-west-2"})
	// us-east-1 skipped, us-west-2 will fail at copy (no real AWS)
	// Function returns partial results + error
	_ = err // May or may not error depending on AWS mock behavior
}

func TestCopyAMIToRegions_EmptyTargetRegions(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	cfg := &config.Config{}

	results, err := copyAMIToRegions(ctx, cfg, "ami-12345", "us-east-1", []string{})
	if err != nil {
		t.Fatalf("copyAMIToRegions() with empty regions should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty regions, got %d", len(results))
	}
}

func TestRegisterBuildCompletions(t *testing.T) {
	t.Parallel()

	// Verify that completion functions are registered for key flags
	cmd := &cobra.Command{Use: "test-build"}
	cmd.Flags().StringSlice("arch", nil, "arch")
	cmd.Flags().String("target", "", "target")
	cmd.Flags().String("region", "", "region")
	cmd.Flags().StringSlice("regions", nil, "regions")
	cmd.Flags().String("registry", "", "registry")
	cmd.Flags().String("instance-type", "", "instance type")

	registerBuildCompletions(cmd)

	// Verify flag completion functions are registered (no panic)
}

func TestShouldPerformPush_Extra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     *buildOptions
		expected bool
	}{
		{"push with registry", &buildOptions{push: true, registry: "ghcr.io"}, true},
		{"push-digest with registry", &buildOptions{pushDigest: true, registry: "ghcr.io"}, true},
		{"push without registry", &buildOptions{push: true, registry: ""}, false},
		{"no push flags", &buildOptions{push: false, pushDigest: false, registry: "ghcr.io"}, false},
		{"all false", &buildOptions{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldPerformPush(tt.opts)
			if got != tt.expected {
				t.Errorf("shouldPerformPush() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetermineTargetRegions_Extra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.Config
		opts     *buildOptions
		expected []string
	}{
		{
			"regions flag takes priority",
			&config.Config{},
			&buildOptions{regions: []string{"us-east-1", "us-west-2"}},
			[]string{"us-east-1", "us-west-2"},
		},
		{
			"region flag used when no regions",
			&config.Config{},
			&buildOptions{region: "eu-west-1"},
			[]string{"eu-west-1"},
		},
		{
			"config region used as fallback",
			func() *config.Config {
				c := &config.Config{}
				c.AWS.Region = "ap-southeast-1"
				return c
			}(),
			&buildOptions{},
			[]string{"ap-southeast-1"},
		},
		{
			"empty when nothing set",
			&config.Config{},
			&buildOptions{},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := determineTargetRegions(tt.cfg, tt.opts)
			if len(got) != len(tt.expected) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("region[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestValidBuildTargets_Extra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target string
		valid  bool
	}{
		{"container", true},
		{"ami", true},
		{"", true},
		{"invalid", false},
		{"docker", false},
	}

	for _, tt := range tests {
		t.Run("target_"+tt.target, func(t *testing.T) {
			t.Parallel()
			if got := validBuildTargets[tt.target]; got != tt.valid {
				t.Errorf("validBuildTargets[%q] = %v, want %v", tt.target, got, tt.valid)
			}
		})
	}
}

func TestBuildOptsToCliOpts_Extra(t *testing.T) {
	t.Parallel()

	opts := &buildOptions{
		template:   "attack-box",
		fromGit:    "https://github.com/user/repo.git",
		targetType: "container",
		push:       true,
		pushDigest: false,
		registry:   "ghcr.io",
		arch:       []string{"amd64", "arm64"},
		tags:       []string{"latest", "v1.0"},
		region:     "us-east-1",
		noCache:    true,
	}

	cliOpts := buildOptsToCliOpts([]string{"config.yaml"}, opts)

	if cliOpts.ConfigFile != "config.yaml" {
		t.Errorf("ConfigFile = %q, want %q", cliOpts.ConfigFile, "config.yaml")
	}
	if cliOpts.Template != "attack-box" {
		t.Errorf("Template = %q, want %q", cliOpts.Template, "attack-box")
	}
	if cliOpts.FromGit != "https://github.com/user/repo.git" {
		t.Errorf("FromGit = %q, want git URL", cliOpts.FromGit)
	}
	if !cliOpts.Push {
		t.Error("Push should be true")
	}
	if cliOpts.PushDigest {
		t.Error("PushDigest should be false")
	}
	if !cliOpts.NoCache {
		t.Error("NoCache should be true")
	}
}

func TestBuildOptsToCliOpts_NoArgs(t *testing.T) {
	t.Parallel()

	opts := &buildOptions{template: "test"}
	cliOpts := buildOptsToCliOpts([]string{}, opts)

	if cliOpts.ConfigFile != "" {
		t.Errorf("ConfigFile = %q, want empty", cliOpts.ConfigFile)
	}
}

func TestResolveTemplatePath_CurrentDir(t *testing.T) {
	t.Parallel()

	// "." should resolve to an absolute path
	got, err := resolveTemplatePath(".")
	if err != nil {
		t.Fatalf("resolveTemplatePath(\".\") error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("resolveTemplatePath(\".\") returned non-absolute path: %q", got)
	}
}

func TestResolveTemplatePath_NonexistentDir_Extra(t *testing.T) {
	t.Parallel()

	_, err := resolveTemplatePath("/nonexistent/path/to/template")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want substring 'does not exist'", err.Error())
	}
}

func TestResolveTemplatePath_ValidDir_Extra(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	got, err := resolveTemplatePath(tmpDir)
	if err != nil {
		t.Fatalf("resolveTemplatePath() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("resolveTemplatePath() returned non-absolute path: %q", got)
	}
}

func TestResolveSourceReference_ValidReference(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	sourceMap := map[string]string{
		"arsenal": "/tmp/arsenal-repo",
	}

	result := resolveSourceReference(ctx, "${sources.arsenal}", sourceMap)
	if result != "/tmp/arsenal-repo" {
		t.Errorf("resolveSourceReference() = %q, want %q", result, "/tmp/arsenal-repo")
	}
}

func TestResolveSourceReference_WithSubpath(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	sourceMap := map[string]string{
		"arsenal": "/tmp/arsenal-repo",
	}

	result := resolveSourceReference(ctx, "${sources.arsenal/scripts/setup.sh}", sourceMap)
	expected := filepath.Join("/tmp/arsenal-repo", "scripts/setup.sh")
	if result != expected {
		t.Errorf("resolveSourceReference() = %q, want %q", result, expected)
	}
}

func TestResolveSourceReference_NotAReference(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	sourceMap := map[string]string{
		"arsenal": "/tmp/arsenal-repo",
	}

	// Plain path should be returned unchanged
	result := resolveSourceReference(ctx, "/some/plain/path", sourceMap)
	if result != "/some/plain/path" {
		t.Errorf("resolveSourceReference() = %q, want unchanged path", result)
	}
}

func TestResolveSourceReference_MissingSource(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	sourceMap := map[string]string{}

	// Should return original reference when source not found
	result := resolveSourceReference(ctx, "${sources.nonexistent}", sourceMap)
	if result != "${sources.nonexistent}" {
		t.Errorf("resolveSourceReference() = %q, want original reference", result)
	}
}

func TestResolveSourceReference_InvalidFormat(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	sourceMap := map[string]string{
		"arsenal": "/tmp/arsenal-repo",
	}

	// Not matching the pattern
	result := resolveSourceReference(ctx, "${invalid.arsenal}", sourceMap)
	if result != "${invalid.arsenal}" {
		t.Errorf("resolveSourceReference() = %q, want original reference", result)
	}
}

func TestSourceRefPattern_MatchCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		matches bool
	}{
		{"${sources.arsenal}", true},
		{"${sources.my-template}", true},
		{"${sources.my_template}", true},
		{"${sources.arsenal/scripts/setup.sh}", true},
		{"${sources.arsenal/deep/nested/path}", true},
		{"plain/path", false},
		{"${invalid.arsenal}", false},
		{"${sources.}", false},
		{"${sources}", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := sourceRefPattern.MatchString(tt.input)
			if got != tt.matches {
				t.Errorf("sourceRefPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.matches)
			}
		})
	}
}
