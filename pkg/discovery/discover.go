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

package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// TemplateDiscovery handles discovering templates from various sources
type TemplateDiscovery struct {
	sources []TemplateSource
}

// TemplateSource represents a source of templates
type TemplateSource interface {
	Discover() ([]DiscoveredTemplate, error)
	Name() string
}

// DiscoveredTemplate represents a discovered template
type DiscoveredTemplate struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Builders  []string `json:"builders"`
	Source    string   `json:"source"`
	ConfigDir string   `json:"config_dir,omitempty"`
}

// LocalFolderSource discovers templates from a local folder
type LocalFolderSource struct {
	path string
	name string
}

// NewLocalFolderSource creates a new local folder template source
func NewLocalFolderSource(name, path string) *LocalFolderSource {
	return &LocalFolderSource{
		path: path,
		name: name,
	}
}

func (s *LocalFolderSource) Name() string {
	return s.name
}

func (s *LocalFolderSource) Discover() ([]DiscoveredTemplate, error) {
	logging.Debug("Discovering templates in %s", s.path)

	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		logging.Warn("Template directory does not exist: %s", s.path)
		return []DiscoveredTemplate{}, nil
	}

	var templates []DiscoveredTemplate

	entries, err := os.ReadDir(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", s.path, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		templatePath := filepath.Join(s.path, entry.Name())

		// Look for warpgate.yaml
		configPath := filepath.Join(templatePath, "warpgate.yaml")
		if _, err := os.Stat(configPath); err == nil {
			templates = append(templates, DiscoveredTemplate{
				Name:      entry.Name(),
				Path:      configPath,
				Builders:  []string{"container", "ami"}, // Default builders
				Source:    s.name,
				ConfigDir: templatePath,
			})
			continue
		}

		// Fallback: Look for legacy packer files
		dockerConfig := filepath.Join(templatePath, "docker.pkr.hcl")
		amiConfig := filepath.Join(templatePath, "ami.pkr.hcl")

		builders := []string{}
		if _, err := os.Stat(dockerConfig); err == nil {
			builders = append(builders, "docker")
		}
		if _, err := os.Stat(amiConfig); err == nil {
			builders = append(builders, "ami")
		}

		if len(builders) > 0 {
			templates = append(templates, DiscoveredTemplate{
				Name:      entry.Name(),
				Path:      templatePath,
				Builders:  builders,
				Source:    s.name,
				ConfigDir: templatePath,
			})
		}
	}

	logging.Info("Discovered %d templates in %s", len(templates), s.name)
	return templates, nil
}

// NewTemplateDiscovery creates a new template discovery instance
func NewTemplateDiscovery() *TemplateDiscovery {
	return &TemplateDiscovery{
		sources: []TemplateSource{},
	}
}

// AddSource adds a template source to discovery
func (td *TemplateDiscovery) AddSource(source TemplateSource) {
	td.sources = append(td.sources, source)
}

// DiscoverAll discovers templates from all sources
func (td *TemplateDiscovery) DiscoverAll() ([]DiscoveredTemplate, error) {
	var allTemplates []DiscoveredTemplate

	for _, source := range td.sources {
		templates, err := source.Discover()
		if err != nil {
			logging.Warn("Failed to discover templates from %s: %v", source.Name(), err)
			continue
		}
		allTemplates = append(allTemplates, templates...)
	}

	return allTemplates, nil
}

// WriteToJSON writes discovered templates to a JSON file
func (td *TemplateDiscovery) WriteToJSON(templates []DiscoveredTemplate, outputPath string) error {
	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal templates: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	logging.Info("Wrote %d templates to %s", len(templates), outputPath)
	return nil
}
