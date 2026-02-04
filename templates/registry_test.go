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

	registry, err := NewTemplateRegistry(context.Background())
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
	registry, err := NewTemplateRegistry(context.Background())
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

func TestTemplateRegistryListUnknownRepo(t *testing.T) {
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
		repos:         map[string]string{},
		cacheDir:      tempDir,
		pathValidator: NewPathValidator(),
	}

	_, err = registry.List(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Expected error for unknown repository")
	}

	expected := "unknown repository: nonexistent"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestDiscoverTemplates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create templates directory structure with two templates
	tmplDir1 := filepath.Join(tmpDir, "templates", "tmpl-one")
	tmplDir2 := filepath.Join(tmpDir, "templates", "tmpl-two")
	tmplDir3 := filepath.Join(tmpDir, "templates", "no-config")
	if err := os.MkdirAll(tmplDir1, 0o755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.MkdirAll(tmplDir2, 0o755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.MkdirAll(tmplDir3, 0o755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Write valid warpgate.yaml for tmpl-one
	config1 := []byte("metadata:\n  description: Template one\n  version: 1.0.0\n  author: tester\n  tags:\n    - test\n")
	if err := os.WriteFile(filepath.Join(tmplDir1, "warpgate.yaml"), config1, 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Write valid warpgate.yaml for tmpl-two
	config2 := []byte("metadata:\n  description: Template two\n  version: 2.0.0\n  author: tester\n  tags:\n    - other\n")
	if err := os.WriteFile(filepath.Join(tmplDir2, "warpgate.yaml"), config2, 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// tmpl-three has no warpgate.yaml and should be skipped

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	templates, err := registry.discoverTemplates(tmpDir)
	if err != nil {
		t.Fatalf("discoverTemplates() error = %v", err)
	}

	if len(templates) != 2 {
		t.Fatalf("discoverTemplates() returned %d templates, want 2", len(templates))
	}

	// Verify template names (order may vary)
	names := map[string]bool{}
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}
	if !names["tmpl-one"] {
		t.Error("discoverTemplates() missing tmpl-one")
	}
	if !names["tmpl-two"] {
		t.Error("discoverTemplates() missing tmpl-two")
	}
}

func TestDiscoverTemplates_NoTemplatesDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	_, err := registry.discoverTemplates(tmpDir)
	if err == nil {
		t.Fatal("discoverTemplates() expected error for missing templates dir, got nil")
	}
}

func TestLoadTemplateInfo_Valid(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "warpgate.yaml")

	content := []byte("metadata:\n  description: Test desc\n  version: 1.0.0\n  author: Test Author\n  tags:\n    - foo\n    - bar\n")
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	registry := &TemplateRegistry{
		pathValidator: NewPathValidator(),
	}

	info, err := registry.loadTemplateInfo(configPath, "my-template")
	if err != nil {
		t.Fatalf("loadTemplateInfo() error = %v", err)
	}

	if info.Name != "my-template" {
		t.Errorf("loadTemplateInfo() Name = %q, want %q", info.Name, "my-template")
	}
	if info.Description != "Test desc" {
		t.Errorf("loadTemplateInfo() Description = %q, want %q", info.Description, "Test desc")
	}
	if info.Version != "1.0.0" {
		t.Errorf("loadTemplateInfo() Version = %q, want %q", info.Version, "1.0.0")
	}
	if info.Author != "Test Author" {
		t.Errorf("loadTemplateInfo() Author = %q, want %q", info.Author, "Test Author")
	}
	if len(info.Tags) != 2 {
		t.Errorf("loadTemplateInfo() Tags length = %d, want 2", len(info.Tags))
	}
}

func TestLoadTemplateInfo_InvalidYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "warpgate.yaml")

	content := []byte("this is: [not: valid: yaml: {{{")
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	registry := &TemplateRegistry{
		pathValidator: NewPathValidator(),
	}

	_, err := registry.loadTemplateInfo(configPath, "bad")
	if err == nil {
		t.Fatal("loadTemplateInfo() expected error for invalid YAML, got nil")
	}
}

func TestLoadTemplateInfo_NonExistentFile(t *testing.T) {
	t.Parallel()

	registry := &TemplateRegistry{
		pathValidator: NewPathValidator(),
	}

	_, err := registry.loadTemplateInfo("/nonexistent/path/warpgate.yaml", "missing")
	if err == nil {
		t.Fatal("loadTemplateInfo() expected error for non-existent file, got nil")
	}
}

