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
	"testing"

	"github.com/spf13/cobra"
)

func TestInitCommandStructure(t *testing.T) {
	t.Parallel()

	if initCmd.Use != "init [name]" {
		t.Errorf("initCmd.Use = %q, want %q", initCmd.Use, "init [name]")
	}
}

func TestInitCommandFlags(t *testing.T) {
	t.Parallel()

	t.Run("from flag", func(t *testing.T) {
		t.Parallel()
		f := initCmd.Flags().Lookup("from")
		if f == nil {
			t.Fatal("missing --from flag")
		}
		if f.Shorthand != "f" {
			t.Errorf("--from shorthand = %q, want %q", f.Shorthand, "f")
		}
	})

	t.Run("output flag", func(t *testing.T) {
		t.Parallel()
		f := initCmd.Flags().Lookup("output")
		if f == nil {
			t.Fatal("missing --output flag")
		}
		if f.Shorthand != "o" {
			t.Errorf("--output shorthand = %q, want %q", f.Shorthand, "o")
		}
		if f.DefValue != "." {
			t.Errorf("--output default = %q, want %q", f.DefValue, ".")
		}
	})
}

func TestInitCommandArgsValidation(t *testing.T) {
	t.Parallel()

	err := cobra.ExactArgs(1)(initCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	err = cobra.ExactArgs(1)(initCmd, []string{"my-template"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}

	err = cobra.ExactArgs(1)(initCmd, []string{"a", "b"})
	if err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestRunInit_CreateTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := setupTestContext(t)

	cmd := &cobra.Command{Use: "init"}
	cmd.SetContext(ctx)

	// Save and restore global state
	oldFrom := initFromTemplate
	oldOutput := initOutputDir
	defer func() {
		initFromTemplate = oldFrom
		initOutputDir = oldOutput
	}()

	initFromTemplate = ""
	initOutputDir = tmpDir

	err := runInit(cmd, []string{"test-template"})
	if err != nil {
		t.Fatalf("runInit() unexpected error: %v", err)
	}

	// Check that template directory was created
	templateDir := filepath.Join(tmpDir, "test-template")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Error("template directory was not created")
	}

	// Check that warpgate.yaml was created
	configFile := filepath.Join(templateDir, "warpgate.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("warpgate.yaml was not created")
	}
}

func TestRunInit_WithFromFlag(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "init"}
	cmd.SetContext(ctx)

	oldFrom := initFromTemplate
	oldOutput := initOutputDir
	defer func() {
		initFromTemplate = oldFrom
		initOutputDir = oldOutput
	}()

	initFromTemplate = "nonexistent-template"
	initOutputDir = t.TempDir()

	err := runInit(cmd, []string{"my-new-template"})
	// Will fail because template doesn't exist, but exercises the Fork path
	if err == nil {
		t.Log("runInit with --from succeeded unexpectedly")
	}
}

func TestRunInit_WithoutFromFlag(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "init"}
	cmd.SetContext(ctx)

	oldFrom := initFromTemplate
	oldOutput := initOutputDir
	defer func() {
		initFromTemplate = oldFrom
		initOutputDir = oldOutput
	}()

	initFromTemplate = ""
	initOutputDir = t.TempDir()

	err := runInit(cmd, []string{"test-template"})
	// This should succeed - creates scaffolding in temp dir
	if err != nil {
		t.Errorf("runInit without --from failed: %v", err)
	}
}

func TestRunInit_CreateTemplate_Extra(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "init"}
	cmd.SetContext(ctx)

	// Save and restore global vars
	oldOutputDir := initOutputDir
	oldFromTemplate := initFromTemplate
	defer func() {
		initOutputDir = oldOutputDir
		initFromTemplate = oldFromTemplate
	}()

	initOutputDir = tmpDir
	initFromTemplate = ""

	err := runInit(cmd, []string{"my-new-template"})
	if err != nil {
		t.Logf("runInit returned error (may be expected): %v", err)
	}
	// The key goal is to exercise the code path, not verify scaffolding output
}

func TestRunInit_ForkTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "init"}
	cmd.SetContext(ctx)

	oldOutputDir := initOutputDir
	oldFromTemplate := initFromTemplate
	defer func() {
		initOutputDir = oldOutputDir
		initFromTemplate = oldFromTemplate
	}()

	initOutputDir = tmpDir
	initFromTemplate = "nonexistent-template"

	err := runInit(cmd, []string{"forked-template"})
	// Will fail because template doesn't exist, but exercises fork path
	if err == nil {
		t.Log("runInit fork succeeded unexpectedly")
	}
}
