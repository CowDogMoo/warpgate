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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
)

func TestValidateCommandStructure(t *testing.T) {
	t.Parallel()

	if validateCmd.Use != "validate [config]" {
		t.Errorf("validateCmd.Use = %q, want %q", validateCmd.Use, "validate [config]")
	}
}

func TestValidateCommandSyntaxOnlyFlag(t *testing.T) {
	t.Parallel()

	f := validateCmd.Flags().Lookup("syntax-only")
	if f == nil {
		t.Fatal("missing --syntax-only flag")
	}
	if f.DefValue != "false" {
		t.Errorf("--syntax-only default = %q, want %q", f.DefValue, "false")
	}
}

func TestRunValidate_NonexistentFile(t *testing.T) {
	ctx := setupTestContext(t)
	logger := logging.FromContext(ctx)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "validate"}
	cmd.SetContext(ctx)

	err := runValidate(cmd, []string{"/nonexistent/path/warpgate.yaml"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Errorf("error should mention failed to load config, got: %v", err)
	}
}

func TestRunValidate_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := tmpDir + "/invalid.yaml"

	if err := os.WriteFile(invalidFile, []byte("not: valid: yaml: [broken"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "validate"}
	cmd.SetContext(ctx)

	err := runValidate(cmd, []string{invalidFile})
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