func TestUpdateCache_UnknownRepo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	err := registry.UpdateCache(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("UpdateCache() expected error for unknown repo, got nil")
	}
	if !strings.Contains(err.Error(), "unknown repository") {
		t.Errorf("UpdateCache() error = %v, want error containing 'unknown repository'", err)
	}
}

func TestUpdateAllCaches_EmptyRepos(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	err := registry.UpdateAllCaches(context.Background())
	// With no repos, should succeed with no errors
	if err != nil {
		t.Errorf("UpdateAllCaches() with empty repos error = %v, want nil", err)
	}
}

func TestScanLocalPaths_ValidPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a local path with templates
	localPath := filepath.Join(tmpDir, "local-templates")
	tmplDir := filepath.Join(localPath, "templates", "my-tmpl")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	content := []byte("metadata:\n  description: Local template\n  version: 1.0.0\n  author: tester\n  tags:\n    - local\n")
	if err := os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), content, 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		localPaths:    []string{localPath},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	templates := registry.scanLocalPaths(context.Background())

	if len(templates) != 1 {
		t.Fatalf("scanLocalPaths() returned %d templates, want 1", len(templates))
	}
	if templates[0].Name != "my-tmpl" {
		t.Errorf("scanLocalPaths() template name = %q, want %q", templates[0].Name, "my-tmpl")
	}
	if !strings.Contains(templates[0].Repository, "local:") {
		t.Errorf("scanLocalPaths() repository = %q, want prefix 'local:'", templates[0].Repository)
	}
}

func TestScanLocalPaths_InvalidPaths(t *testing.T) {
	t.Parallel()

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		localPaths:    []string{"/nonexistent/path/that/does/not/exist"},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	// Should not panic, just return empty
	templates := registry.scanLocalPaths(context.Background())
	if len(templates) != 0 {
		t.Errorf("scanLocalPaths() with invalid paths returned %d templates, want 0", len(templates))
	}
}

func TestScanLocalPaths_GitURL(t *testing.T) {
	t.Parallel()

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		localPaths:    []string{"https://github.com/user/repo.git"},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	// Git URLs should be skipped by scanLocalPaths
	templates := registry.scanLocalPaths(context.Background())
	if len(templates) != 0 {
		t.Errorf("scanLocalPaths() with git URL returned %d templates, want 0", len(templates))
	}
}

func TestCacheSaveAndLoad(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	registry := &TemplateRegistry{
		repos: map[string]string{
			"test": "https://github.com/test/repo.git",
		},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	templates := []TemplateInfo{
		{Name: "tmpl-a", Description: "Template A", Version: "1.0.0", Tags: []string{"a"}},
		{Name: "tmpl-b", Description: "Template B", Version: "2.0.0", Tags: []string{"b"}},
	}

	// Save
	err := registry.saveCache("test-repo", templates)
	if err != nil {
		t.Fatalf("saveCache() error = %v", err)
	}

	// Load
	cache, err := registry.loadCache("test-repo")
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}

	if cache == nil {
		t.Fatal("loadCache() returned nil")
	}
	if len(cache.Templates) != 2 {
		t.Errorf("loadCache() Templates length = %d, want 2", len(cache.Templates))
	}
	if cache.Templates["tmpl-a"].Description != "Template A" {
		t.Errorf("loadCache() tmpl-a description = %q, want %q", cache.Templates["tmpl-a"].Description, "Template A")
	}
}

func TestLoadCache_NonExistent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	registry := &TemplateRegistry{
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	_, err := registry.loadCache("does-not-exist")
	if err == nil {
		t.Fatal("loadCache() expected error for non-existent cache, got nil")
	}
}

func TestTemplateRegistryListLocalRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a local repo structure with templates/
	repoDir := filepath.Join(tempDir, "local-repo")
	templatesDir := filepath.Join(repoDir, "templates", "test-tmpl")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	// Write a minimal warpgate.yaml
	configContent := []byte("metadata:\n  description: Local test\n  version: 0.1.0\n  author: tester\n  tags:\n    - test\n")
	if err := os.WriteFile(filepath.Join(templatesDir, "warpgate.yaml"), configContent, 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	registry := &TemplateRegistry{
		repos: map[string]string{
			"local": repoDir,
		},
		cacheDir:      tempDir,
		pathValidator: NewPathValidator(),
	}

	templates, err := registry.List(context.Background(), "local")
	if err != nil {
		t.Fatalf("Failed to list local repo: %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}

	if templates[0].Name != "test-tmpl" {
		t.Errorf("Expected template name 'test-tmpl', got %q", templates[0].Name)
	}
}
