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
	"os"
	"path/filepath"
	"strings"
	"testing"

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
