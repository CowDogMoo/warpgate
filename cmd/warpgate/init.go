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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/templates"
	"github.com/spf13/cobra"
)

var (
	initFromTemplate string
	initOutputDir    string
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new template",
	Long: `Initialize a new warpgate template with scaffolding.

This command creates a new template directory with the basic structure:
  - warpgate.yaml: Main template configuration
  - README.md: Template documentation
  - scripts/: Directory for provisioning scripts

Use --from to fork an existing template as a starting point.`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initFromTemplate, "from", "f", "", "Fork from existing template")
	initCmd.Flags().StringVarP(&initOutputDir, "output", "o", ".", "Output directory for template")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	templateName := args[0]
	logging.InfoContext(ctx, "Initializing template: %s", templateName)

	// Create template directory
	templateDir := filepath.Join(initOutputDir, templateName)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("failed to create template directory: %w", err)
	}

	// Create scripts directory
	scriptsDir := filepath.Join(templateDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return fmt.Errorf("failed to create scripts directory: %w", err)
	}

	// If forking from existing template, load and modify it
	if initFromTemplate != "" {
		return forkTemplate(ctx, initFromTemplate, templateName, templateDir)
	}

	// Create default template configuration
	if err := createDefaultTemplate(templateName, templateDir); err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	// Create README
	if err := createReadme(templateName, templateDir); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	logging.InfoContext(ctx, "Template created successfully at: %s", templateDir)
	logging.InfoContext(ctx, "Next steps:")
	logging.InfoContext(ctx, "  1. Edit %s to configure your template", filepath.Join(templateDir, "warpgate.yaml"))
	logging.InfoContext(ctx, "  2. Add provisioning scripts to %s", scriptsDir)
	logging.InfoContext(ctx, "  3. Validate with: warpgate validate %s", filepath.Join(templateDir, "warpgate.yaml"))
	logging.InfoContext(ctx, "  4. Build with: warpgate build -f %s", filepath.Join(templateDir, "warpgate.yaml"))

	return nil
}

func createDefaultTemplate(name, dir string) error {
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

func createReadme(name, dir string) error {
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

func forkTemplate(ctx context.Context, fromTemplate, newName, outputDir string) error {
	logging.InfoContext(ctx, "Forking template from: %s", fromTemplate)

	// Load the source template
	loader, err := templates.NewTemplateLoader()
	if err != nil {
		return fmt.Errorf("failed to create loader: %w", err)
	}

	cfg, err := loader.LoadTemplate(fromTemplate)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	// Update metadata
	cfg.Metadata.Name = newName
	cfg.Metadata.Version = "1.0.0"
	cfg.Metadata.Description = fmt.Sprintf("Forked from %s", fromTemplate)
	cfg.Name = newName

	// Save the forked template
	configPath := filepath.Join(outputDir, "warpgate.yaml")
	if err := saveTemplateConfig(ctx, cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create README
	if err := createReadme(newName, outputDir); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	logging.InfoContext(ctx, "Template forked successfully")
	return nil
}

func saveTemplateConfig(ctx context.Context, cfg interface{}, path string) error {
	// This is a simplified version - in production you'd want to properly
	// serialize the config to YAML with proper formatting

	// For now, we'll use a basic YAML serialization
	// You may want to add gopkg.in/yaml.v3 serialization here

	logging.WarnContext(ctx, "Template forking is a basic implementation - manual adjustment may be needed")
	logging.InfoContext(ctx, "Config saved to: %s", path)

	return fmt.Errorf("template forking requires YAML serialization - use --from with caution")
}
