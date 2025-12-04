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

	// Strategy 1: Local file path
	if filepath.IsAbs(ref) || fileExists(ref) {
		logging.Debug("Loading template from local file: %s", ref)
		return tl.loadFromFile(ref)
	}

	// Strategy 2: Template name (use official repo)
	if !strings.Contains(ref, "/") && !strings.HasPrefix(ref, "https://") {
		logging.Debug("Loading template by name from registry: %s", ref)
		return tl.loadFromRegistry("cowdogmoo/warpgate-templates", ref)
	}

	// Strategy 3: Full git URL
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "git@") {
		logging.Debug("Loading template from git URL: %s", ref)
		return tl.loadFromGit(ref)
	}

	return nil, fmt.Errorf("unknown template reference: %s", ref)
}

// loadFromRegistry loads a template from a git repository by name
func (tl *TemplateLoader) loadFromRegistry(repo, templateName string) (*builder.Config, error) {
	// Parse version if specified: attack-box@v1.2.0
	name, version := parseTemplateRef(templateName)

	// Build git URL
	gitURL := fmt.Sprintf("https://github.com/%s.git", repo)

	// Clone or update repo
	localPath, err := tl.gitOps.CloneOrUpdate(gitURL, version)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Load config from cloned repo
	configPath := filepath.Join(localPath, "templates", name, "warpgate.yaml")
	return tl.loadFromFile(configPath)
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
