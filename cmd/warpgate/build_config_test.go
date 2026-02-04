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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestBuildOptsToCliOpts(t *testing.T) {
	t.Parallel()

	t.Run("with args", func(t *testing.T) {
		t.Parallel()
		opts := &buildOptions{
			template:   "my-template",
			fromGit:    "https://github.com/user/repo.git",
			targetType: "container",
			push:       true,
			pushDigest: false,
			registry:   "ghcr.io/test",
			arch:       []string{"amd64", "arm64"},
			tags:       []string{"latest", "v1.0"},
			noCache:    true,
			digestDir:  "/tmp/digests",
		}
		args := []string{"warpgate.yaml"}

		cliOpts := buildOptsToCliOpts(args, opts)

		if cliOpts.ConfigFile != "warpgate.yaml" {
			t.Errorf("ConfigFile = %q, want %q", cliOpts.ConfigFile, "warpgate.yaml")
		}
		if cliOpts.Template != "my-template" {
			t.Errorf("Template = %q, want %q", cliOpts.Template, "my-template")
		}
		if cliOpts.FromGit != "https://github.com/user/repo.git" {
			t.Errorf("FromGit = %q, want %q", cliOpts.FromGit, "https://github.com/user/repo.git")
		}
		if cliOpts.TargetType != "container" {
			t.Errorf("TargetType = %q, want %q", cliOpts.TargetType, "container")
		}
		if !cliOpts.Push {
			t.Error("Push = false, want true")
		}
		if cliOpts.PushDigest {
			t.Error("PushDigest = true, want false")
		}
		if cliOpts.Registry != "ghcr.io/test" {
			t.Errorf("Registry = %q, want %q", cliOpts.Registry, "ghcr.io/test")
		}
		if len(cliOpts.Architectures) != 2 {
			t.Errorf("Architectures length = %d, want 2", len(cliOpts.Architectures))
		}
		if !cliOpts.NoCache {
			t.Error("NoCache = false, want true")
		}
		if cliOpts.DigestDir != "/tmp/digests" {
			t.Errorf("DigestDir = %q, want %q", cliOpts.DigestDir, "/tmp/digests")
		}
	})

	t.Run("without args", func(t *testing.T) {
		t.Parallel()
		opts := &buildOptions{
			template: "attack-box",
		}
		args := []string{}

		cliOpts := buildOptsToCliOpts(args, opts)

		if cliOpts.ConfigFile != "" {
			t.Errorf("ConfigFile = %q, want empty", cliOpts.ConfigFile)
		}
		if cliOpts.Template != "attack-box" {
			t.Errorf("Template = %q, want %q", cliOpts.Template, "attack-box")
		}
	})
}

func TestEnhanceBuildKitError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputErr    error
		wantContain string
	}{
		{
			name:        "no active buildx builder",
			inputErr:    errors.New("no active buildx builder found"),
			wantContain: "docker buildx create",
		},
		{
			name:        "docker daemon not running",
			inputErr:    errors.New("Cannot connect to the Docker daemon"),
			wantContain: "docker is not running",
		},
		{
			name:        "connection refused",
			inputErr:    errors.New("connection refused to socket"),
			wantContain: "docker is not running",
		},
		{
			name:        "docker buildx not available",
			inputErr:    errors.New("docker buildx command not found"),
			wantContain: "docker buildx not available",
		},
		{
			name:        "generic error",
			inputErr:    errors.New("some unknown error"),
			wantContain: "BuildKit error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := enhanceBuildKitError(tt.inputErr)
			if !strings.Contains(got.Error(), tt.wantContain) {
				t.Errorf("enhanceBuildKitError() = %q, want to contain %q", got.Error(), tt.wantContain)
			}
		})
	}
}

func TestLoadBuildConfig_NoInputs(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	opts := &buildOptions{}
	args := []string{}

	_, err := loadBuildConfig(ctx, args, opts)
	if err == nil {
		t.Fatal("expected error when no inputs are provided")
	}
	if !strings.Contains(err.Error(), "specify config file") {
		t.Errorf("error should mention specifying config file, got: %v", err)
	}
}

func TestLoadFromFile_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := loadFromFile("/nonexistent/path/warpgate.yaml", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadFromFile_ValidConfig(t *testing.T) {
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

	cfg, err := loadFromFile(configFile, nil)
	if err != nil {
		t.Fatalf("loadFromFile() unexpected error: %v", err)
	}

	if cfg.Name != "test-image" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-image")
	}
	if cfg.Base.Image != "ubuntu:22.04" {
		t.Errorf("Base.Image = %q, want %q", cfg.Base.Image, "ubuntu:22.04")
	}
	if !cfg.IsLocalTemplate {
		t.Error("IsLocalTemplate should be true for file-loaded config")
	}
}

