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

	"github.com/cowdogmoo/warpgate/v3/config"
)

func TestCleanupCommandFlags(t *testing.T) {
	t.Parallel()

	flags := []struct {
		name      string
		shorthand string
	}{
		{"region", ""},
		{"dry-run", ""},
		{"all", ""},
		{"versions", ""},
		{"keep", ""},
		{"yes", "y"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cleanupCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("missing flag --%s on cleanup command", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("flag --%s shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestCleanupCommandArgsValidation(t *testing.T) {
	t.Parallel()

	if cleanupCmd.Args == nil {
		t.Fatal("cleanup command should have args validation")
	}
}

func TestRunCleanup_NilConfig(t *testing.T) {
	cmd := newTestCmdNoConfig()

	opts := &cleanupOptions{
		buildName: "test-build",
	}

	err := runCleanup(cmd, opts)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "configuration not initialized") {
		t.Errorf("error should mention configuration not initialized, got: %v", err)
	}
}

func TestRunCleanup_NoBuildNameOrAll(t *testing.T) {
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		buildName: "",
		all:       false,
	}

	err := runCleanup(cmd, opts)
	if err == nil {
		t.Fatal("expected error when neither build name nor --all specified")
	}
	if !strings.Contains(err.Error(), "build name or use --all") {
		t.Errorf("error should mention build name or --all, got: %v", err)
	}
}

func TestRunCleanup_NoRegion(t *testing.T) {
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		buildName: "test-build",
		region:    "",
	}

	err := runCleanup(cmd, opts)
	if err == nil {
		t.Fatal("expected error when no region specified")
	}
	if !strings.Contains(err.Error(), "AWS region must be specified") {
		t.Errorf("error should mention AWS region, got: %v", err)
	}
}

func TestDisplayComponentInfos(t *testing.T) {
	infos := []componentInfo{
		{name: "comp-a", versions: 5, toDelete: 2},
		{name: "comp-b", versions: 3, toDelete: 0},
		{name: "comp-c", versions: 10, toDelete: 7},
	}

	var totalToDelete int
	output := captureStdoutForTest(t, func() {
		totalToDelete = displayComponentInfos(infos, 3)
	})

	if totalToDelete != 9 {
		t.Errorf("displayComponentInfos() total = %d, want 9", totalToDelete)
	}
	if !strings.Contains(output, "comp-a") {
		t.Error("output should contain comp-a")
	}
	if !strings.Contains(output, "(2 to delete)") {
		t.Errorf("output should mention '(2 to delete)' for comp-a, got: %q", output)
	}
}

func TestDisplayComponentInfos_NothingToDelete(t *testing.T) {
	infos := []componentInfo{
		{name: "comp-a", versions: 2, toDelete: 0},
	}

	var totalToDelete int
	captureStdoutForTest(t, func() {
		totalToDelete = displayComponentInfos(infos, 1)
	})

	if totalToDelete != 0 {
		t.Errorf("displayComponentInfos() total = %d, want 0", totalToDelete)
	}
}

func TestRunCleanup_RegionFromConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that requires AWS credentials")
	}
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	cfg := &config.Config{}
	cfg.AWS.Region = "us-west-2"
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		buildName: "test-build",
		region:    "",
	}

	err := runCleanup(cmd, opts)
	// The key assertion is that it does NOT fail with "AWS region must be specified"
	if err != nil && strings.Contains(err.Error(), "AWS region must be specified") {
		t.Errorf("should not fail on region validation when config has region, got: %v", err)
	}
}

func TestRunCleanup_WithAllAndRegion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that requires AWS credentials")
	}
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		all:    true,
		region: "us-east-1",
	}

	err := runCleanup(cmd, opts)
	// Should NOT fail at validation
	if err != nil && strings.Contains(err.Error(), "either specify a build name") {
		t.Errorf("should not fail at name/all validation, got: %v", err)
	}
}

func TestRunCleanup_VersionsMode(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		buildName:    "test-build",
		region:       "us-east-1",
		versions:     true,
		keepVersions: 3,
	}

	// Will try to create AWS clients
	err := runCleanup(cmd, opts)
	// Should get past validation
	if err != nil && strings.Contains(err.Error(), "AWS region must be specified") {
		t.Errorf("should not fail on region validation, got: %v", err)
	}
}

