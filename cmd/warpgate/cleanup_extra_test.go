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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
)

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
