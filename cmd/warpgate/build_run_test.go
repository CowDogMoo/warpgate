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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
)

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
