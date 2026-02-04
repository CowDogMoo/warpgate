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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTemplateLoader(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, loader)
	assert.NotEmpty(t, loader.cacheDir)
	assert.NotNil(t, loader.registry)
	assert.NotNil(t, loader.configLoad)
	assert.NotNil(t, loader.gitOps)
}

func TestParseTemplateRef(t *testing.T) {
	tests := []struct {
		name            string
		ref             string
		expectedName    string
		expectedVersion string
	}{
		{
			name:            "template without version",
			ref:             "attack-box",
			expectedName:    "attack-box",
			expectedVersion: "",
		},
		{
			name:            "template with version",
			ref:             "attack-box@v1.2.0",
			expectedName:    "attack-box",
			expectedVersion: "v1.2.0",
		},
		{
			name:            "template with complex name",
			ref:             "my-template-name@v2.0.0-beta",
			expectedName:    "my-template-name",
			expectedVersion: "v2.0.0-beta",
		},
		{
			name:            "simple name",
			ref:             "sliver",
			expectedName:    "sliver",
			expectedVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := parseTemplateRef(tt.ref)
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedVersion, version)
		})
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte("test"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing file",
			path:     tmpFile,
			expected: true,
		},
		{
			name:     "existing directory",
			path:     tmpDir,
			expected: true,
		},
		{
			name:     "non-existent file",
			path:     filepath.Join(tmpDir, "does-not-exist.txt"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileExists(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplateLoader_LoadTemplate_LocalFile(t *testing.T) {
	// Create a minimal warpgate.yaml for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "warpgate.yaml")

	content := `metadata:
  name: test-template
  version: 1.0.0
  description: Test template
  author: Test Author
  license: MIT
  requires:
    warpgate: ">=1.0.0"

name: test-template
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo "test"
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`

	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// Test loading from local file (absolute path)
	cfg, err := loader.LoadTemplateWithVars(context.Background(), configPath, nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "test-template", cfg.Name)
	assert.Equal(t, "Test template", cfg.Metadata.Description)
}

func TestTemplateLoader_LoadTemplate_InvalidFile(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// Test loading from non-existent file
	_, err = loader.LoadTemplateWithVars(context.Background(), "/non/existent/path/warpgate.yaml", nil)
	assert.Error(t, err)
}

func TestTemplateLoader_LoadTemplate_ReferenceTypes(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "absolute path",
			ref:      "/absolute/path/to/template.yaml",
			expected: "absolute",
		},
		{
			name:     "template name only",
			ref:      "attack-box",
			expected: "registry",
		},
		{
			name:     "template with version",
			ref:      "attack-box@v1.0.0",
			expected: "registry",
		},
		{
			name:     "https git url",
			ref:      "https://git.example.com/jdoe/repo.git",
			expected: "git",
		},
		{
			name:     "git ssh url",
			ref:      "git@github.com:user/repo.git",
			expected: "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Classify the reference type based on the logic in LoadTemplate
			var refType string
			switch {
			case filepath.IsAbs(tt.ref):
				refType = "absolute"
			case !filepath.IsAbs(tt.ref) && fileExists(tt.ref):
				refType = "local"
			case strings.Contains(tt.ref, "https://") || strings.Contains(tt.ref, "git@"):
				refType = "git"
			default:
				refType = "registry"
			}

			assert.Equal(t, tt.expected, refType)
		})
	}
}

func TestTemplateLoader_CacheDirectory(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// Verify cache directory exists
	info, err := os.Stat(loader.cacheDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify cache directory is set (exact path may vary based on global config)
	assert.NotEmpty(t, loader.cacheDir)
	assert.Contains(t, loader.cacheDir, "warpgate")
}

func TestSetVariables_NilMap(t *testing.T) {
	t.Parallel()

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// Set some initial variables
	loader.SetVariables(map[string]string{"key": "value"})
	assert.Len(t, loader.variables, 1)

	// Setting nil should reset to empty map, not nil
	loader.SetVariables(nil)
	assert.NotNil(t, loader.variables)
	assert.Empty(t, loader.variables)
}

func TestSetVariables_Merge(t *testing.T) {
	t.Parallel()

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	vars := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	loader.SetVariables(vars)

	assert.Equal(t, "value1", loader.variables["key1"])
	assert.Equal(t, "value2", loader.variables["key2"])
}

func TestLoadTemplateWithVars_DirectoryPath(t *testing.T) {
	t.Parallel()

	// Create a temp directory with warpgate.yaml inside
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "warpgate.yaml")

	content := `metadata:
  name: dir-template
  version: 1.0.0
  description: Directory template test
  author: Test Author
  license: MIT
  requires:
    warpgate: ">=1.0.0"

name: dir-template
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo "test"
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// Load by directory path (should find warpgate.yaml inside)
	cfg, err := loader.LoadTemplateWithVars(context.Background(), tmpDir, nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "dir-template", cfg.Name)
}

func TestLoadTemplateWithVars_DirectoryWithoutConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// Load a directory that has no warpgate.yaml
	_, err = loader.LoadTemplateWithVars(context.Background(), tmpDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no warpgate.yaml found")
}

func TestParseTemplateRef_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		ref             string
		expectedName    string
		expectedVersion string
	}{
		{
			name:            "empty string",
			ref:             "",
			expectedName:    "",
			expectedVersion: "",
		},
		{
			name:            "just an at sign",
			ref:             "@",
			expectedName:    "",
			expectedVersion: "",
		},
		{
			name:            "multiple at signs",
			ref:             "name@v1@extra",
			expectedName:    "name",
			expectedVersion: "v1",
		},
		{
			name:            "version only",
			ref:             "@v1.0.0",
			expectedName:    "",
			expectedVersion: "v1.0.0",
		},
		{
			name:            "name with dots",
			ref:             "my.template@v2.0",
			expectedName:    "my.template",
			expectedVersion: "v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := parseTemplateRef(tt.ref)
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedVersion, version)
		})
	}
}

func TestLoadTemplateWithVars_UnknownReference(t *testing.T) {
	t.Parallel()

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// A template name that doesn't exist in any repo
	_, err = loader.LoadTemplateWithVars(context.Background(), "nonexistent-template-xyz", nil)
	assert.Error(t, err)
}

func TestTemplateLoader_LoadFromFile_Validation(t *testing.T) {
	// Create an invalid warpgate.yaml
	tmpDir := t.TempDir()
	invalidConfig := filepath.Join(tmpDir, "invalid.yaml")

	content := `metadata:
  name: test
  version: 1.0.0
  description: Test
  author: Test
  license: MIT
  requires:
    warpgate: ">=1.0.0"
name: test
# Missing required fields like base, provisioners, targets
`

	err := os.WriteFile(invalidConfig, []byte(content), 0644)
	require.NoError(t, err)

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	// Should fail validation
	_, err = loader.loadFromFileWithVars(context.Background(), invalidConfig, nil)
	assert.Error(t, err)
}
