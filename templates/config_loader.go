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

// Package config provides functionality for loading and parsing template configurations.
package templates

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/errors"
	"gopkg.in/yaml.v3"
)

// Loader handles loading and parsing template configurations
type Loader struct{}

// NewLoader creates a new config loader
func NewLoader() *Loader {
	return &Loader{}
}

// LoadFromFileWithVars loads a template configuration from a YAML file with variable substitution
// Variables from the vars map take precedence over environment variables
func (l *Loader) LoadFromFileWithVars(path string, vars map[string]string) (*builder.Config, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap("read config file", path, err)
	}

	// Expand variables in the YAML content
	// Precedence: CLI vars > Environment variables
	expandedData := l.expandVariables(string(data), vars)

	// Parse YAML
	var config builder.Config
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, errors.Wrap("parse config file", path, err)
	}

	// Resolve relative paths based on config file location
	baseDir := filepath.Dir(path)
	// Make baseDir absolute if it's not already
	if !filepath.IsAbs(baseDir) {
		absBaseDir, err := filepath.Abs(baseDir)
		if err != nil {
			return nil, errors.Wrap("resolve base directory", "", err)
		}
		baseDir = absBaseDir
	}
	l.resolveRelativePaths(&config, baseDir)

	return &config, nil
}

// expandVariables expands ${VAR} style variables only
// Variables from the vars map take precedence over environment variables
// $VAR syntax (without braces) is left untouched for container-level expansion
func (l *Loader) expandVariables(s string, vars map[string]string) string {
	result := strings.Builder{}
	result.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			// Found ${VAR} syntax - expand it
			end := strings.IndexByte(s[i+2:], '}')
			if end >= 0 {
				varName := s[i+2 : i+2+end]
				var value string
				if vars != nil {
					if v, ok := vars[varName]; ok {
						value = v
					}
				}
				if value == "" {
					value = os.Getenv(varName)
				}
				value = MustExpandPath(value)
				result.WriteString(value)
				i += end + 2 // skip past ${varName}
				continue
			}
		}
		// Not a ${VAR} pattern, keep the character as-is
		result.WriteByte(s[i])
	}

	return result.String()
}

// resolveRelativePaths converts relative paths in the config to absolute paths
func (l *Loader) resolveRelativePaths(config *builder.Config, baseDir string) {
	// Resolve Dockerfile paths if using Dockerfile mode
	l.resolveDockerfilePaths(config, baseDir)

	// Resolve provisioner paths
	for i := range config.Provisioners {
		l.resolveProvisionerPaths(&config.Provisioners[i], baseDir)
	}
}

// resolveDockerfilePaths resolves Dockerfile-related paths
func (l *Loader) resolveDockerfilePaths(config *builder.Config, baseDir string) {
	if config.Dockerfile == nil {
		return
	}
	if config.Dockerfile.Path != "" {
		config.Dockerfile.Path = MustExpandPath(config.Dockerfile.Path)
		if !filepath.IsAbs(config.Dockerfile.Path) {
			config.Dockerfile.Path = filepath.Join(baseDir, config.Dockerfile.Path)
		}
	}
	if config.Dockerfile.Context != "" {
		config.Dockerfile.Context = MustExpandPath(config.Dockerfile.Context)
		if !filepath.IsAbs(config.Dockerfile.Context) {
			config.Dockerfile.Context = filepath.Join(baseDir, config.Dockerfile.Context)
		}
	}
}

// resolveProvisionerPaths resolves paths within a single provisioner
func (l *Loader) resolveProvisionerPaths(prov *builder.Provisioner, baseDir string) {
	switch prov.Type {
	case "ansible":
		l.resolveAnsiblePaths(prov, baseDir)
	case "script":
		l.resolveScriptPaths(prov, baseDir)
	case "powershell":
		l.resolvePowerShellPaths(prov, baseDir)
	}
}

// resolvePathList converts paths to absolute. Absolute paths are left unchanged.
func resolvePathList(paths []string, baseDir string) {
	for i := range paths {
		if paths[i] != "" {
			paths[i] = MustExpandPath(paths[i])
			if !filepath.IsAbs(paths[i]) {
				paths[i] = filepath.Join(baseDir, paths[i])
			}
		}
	}
}

// resolveSinglePath converts path to absolute. Absolute paths are left unchanged.
func resolveSinglePath(path *string, baseDir string) {
	if *path != "" {
		*path = MustExpandPath(*path)
		if !filepath.IsAbs(*path) {
			*path = filepath.Join(baseDir, *path)
		}
	}
}

// resolveAnsiblePaths resolves Ansible provisioner paths
func (l *Loader) resolveAnsiblePaths(prov *builder.Provisioner, baseDir string) {
	resolveSinglePath(&prov.PlaybookPath, baseDir)
	resolveSinglePath(&prov.GalaxyFile, baseDir)
	resolveSinglePath(&prov.Inventory, baseDir)
}

// resolveScriptPaths resolves script provisioner paths
func (l *Loader) resolveScriptPaths(prov *builder.Provisioner, baseDir string) {
	resolvePathList(prov.Scripts, baseDir)
}

// resolvePowerShellPaths resolves PowerShell provisioner paths
func (l *Loader) resolvePowerShellPaths(prov *builder.Provisioner, baseDir string) {
	resolvePathList(prov.PSScripts, baseDir)
}

// LoadFromYAML loads a template configuration from YAML bytes
func (l *Loader) LoadFromYAML(data []byte) (*builder.Config, error) {
	var config builder.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap("parse YAML", "", err)
	}

	return &config, nil
}

// SaveToFile saves a template configuration to a YAML file
func (l *Loader) SaveToFile(config *builder.Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrap("marshal config", "", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.Wrap("write config file", path, err)
	}

	return nil
}
