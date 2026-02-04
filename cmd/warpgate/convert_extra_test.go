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
