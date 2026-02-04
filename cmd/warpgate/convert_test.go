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

	"github.com/cowdogmoo/warpgate/v3/builder"
)

func TestConvertCommandStructure(t *testing.T) {
	t.Parallel()

	if convertCmd.Use != "convert" {
		t.Errorf("convertCmd.Use = %q, want %q", convertCmd.Use, "convert")
	}

	// Verify packer subcommand is registered
	subCmds := convertCmd.Commands()
	found := false
	for _, c := range subCmds {
		if c.Name() == "packer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("convert packer subcommand not registered")
	}
}

func TestConvertPackerCommandFlags(t *testing.T) {
	t.Parallel()

	flags := []struct {
		name      string
		shorthand string
	}{
		{"author", ""},
		{"license", ""},
		{"version", ""},
		{"base-image", ""},
		{"include-ami", ""},
		{"output", "o"},
		{"dry-run", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := convertPackerCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("missing flag --%s on convert packer command", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("flag --%s shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestResolveTemplatePath_ValidDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	got, err := resolveTemplatePath(tmpDir)
	if err != nil {
		t.Fatalf("resolveTemplatePath() unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("resolveTemplatePath() returned non-absolute path: %q", got)
	}
}

func TestResolveTemplatePath_NonexistentDir(t *testing.T) {
	t.Parallel()

	_, err := resolveTemplatePath("/nonexistent/directory/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention 'does not exist', got: %v", err)
	}
}

func TestDetermineOutputPath_Default(t *testing.T) {
	tmpDir := t.TempDir()

	// Save and restore convertOpts
	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.output = ""

	got, err := determineOutputPath(tmpDir)
	if err != nil {
		t.Fatalf("determineOutputPath() unexpected error: %v", err)
	}

	expected := filepath.Join(tmpDir, "warpgate.yaml")
	if got != expected {
		t.Errorf("determineOutputPath() = %q, want %q", got, expected)
	}
}

func TestDetermineOutputPath_CustomOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "custom.yaml")

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.output = outputFile

	got, err := determineOutputPath(tmpDir)
	if err != nil {
		t.Fatalf("determineOutputPath() unexpected error: %v", err)
	}

	if got != outputFile {
		t.Errorf("determineOutputPath() = %q, want %q", got, outputFile)
	}
}

func TestDetermineOutputPath_DirectoryAsOutput(t *testing.T) {
	tmpDir := t.TempDir()

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.output = tmpDir

	_, err := determineOutputPath(tmpDir)
	if err == nil {
		t.Fatal("expected error when output path is a directory")
	}
	if !strings.Contains(err.Error(), "output path is a directory") {
		t.Errorf("error should mention directory, got: %v", err)
	}
}

func TestWriteConvertedTemplate_DryRun(t *testing.T) {
	ctx := setupTestContext(t)

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.dryRun = true

	yamlData := []byte("name: test\n")

	output := captureStdoutForTest(t, func() {
		err := writeConvertedTemplate(ctx, yamlData, "/any/path")
		if err != nil {
			t.Fatalf("writeConvertedTemplate() unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "Dry run") {
		t.Error("output should contain 'Dry run'")
	}
	if !strings.Contains(output, "name: test") {
		t.Error("output should contain the YAML content")
	}
}

func TestWriteConvertedTemplate_WriteFile(t *testing.T) {
	ctx := setupTestContext(t)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.yaml")

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.dryRun = false

	yamlData := []byte("name: test-template\n")

	err := writeConvertedTemplate(ctx, yamlData, outputPath)
	if err != nil {
		t.Fatalf("writeConvertedTemplate() unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(data) != "name: test-template\n" {
		t.Errorf("output file contents = %q, want %q", string(data), "name: test-template\n")
	}
}

func TestDisplayConversionSummary(t *testing.T) {
	t.Parallel()

	cfg := &builder.Config{
		Name: "test-template",
		Metadata: builder.Metadata{
			Description: "A test template",
		},
		Provisioners: []builder.Provisioner{
			{Type: "shell"},
			{Type: "file"},
		},
		Targets: []builder.Target{
			{Type: "container"},
			{Type: "ami"},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayConversionSummary(cfg, "/tmp/output.yaml")
	})

	if !strings.Contains(output, "test-template") {
		t.Error("output should contain template name")
	}
	if !strings.Contains(output, "A test template") {
		t.Error("output should contain description")
	}
	if !strings.Contains(output, "Provisioners: 2") {
		t.Error("output should contain provisioner count")
	}
	if !strings.Contains(output, "Targets:      2") {
		t.Error("output should contain target count")
	}
	if !strings.Contains(output, "container, ami") {
		t.Error("output should contain target types")
	}
}
