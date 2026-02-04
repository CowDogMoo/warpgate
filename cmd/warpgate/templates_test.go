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
	"github.com/spf13/cobra"
)

func TestTemplatesCommandStructure(t *testing.T) {
	t.Parallel()

	if templatesCmd.Use != "templates" {
		t.Errorf("templatesCmd.Use = %q, want %q", templatesCmd.Use, "templates")
	}

	expectedSubcmds := []string{"add", "remove", "list", "search", "info", "update"}
	subCmds := templatesCmd.Commands()
	subCmdNames := make(map[string]bool)
	for _, c := range subCmds {
		subCmdNames[c.Name()] = true
	}

	for _, name := range expectedSubcmds {
		if !subCmdNames[name] {
			t.Errorf("missing templates subcommand: %s", name)
		}
	}
}

func TestTemplatesListFlags(t *testing.T) {
	t.Parallel()

	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"format", "f", "table"},
		{"source", "s", "all"},
		{"quiet", "q", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := templatesListCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("missing flag --%s on templates list command", tt.name)
			}
			if f.Shorthand != tt.shorthand {
				t.Errorf("flag --%s shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestValidTemplatesListFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format string
		valid  bool
	}{
		{"table", true},
		{"json", true},
		{"gha-matrix", true},
		{"csv", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run("format_"+tt.format, func(t *testing.T) {
			t.Parallel()
			if got := validTemplatesListFormats[tt.format]; got != tt.valid {
				t.Errorf("validTemplatesListFormats[%q] = %v, want %v", tt.format, got, tt.valid)
			}
		})
	}
}

func TestTemplatesAddArgsValidation(t *testing.T) {
	t.Parallel()

	err := cobra.RangeArgs(1, 2)(templatesAddCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	err = cobra.RangeArgs(1, 2)(templatesAddCmd, []string{"arg1"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}

	err = cobra.RangeArgs(1, 2)(templatesAddCmd, []string{"arg1", "arg2"})
	if err != nil {
		t.Errorf("expected no error for 2 args, got: %v", err)
	}

	err = cobra.RangeArgs(1, 2)(templatesAddCmd, []string{"a", "b", "c"})
	if err == nil {
		t.Error("expected error for 3 args")
	}
}

func TestTemplatesRemoveArgsValidation(t *testing.T) {
	t.Parallel()

	err := cobra.ExactArgs(1)(templatesRemoveCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	err = cobra.ExactArgs(1)(templatesRemoveCmd, []string{"source-name"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}
}

func TestTemplatesSearchArgsValidation(t *testing.T) {
	t.Parallel()

	err := cobra.ExactArgs(1)(templatesSearchCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	err = cobra.ExactArgs(1)(templatesSearchCmd, []string{"query"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}
}

func TestTemplatesInfoArgsValidation(t *testing.T) {
	t.Parallel()

	err := cobra.ExactArgs(1)(templatesInfoCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	err = cobra.ExactArgs(1)(templatesInfoCmd, []string{"template-name"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}
}

func TestParseTemplatesAddArgs_OneArg(t *testing.T) {
	t.Parallel()

	name, urlOrPath, err := parseTemplatesAddArgs([]string{"/path/to/templates"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "" {
		t.Errorf("name = %q, want empty", name)
	}
	if urlOrPath != "/path/to/templates" {
		t.Errorf("urlOrPath = %q, want %q", urlOrPath, "/path/to/templates")
	}
}

func TestParseTemplatesAddArgs_TwoArgs_GitURL(t *testing.T) {
	t.Parallel()

	name, urlOrPath, err := parseTemplatesAddArgs([]string{"my-name", "https://github.com/user/repo.git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "my-name" {
		t.Errorf("name = %q, want %q", name, "my-name")
	}
	if urlOrPath != "https://github.com/user/repo.git" {
		t.Errorf("urlOrPath = %q, want %q", urlOrPath, "https://github.com/user/repo.git")
	}
}

func TestParseTemplatesAddArgs_TwoArgs_LocalPath(t *testing.T) {
	t.Parallel()

	_, _, err := parseTemplatesAddArgs([]string{"my-name", "/local/path"})
	if err == nil {
		t.Fatal("expected error when providing name with local path")
	}
	if !strings.Contains(err.Error(), "must be a git URL") {
		t.Errorf("error should mention git URL, got: %v", err)
	}
}

func TestDisplayTemplateInfo_FullConfig(t *testing.T) {
	cfg := &builder.Config{
		Name: "full-template",
		Metadata: builder.Metadata{
			Description: "Full test description",
			Version:     "2.0.0",
			Author:      "Test Author",
			Tags:        []string{"security", "offensive"},
		},
		Base: builder.BaseImage{
			Image: "ubuntu:22.04",
			Env: map[string]string{
				"PATH": "/usr/bin",
			},
		},
		Targets: []builder.Target{
			{
				Type:      "container",
				Registry:  "ghcr.io/test",
				Tags:      []string{"latest"},
				Platforms: []string{"linux/amd64"},
			},
		},
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"echo hello"}},
			{Type: "file", Source: "/src", Destination: "/dst"},
			{Type: "ansible", PlaybookPath: "playbook.yml"},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayTemplateInfo("full-template", cfg)
	})

	expectedStrings := []string{
		"full-template",
		"Full test description",
		"2.0.0",
		"Test Author",
		"ubuntu:22.04",
		"container",
		"shell",
		"file",
		"ansible",
		"Environment Variables",
		"PATH",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(output, s) {
			t.Errorf("output should contain %q", s)
		}
	}
}

func TestDisplayTemplateInfo_MinimalConfig(t *testing.T) {
	cfg := &builder.Config{
		Name: "minimal",
	}

	output := captureStdoutForTest(t, func() {
		displayTemplateInfo("minimal", cfg)
	})

	if !strings.Contains(output, "minimal") {
		t.Error("output should contain template name")
	}
	// Should not contain sections for empty data
	if strings.Contains(output, "Environment Variables") {
		t.Error("output should not contain Environment Variables for empty config")
	}
}

func TestDisplayProvisionerDetails_PowerShell(t *testing.T) {
	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{"setup.ps1", "install.ps1"},
	}
	output := captureStdoutForTest(t, func() {
		displayProvisionerDetails(prov)
	})
	if !strings.Contains(output, "setup.ps1") {
		t.Error("output should contain PowerShell script name")
	}
}

func TestDisplayTargets_MultipleTargets(t *testing.T) {
	cfg := &builder.Config{
		Targets: []builder.Target{
			{Type: "container", Registry: "ghcr.io/test", Tags: []string{"latest"}, Platforms: []string{"linux/amd64"}},
			{Type: "ami"},
		},
	}
	output := captureStdoutForTest(t, func() {
		displayTargets(cfg)
	})
	if !strings.Contains(output, "container") {
		t.Error("output should contain container target")
	}
	if !strings.Contains(output, "ami") {
		t.Error("output should contain ami target")
	}
}