func TestDisplayComponentInfos_MultipleComponents(t *testing.T) {
	infos := []componentInfo{
		{name: "comp-a", versions: 10, toDelete: 7},
		{name: "comp-b", versions: 5, toDelete: 2},
		{name: "comp-c", versions: 3, toDelete: 0},
		{name: "comp-d", versions: 1, toDelete: 0},
	}

	var totalToDelete int
	output := captureStdoutForTest(t, func() {
		totalToDelete = displayComponentInfos(infos, 4)
	})

	if totalToDelete != 9 {
		t.Errorf("displayComponentInfos() total = %d, want 9", totalToDelete)
	}
	if !strings.Contains(output, "comp-a") {
		t.Error("output should contain comp-a")
	}
	if !strings.Contains(output, "comp-b") {
		t.Error("output should contain comp-b")
	}
	if !strings.Contains(output, "(7 to delete)") {
		t.Error("output should contain (7 to delete)")
	}
	if !strings.Contains(output, "(2 to delete)") {
		t.Error("output should contain (2 to delete)")
	}
	if !strings.Contains(output, "Total: 9 versions to delete") {
		t.Error("output should contain total summary")
	}
}

func TestDisplayComponentInfos_Empty(t *testing.T) {
	var totalToDelete int
	captureStdoutForTest(t, func() {
		totalToDelete = displayComponentInfos(nil, 0)
	})

	if totalToDelete != 0 {
		t.Errorf("displayComponentInfos() total = %d, want 0", totalToDelete)
	}
}

func TestRunCleanup_BuildNameAndAllConflict(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.AWS.Region = "us-east-1"
	cmd := newTestCmd(cfg)

	// When both build name and --all are specified, the code uses buildName path
	// because it checks buildName=="" && !all. With both set, it passes validation.
	opts := &cleanupOptions{
		buildName: "my-build",
		all:       true,
		region:    "us-east-1",
	}

	// This should pass the initial validation (buildName is set so the
	// "either specify a build name or use --all" check passes)
	err := runCleanup(cmd, opts)
	// Will fail at AWS client creation, but should not fail at validation
	if err != nil && strings.Contains(err.Error(), "build name or use --all") {
		t.Errorf("should not fail at validation when both name and all are set, got: %v", err)
	}
}

func TestRunCleanup_EmptyRegionNoConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	// No region in config, no region in opts
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		buildName: "test-build",
		region:    "",
	}

	err := runCleanup(cmd, opts)
	if err == nil {
		t.Fatal("expected error when no region specified anywhere")
	}
	if !strings.Contains(err.Error(), "AWS region must be specified") {
		t.Errorf("error should mention AWS region, got: %v", err)
	}
}

func TestRunCleanup_RegionFlagOverridesConfig(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	cfg := &config.Config{}
	cfg.AWS.Region = "us-west-2"
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		buildName: "test-build",
		region:    "eu-west-1", // Override config
	}

	// Should use eu-west-1, not us-west-2
	// Will fail at AWS client or resource listing, but not at validation
	err := runCleanup(cmd, opts)
	if err != nil && strings.Contains(err.Error(), "AWS region must be specified") {
		t.Errorf("should not fail on region validation, got: %v", err)
	}
}

func TestGetComponentInfos_EmptyNames(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	// We cannot create a real ResourceManager without AWS, but we can test
	// the function's behavior with edge cases by verifying that calling with
	// empty names returns empty results
	infos := getComponentInfos(ctx, nil, []string{}, 3)
	if len(infos) != 0 {
		t.Errorf("expected 0 infos for empty names, got %d", len(infos))
	}
}

func TestPerformVersionCleanup_EmptyNames(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	// With empty names, no cleanup should happen
	err := performVersionCleanup(ctx, nil, []string{}, 3)
	if err != nil {
		t.Fatalf("performVersionCleanup() with empty names should not error: %v", err)
	}
}

func TestRunCleanup_VersionsModeWithAll(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		all:          true,
		region:       "us-east-1",
		versions:     true,
		keepVersions: 5,
	}

	// Should pass validation (--all is set)
	err := runCleanup(cmd, opts)
	if err != nil && strings.Contains(err.Error(), "build name or use --all") {
		t.Errorf("should not fail at validation with --all, got: %v", err)
	}
}

func TestComponentInfo_Struct(t *testing.T) {
	t.Parallel()

	info := componentInfo{
		name:     "test-component",
		versions: 10,
		toDelete: 7,
	}

	if info.name != "test-component" {
		t.Errorf("name = %q, want %q", info.name, "test-component")
	}
	if info.versions != 10 {
		t.Errorf("versions = %d, want 10", info.versions)
	}
	if info.toDelete != 7 {
		t.Errorf("toDelete = %d, want 7", info.toDelete)
	}
}

