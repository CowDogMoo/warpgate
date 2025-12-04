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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"gopkg.in/yaml.v3"
)

// TemplateRegistry manages template repositories
type TemplateRegistry struct {
	repos map[string]string // name -> git URL
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
func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{
		repos: map[string]string{
			"official": "https://github.com/cowdogmoo/warpgate-templates.git",
			// Future: support for community template repos
		},
	}
}

// List returns all available templates in a repository
func (tr *TemplateRegistry) List(repoName string) ([]TemplateInfo, error) {
	repoURL, ok := tr.repos[repoName]
	if !ok {
		return nil, fmt.Errorf("unknown repository: %s", repoName)
	}

	// Clone or update the repository
	gitOps := NewGitOperations(filepath.Join(os.TempDir(), "warpgate-registry"))
	repoPath, err := gitOps.CloneOrUpdate(repoURL, "")
	if err != nil {
		return nil, fmt.Errorf("failed to access repository: %w", err)
	}

	// Discover templates in the repository
	templates, err := tr.discoverTemplates(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover templates: %w", err)
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
func (tr *TemplateRegistry) matchesQuery(tmpl TemplateInfo, query string) bool {
	query = strings.ToLower(query)

	// Check name
	if strings.Contains(strings.ToLower(tmpl.Name), query) {
		return true
	}

	// Check description
	if strings.Contains(strings.ToLower(tmpl.Description), query) {
		return true
	}

	// Check tags
	for _, tag := range tmpl.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
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
