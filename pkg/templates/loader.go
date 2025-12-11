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

// Package templates provides template discovery, loading, and management capabilities.
//
// This package enables flexible template sourcing from multiple locations including
// local directories, git repositories, and official registries. It handles template
// versioning, caching, and lifecycle management.
//
// # Architecture
//
// The package is organized into several key components:
//
//   - Loader (loader.go): Loads templates from various sources (files, git, registry)
//   - Manager (manager.go): Manages template sources (add/remove repositories and paths)
//   - Registry (registry.go): Fetches templates from official registries
//   - Git (git.go): Clones and updates templates from git repositories
//   - Scaffold (scaffold.go): Creates new template scaffolds from scratch
//   - Paths (paths.go): Path validation, expansion, and normalization
//   - Filters (filters.go): Filters templates by criteria (name, tag, type)
//
// # Loading Templates
//
// TemplateLoader provides unified access to templates from multiple sources:
//
//	loader, err := templates.NewTemplateLoader()
//	if err != nil {
//	    return err
//	}
//
//	// Load from local file
//	config, err := loader.LoadTemplate("./my-template/warpgate.yaml")
//
//	// Load from registry (auto-fetches if not cached)
//	config, err := loader.LoadTemplate("attack-box")
//
//	// Load specific version
//	config, err := loader.LoadTemplate("attack-box@v1.2.0")
//
//	// Load from git URL
//	config, err := loader.LoadTemplate("https://github.com/user/repo.git//template-name")
//
// # Managing Template Sources
//
// The Manager handles adding and removing template sources:
//
//	mgr := templates.NewManager(globalConfig)
//
//	// Add git repository
//	err := mgr.AddGitRepository(ctx, "my-templates", "https://github.com/user/templates.git")
//
//	// Add local directory
//	err := mgr.AddLocalPath(ctx, "/path/to/templates")
//
//	// Remove source
//	err := mgr.RemoveSource(ctx, "my-templates")
//
// # Template Discovery
//
// Templates can be discovered from configured sources:
//
//	// List all available templates
//	templates, err := templates.List(ctx)
//
//	// Filter by criteria
//	filtered := templates.FilterByName("attack")
//	filtered = filtered.FilterByType("container")
//
// # Creating Templates
//
// Scaffold new templates with sensible defaults:
//
//	scaffolder := templates.NewScaffolder()
//	err := scaffolder.CreateTemplate(ctx, "my-template", templates.ScaffoldOptions{
//	    Description: "My custom template",
//	    BaseImage:   "ubuntu:22.04",
//	})
//
// # Versioning
//
// Templates support semantic versioning with flexible version resolution:
//
//	// Exact version
//	config, err := loader.LoadTemplate("attack-box@1.2.3")
//
//	// Version constraint
//	config, err := loader.LoadTemplate("attack-box@^1.2")
//
//	// Latest (default)
//	config, err := loader.LoadTemplate("attack-box")
//
// # Caching
//
// Templates are cached locally for performance:
//
//   - Git repositories: Cloned to ~/.warpgate/templates/repositories/
//   - Registry templates: Cached in ~/.warpgate/templates/cache/
//   - Local paths: Used directly without caching
//
// Use the update command to refresh cached templates:
//
//	warpgate templates update
//
// # Configuration
//
// Template sources are configured in the global config (~/.warpgate/config.yaml):
//
//	templates:
//	  repositories:
//	    official: https://github.com/cowdogmoo/warpgate-templates.git
//	    custom: https://github.com/myorg/templates.git
//	  local_paths:
//	    - /home/user/my-templates
//	    - ./templates
//
// # Design Principles
//
//   - Source Flexibility: Support multiple template sources (local, git, registry)
//   - Caching: Optimize performance through intelligent caching
//   - Version Control: Full semantic versioning support
//   - Path Safety: Careful validation and normalization of file paths
//   - User Experience: Clear error messages and helpful defaults
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/config"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// TemplateLoader handles template discovery and loading
type TemplateLoader struct {
	cacheDir   string
	registry   *TemplateRegistry
	configLoad *config.Loader
	gitOps     *GitOperations
}

// NewTemplateLoader creates a new template loader
func NewTemplateLoader() (*TemplateLoader, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".warpgate", "cache", "templates")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	registry, err := NewTemplateRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create template registry: %w", err)
	}

	return &TemplateLoader{
		cacheDir:   cacheDir,
		registry:   registry,
		configLoad: config.NewLoader(),
		gitOps:     NewGitOperations(cacheDir),
	}, nil
}

