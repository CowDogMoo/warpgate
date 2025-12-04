/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"gopkg.in/yaml.v3"
)

// Loader handles loading and parsing template configurations
type Loader struct{}

// NewLoader creates a new config loader
func NewLoader() *Loader {
	return &Loader{}
}

// LoadFromFile loads a template configuration from a YAML file
func (l *Loader) LoadFromFile(path string) (*builder.Config, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Parse YAML
	var config builder.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Resolve relative paths based on config file location
	baseDir := filepath.Dir(path)
	l.resolveRelativePaths(&config, baseDir)

	return &config, nil
}

// resolveRelativePaths converts relative paths in the config to absolute paths
func (l *Loader) resolveRelativePaths(config *builder.Config, baseDir string) {
	for i := range config.Provisioners {
		prov := &config.Provisioners[i]

		switch prov.Type {
		case "ansible":
			if prov.PlaybookPath != "" && !filepath.IsAbs(prov.PlaybookPath) {
				prov.PlaybookPath = filepath.Join(baseDir, prov.PlaybookPath)
			}
			if prov.GalaxyFile != "" && !filepath.IsAbs(prov.GalaxyFile) {
				prov.GalaxyFile = filepath.Join(baseDir, prov.GalaxyFile)
			}
			if prov.Inventory != "" && !filepath.IsAbs(prov.Inventory) {
				prov.Inventory = filepath.Join(baseDir, prov.Inventory)
			}

		case "script":
			for j := range prov.Scripts {
				if !filepath.IsAbs(prov.Scripts[j]) {
					prov.Scripts[j] = filepath.Join(baseDir, prov.Scripts[j])
				}
			}

		case "powershell":
			for j := range prov.PSScripts {
				if !filepath.IsAbs(prov.PSScripts[j]) {
					prov.PSScripts[j] = filepath.Join(baseDir, prov.PSScripts[j])
				}
			}
		}
	}
}

// LoadFromYAML loads a template configuration from YAML bytes
func (l *Loader) LoadFromYAML(data []byte) (*builder.Config, error) {
	var config builder.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

// SaveToFile saves a template configuration to a YAML file
func (l *Loader) SaveToFile(config *builder.Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
