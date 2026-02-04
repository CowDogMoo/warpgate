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
