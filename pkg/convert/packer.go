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

package convert

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// PackerConverterOptions contains options for the Packer converter
type PackerConverterOptions struct {
	TemplateDir string
	Author      string
	License     string
	Version     string
	BaseImage   string
	IncludeAMI  bool
}

// PackerConverter converts Packer HCL templates to Warpgate YAML
type PackerConverter struct {
	options PackerConverterOptions
}

// NewPackerConverter creates a new Packer converter
func NewPackerConverter(opts PackerConverterOptions) *PackerConverter {
	return &PackerConverter{
		options: opts,
	}
}

// Convert performs the conversion from Packer to Warpgate format
func (c *PackerConverter) Convert() (*builder.Config, error) {
	logging.Info("Starting Packer template conversion for: %s", c.options.TemplateDir)

	// Extract template name from directory
	templateName := filepath.Base(c.options.TemplateDir)

	// Parse README for description
	description := c.extractDescription()

	// Parse variables file for base image info
	baseImage, baseVersion := c.extractBaseImage()
	if c.options.BaseImage != "" {
		baseImage = c.options.BaseImage
		baseVersion = "latest"
	}

	// Parse docker.pkr.hcl for container provisioners
	dockerProvisioners := c.parseDockerProvisioners()

	// Parse ami.pkr.hcl for AMI-specific provisioners
	amiProvisioners := c.parseAMIProvisioners()

	// Merge provisioners (prefer docker as it's usually more complete)
	provisioners := dockerProvisioners
	if len(provisioners) == 0 {
		provisioners = amiProvisioners
	}

	// Build config
	config := &builder.Config{
		Metadata: builder.Metadata{
			Name:        templateName,
			Version:     c.options.Version,
			Description: description,
			Author:      c.options.Author,
			License:     c.options.License,
			Tags:        []string{"security", "red-team"},
			Requires: builder.Requirements{
				Warpgate: ">=1.0.0",
			},
		},
		Name:    templateName,
		Version: "latest",
		Base: builder.BaseImage{
			Image: fmt.Sprintf("%s:%s", baseImage, baseVersion),
			Pull:  true,
		},
		Provisioners: provisioners,
		Targets:      c.buildTargets(),
	}

	logging.Info("Conversion complete: %d provisioners, %d targets", len(provisioners), len(config.Targets))

	return config, nil
}

// extractDescription reads the README.md and extracts a description
func (c *PackerConverter) extractDescription() string {
	readmePath := filepath.Join(c.options.TemplateDir, "README.md")

	file, err := os.Open(readmePath)
	if err != nil {
		logging.Debug("No README.md found, using default description")
		return fmt.Sprintf("%s security tooling image", filepath.Base(c.options.TemplateDir))
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var description string
	foundFirstHeader := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip the first header line
		if strings.HasPrefix(line, "#") && !foundFirstHeader {
			foundFirstHeader = true
			continue
		}

		// Get the first non-empty paragraph after the header
		if foundFirstHeader && line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") {
			description = line
			break
		}
	}

	if description == "" {
		description = fmt.Sprintf("%s security tooling image", filepath.Base(c.options.TemplateDir))
	}

	return description
}

// extractBaseImage parses variables.pkr.hcl to extract base image info
func (c *PackerConverter) extractBaseImage() (string, string) {
	varsPath := filepath.Join(c.options.TemplateDir, "variables.pkr.hcl")

	content, err := os.ReadFile(varsPath)
	if err != nil {
		logging.Debug("No variables.pkr.hcl found, using defaults")
		return "ubuntu", "latest"
	}

	// Extract base_image variable default value
	baseImageRe := regexp.MustCompile(`variable\s+"base_image"\s+{[^}]*default\s*=\s*"([^"]+)"`)
	baseVersionRe := regexp.MustCompile(`variable\s+"base_image_version"\s+{[^}]*default\s*=\s*"([^"]+)"`)

	baseImage := "ubuntu"
	baseVersion := "latest"

	if matches := baseImageRe.FindSubmatch(content); len(matches) > 1 {
		baseImage = string(matches[1])
	}

	if matches := baseVersionRe.FindSubmatch(content); len(matches) > 1 {
		baseVersion = string(matches[1])
	}

	return baseImage, baseVersion
}

// parseDockerProvisioners parses docker.pkr.hcl for provisioner blocks
func (c *PackerConverter) parseDockerProvisioners() []builder.Provisioner {
	dockerPath := filepath.Join(c.options.TemplateDir, "docker.pkr.hcl")

	content, err := os.ReadFile(dockerPath)
	if err != nil {
		logging.Debug("No docker.pkr.hcl found")
		return nil
	}

	return c.parseProvisionersFromContent(string(content))
}

// parseAMIProvisioners parses ami.pkr.hcl for provisioner blocks
func (c *PackerConverter) parseAMIProvisioners() []builder.Provisioner {
	amiPath := filepath.Join(c.options.TemplateDir, "ami.pkr.hcl")

	content, err := os.ReadFile(amiPath)
	if err != nil {
		logging.Debug("No ami.pkr.hcl found")
		return nil
	}

	return c.parseProvisionersFromContent(string(content))
}

