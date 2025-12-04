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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"gopkg.in/yaml.v3"
)

// TemplateRegistry manages template repositories
type TemplateRegistry struct {
	repos    map[string]string // name -> git URL
	cacheDir string            // persistent cache directory
}

// CacheMetadata stores information about cached templates
type CacheMetadata struct {
	LastUpdated  time.Time               `json:"last_updated"`
	Templates    map[string]TemplateInfo `json:"templates"`
	Repositories map[string]string       `json:"repositories"`
}

// TemplateInfo contains information about a template
type TemplateInfo struct {
	Name        string
	Description string
	Version     string
	Repository  string
	Path        string
	Tags        []string
	Author      string
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry() (*TemplateRegistry, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".warpgate", "cache", "registry")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	tr := &TemplateRegistry{
		repos: map[string]string{
			"official": "https://github.com/cowdogmoo/warpgate-templates.git",
			// Future: support for community template repos
		},
		cacheDir: cacheDir,
	}

	// Load custom repositories from config
	if err := tr.LoadRepositories(); err != nil {
		logging.Warn("Failed to load repository config: %v", err)
	}

	return tr, nil
}

// List returns all available templates in a repository
func (tr *TemplateRegistry) List(repoName string) ([]TemplateInfo, error) {
	// Try to load from cache first
	cache, err := tr.loadCache(repoName)
	if err == nil && cache != nil {
		// Check if cache is recent (less than 1 hour old)
		if time.Since(cache.LastUpdated) < time.Hour {
			logging.Debug("Using cached templates for repository: %s", repoName)
			templates := make([]TemplateInfo, 0, len(cache.Templates))
			for _, tmpl := range cache.Templates {
				templates = append(templates, tmpl)
			}
			return templates, nil
		}
		logging.Debug("Cache expired for repository: %s", repoName)
	}

	// Cache miss or expired - fetch fresh data
	logging.Debug("Fetching templates from repository: %s", repoName)
	repoURL, ok := tr.repos[repoName]
	if !ok {
		return nil, fmt.Errorf("unknown repository: %s", repoName)
	}

	// Clone or update the repository using persistent cache
	repoCache := filepath.Join(tr.cacheDir, "repos", repoName)
	gitOps := NewGitOperations(repoCache)
	repoPath, err := gitOps.CloneOrUpdate(repoURL, "")
	if err != nil {
		return nil, fmt.Errorf("failed to access repository: %w", err)
	}

	// Discover templates in the repository
	templates, err := tr.discoverTemplates(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover templates: %w", err)
	}

	// Save to cache
	if err := tr.saveCache(repoName, templates); err != nil {
		logging.Warn("Failed to save cache: %v", err)
	}

	return templates, nil
}

// discoverTemplates finds all templates in a repository
func (tr *TemplateRegistry) discoverTemplates(repoPath string) ([]TemplateInfo, error) {
	templatesDir := filepath.Join(repoPath, "templates")
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("templates directory not found in repository")
	}

	var templates []TemplateInfo

	// Walk through templates directory
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Look for warpgate.yaml in each template directory
		configPath := filepath.Join(templatesDir, entry.Name(), "warpgate.yaml")
		if _, err := os.Stat(configPath); err != nil {
			continue // Skip if no config found
		}

		// Load template metadata
		info, err := tr.loadTemplateInfo(configPath, entry.Name())
		if err != nil {
			continue // Skip templates with errors
		}

		templates = append(templates, info)
	}

	return templates, nil
}

// loadTemplateInfo loads metadata from a template config
func (tr *TemplateRegistry) loadTemplateInfo(configPath, name string) (TemplateInfo, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return TemplateInfo{}, err
	}

	var config builder.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return TemplateInfo{}, err
	}

	return TemplateInfo{
		Name:        name,
		Description: config.Metadata.Description,
		Version:     config.Metadata.Version,
		Path:        filepath.Dir(configPath),
		Tags:        config.Metadata.Tags,
		Author:      config.Metadata.Author,
	}, nil
}

// Search searches for templates matching a query
func (tr *TemplateRegistry) Search(query string) ([]TemplateInfo, error) {
	allTemplates := []TemplateInfo{}

	// Search all registered repositories
	for repoName := range tr.repos {
		templates, err := tr.List(repoName)
		if err != nil {
			continue // Skip repos that fail
		}

		// Filter templates matching the query
		for _, tmpl := range templates {
			if tr.matchesQuery(tmpl, query) {
				tmpl.Repository = repoName
				allTemplates = append(allTemplates, tmpl)
			}
		}
	}

	return allTemplates, nil
}

