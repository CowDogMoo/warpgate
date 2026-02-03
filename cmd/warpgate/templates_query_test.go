/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

// captureStdout redirects os.Stdout to capture output from functions that
// use fmt.Print* directly. Tests using this helper must NOT use t.Parallel()
// since os.Stdout is a global.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return ""
	}
	return buf.String()
}

func TestDisplayTemplateInfo_Full(t *testing.T) {
	cfg := &builder.Config{
		Name: "test-image",
		Metadata: builder.Metadata{
			Description: "A test template",
			Version:     "1.0.0",
			Author:      "Test Author",
			Tags:        []string{"test", "security"},
		},
		Base: builder.BaseImage{
			Image: "ubuntu:22.04",
			Env:   map[string]string{"KEY": "value"},
		},
		Targets: []builder.Target{
			{
				Type:      "container",
				Registry:  "ghcr.io/test",
				Tags:      []string{"latest", "v1.0"},
				Platforms: []string{"linux/amd64", "linux/arm64"},
			},
		},
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"apt-get update", "apt-get install -y curl"}},
			{Type: "ansible", PlaybookPath: "/path/to/playbook.yml"},
			{Type: "script", Scripts: []string{"setup.sh", "install.sh"}},
			{Type: "file", Source: "/local/file", Destination: "/remote/file"},
			{Type: "powershell", PSScripts: []string{"setup.ps1"}},
		},
	}

	output := captureStdout(t, func() {
		displayTemplateInfo("my-template", cfg)
	})

	expectations := []string{
		"Template: my-template",
		"A test template",
		"Version: 1.0.0",
		"Author: Test Author",
		"Tags:",
		"Name: test-image",
		"Base Image: ubuntu:22.04",
		"container",
		"ghcr.io/test",
		"Provisioners (5)",
		"shell",
		"2 inline",
		"ansible",
		"Playbook:",
		"script",
		"setup.sh",
		"file",
		"/local/file",
		"/remote/file",
		"powershell",
		"setup.ps1",
		"Environment Variables",
		"KEY: value",
	}

	for _, expected := range expectations {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing %q", expected)
		}
	}
}

func TestDisplayTemplateInfo_Minimal(t *testing.T) {
	cfg := &builder.Config{
		Name: "minimal",
	}

	output := captureStdout(t, func() {
		displayTemplateInfo("minimal", cfg)
	})

	if !strings.Contains(output, "Template: minimal") {
		t.Error("output missing template name")
	}
	if !strings.Contains(output, "Name: minimal") {
		t.Error("output missing build config name")
	}
	if strings.Contains(output, "Description:") {
		t.Error("output should not contain description for minimal config")
	}
	if strings.Contains(output, "Targets:") {
		t.Error("output should not contain targets for minimal config")
	}
	if strings.Contains(output, "Provisioners") {
		t.Error("output should not contain provisioners for minimal config")
	}
	if strings.Contains(output, "Environment Variables") {
		t.Error("output should not contain env vars for minimal config")
	}
}

func TestDisplayMetadata(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		cfg := &builder.Config{
			Metadata: builder.Metadata{
				Description: "My description",
				Version:     "2.0.0",
				Author:      "Jane Doe",
				Tags:        []string{"a", "b"},
			},
		}
		output := captureStdout(t, func() { displayMetadata(cfg) })
		if !strings.Contains(output, "My description") {
			t.Error("missing description")
		}
		if !strings.Contains(output, "2.0.0") {
			t.Error("missing version")
		}
		if !strings.Contains(output, "Jane Doe") {
			t.Error("missing author")
		}
	})

	t.Run("empty metadata", func(t *testing.T) {
		cfg := &builder.Config{}
		output := captureStdout(t, func() { displayMetadata(cfg) })
		if strings.Contains(output, "Description:") {
			t.Error("should not have description")
		}
	})
}

