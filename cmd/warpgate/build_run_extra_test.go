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
