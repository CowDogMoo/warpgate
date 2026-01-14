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
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"gopkg.in/yaml.v3"
)

// Scaffolder creates new template projects with default structure and files.
type Scaffolder struct{}

// NewScaffolder creates a new template scaffolder.
func NewScaffolder() *Scaffolder {
	return &Scaffolder{}
}

// Create creates a new template with default structure.
func (s *Scaffolder) Create(ctx context.Context, name, outputDir string) error {
	templateDir := filepath.Join(outputDir, name)
	if err := os.MkdirAll(templateDir, config.DirPermReadWriteExec); err != nil {
		return fmt.Errorf("failed to create template directory: %w", err)
	}

	scriptsDir := filepath.Join(templateDir, "scripts")
	if err := os.MkdirAll(scriptsDir, config.DirPermReadWriteExec); err != nil {
		return fmt.Errorf("failed to create scripts directory: %w", err)
	}

	if err := s.createDefaultTemplate(name, templateDir); err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	if err := s.createReadme(name, templateDir); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	logging.InfoContext(ctx, "Template created successfully at: %s", templateDir)
	s.printNextSteps(ctx, templateDir, scriptsDir)

	return nil
}

// Fork creates a new template by copying and modifying an existing one.
func (s *Scaffolder) Fork(ctx context.Context, fromTemplate, newName, outputDir string) error {
	logging.InfoContext(ctx, "Forking template from: %s", fromTemplate)

	loader, err := NewTemplateLoader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create loader: %w", err)
	}

	cfg, err := loader.LoadTemplateWithVars(ctx, fromTemplate, nil)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	templateDir := filepath.Join(outputDir, newName)
	if err := os.MkdirAll(templateDir, config.DirPermReadWriteExec); err != nil {
		return fmt.Errorf("failed to create template directory: %w", err)
	}

	scriptsDir := filepath.Join(templateDir, "scripts")
	if err := os.MkdirAll(scriptsDir, config.DirPermReadWriteExec); err != nil {
		return fmt.Errorf("failed to create scripts directory: %w", err)
	}

	cfg.Metadata.Name = newName
	cfg.Metadata.Version = "1.0.0"
	cfg.Metadata.Description = fmt.Sprintf("Forked from %s", fromTemplate)
	cfg.Name = newName

	// Save the forked template
	configPath := filepath.Join(templateDir, "warpgate.yaml")
	if err := s.saveTemplateConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if err := s.createReadme(newName, templateDir); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	logging.InfoContext(ctx, "Template forked successfully")
	return nil
}

// createDefaultTemplate creates a default warpgate.yaml configuration file.
func (s *Scaffolder) createDefaultTemplate(name, dir string) error {
	configPath := filepath.Join(dir, "warpgate.yaml")

	template := fmt.Sprintf(`%smetadata:
  name: %s
  version: 1.0.0
  description: "A new warpgate template"
  author: ""
  license: MIT
  tags:
    - custom
  requires:
    warpgate: ">=1.0.0"

name: %s
version: latest

base:
  image: ubuntu:22.04
  pull: true

provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y curl
      - echo "Hello from %s!"

  # Example: Using a script file
  # - type: script
  #   path: scripts/setup.sh

  # Example: Using Ansible
  # - type: ansible
  #   playbook: playbooks/main.yaml
  #   galaxy_requirements: requirements.yaml

targets:
  - type: container
    registry: ghcr.io/yourorg
    tags:
      - latest
      - "{{.Version}}"
    platforms:
      - linux/amd64
      - linux/arm64

variables:
  example_var: "example_value"
`, builder.SchemaComment, name, name, name)

	if err := os.WriteFile(configPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// createReadme creates a README.md file for the template.
func (s *Scaffolder) createReadme(name, dir string) error {
	readmePath := filepath.Join(dir, "README.md")

	readme := fmt.Sprintf(`# %s

## Description

A warpgate template for building container images.

## Usage

### Build

Build the image locally:

`+"```bash"+`
warpgate build -f warpgate.yaml
`+"```"+`

### Build and Push

Build and push to registry:

`+"```bash"+`
warpgate build -f warpgate.yaml --push
`+"```"+`

### Customize

Edit `+"`warpgate.yaml`"+` to customize:

- Base image
- Provisioners (shell commands, scripts, Ansible playbooks)
- Target registries and tags
- Build platforms (architectures)

## Structure

- `+"`warpgate.yaml`"+` - Main template configuration
- `+"`scripts/`"+` - Provisioning scripts
- `+"`README.md`"+` - This file

## Requirements

- warpgate >= 1.0.0

## License

MIT
`, name)

	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	return nil
}

// saveTemplateConfig saves a template configuration to a YAML file.
func (s *Scaffolder) saveTemplateConfig(cfg *builder.Config, path string) error {
	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Prepend schema comment
	content := builder.SchemaComment + string(data)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// printNextSteps prints helpful next steps to the user.
func (s *Scaffolder) printNextSteps(ctx context.Context, templateDir, scriptsDir string) {
	logging.InfoContext(ctx, "Next steps:")
	logging.InfoContext(ctx, "  1. Edit %s to configure your template", filepath.Join(templateDir, "warpgate.yaml"))
	logging.InfoContext(ctx, "  2. Add provisioning scripts to %s", scriptsDir)
	logging.InfoContext(ctx, "  3. Validate with: warpgate validate %s", filepath.Join(templateDir, "warpgate.yaml"))
	logging.InfoContext(ctx, "  4. Build with: warpgate build %s", filepath.Join(templateDir, "warpgate.yaml"))
}
