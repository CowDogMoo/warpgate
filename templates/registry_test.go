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

// TestIsPlaceholderURL tests detection of RFC 2606 reserved documentation domains
func TestIsPlaceholderURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		shouldMatch bool
	}{
		// RFC 2606 reserved example domains - should be flagged
		{"example.com domain", "https://example.com/repo.git", true},
		{"example.org domain", "https://example.org/repo.git", true},
		{"example.net domain", "https://example.net/repo.git", true},
		{"subdomain of example.com", "https://api.example.com/repo.git", true},
		{"subdomain of example.org", "https://git.example.org/repo.git", true},
		{".example TLD", "https://foo.example/repo.git", true},
		{".test TLD", "https://foo.test/repo.git", true},
		{".invalid TLD", "https://foo.invalid/repo.git", true},
		{".localhost TLD", "https://foo.localhost/repo.git", true},

		// git@ format with RFC 2606 domains - should be flagged
		{"git@ with example.com", "git@example.com:user/repo.git", true},
		{"git@ with example.org", "git@example.org:user/repo.git", true},
		{"git@ with .test", "git@git.test:user/repo.git", true},

		// Real URLs that should NOT be flagged
		{"real cowdogmoo repo", "https://github.com/cowdogmoo/warpgate-templates.git", false},
		{"real organization", "https://github.com/kubernetes/kubernetes.git", false},
		{"real user repo", "https://github.com/torvalds/linux.git", false},
		{"gitlab real repo", "https://gitlab.com/gitlab-org/gitlab.git", false},
		{"bitbucket real repo", "https://bitbucket.org/atlassian/python-bitbucket.git", false},
		{"git@ real repo", "git@github.com:cowdogmoo/warpgate.git", false},
		{"git@ real user", "git@github.com:torvalds/linux.git", false},

		// Edge cases - repos with "example" in name but real domains
		{"example in path", "https://github.com/acme/example-app.git", false},
		{"examples repo name", "https://github.com/company/examples.git", false},
		{"example- prefix", "https://github.com/org/example-configs.git", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPlaceholderURL(tt.url)
			if result != tt.shouldMatch {
				t.Errorf("isPlaceholderURL(%q) = %v, want %v", tt.url, result, tt.shouldMatch)
			}
		})
	}
}

// TestIsValidGitURL tests Git URL validation
func TestIsValidGitURL(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		valid bool
	}{
		{"https URL", "https://git.example.com/jdoe/repo.git", true},
		{"http URL", "http://git.example.com/jdoe/repo.git", true},
		{"git@ URL", "git@git.example.com:jdoe/repo.git", true},
		{"local path", "/home/jdoe/repo", false},
		{"relative path", "../templates", false},
		{"invalid protocol", "ftp://git.example.com/jdoe/repo.git", false},
	}

	pv := NewPathValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pv.IsGitURL(tt.url)
			if result != tt.valid {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.url, result, tt.valid)
			}
		})
	}
}

func TestNewTemplateRegistry(t *testing.T) {
	// Clear any config-related env vars to ensure clean test environment
	originalLocalPaths := os.Getenv("WARPGATE_TEMPLATES_LOCAL_PATHS")
	if err := os.Unsetenv("WARPGATE_TEMPLATES_LOCAL_PATHS"); err != nil {
		t.Logf("Failed to unset WARPGATE_TEMPLATES_LOCAL_PATHS: %v", err)
	}
	defer func() {
		if originalLocalPaths != "" {
			if err := os.Setenv("WARPGATE_TEMPLATES_LOCAL_PATHS", originalLocalPaths); err != nil {
				t.Logf("Failed to restore WARPGATE_TEMPLATES_LOCAL_PATHS: %v", err)
			}
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

	// Check that official repo is always registered by default
	repos := registry.GetRepositories()
	if len(repos) == 0 {
		t.Error("Expected at least one repository to be configured")
	}

	if url, ok := repos["official"]; !ok {
		t.Error("Expected official repository to be registered by default")
	} else if url != "https://github.com/cowdogmoo/warpgate-templates.git" {
		t.Errorf("Expected official repo URL, got %s", url)
	}
}

func TestTemplateRegistryDefaultOfficialRepo(t *testing.T) {
	// Test that official repo persists when adding custom repos
	tempDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	registry := &TemplateRegistry{
		repos: map[string]string{
			"official": "https://github.com/cowdogmoo/warpgate-templates.git",
			"custom":   "https://github.com/acme-corp/repo.git",
		},
		cacheDir: tempDir,
	}

	repos := registry.GetRepositories()

	if len(repos) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(repos))
	}

	if _, ok := repos["official"]; !ok {
		t.Error("Official repository should be present")
	}

	if _, ok := repos["custom"]; !ok {
		t.Error("Custom repository should be present")
	}
}

func TestTemplateRegistryDisableOfficialRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	repos := map[string]string{
		"official": "https://github.com/cowdogmoo/warpgate-templates.git",
	}

	configRepos := map[string]string{
		"official": "",
		"custom":   "https://github.com/acme-corp/repo.git",
	}

	for name, url := range configRepos {
		if url != "" {
			repos[name] = url
		} else {
			delete(repos, name)
		}
	}

	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}

	if _, ok := repos["official"]; ok {
		t.Error("Official repository should be disabled")
	}

	if _, ok := repos["custom"]; !ok {
		t.Error("Custom repository should be present")
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
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.matchesQuery(tt.template, tt.query)
			if result != tt.expected {
				t.Errorf("matchesQuery() = %v, want %v for query %q", result, tt.expected, tt.query)
			}
		})
	}
}

func TestIsLocalPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "absolute path exists",
			path:     tempDir,
			expected: true,
		},
		{
			name:     "https git url",
			path:     "https://github.com/cowdogmoo/warpgate-templates.git",
			expected: false,
		},
		{
			name:     "ssh git url",
			path:     "git@github.com:cowdogmoo/warpgate-templates.git",
			expected: false,
		},
		{
			name:     "http url",
			path:     "http://example.com/repo.git",
			expected: false,
		},
		{
			name:     "non-existent absolute path",
			path:     "/non/existent/path",
			expected: false,
		},
	}

	pv := NewPathValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pv.IsLocalPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsLocalPath(%q) = %v, want %v", tt.path, result, tt.expected)
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
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

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
