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
	"fmt"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/viper"
)

// Manager handles template source management including adding, removing,
// and persisting template repositories and local paths.
type Manager struct {
	config    *config.Config
	validator *PathValidator
}

// NewManager creates a new template manager with the given configuration.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:    cfg,
		validator: NewPathValidator(),
	}
}

// AddGitRepository adds a git repository to template sources.
// If name is empty, it auto-generates one from the URL.
func (m *Manager) AddGitRepository(ctx context.Context, name, gitURL string) error {
	// Initialize repositories map if nil
	if m.config.Templates.Repositories == nil {
		m.config.Templates.Repositories = make(map[string]string)
	}

	// Auto-generate name if not provided
	if name == "" {
		name = ExtractRepoName(gitURL)
	}

	if !m.validator.IsGitURL(gitURL) {
		return fmt.Errorf("invalid Git URL '%s'; expected https://, http://, or git@ URL", gitURL)
	}

	if isPlaceholderURL(gitURL) {
		return fmt.Errorf("URL '%s' appears to be a placeholder from documentation examples; please use a real repository URL", gitURL)
	}

	// Check if already exists
	if existing, ok := m.config.Templates.Repositories[name]; ok {
		if existing == gitURL {
			logging.WarnContext(ctx, "Repository '%s' already exists", name)
			return nil
		}
		return fmt.Errorf("repository name '%s' already exists with different URL: %s", name, existing)
	}

	logging.InfoContext(ctx, "Adding git repository: %s -> %s", name, gitURL)
	m.config.Templates.Repositories[name] = gitURL

	// Save to config file
	if err := m.saveConfigValue("templates.repositories", m.config.Templates.Repositories); err != nil {
		return err
	}

	configPath, _ := config.ConfigFile("config.yaml")
	logging.InfoContext(ctx, "Repository added successfully as '%s' to %s", name, configPath)
	logging.InfoContext(ctx, "Run 'warpgate templates update' to fetch templates")

	return nil
}

// AddLocalPath adds a local directory to template sources.
func (m *Manager) AddLocalPath(ctx context.Context, path string) error {
	// Expand and normalize path
	expandedPath, err := m.validator.ExpandPath(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Validate path exists and is a directory
	if err := m.validator.ValidateLocalPath(expandedPath); err != nil {
		return err
	}

	logging.InfoContext(ctx, "Adding local template directory: %s", expandedPath)

	// Check if already exists
	for _, existingPath := range m.config.Templates.LocalPaths {
		if existingPath == expandedPath {
			logging.WarnContext(ctx, "Path already exists in local_paths")
			return nil
		}
	}

	// Add to local_paths
	m.config.Templates.LocalPaths = append(m.config.Templates.LocalPaths, expandedPath)

	// Save to config file
	if err := m.saveConfigValue("templates.local_paths", m.config.Templates.LocalPaths); err != nil {
		return err
	}

	configPath, _ := config.ConfigFile("config.yaml")
	logging.InfoContext(ctx, "Template directory added successfully to %s", configPath)

	return nil
}

// RemoveSource removes a template source by path or repository name.
func (m *Manager) RemoveSource(ctx context.Context, pathOrName string) error {
	// Normalize path for comparison
	normalizedPath, err := m.validator.NormalizePath(pathOrName)
	if err != nil {
		// If normalization fails, just use the original value
		normalizedPath = pathOrName
	}

	// Try to remove from local_paths
	removedFromPaths := m.removeFromLocalPaths(normalizedPath, pathOrName)

	// Try to remove from repositories by name
	removedFromRepos := m.removeFromRepositories(pathOrName)

	if !removedFromPaths && !removedFromRepos {
		return fmt.Errorf("template source not found: %s", pathOrName)
	}

	// Save both updated values
	if err := m.saveTemplatesConfig(); err != nil {
		return err
	}

	// Log results
	configPath, _ := config.ConfigFile("config.yaml")
	if removedFromPaths {
		logging.InfoContext(ctx, "Removed from local_paths in %s", configPath)
	}
	if removedFromRepos {
		logging.InfoContext(ctx, "Removed from repositories in %s", configPath)
	}

	return nil
}

// removeFromLocalPaths removes a path from local_paths.
func (m *Manager) removeFromLocalPaths(normalizedPath, originalPath string) bool {
	newLocalPaths := []string{}
	removed := false

	for _, existingPath := range m.config.Templates.LocalPaths {
		if existingPath != normalizedPath && existingPath != originalPath {
			newLocalPaths = append(newLocalPaths, existingPath)
		} else {
			removed = true
		}
	}

	m.config.Templates.LocalPaths = newLocalPaths
	return removed
}

// removeFromRepositories removes a repository by name.
func (m *Manager) removeFromRepositories(name string) bool {
	if _, exists := m.config.Templates.Repositories[name]; exists {
		delete(m.config.Templates.Repositories, name)
		return true
	}
	return false
}

// saveConfigValue saves a single configuration value to the config file.
func (m *Manager) saveConfigValue(key string, value interface{}) error {
	configPath, err := config.ConfigFile("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	v.Set(key, value)

	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// saveTemplatesConfig saves the entire templates configuration to the config file.
func (m *Manager) saveTemplatesConfig() error {
	configPath, err := config.ConfigFile("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	v.Set("templates.local_paths", m.config.Templates.LocalPaths)
	v.Set("templates.repositories", m.config.Templates.Repositories)

	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}