func TestDisplayBuildConfig(t *testing.T) {
	t.Run("with base image", func(t *testing.T) {
		cfg := &builder.Config{
			Name: "my-image",
			Base: builder.BaseImage{Image: "debian:12"},
		}
		output := captureStdout(t, func() { displayBuildConfig(cfg) })
		if !strings.Contains(output, "my-image") {
			t.Error("missing name")
		}
		if !strings.Contains(output, "debian:12") {
			t.Error("missing base image")
		}
	})

	t.Run("without base image", func(t *testing.T) {
		cfg := &builder.Config{Name: "my-image"}
		output := captureStdout(t, func() { displayBuildConfig(cfg) })
		if strings.Contains(output, "Base Image:") {
			t.Error("should not show base image if empty")
		}
	})
}

func TestDisplayTargets(t *testing.T) {
	t.Run("no targets", func(t *testing.T) {
		cfg := &builder.Config{}
		output := captureStdout(t, func() { displayTargets(cfg) })
		if output != "" {
			t.Error("should output nothing for no targets")
		}
	})

	t.Run("with targets", func(t *testing.T) {
		cfg := &builder.Config{
			Targets: []builder.Target{
				{
					Type:      "container",
					Registry:  "docker.io/test",
					Tags:      []string{"latest"},
					Platforms: []string{"linux/amd64"},
				},
			},
		}
		output := captureStdout(t, func() { displayTargets(cfg) })
		if !strings.Contains(output, "container") {
			t.Error("missing target type")
		}
		if !strings.Contains(output, "docker.io/test") {
			t.Error("missing registry")
		}
	})
}

func TestDisplayProvisioners(t *testing.T) {
	t.Run("no provisioners", func(t *testing.T) {
		cfg := &builder.Config{}
		output := captureStdout(t, func() { displayProvisioners(cfg) })
		if output != "" {
			t.Error("should output nothing for no provisioners")
		}
	})

	t.Run("with provisioners", func(t *testing.T) {
		cfg := &builder.Config{
			Provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo hello"}},
			},
		}
		output := captureStdout(t, func() { displayProvisioners(cfg) })
		if !strings.Contains(output, "Provisioners (1)") {
			t.Error("missing provisioner count")
		}
		if !strings.Contains(output, "shell") {
			t.Error("missing provisioner type")
		}
	})
}

func TestDisplayProvisionerDetails(t *testing.T) {
	tests := []struct {
		name     string
		prov     builder.Provisioner
		contains string
	}{
		{"shell", builder.Provisioner{Type: "shell", Inline: []string{"a", "b"}}, "2 inline"},
		{"script", builder.Provisioner{Type: "script", Scripts: []string{"s.sh"}}, "s.sh"},
		{"ansible", builder.Provisioner{Type: "ansible", PlaybookPath: "/p.yml"}, "/p.yml"},
		{"powershell", builder.Provisioner{Type: "powershell", PSScripts: []string{"s.ps1"}}, "s.ps1"},
		{"file", builder.Provisioner{Type: "file", Source: "/src", Destination: "/dst"}, "/src -> /dst"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() { displayProvisionerDetails(tt.prov) })
			if !strings.Contains(output, tt.contains) {
				t.Errorf("output %q missing %q", output, tt.contains)
			}
		})
	}
}

func TestDisplayEnvironmentVars(t *testing.T) {
	t.Run("no env vars", func(t *testing.T) {
		cfg := &builder.Config{}
		output := captureStdout(t, func() { displayEnvironmentVars(cfg) })
		if output != "" {
			t.Error("should output nothing for no env vars")
		}
	})

	t.Run("with env vars", func(t *testing.T) {
		cfg := &builder.Config{
			Base: builder.BaseImage{
				Env: map[string]string{"FOO": "bar", "BAZ": "qux"},
			},
		}
		output := captureStdout(t, func() { displayEnvironmentVars(cfg) })
		if !strings.Contains(output, "FOO: bar") {
			t.Error("missing FOO env var")
		}
		if !strings.Contains(output, "BAZ: qux") {
			t.Error("missing BAZ env var")
		}
	})
}

func TestRunTemplatesList_InvalidFormat(t *testing.T) {
	oldFormat := templatesListFormat
	defer func() { templatesListFormat = oldFormat }()
	templatesListFormat = "invalid-format"

	cmd := newTestCmd(nil)

	err := runTemplatesList(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error should mention invalid format, got: %v", err)
	}
}
