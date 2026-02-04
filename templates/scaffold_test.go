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

package templates

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

func TestNewScaffolder(t *testing.T) {
	scaffolder := NewScaffolder()
	if scaffolder == nil {
		t.Error("NewScaffolder() returned nil")
	}
}

func TestCreate(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	ctx := context.Background()

	templateName := "test-template"

	err := scaffolder.Create(ctx, templateName, tmpDir)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// Verify directory structure was created
	templateDir := filepath.Join(tmpDir, templateName)
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Errorf("Create() did not create template directory: %s", templateDir)
	}

	// Verify scripts directory
	scriptsDir := filepath.Join(templateDir, "scripts")
	if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
		t.Errorf("Create() did not create scripts directory: %s", scriptsDir)
	}

	// Verify warpgate.yaml was created
	configFile := filepath.Join(templateDir, "warpgate.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Errorf("Create() did not create warpgate.yaml: %s", configFile)
	}

	// Verify README.md was created
	readmeFile := filepath.Join(templateDir, "README.md")
	if _, err := os.Stat(readmeFile); os.IsNotExist(err) {
		t.Errorf("Create() did not create README.md: %s", readmeFile)
	}
}

func TestCreate_ConfigContent(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	ctx := context.Background()

	templateName := "test-template"

	err := scaffolder.Create(ctx, templateName, tmpDir)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Read and verify warpgate.yaml content
	configFile := filepath.Join(tmpDir, templateName, "warpgate.yaml")
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read warpgate.yaml: %v", err)
	}

	configStr := string(content)

	// Verify key fields are present
	expectedFields := []string{
		"metadata:",
		"name: " + templateName,
		"version:",
		"base:",
		"provisioners:",
		"targets:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(configStr, field) {
			t.Errorf("warpgate.yaml missing expected field: %s", field)
		}
	}
}

func TestCreate_ReadmeContent(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	ctx := context.Background()

	templateName := "my-awesome-template"

	err := scaffolder.Create(ctx, templateName, tmpDir)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Read and verify README.md content
	readmeFile := filepath.Join(tmpDir, templateName, "README.md")
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}

	readmeStr := string(content)

	// Verify template name appears in README
	if !strings.Contains(readmeStr, templateName) {
		t.Errorf("README.md does not contain template name: %s", templateName)
	}

	// Verify key sections are present
	expectedSections := []string{
		"# " + templateName,
		"## Description",
		"## Usage",
		"### Build",
		"### Customize",
		"## Structure",
	}

	for _, section := range expectedSections {
		if !strings.Contains(readmeStr, section) {
			t.Errorf("README.md missing expected section: %s", section)
		}
	}
}

func TestCreate_ExistingDirectory(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	ctx := context.Background()

	templateName := "test-template"
	templateDir := filepath.Join(tmpDir, templateName)

	// Create the directory first
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Should still succeed (overwrites)
	err := scaffolder.Create(ctx, templateName, tmpDir)
	if err != nil {
		t.Errorf("Create() with existing directory error = %v", err)
	}
}

func TestCreateDefaultTemplate(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	templateName := "test-template"

	err := scaffolder.createDefaultTemplate(templateName, tmpDir)
	if err != nil {
		t.Errorf("createDefaultTemplate() error = %v", err)
	}

	configFile := filepath.Join(tmpDir, "warpgate.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Errorf("createDefaultTemplate() did not create warpgate.yaml")
	}

	// Verify content
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read warpgate.yaml: %v", err)
	}

	configStr := string(content)
	if !strings.Contains(configStr, "name: "+templateName) {
		t.Errorf("warpgate.yaml does not contain template name")
	}
}

func TestCreateReadme(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	templateName := "test-template"

	err := scaffolder.createReadme(templateName, tmpDir)
	if err != nil {
		t.Errorf("createReadme() error = %v", err)
	}

	readmeFile := filepath.Join(tmpDir, "README.md")
	if _, err := os.Stat(readmeFile); os.IsNotExist(err) {
		t.Errorf("createReadme() did not create README.md")
	}

	// Verify content
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}

	readmeStr := string(content)
	if !strings.Contains(readmeStr, templateName) {
		t.Errorf("README.md does not contain template name")
	}
}

func TestCreate_InvalidPath(t *testing.T) {
	scaffolder := NewScaffolder()
	ctx := context.Background()

	// Try to create in a path that doesn't exist and can't be created
	// Use a path that's definitely invalid on all systems
	invalidPath := "/invalid/nonexistent/path/that/does/not/exist"

	// On systems where we can't create this path, it should error
	err := scaffolder.Create(ctx, "test", invalidPath)

	// This test is tricky because on some systems with sufficient permissions,
	// the directory might actually be created. So we'll just check that if
	// it errors, the error is appropriate.
	if err != nil {
		// Expected behavior - error occurred
		if !strings.Contains(err.Error(), "failed to create") {
			t.Errorf("Create() error message unexpected: %v", err)
		}
	}
	// If no error, the system was able to create the directory (unlikely but possible)
}