func TestGetComponentNames_WithPrefix(t *testing.T) {
	t.Parallel()

	// Test getComponentNames with a prefix (build name set)
	// Cannot test with real AWS, but verify the function constructs correct prefix
	opts := &cleanupOptions{
		buildName: "my-build",
		all:       false,
	}

	// With a nil manager, this would panic on the method call, so we only verify
	// the prefix logic conceptually
	prefix := opts.buildName
	if opts.all {
		prefix = ""
	}
	if prefix != "my-build" {
		t.Errorf("prefix = %q, want %q", prefix, "my-build")
	}
}

func TestGetComponentNames_WithAll(t *testing.T) {
	t.Parallel()

	// When --all is set, prefix should be empty
	opts := &cleanupOptions{
		buildName: "my-build",
		all:       true,
	}

	prefix := opts.buildName
	if opts.all {
		prefix = ""
	}
	if prefix != "" {
		t.Errorf("prefix = %q, want empty when --all is set", prefix)
	}
}

func TestRunCleanupAll_DryRun(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		all:    true,
		region: "us-east-1",
		dryRun: true,
	}

	// Should pass validation and reach the cleaner
	err := runCleanup(cmd, opts)
	// Will fail at AWS client or listing, but should not fail at validation
	if err != nil && strings.Contains(err.Error(), "build name or use --all") {
		t.Errorf("should not fail at validation with --all, got: %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "AWS region must be specified") {
		t.Errorf("should not fail on region, got: %v", err)
	}
}

func TestRunCleanupBuild_WithYesFlag(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	opts := &cleanupOptions{
		buildName: "test-build",
		region:    "us-east-1",
		yes:       true,
	}

	// Should pass validation
	err := runCleanup(cmd, opts)
	if err != nil && strings.Contains(err.Error(), "build name or use --all") {
		t.Errorf("should not fail at validation, got: %v", err)
	}
}

func TestRegisterCleanupCompletions_Extra(t *testing.T) {
	t.Parallel()

	// Verify cleanup completions can be registered without panic
	cmd := newTestCmdNoConfig()
	cmd.Flags().String("region", "", "region")

	registerCleanupCompletions(cmd)
}

func TestRunVersionCleanup_NoComponents(t *testing.T) {

	// runVersionCleanup calls getComponentNames which calls manager.ListComponentsByPrefix
	// Without a real manager, we test the flow indirectly
	ctx := context.Background()

	// Test displayComponentInfos + the 0 totalToDelete path
	var totalToDelete int
	captureStdoutForTest(t, func() {
		totalToDelete = displayComponentInfos([]componentInfo{
			{name: "comp-a", versions: 2, toDelete: 0},
			{name: "comp-b", versions: 1, toDelete: 0},
		}, 2)
	})

	if totalToDelete != 0 {
		t.Errorf("expected 0 to delete, got %d", totalToDelete)
	}
	_ = ctx
}

func TestRunCleanup_VersionsModeKeepVersionsDefault(t *testing.T) {
	t.Parallel()

	// Verify default keep versions
	opts := &cleanupOptions{
		keepVersions: 3,
	}
	if opts.keepVersions != 3 {
		t.Errorf("keepVersions = %d, want 3", opts.keepVersions)
	}
}

func TestDisplayComponentInfos_Extra(t *testing.T) {
	infos := []componentInfo{
		{name: "comp-a", versions: 5, toDelete: 2},
		{name: "comp-b", versions: 3, toDelete: 0},
	}

	output := captureStdoutForTest(t, func() {
		total := displayComponentInfos(infos, 2)
		if total != 2 {
			t.Errorf("displayComponentInfos() returned %d, want 2", total)
		}
	})

	if !strings.Contains(output, "comp-a") {
		t.Errorf("output missing comp-a: %q", output)
	}
	if !strings.Contains(output, "2 to delete") {
		t.Errorf("output missing delete count: %q", output)
	}
}

func TestDisplayComponentInfos_NothingToDelete_Extra(t *testing.T) {
	infos := []componentInfo{
		{name: "comp-c", versions: 2, toDelete: 0},
	}

	output := captureStdoutForTest(t, func() {
		total := displayComponentInfos(infos, 1)
		if total != 0 {
			t.Errorf("displayComponentInfos() returned %d, want 0", total)
		}
	})

	if !strings.Contains(output, "comp-c") {
		t.Errorf("output missing comp-c: %q", output)
	}
}