func TestLoadFromFile_WithVariables(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	configContent := `name: test-image
base:
  image: "ubuntu:22.04"
targets:
  - type: container
provisioners:
  - type: shell
    inline:
      - echo hello
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{
		"KEY1": "value1",
	}

	cfg, err := loadFromFile(configFile, vars)
	if err != nil {
		t.Fatalf("loadFromFile() with variables unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("loadFromFile() returned nil config")
	}
}

func TestLoadBuildConfig_FromFile(t *testing.T) {
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

	cfg, err := loadBuildConfig(ctx, []string{configFile}, opts)
	if err != nil {
		t.Fatalf("loadBuildConfig() unexpected error: %v", err)
	}
	if cfg.Name != "test-image" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-image")
	}
}

func TestLoadBuildConfig_FromFileWithVars(t *testing.T) {
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
		vars: []string{"KEY=value"},
	}

	cfg, err := loadBuildConfig(ctx, []string{configFile}, opts)
	if err != nil {
		t.Fatalf("loadBuildConfig() with vars unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("loadBuildConfig() returned nil config")
	}
}

func TestLoadBuildConfig_InvalidVars(t *testing.T) {
	ctx := setupTestContext(t)
	opts := &buildOptions{
		varFiles: []string{"/nonexistent/vars.yaml"},
	}

	_, err := loadBuildConfig(ctx, []string{"something"}, opts)
	if err == nil {
		t.Fatal("expected error for nonexistent var file")
	}
	if !strings.Contains(err.Error(), "failed to parse variables") {
		t.Errorf("error should mention failed to parse variables, got: %v", err)
	}
}

func TestBuildOptsToCliOpts_AllFields(t *testing.T) {
	t.Parallel()

	opts := &buildOptions{
		template:     "my-template",
		fromGit:      "https://github.com/user/repo.git",
		targetType:   "ami",
		push:         false,
		pushDigest:   true,
		registry:     "ghcr.io/test",
		arch:         []string{"arm64"},
		tags:         []string{"v2.0"},
		saveDigests:  true,
		digestDir:    "/tmp/d",
		region:       "us-west-2",
		instanceType: "t3.large",
		vars:         []string{"A=1"},
		varFiles:     []string{"vars.yaml"},
		cacheFrom:    []string{"type=local"},
		cacheTo:      []string{"type=local"},
		labels:       []string{"l=v"},
		buildArgs:    []string{"B=2"},
		noCache:      false,
	}
	args := []string{"config.yaml"}

	cliOpts := buildOptsToCliOpts(args, opts)

	if cliOpts.ConfigFile != "config.yaml" {
		t.Errorf("ConfigFile = %q, want %q", cliOpts.ConfigFile, "config.yaml")
	}
	if cliOpts.TargetType != "ami" {
		t.Errorf("TargetType = %q, want %q", cliOpts.TargetType, "ami")
	}
	if !cliOpts.PushDigest {
		t.Error("PushDigest should be true")
	}
	if !cliOpts.SaveDigests {
		t.Error("SaveDigests should be true")
	}
	if cliOpts.Region != "us-west-2" {
		t.Errorf("Region = %q, want %q", cliOpts.Region, "us-west-2")
	}
	if cliOpts.InstanceType != "t3.large" {
		t.Errorf("InstanceType = %q, want %q", cliOpts.InstanceType, "t3.large")
	}
	if len(cliOpts.Variables) != 1 {
		t.Errorf("Variables length = %d, want 1", len(cliOpts.Variables))
	}
	if len(cliOpts.VarFiles) != 1 {
		t.Errorf("VarFiles length = %d, want 1", len(cliOpts.VarFiles))
	}
	if len(cliOpts.Labels) != 1 {
		t.Errorf("Labels length = %d, want 1", len(cliOpts.Labels))
	}
	if len(cliOpts.BuildArgs) != 1 {
		t.Errorf("BuildArgs length = %d, want 1", len(cliOpts.BuildArgs))
	}
	if len(cliOpts.CacheFrom) != 1 {
		t.Errorf("CacheFrom length = %d, want 1", len(cliOpts.CacheFrom))
	}
}

func TestLoadBuildConfig_InvalidVarFilePath(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	// Use a nonexistent var-file to trigger variable parse error
	opts := &buildOptions{
		varFiles: []string{"/nonexistent/vars.yaml"},
	}

	_, err := loadBuildConfig(ctx, []string{"some-config.yaml"}, opts)
	if err == nil {
		t.Fatal("expected error when var file does not exist")
	}
	if !strings.Contains(err.Error(), "failed to parse variables") {
		t.Errorf("error = %q, want substring 'failed to parse variables'", err.Error())
	}
}

func TestLoadBuildConfig_TemplateDispatch(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	opts := &buildOptions{
		template: "nonexistent-template-xyz",
	}

	_, err := loadBuildConfig(ctx, []string{}, opts)
	// Should NOT fail with "specify config file" - it should attempt the template path
	if err != nil && strings.Contains(err.Error(), "specify config file") {
		t.Errorf("should have dispatched to template loader, not default error: %v", err)
	}
	// It should fail because template cannot be loaded, but that confirms dispatch
	if err == nil {
		t.Log("loadBuildConfig succeeded unexpectedly with nonexistent template")
	}
}

func TestLoadBuildConfig_GitDispatch(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	opts := &buildOptions{
		fromGit: "https://github.com/nonexistent/repo.git",
	}

	_, err := loadBuildConfig(ctx, []string{}, opts)
	// Should NOT fail with "specify config file" - it should attempt the git path
	if err != nil && strings.Contains(err.Error(), "specify config file") {
		t.Errorf("should have dispatched to git loader, not default error: %v", err)
	}
}

func TestLoadFromFile_SetsIsLocalTemplate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	content := `name: test-image
base:
  image: "ubuntu:22.04"
targets:
  - type: container
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromFile(configFile, nil)
	if err != nil {
		t.Fatalf("loadFromFile() error = %v", err)
	}
	if !cfg.IsLocalTemplate {
		t.Error("loadFromFile should set IsLocalTemplate = true")
	}
}

