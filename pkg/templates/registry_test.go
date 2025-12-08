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
	"os"
	"path/filepath"
	"testing"
)

func TestNewTemplateRegistry(t *testing.T) {
	// Clear any config-related env vars to ensure clean test environment
	originalLocalPaths := os.Getenv("WARPGATE_TEMPLATES_LOCAL_PATHS")
	os.Unsetenv("WARPGATE_TEMPLATES_LOCAL_PATHS")
	defer func() {
		if originalLocalPaths != "" {
			os.Setenv("WARPGATE_TEMPLATES_LOCAL_PATHS", originalLocalPaths)
		}
	}()

	registry, err := NewTemplateRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	if registry == nil {
		t.Fatal("Registry is nil")
	}

	if registry.cacheDir == "" {
		t.Error("Cache directory not set")
	}

	// Check that official repo is registered by default when no config exists
	// Note: If user has a config file with custom repositories, this test may fail
	// The registry should only add "official" if no repos or local paths are configured
	repos := registry.GetRepositories()
	if len(repos) == 0 && len(registry.localPaths) == 0 {
		t.Error("Expected at least one repository or local path to be configured")
	}
}

func TestTemplateRegistryAddRemoveRepository(t *testing.T) {
	registry, err := NewTemplateRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Add a repository
	testName := "test-repo"
	testURL := "https://github.com/test/repo.git"
	registry.AddRepository(testName, testURL)

	// Verify it was added
	repos := registry.GetRepositories()
	if url, ok := repos[testName]; !ok {
		t.Error("Repository not added")
	} else if url != testURL {
		t.Errorf("Expected URL %s, got %s", testURL, url)
	}

	// Remove the repository
	registry.RemoveRepository(testName)

	// Verify it was removed
	repos = registry.GetRepositories()
	if _, ok := repos[testName]; ok {
		t.Error("Repository not removed")
	}
}

func TestTemplateRegistrySaveLoadRepositories(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create registry with temp cache dir
	registry := &TemplateRegistry{
		repos: map[string]string{
			"official": "https://github.com/cowdogmoo/warpgate-templates.git",
			"test":     "https://github.com/test/repo.git",
		},
		cacheDir: tempDir,
	}

	// Save repositories
	if err := registry.SaveRepositories(); err != nil {
		t.Fatalf("Failed to save repositories: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tempDir, "repositories.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Repository config file was not created")
	}

	// Create new registry and load
	newRegistry := &TemplateRegistry{
		repos: map[string]string{
			"official": "https://github.com/cowdogmoo/warpgate-templates.git",
		},
		cacheDir: tempDir,
	}

	if err := newRegistry.LoadRepositories(); err != nil {
		t.Fatalf("Failed to load repositories: %v", err)
	}

	// Verify loaded repositories
	repos := newRegistry.GetRepositories()
	if url, ok := repos["test"]; !ok {
		t.Error("Test repository not loaded")
	} else if url != "https://github.com/test/repo.git" {
		t.Errorf("Expected URL https://github.com/test/repo.git, got %s", url)
	}
}

func TestTemplateRegistryMatchesQuery(t *testing.T) {
	registry := &TemplateRegistry{}

	tests := []struct {
		name     string
		template TemplateInfo
		query    string
		expected bool
	}{
		{
			name: "exact name match",
			template: TemplateInfo{
				Name:        "attack-box",
				Description: "Security testing container",
				Tags:        []string{"security", "testing"},
			},
			query:    "attack",
			expected: true,
		},
		{
			name: "description match",
			template: TemplateInfo{
				Name:        "sliver",
				Description: "Command and control framework",
				Tags:        []string{"c2", "security"},
			},
			query:    "command",
			expected: true,
		},
		{
			name: "tag match",
			template: TemplateInfo{
				Name:        "atomic-red-team",
				Description: "Adversary simulation",
				Tags:        []string{"testing", "security", "red-team"},
			},
			query:    "red-team",
			expected: true,
		},
		{
			name: "fuzzy name match",
			template: TemplateInfo{
				Name:        "attack-box",
				Description: "Security testing",
				Tags:        []string{"security"},
			},
			query:    "atackbox", // typo
			expected: true,
		},
		{
			name: "no match",
			template: TemplateInfo{
				Name:        "sliver",
				Description: "C2 framework",
				Tags:        []string{"c2"},
			},
			query:    "kubernetes",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.matchesQuery(tt.template, tt.query)
			if result != tt.expected {
				t.Errorf("matchesQuery() = %v, want %v for query %q", result, tt.expected, tt.query)
			}
		})
	}
}

func TestTemplateRegistryCacheMetadata(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	registry := &TemplateRegistry{
		repos: map[string]string{
			"official": "https://github.com/cowdogmoo/warpgate-templates.git",
		},
		cacheDir: tempDir,
	}

	// Create test template data
	templates := []TemplateInfo{
		{
			Name:        "test-template",
			Description: "Test template",
			Version:     "1.0.0",
			Tags:        []string{"test"},
		},
	}

	// Save cache
	if err := registry.saveCache("test-repo", templates); err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Load cache
	cache, err := registry.loadCache("test-repo")
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	if cache == nil {
		t.Fatal("Loaded cache is nil")
	}

	if len(cache.Templates) != 1 {
		t.Errorf("Expected 1 template in cache, got %d", len(cache.Templates))
	}

	tmpl, ok := cache.Templates["test-template"]
	if !ok {
		t.Fatal("Template not found in cache")
	}

	if tmpl.Description != "Test template" {
		t.Errorf("Expected description 'Test template', got %q", tmpl.Description)
	}
}