// LoadTemplate handles all template loading strategies
func (tl *TemplateLoader) LoadTemplate(ref string) (*builder.Config, error) {
	logging.Debug("Loading template: %s", ref)

	// Strategy 1: Local file path (absolute or relative warpgate.yaml)
	if filepath.IsAbs(ref) || fileExists(ref) {
		logging.Debug("Loading template from local file: %s", ref)
		// If it's a directory, look for warpgate.yaml inside
		if info, err := os.Stat(ref); err == nil && info.IsDir() {
			configPath := filepath.Join(ref, "warpgate.yaml")
			if fileExists(configPath) {
				return tl.loadFromFile(configPath)
			}
			return nil, fmt.Errorf("no warpgate.yaml found in directory: %s", ref)
		}
		return tl.loadFromFile(ref)
	}

	// Strategy 2: Template name (search all repos/local paths)
	if !strings.Contains(ref, "/") && !strings.HasPrefix(ref, "https://") && !strings.HasPrefix(ref, "git@") {
		logging.Debug("Loading template by name from registry: %s", ref)
		return tl.loadTemplateByName(ref)
	}

	// Strategy 3: Full git URL
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "git@") {
		logging.Debug("Loading template from git URL: %s", ref)
		return tl.loadFromGit(ref)
	}

	return nil, fmt.Errorf("unknown template reference: %s", ref)
}

// loadTemplateByName searches all configured repositories and local paths for a template
func (tl *TemplateLoader) loadTemplateByName(name string) (*builder.Config, error) {
	// Parse version if specified: attack-box@v1.2.0
	templateName, version := parseTemplateRef(name)

	// Get all repos from registry
	repos := tl.registry.GetRepositories()

	// Try each repository
	for repoName, repoURL := range repos {
		// Check if it's a local path
		if tl.isLocalPath(repoURL) {
			configPath := filepath.Join(repoURL, "templates", templateName, "warpgate.yaml")
			if fileExists(configPath) {
				logging.Debug("Found template %s in local path: %s", templateName, repoURL)
				return tl.loadFromFile(configPath)
			}
			continue
		}

		// Try loading from git repo
		logging.Debug("Searching for template %s in repository: %s", templateName, repoName)
		cfg, err := tl.loadFromRegistry(repoURL, templateName, version)
		if err == nil {
			return cfg, nil
		}
		logging.Debug("Template not found in %s: %v", repoName, err)
	}

	// Also check local paths from config
	localPaths := tl.registry.GetLocalPaths()
	for _, localPath := range localPaths {
		if !tl.isLocalPath(localPath) {
			continue
		}
		configPath := filepath.Join(localPath, "templates", templateName, "warpgate.yaml")
		if fileExists(configPath) {
			logging.Debug("Found template %s in local path: %s", templateName, localPath)
			return tl.loadFromFile(configPath)
		}
		logging.Debug("Template not found in local path: %s", localPath)
	}

	return nil, fmt.Errorf("template not found in any configured repository: %s", name)
}

// loadFromRegistry loads a template from a git repository by name
func (tl *TemplateLoader) loadFromRegistry(repoURL, templateName, version string) (*builder.Config, error) {
	// Clone or update repo
	localPath, err := tl.gitOps.CloneOrUpdate(repoURL, version)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Load config from cloned repo
	configPath := filepath.Join(localPath, "templates", templateName, "warpgate.yaml")
	return tl.loadFromFile(configPath)
}

// isLocalPath checks if a path is a local directory
func (tl *TemplateLoader) isLocalPath(path string) bool {
	// Absolute paths
	if filepath.IsAbs(path) {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}

	// Relative paths or home paths
	if strings.HasPrefix(path, ".") || strings.HasPrefix(path, "~") {
		// Expand home directory
		if strings.HasPrefix(path, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				path = filepath.Join(home, path[1:])
			}
		}
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}

	// Not a URL
	return !strings.HasPrefix(path, "http://") &&
		!strings.HasPrefix(path, "https://") &&
		!strings.HasPrefix(path, "git@")
}

// loadFromGit loads a template from a git URL
func (tl *TemplateLoader) loadFromGit(gitURL string) (*builder.Config, error) {
	// Parse git URL to extract path within repo
	// Format: https://github.com/user/repo.git//path/to/template
	parts := strings.Split(gitURL, "//")
	repoURL := parts[0]
	templatePath := ""
	if len(parts) > 1 {
		templatePath = parts[1]
	}

	// Clone or update repo
	localPath, err := tl.gitOps.CloneOrUpdate(repoURL, "")
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Build config path
	configPath := filepath.Join(localPath, templatePath, "warpgate.yaml")
	return tl.loadFromFile(configPath)
}

// loadFromFile loads a template configuration from a file
func (tl *TemplateLoader) loadFromFile(path string) (*builder.Config, error) {
	cfg, err := tl.configLoad.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	// Validate the configuration
	validator := config.NewValidator()
	if err := validator.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// List returns all available templates from the registry
func (tl *TemplateLoader) List() ([]TemplateInfo, error) {
	return tl.registry.List("official")
}

// parseTemplateRef parses a template reference like "attack-box@v1.2.0"
func parseTemplateRef(ref string) (name, version string) {
	parts := strings.Split(ref, "@")
	name = parts[0]
	if len(parts) > 1 {
		version = parts[1]
	}
	return name, version
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