// matchesQuery checks if a template matches a search query
// Uses both exact substring matching and fuzzy matching for better results
func (tr *TemplateRegistry) matchesQuery(tmpl TemplateInfo, query string) bool {
	query = strings.ToLower(query)

	// Exact substring match on name (highest priority)
	if strings.Contains(strings.ToLower(tmpl.Name), query) {
		return true
	}

	// Exact substring match on description
	if strings.Contains(strings.ToLower(tmpl.Description), query) {
		return true
	}

	// Exact substring match on tags
	for _, tag := range tmpl.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}

	// Fuzzy match on name
	if fuzzy.Match(query, strings.ToLower(tmpl.Name)) {
		return true
	}

	// Fuzzy match on tags
	for _, tag := range tmpl.Tags {
		if fuzzy.Match(query, strings.ToLower(tag)) {
			return true
		}
	}

	// Fuzzy match on description words (split description into words for better matching)
	descWords := strings.Fields(strings.ToLower(tmpl.Description))
	for _, word := range descWords {
		if fuzzy.Match(query, word) {
			return true
		}
	}

	return false
}

// AddRepository adds a new template repository to the registry
func (tr *TemplateRegistry) AddRepository(name, gitURL string) {
	tr.repos[name] = gitURL
}

// RemoveRepository removes a template repository from the registry
func (tr *TemplateRegistry) RemoveRepository(name string) {
	delete(tr.repos, name)
}

// loadCache loads cached template information for a repository
func (tr *TemplateRegistry) loadCache(repoName string) (*CacheMetadata, error) {
	cachePath := filepath.Join(tr.cacheDir, repoName+".json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache CacheMetadata
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveCache saves template information to cache
func (tr *TemplateRegistry) saveCache(repoName string, templates []TemplateInfo) error {
	cache := CacheMetadata{
		LastUpdated:  time.Now(),
		Templates:    make(map[string]TemplateInfo),
		Repositories: tr.repos,
	}

	for _, tmpl := range templates {
		cache.Templates[tmpl.Name] = tmpl
	}

	cachePath := filepath.Join(tr.cacheDir, repoName+".json")
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// UpdateCache forces a cache refresh for a repository
func (tr *TemplateRegistry) UpdateCache(repoName string) error {
	logging.Info("Updating cache for repository: %s", repoName)

	repoURL, ok := tr.repos[repoName]
	if !ok {
		return fmt.Errorf("unknown repository: %s", repoName)
	}

	// Clone or update the repository
	repoCache := filepath.Join(tr.cacheDir, "repos", repoName)
	gitOps := NewGitOperations(repoCache)
	repoPath, err := gitOps.CloneOrUpdate(repoURL, "")
	if err != nil {
		return fmt.Errorf("failed to access repository: %w", err)
	}

	// Discover templates
	templates, err := tr.discoverTemplates(repoPath)
	if err != nil {
		return fmt.Errorf("failed to discover templates: %w", err)
	}

	// Save to cache
	if err := tr.saveCache(repoName, templates); err != nil {
		return fmt.Errorf("failed to save cache: %w", err)
	}

	logging.Info("Cache updated successfully for repository: %s (%d templates)", repoName, len(templates))
	return nil
}

// UpdateAllCaches forces a cache refresh for all repositories
func (tr *TemplateRegistry) UpdateAllCaches() error {
	var errors []string

	for repoName := range tr.repos {
		if err := tr.UpdateCache(repoName); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", repoName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to update some caches: %s", strings.Join(errors, "; "))
	}

	return nil
}

// GetRepositories returns all registered repositories
func (tr *TemplateRegistry) GetRepositories() map[string]string {
	repos := make(map[string]string)
	for name, url := range tr.repos {
		repos[name] = url
	}
	return repos
}

// LoadRepositories loads repository configuration from file
func (tr *TemplateRegistry) LoadRepositories() error {
	configPath := filepath.Join(tr.cacheDir, "repositories.json")

	// If file doesn't exist, use defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read repository config: %w", err)
	}

	var repos map[string]string
	if err := json.Unmarshal(data, &repos); err != nil {
		return fmt.Errorf("failed to parse repository config: %w", err)
	}

	// Merge with existing repos (keeping defaults if not overridden)
	for name, url := range repos {
		tr.repos[name] = url
	}

	return nil
}

// SaveRepositories saves repository configuration to file
func (tr *TemplateRegistry) SaveRepositories() error {
	configPath := filepath.Join(tr.cacheDir, "repositories.json")

	data, err := json.MarshalIndent(tr.repos, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal repository config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write repository config: %w", err)
	}

	return nil
}