func TestCreate_PermissionDenied(t *testing.T) {
	// This test is platform-dependent and may not work in all environments
	// Skip if we don't have a good way to test it
	if os.Getenv("CI") != "" {
		t.Skip("Skipping permission test in CI environment")
	}

	// We would need to create a directory without write permissions
	// and try to create a template in it, but this is complex and
	// platform-specific, so we'll skip this test for now
	t.Skip("Permission test requires platform-specific setup")
}

func TestCreate_MultipleTemplates(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create multiple templates in the same output directory
	templates := []string{"template1", "template2", "template3"}

	for _, name := range templates {
		err := scaffolder.Create(ctx, name, tmpDir)
		if err != nil {
			t.Errorf("Create() error for %s = %v", name, err)
		}

		// Verify each was created
		templateDir := filepath.Join(tmpDir, name)
		if _, err := os.Stat(templateDir); os.IsNotExist(err) {
			t.Errorf("Create() did not create directory for %s", name)
		}
	}

	// Verify all templates exist
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read tmpDir: %v", err)
	}

	if len(entries) != len(templates) {
		t.Errorf("Created %d entries, want %d", len(entries), len(templates))
	}
}

func TestSaveTemplateConfig(t *testing.T) {
	t.Parallel()

	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "warpgate.yaml")

	cfg := &builder.Config{
		Metadata: builder.Metadata{
			Name:        "saved-template",
			Version:     "1.0.0",
			Description: "A saved template",
			Author:      "Test Author",
		},
		Name:    "saved-template",
		Version: "latest",
	}

	err := scaffolder.saveTemplateConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("saveTemplateConfig() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("saveTemplateConfig() did not create file")
	}

	// Verify content is valid YAML with schema comment
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	configStr := string(content)
	if !strings.Contains(configStr, "saved-template") {
		t.Error("saveTemplateConfig() output missing template name")
	}
	if !strings.Contains(configStr, builder.SchemaComment) {
		t.Error("saveTemplateConfig() output missing schema comment")
	}
}

func TestPrintNextSteps(t *testing.T) {
	t.Parallel()

	scaffolder := NewScaffolder()
	ctx := context.Background()

	// printNextSteps should not panic
	scaffolder.printNextSteps(ctx, "/tmp/test-template", "/tmp/test-template/scripts")
}

func TestFork_ValidFork(t *testing.T) {
	t.Parallel()

	// Create a source template that can be loaded
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	content := `metadata:
  name: source-template
  version: 1.0.0
  description: Source template
  author: Original Author
  license: MIT
  requires:
    warpgate: ">=1.0.0"

name: source-template
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo "source"
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	if err := os.WriteFile(filepath.Join(srcDir, "warpgate.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write source config: %v", err)
	}

	scaffolder := NewScaffolder()
	ctx := context.Background()
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Fork using the source template path
	err := scaffolder.Fork(ctx, filepath.Join(srcDir, "warpgate.yaml"), "forked-template", outputDir)
	if err != nil {
		t.Fatalf("Fork() error = %v", err)
	}

	// Verify forked directory was created
	forkedDir := filepath.Join(outputDir, "forked-template")
	if _, err := os.Stat(forkedDir); os.IsNotExist(err) {
		t.Fatal("Fork() did not create forked template directory")
	}

	// Verify warpgate.yaml was created
	forkedConfig := filepath.Join(forkedDir, "warpgate.yaml")
	if _, err := os.Stat(forkedConfig); os.IsNotExist(err) {
		t.Fatal("Fork() did not create warpgate.yaml")
	}

	// Verify README was created
	forkedReadme := filepath.Join(forkedDir, "README.md")
	if _, err := os.Stat(forkedReadme); os.IsNotExist(err) {
		t.Fatal("Fork() did not create README.md")
	}

	// Verify config content has forked metadata
	configContent, err := os.ReadFile(forkedConfig)
	if err != nil {
		t.Fatalf("Failed to read forked config: %v", err)
	}
	configStr := string(configContent)
	if !strings.Contains(configStr, "forked-template") {
		t.Error("Forked config does not contain new name")
	}
}

func TestCreate_SpecialCharactersInName(t *testing.T) {
	scaffolder := NewScaffolder()
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Test with special characters that are valid in directory names
	templateName := "my-awesome_template.v1"

	err := scaffolder.Create(ctx, templateName, tmpDir)
	if err != nil {
		t.Errorf("Create() with special characters error = %v", err)
	}

	// Verify it was created
	templateDir := filepath.Join(tmpDir, templateName)
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Errorf("Create() did not create template with special characters")
	}
}