// parseProvisionersFromContent extracts provisioner blocks from HCL content
func (c *PackerConverter) parseProvisionersFromContent(content string) []builder.Provisioner {
	var provisioners []builder.Provisioner

	// Find all provisioner blocks using a more robust approach
	// Match provisioner "type" { ... } with proper brace counting
	lines := strings.Split(content, "\n")
	inProvisioner := false
	provType := ""
	provBody := ""
	braceCount := 0

	for _, line := range lines {
		// Check for provisioner start
		if !inProvisioner {
			provStartRe := regexp.MustCompile(`^\s*provisioner\s+"(\w+)"\s+\{`)
			if matches := provStartRe.FindStringSubmatch(line); len(matches) > 1 {
				inProvisioner = true
				provType = matches[1]
				provBody = ""
				braceCount = 1
				continue
			}
		}

		// If we're in a provisioner, accumulate the body
		if inProvisioner {
			// Count braces
			braceCount += strings.Count(line, "{")
			braceCount -= strings.Count(line, "}")

			provBody += line + "\n"

			// If braces are balanced, we've reached the end
			if braceCount == 0 {
				// Parse the provisioner
				switch provType {
				case "shell":
					if prov := c.parseShellProvisioner(provBody); prov != nil {
						provisioners = append(provisioners, *prov)
					}
				case "ansible":
					if prov := c.parseAnsibleProvisioner(provBody); prov != nil {
						provisioners = append(provisioners, *prov)
					}
				}
				inProvisioner = false
			}
		}
	}

	return provisioners
}

// parseShellProvisioner parses a shell provisioner block
func (c *PackerConverter) parseShellProvisioner(body string) *builder.Provisioner {
	// Extract inline commands
	inlineRe := regexp.MustCompile(`inline\s*=\s*\[((?:[^][]|\[[^]]*\])*)\]`)
	matches := inlineRe.FindStringSubmatch(body)

	if len(matches) < 2 {
		return nil
	}

	// Parse inline commands
	commandsStr := matches[1]
	commandRe := regexp.MustCompile(`"([^"]*)"`)
	commandMatches := commandRe.FindAllStringSubmatch(commandsStr, -1)

	var commands []string
	for _, cmdMatch := range commandMatches {
		if len(cmdMatch) > 1 {
			commands = append(commands, cmdMatch[1])
		}
	}

	if len(commands) == 0 {
		return nil
	}

	return &builder.Provisioner{
		Type:   "shell",
		Inline: commands,
	}
}

// parseAnsibleProvisioner parses an ansible provisioner block
func (c *PackerConverter) parseAnsibleProvisioner(body string) *builder.Provisioner {
	// Extract playbook_file
	playbookRe := regexp.MustCompile(`playbook_file\s*=\s*"([^"]*)"`)
	galaxyRe := regexp.MustCompile(`galaxy_file\s*=\s*"([^"]*)"`)

	playbookMatches := playbookRe.FindStringSubmatch(body)
	galaxyMatches := galaxyRe.FindStringSubmatch(body)

	if len(playbookMatches) < 2 {
		return nil
	}

	playbook := playbookMatches[1]
	galaxy := ""

	if len(galaxyMatches) > 1 {
		galaxy = galaxyMatches[1]
	}

	// Extract variable references (e.g., ${var.provision_repo_path})
	// Replace with environment variable placeholders
	playbook = c.replaceVarReferences(playbook)
	galaxy = c.replaceVarReferences(galaxy)

	prov := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
	}

	if galaxy != "" {
		prov.GalaxyFile = galaxy
	}

	// Extract extra_arguments for environment variables
	prov.ExtraVars = c.extractAnsibleExtraVars(body)

	return prov
}

// replaceVarReferences replaces Packer variable references with placeholders
func (c *PackerConverter) replaceVarReferences(s string) string {
	// Replace ${var.provision_repo_path} with ${PROVISION_REPO_PATH}
	varRe := regexp.MustCompile(`\$\{var\.([^}]+)\}`)
	return varRe.ReplaceAllStringFunc(s, func(match string) string {
		matches := varRe.FindStringSubmatch(match)
		if len(matches) > 1 {
			varName := strings.ToUpper(matches[1])
			return fmt.Sprintf("${%s}", varName)
		}
		return match
	})
}

// extractAnsibleExtraVars extracts extra variables from ansible provisioner
func (c *PackerConverter) extractAnsibleExtraVars(body string) map[string]string {
	extraVars := make(map[string]string)

	// Extract shell executable if present
	shellRe := regexp.MustCompile(`ansible_shell_executable=\$\{var\.shell\}`)
	if shellRe.MatchString(body) {
		extraVars["ansible_shell_executable"] = "/bin/bash"
	}

	return extraVars
}

// buildTargets creates target configurations
func (c *PackerConverter) buildTargets() []builder.Target {
	var targets []builder.Target

	// Add container target
	targets = append(targets, builder.Target{
		Type: "container",
		Platforms: []string{
			"linux/amd64",
			"linux/arm64",
		},
		Tags: []string{"latest"},
	})

	// Add AMI target if requested
	if c.options.IncludeAMI {
		targets = append(targets, builder.Target{
			Type:         "ami",
			Region:       "us-east-1",
			InstanceType: "t3.micro",
			VolumeSize:   50,
		})
	}

	return targets
}
