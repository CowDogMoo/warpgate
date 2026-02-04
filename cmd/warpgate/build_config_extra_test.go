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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
