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

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/pkg/templates"
)

func TestNewOutputFormatter(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{
			name:   "text format",
			format: "text",
			want:   "text",
		},
		{
			name:   "json format",
			format: "json",
			want:   "json",
		},
		{
			name:   "table format",
			format: "table",
			want:   "table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewOutputFormatter(tt.format)
			if formatter.format != tt.want {
				t.Errorf("NewOutputFormatter() format = %v, want %v", formatter.format, tt.want)
			}
		})
	}
}

func TestDisplayTemplateList_Table(t *testing.T) {
	formatter := NewOutputFormatter("table")
	templateList := []templates.TemplateInfo{
		{
			Name:        "attack-box",
			Version:     "1.0.0",
			Repository:  "official",
			Description: "Security testing container",
			Author:      "team",
		},
		{
			Name:        "sliver",
			Version:     "2.0.0",
			Repository:  "local:/path/to/templates",
			Description: "C2 framework container",
			Author:      "security-team",
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := formatter.DisplayTemplateList(templateList)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("DisplayTemplateList() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected content
	if !strings.Contains(output, "attack-box") {
		t.Errorf("DisplayTemplateList() output missing 'attack-box'")
	}
	if !strings.Contains(output, "sliver") {
		t.Errorf("DisplayTemplateList() output missing 'sliver'")
	}
	if !strings.Contains(output, "Total templates: 2") {
		t.Errorf("DisplayTemplateList() output missing total count")
	}
}

func TestDisplayTemplateList_JSON(t *testing.T) {
	formatter := NewOutputFormatter("json")
	templateList := []templates.TemplateInfo{
		{
			Name:        "attack-box",
			Version:     "1.0.0",
			Repository:  "official",
			Description: "Security testing container",
			Author:      "team",
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := formatter.DisplayTemplateList(templateList)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("DisplayTemplateList() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify JSON is valid
	var result []templates.TemplateInfo
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("DisplayTemplateList() output is not valid JSON: %v", err)
	}

	// Verify content
	if len(result) != 1 {
		t.Errorf("DisplayTemplateList() got %d templates, want 1", len(result))
	}
	if result[0].Name != "attack-box" {
		t.Errorf("DisplayTemplateList() template name = %v, want attack-box", result[0].Name)
	}
}

func TestDisplayTemplateList_GHAMatrix(t *testing.T) {
	formatter := NewOutputFormatter("gha-matrix")
	templateList := []templates.TemplateInfo{
		{
			Name:        "attack-box",
			Version:     "1.0.0",
			Repository:  "official",
			Description: "Security testing container",
		},
		{
			Name:        "sliver",
			Version:     "2.0.0",
			Repository:  "local:custom",
			Description: "C2 framework",
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := formatter.DisplayTemplateList(templateList)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("DisplayTemplateList() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify JSON is valid
	var result GHAMatrix
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("DisplayTemplateList() output is not valid JSON: %v", err)
	}

	// Verify structure
	if len(result.Template) != 2 {
		t.Errorf("DisplayTemplateList() got %d templates, want 2", len(result.Template))
	}

	// Verify first template
	if result.Template[0].Name != "attack-box" {
		t.Errorf("DisplayTemplateList() first template name = %v, want attack-box", result.Template[0].Name)
	}

	// Verify second template
	if result.Template[1].Name != "sliver" {
		t.Errorf("DisplayTemplateList() second template name = %v, want sliver", result.Template[1].Name)
	}
	if result.Template[1].Namespace != "custom" {
		t.Errorf("DisplayTemplateList() second template namespace = %v, want custom", result.Template[1].Namespace)
	}
}

func TestDisplayTemplateList_InvalidFormat(t *testing.T) {
	formatter := NewOutputFormatter("invalid-format")
	templateList := []templates.TemplateInfo{
		{
			Name:       "attack-box",
			Version:    "1.0.0",
			Repository: "official",
		},
	}

	err := formatter.DisplayTemplateList(templateList)
	if err == nil {
		t.Error("DisplayTemplateList() expected error for invalid format, got nil")
	}
}

func TestExtractNamespace(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		want       string
	}{
		{
			name:       "local repository with path",
			repository: "local:/path/to/templates",
			want:       "/path/to/templates",
		},
		{
			name:       "local repository simple",
			repository: "local:mytemplates",
			want:       "mytemplates",
		},
		{
			name:       "git repository",
			repository: "https://github.com/user/templates.git",
			want:       "cowdogmoo",
		},
		{
			name:       "official repository",
			repository: "official",
			want:       "cowdogmoo",
		},
		{
			name:       "empty repository",
			repository: "",
			want:       "cowdogmoo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractNamespace(tt.repository); got != tt.want {
				t.Errorf("extractNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsProvisionRepo(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		want         bool
	}{
		{
			name:         "attack-box needs provision repo",
			templateName: "attack-box",
			want:         true,
		},
		{
			name:         "atomic-red-team needs provision repo",
			templateName: "atomic-red-team",
			want:         true,
		},
		{
			name:         "sliver needs provision repo",
			templateName: "sliver",
			want:         true,
		},
		{
			name:         "ttpforge needs provision repo",
			templateName: "ttpforge",
			want:         true,
		},
		{
			name:         "custom template does not need provision repo",
			templateName: "custom-template",
			want:         false,
		},
		{
			name:         "empty template name",
			templateName: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsProvisionRepo(tt.templateName); got != tt.want {
				t.Errorf("needsProvisionRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisplayTemplateSearchResults(t *testing.T) {
	formatter := NewOutputFormatter("text")
	results := []templates.TemplateInfo{
		{
			Name:        "attack-box",
			Version:     "1.0.0",
			Repository:  "official",
			Description: "Security testing container",
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := formatter.DisplayTemplateSearchResults(results, "attack")

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("DisplayTemplateSearchResults() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected content
	if !strings.Contains(output, "attack-box") {
		t.Errorf("DisplayTemplateSearchResults() output missing 'attack-box'")
	}
	if !strings.Contains(output, "Found 1 template(s)") {
		t.Errorf("DisplayTemplateSearchResults() output missing found count")
	}
	if !strings.Contains(output, "query: attack") {
		t.Errorf("DisplayTemplateSearchResults() output missing query")
	}
}

func TestDisplayTemplateList_EmptyList(t *testing.T) {
	formatter := NewOutputFormatter("table")
	templateList := []templates.TemplateInfo{}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := formatter.DisplayTemplateList(templateList)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("DisplayTemplateList() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains total count
	if !strings.Contains(output, "Total templates: 0") {
		t.Errorf("DisplayTemplateList() output missing total count for empty list")
	}
}

func TestDisplayTemplateList_LongDescription(t *testing.T) {
	formatter := NewOutputFormatter("table")
	templateList := []templates.TemplateInfo{
		{
			Name:        "test-template",
			Version:     "1.0.0",
			Repository:  "official",
			Description: "This is a very long description that should be truncated in the table output to avoid making the table too wide",
			Author:      "team",
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := formatter.DisplayTemplateList(templateList)

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Errorf("DisplayTemplateList() error = %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify description is truncated (should contain "...")
	if !strings.Contains(output, "...") {
		t.Errorf("DisplayTemplateList() long description not truncated")
	}
}