func TestLoadFromFile_WithVariableSubstitution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	content := `name: ${IMAGE_NAME}
base:
  image: "ubuntu:22.04"
targets:
  - type: container
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{"IMAGE_NAME": "my-custom-image"}
	cfg, err := loadFromFile(configFile, vars)
	if err != nil {
		t.Fatalf("loadFromFile() error = %v", err)
	}
	if cfg.Name != "my-custom-image" {
		t.Errorf("Name = %q, want %q (variable substitution failed)", cfg.Name, "my-custom-image")
	}
}

func TestLoadFromFile_EmptyVariables(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "warpgate.yaml")

	content := `name: test-image
base:
  image: "ubuntu:22.04"
targets:
  - type: container
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Pass empty variables map
	cfg, err := loadFromFile(configFile, map[string]string{})
	if err != nil {
		t.Fatalf("loadFromFile() error = %v", err)
	}
	if cfg.Name != "test-image" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-image")
	}
	if !cfg.IsLocalTemplate {
		t.Error("IsLocalTemplate should be true")
	}
}

func TestNewBuildKitBuilderFunc_ErrorPath(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	// This will try to create a real BuildKit builder and likely fail
	// because Docker/BuildKit isn't available in test environment
	_, err := newBuildKitBuilderFunc(ctx)
	if err != nil {
		// Verify the error was enhanced with helpful messages
		errMsg := err.Error()
		hasEnhancement := strings.Contains(errMsg, "BuildKit") ||
			strings.Contains(errMsg, "docker") ||
			strings.Contains(errMsg, "Docker")
		if !hasEnhancement {
			t.Errorf("error should contain enhanced BuildKit message, got: %q", errMsg)
		}
	}
	// If err is nil, Docker/BuildKit is available - that's fine too
}

func TestEnhanceBuildKitError_AllCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		errMsg      string
		wantContain string
	}{
		{
			name:        "buildx builder case",
			errMsg:      "failed: no active buildx builder",
			wantContain: "docker buildx create",
		},
		{
			name:        "docker daemon capital case",
			errMsg:      "Cannot connect to the Docker daemon",
			wantContain: "start Docker Desktop",
		},
		{
			name:        "docker daemon lowercase case",
			errMsg:      "error from docker daemon",
			wantContain: "start Docker Desktop",
		},
		{
			name:        "connection refused case",
			errMsg:      "dial tcp: connection refused",
			wantContain: "start Docker Desktop",
		},
		{
			name:        "buildx availability case",
			errMsg:      "docker buildx not found",
			wantContain: "Docker Desktop is installed",
		},
		{
			name:        "unknown error default case",
			errMsg:      "some completely unknown error",
			wantContain: "BuildKit error:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := enhanceBuildKitError(errFromString(tt.errMsg))
			if result == nil {
				t.Fatal("enhanceBuildKitError returned nil")
			}
			if !strings.Contains(result.Error(), tt.wantContain) {
				t.Errorf("error = %q, want substring %q", result.Error(), tt.wantContain)
			}
		})
	}
}

// errFromString creates an error from a string for testing.
func errFromString(s string) error {
	return &simpleError{msg: s}
}

type simpleError struct {
	msg string
}

func (e *simpleError) Error() string {
	return e.msg
}

func TestBindCommandFlagsToViper_NestedKeyMapping(t *testing.T) {
	t.Parallel()

	// Verify that BindCommandFlagsToViper maps inherited "registry" flag
	// to "registry.default" viper key via nestedKeyMap
	v := viper.New()
	root := &cobra.Command{Use: "warpgate"}
	root.PersistentFlags().String("registry", "", "container registry")
	child := &cobra.Command{Use: "manifests"}
	child.Flags().String("name", "", "image name")
	root.AddCommand(child)

	BindCommandFlagsToViper(v, child)

	// Set the inherited registry flag value
	_ = root.PersistentFlags().Set("registry", "ghcr.io/myorg")

	// The inherited "registry" flag should be bound to "registry.default" in viper
	got := v.GetString("registry.default")
	if got != "ghcr.io/myorg" {
		t.Errorf("viper key registry.default = %q, want %q", got, "ghcr.io/myorg")
	}
}
