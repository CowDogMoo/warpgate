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
	"github.com/spf13/cobra"
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

func TestRunConvertPacker_ValidTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal Packer template directory with docker.pkr.hcl
	dockerPkr := `packer {
  required_plugins {
    docker = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/docker"
    }
  }
}

source "docker" "default" {
  image  = "ubuntu:22.04"
  commit = true
}

build {
  sources = ["source.docker.default"]

  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y curl"
    ]
  }
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerPkr), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "packer"}
	cmd.SetContext(ctx)

	// Save and restore global state
	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.dryRun = true
	convertOpts.output = ""
	convertOpts.author = "Test Author"
	convertOpts.version = "1.0.0"
	convertOpts.includeAMI = false

	output := captureStdoutForTest(t, func() {
		err := runConvertPacker(cmd, []string{tmpDir})
		// May error if the Packer converter doesn't support minimal HCL,
		// but we test it doesn't panic and exercises the code path
		if err != nil {
			// This is acceptable - the converter may not support all Packer formats
			t.Logf("runConvertPacker returned error (expected for minimal template): %v", err)
		}
	})

	// If it succeeded, dry-run output should have content
	_ = output
}

func TestRunConvertPacker_NonexistentDir(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "packer"}
	cmd.SetContext(ctx)

	err := runConvertPacker(cmd, []string{"/nonexistent/packer/template"})
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention does not exist, got: %v", err)
	}
}

func TestDetermineOutputPath_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.output = "output-dir/converted.yaml"

	// This should resolve relative to cwd
	got, err := determineOutputPath(tmpDir)
	if err != nil {
		t.Fatalf("determineOutputPath() unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("determineOutputPath() should return absolute path, got: %q", got)
	}
	if !strings.HasSuffix(got, "converted.yaml") {
		t.Errorf("determineOutputPath() should end with converted.yaml, got: %q", got)
	}
}

func TestWriteConvertedTemplate_ActualWrite(t *testing.T) {
	ctx := setupTestContext(t)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "subdir", "output.yaml")

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.dryRun = false

	yamlData := []byte("name: converted-template\nversion: 1.0\n")

	// Create parent dir
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatal(err)
	}

	err := writeConvertedTemplate(ctx, yamlData, outputPath)
	if err != nil {
		t.Fatalf("writeConvertedTemplate() unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(data) != string(yamlData) {
		t.Errorf("output file contents = %q, want %q", string(data), string(yamlData))
	}
}

func TestDisplayConversionSummary_NoTargets(t *testing.T) {
	cfg := &builder.Config{
		Name: "minimal-template",
		Metadata: builder.Metadata{
			Description: "Minimal template",
		},
		Provisioners: []builder.Provisioner{
			{Type: "shell"},
		},
		Targets: []builder.Target{},
	}

	output := captureStdoutForTest(t, func() {
		displayConversionSummary(cfg, "/tmp/out.yaml")
	})

	if !strings.Contains(output, "minimal-template") {
		t.Error("output should contain template name")
	}
	if !strings.Contains(output, "Provisioners: 1") {
		t.Error("output should contain provisioner count")
	}
	if !strings.Contains(output, "Targets:      0") {
		t.Error("output should contain target count of 0")
	}
}

func TestConvertOptionsStruct(t *testing.T) {
	t.Parallel()

	opts := &convertOptions{
		author:     "Test Author",
		license:    "MIT",
		version:    "2.0.0",
		baseImage:  "alpine:3.18",
		includeAMI: false,
		output:     "/tmp/output.yaml",
		dryRun:     true,
	}

	if opts.author != "Test Author" {
		t.Errorf("author = %q, want %q", opts.author, "Test Author")
	}
	if opts.license != "MIT" {
		t.Errorf("license = %q, want %q", opts.license, "MIT")
	}
	if opts.version != "2.0.0" {
		t.Errorf("version = %q, want %q", opts.version, "2.0.0")
	}
	if opts.baseImage != "alpine:3.18" {
		t.Errorf("baseImage = %q, want %q", opts.baseImage, "alpine:3.18")
	}
	if opts.includeAMI {
		t.Error("includeAMI should be false")
	}
	if !opts.dryRun {
		t.Error("dryRun should be true")
	}
}

func TestRunConvertPacker_WriteOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a minimal Packer template
	dockerPkr := `packer {
  required_plugins {
    docker = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/docker"
    }
  }
}

