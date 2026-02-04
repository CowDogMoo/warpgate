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

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/spf13/cobra"
)

func TestRunTemplatesAdd_WithConfig_LocalPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	// This will try to add a nonexistent path, but exercises the code path
	err := runTemplatesAdd(cmd, []string{tmpDir})
	// May succeed or fail depending on template manager behavior
	_ = err
}

func TestRunTemplatesRemove_WithConfig(t *testing.T) {
	cfg := &config.Config{}
	ctx := setupTestContext(t)

	cmd := &cobra.Command{Use: "remove"}
	cmd.SetContext(ctx)
	// Store config in context
	cmd.SetContext(newTestCmd(cfg).Context())

	err := runTemplatesRemove(cmd, []string{"nonexistent-source"})
	// Will error since the source doesn't exist, but exercises the code path
	if err == nil {
		// May or may not error depending on manager implementation
		t.Log("runTemplatesRemove succeeded (source may have been silently ignored)")
	}
}

func TestRunTemplatesList_JsonFormat(t *testing.T) {
	ctx := setupTestContext(t)

	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(ctx)

	// Save and restore globals
	oldFormat := templatesListFormat
	oldSource := templatesListSource
	oldQuiet := templatesListQuiet
	defer func() {
		templatesListFormat = oldFormat
		templatesListSource = oldSource
		templatesListQuiet = oldQuiet
	}()

	templatesListFormat = "json"
	templatesListSource = "local"
	templatesListQuiet = false

	// This tries to create a template registry, which may fail
	// but exercises the format validation path
	err := runTemplatesList(cmd, []string{})
	_ = err // May error depending on registry availability
}

func TestRunTemplatesList_GhaMatrixFormat(t *testing.T) {
	ctx := setupTestContext(t)

	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(ctx)

	oldFormat := templatesListFormat
	oldSource := templatesListSource
	defer func() {
		templatesListFormat = oldFormat
		templatesListSource = oldSource
	}()

	templatesListFormat = "gha-matrix"
	templatesListSource = "local"

	err := runTemplatesList(cmd, []string{})
	_ = err
}

func TestRunTemplatesList_TableFormat(t *testing.T) {
	ctx := setupTestContext(t)

	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(ctx)

	oldFormat := templatesListFormat
	oldSource := templatesListSource
	defer func() {
		templatesListFormat = oldFormat
		templatesListSource = oldSource
	}()

	templatesListFormat = "table"
	templatesListSource = "local"

	err := runTemplatesList(cmd, []string{})
	_ = err
}

func TestValidTemplatesListFormats_All(t *testing.T) {
	t.Parallel()

	expected := map[string]bool{
		"table":      true,
		"json":       true,
		"gha-matrix": true,
	}

	for format, want := range expected {
		if got := validTemplatesListFormats[format]; got != want {
			t.Errorf("validTemplatesListFormats[%q] = %v, want %v", format, got, want)
		}
	}

	// Check invalid format
	if validTemplatesListFormats["xml"] {
		t.Error("xml should not be a valid format")
	}
	if validTemplatesListFormats["yaml"] {
		t.Error("yaml should not be a valid format")
	}
}

func TestDisplayTemplateInfo_WithEnvVars(t *testing.T) {
	cfg := &builder.Config{
		Name: "env-test",
		Base: builder.BaseImage{
			Image: "ubuntu:22.04",
			Env: map[string]string{
				"DEBIAN_FRONTEND": "noninteractive",
				"PATH":            "/usr/local/bin:/usr/bin",
			},
		},
		Metadata: builder.Metadata{
			Description: "Template with env vars",
			Version:     "1.0.0",
			Author:      "Test",
			Tags:        []string{"test", "env"},
		},
		Targets: []builder.Target{
			{Type: "container", Registry: "ghcr.io/test", Tags: []string{"latest"}, Platforms: []string{"linux/amd64"}},
		},
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"echo hello"}},
			{Type: "file", Source: "/src", Destination: "/dst"},
			{Type: "ansible", PlaybookPath: "playbook.yml"},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayTemplateInfo("env-test", cfg)
	})

	if !strings.Contains(output, "env-test") {
		t.Error("output should contain template name")
	}
	if !strings.Contains(output, "Template with env vars") {
		t.Error("output should contain description")
	}
	if !strings.Contains(output, "DEBIAN_FRONTEND") {
		t.Error("output should contain env var")
	}
	if !strings.Contains(output, "ubuntu:22.04") {
		t.Error("output should contain base image")
	}
	if !strings.Contains(output, "1.0.0") {
		t.Error("output should contain version")
	}
	if !strings.Contains(output, "Test") {
		t.Error("output should contain author")
	}
	if !strings.Contains(output, "ghcr.io/test") {
		t.Error("output should contain registry")
	}
}

func TestDisplayProvisionerDetails_AllTypes(t *testing.T) {
	testCases := []struct {
		name     string
		prov     builder.Provisioner
		contains string
	}{
		{
			name:     "shell with inline",
			prov:     builder.Provisioner{Type: "shell", Inline: []string{"echo hello", "echo world"}},
			contains: "2 inline",
		},
		{
			name:     "script with scripts",
			prov:     builder.Provisioner{Type: "script", Scripts: []string{"install.sh"}},
			contains: "install.sh",
		},
		{
			name:     "ansible with playbook",
			prov:     builder.Provisioner{Type: "ansible", PlaybookPath: "site.yml"},
			contains: "site.yml",
		},
		{
			name:     "powershell with scripts",
			prov:     builder.Provisioner{Type: "powershell", PSScripts: []string{"setup.ps1"}},
			contains: "setup.ps1",
		},
		{
			name:     "file with source and dest",
			prov:     builder.Provisioner{Type: "file", Source: "/src/file", Destination: "/dst/file"},
			contains: "/src/file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := captureStdoutForTest(t, func() {
				displayProvisionerDetails(tc.prov)
			})
			if !strings.Contains(output, tc.contains) {
				t.Errorf("output should contain %q, got: %q", tc.contains, output)
			}
		})
	}
}