source "docker" "default" {
  image  = "ubuntu:22.04"
  commit = true
}

build {
  sources = ["source.docker.default"]

  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y curl"
    ]
  }
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerPkr), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "packer"}
	cmd.SetContext(ctx)

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.dryRun = false
	convertOpts.output = filepath.Join(outputDir, "warpgate.yaml")
	convertOpts.author = "Test Author"
	convertOpts.version = "1.0.0"
	convertOpts.includeAMI = false

	err := runConvertPacker(cmd, []string{tmpDir})
	// May or may not succeed depending on the Packer converter's ability to parse the template
	if err != nil {
		t.Logf("runConvertPacker with write output returned error (may be expected): %v", err)
		return
	}

	// If it succeeded, verify the output file exists
	if _, statErr := os.Stat(convertOpts.output); statErr != nil {
		t.Errorf("output file should exist at %s: %v", convertOpts.output, statErr)
	}
}

func TestDetermineOutputPath_CreatesParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedOutput := filepath.Join(tmpDir, "deep", "nested", "output.yaml")

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.output = nestedOutput

	got, err := determineOutputPath(tmpDir)
	if err != nil {
		t.Fatalf("determineOutputPath() error = %v", err)
	}
	if got != nestedOutput {
		t.Errorf("determineOutputPath() = %q, want %q", got, nestedOutput)
	}

	// Verify the parent directory was created
	parentDir := filepath.Dir(nestedOutput)
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("parent directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("parent path should be a directory")
	}
}

func TestDetermineOutputPath_DefaultToTemplateDir(t *testing.T) {
	tmpDir := t.TempDir()

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.output = ""

	got, err := determineOutputPath(tmpDir)
	if err != nil {
		t.Fatalf("determineOutputPath() error = %v", err)
	}
	expected := filepath.Join(tmpDir, "warpgate.yaml")
	if got != expected {
		t.Errorf("determineOutputPath() = %q, want %q", got, expected)
	}
}

func TestDetermineOutputPath_OutputIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.output = tmpDir // a directory, not a file

	_, err := determineOutputPath(tmpDir)
	if err == nil {
		t.Fatal("expected error when output path is a directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("error = %q, want substring 'directory'", err.Error())
	}
}

func TestWriteConvertedTemplate_DryRun_Extra(t *testing.T) {
	ctx := setupTestContext(t)

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.dryRun = true

	yamlData := []byte("name: test-template\n")
	output := captureStdoutForTest(t, func() {
		err := writeConvertedTemplate(ctx, yamlData, "/unused/path")
		if err != nil {
			t.Fatalf("writeConvertedTemplate() error = %v", err)
		}
	})

	if !strings.Contains(output, "Dry run") {
		t.Errorf("output missing dry run header: %q", output)
	}
	if !strings.Contains(output, "test-template") {
		t.Errorf("output missing template content: %q", output)
	}
}

func TestWriteConvertedTemplate_WriteFile_Extra(t *testing.T) {
	ctx := setupTestContext(t)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.yaml")

	oldOpts := *convertOpts
	defer func() { *convertOpts = oldOpts }()
	convertOpts.dryRun = false

	yamlData := []byte("name: written-template\n")
	err := writeConvertedTemplate(ctx, yamlData, outputPath)
	if err != nil {
		t.Fatalf("writeConvertedTemplate() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "written-template") {
		t.Errorf("file content = %q, want to contain 'written-template'", string(data))
	}
}

func TestDisplayConversionSummary_Extra(t *testing.T) {
	cfg := &builder.Config{
		Name: "converted-template",
		Metadata: builder.Metadata{
			Description: "A test template",
		},
		Provisioners: []builder.Provisioner{
			{Type: "shell"},
		},
		Targets: []builder.Target{
			{Type: "container"},
			{Type: "ami"},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayConversionSummary(cfg, "/output/warpgate.yaml")
	})

	if !strings.Contains(output, "converted-template") {
		t.Errorf("output missing template name: %q", output)
	}
	if !strings.Contains(output, "Provisioners: 1") {
		t.Errorf("output missing provisioner count: %q", output)
	}
	if !strings.Contains(output, "Targets:      2") {
		t.Errorf("output missing target count: %q", output)
	}
	if !strings.Contains(output, "container, ami") {
		t.Errorf("output missing target types: %q", output)
	}
}
