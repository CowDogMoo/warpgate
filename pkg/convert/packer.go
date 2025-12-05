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
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
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
	options      PackerConverterOptions
	globalConfig *globalconfig.Config
}

// NewPackerConverter creates a new Packer converter
func NewPackerConverter(opts PackerConverterOptions) (*PackerConverter, error) {
	// Load global config
	globalCfg, err := globalconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	return &PackerConverter{
		options:      opts,
		globalConfig: globalCfg,
	}, nil
}

// Convert performs the conversion from Packer to Warpgate format
func (c *PackerConverter) Convert() (*builder.Config, error) {
	logging.Info("Starting Packer template conversion for: %s", c.options.TemplateDir)

	// Extract template name from directory
	templateName := filepath.Base(c.options.TemplateDir)

	// Parse README for description
	description := c.extractDescription()

	// Create HCL parser
	hclParser := NewHCLParser()

	// Parse variables file for base image info
	baseImage, baseVersion := c.extractBaseImageHCL(hclParser)
	if c.options.BaseImage != "" {
		baseImage = c.options.BaseImage
		baseVersion = "latest"
	}

	// Parse docker.pkr.hcl for container provisioners using HCL parser
	dockerProvisioners := c.parseDockerProvisionersHCL(hclParser)

	// Parse ami.pkr.hcl for AMI-specific provisioners using HCL parser
	amiProvisioners := c.parseAMIProvisionersHCL(hclParser)

	// Parse post-processors from both docker and AMI builds
	allBuilds, _ := hclParser.ParseBuildFile(filepath.Join(c.options.TemplateDir, "docker.pkr.hcl"))
	amiBuilds, _ := hclParser.ParseBuildFile(filepath.Join(c.options.TemplateDir, "ami.pkr.hcl"))
	allBuilds = append(allBuilds, amiBuilds...)
	postProcessors := c.convertHCLPostProcessors(allBuilds)

	// Merge provisioners (prefer docker as it's usually more complete)
	provisioners := dockerProvisioners
	if len(provisioners) == 0 {
		provisioners = amiProvisioners
	}

	// Use config defaults if options are not provided
	version := c.options.Version
	if version == "" {
		version = c.globalConfig.Convert.DefaultVersion
	}

	license := c.options.License
	if license == "" {
		license = c.globalConfig.Convert.DefaultLicense
	}

	// Build config
	config := &builder.Config{
		Metadata: builder.Metadata{
			Name:        templateName,
			Version:     version,
			Description: description,
			Author:      c.options.Author,
			License:     license,
			Requires: builder.Requirements{
				Warpgate: c.globalConfig.Convert.WarpgateVersion,
			},
		},
		Name:    templateName,
		Version: "latest",
		Base: builder.BaseImage{
			Image: fmt.Sprintf("%s:%s", baseImage, baseVersion),
			Pull:  true,
		},
		Provisioners:   provisioners,
		PostProcessors: postProcessors,
		Targets:        c.buildTargets(),
	}

	logging.Info("Conversion complete: %d provisioners, %d post-processors, %d targets", len(provisioners), len(postProcessors), len(config.Targets))

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

// buildTargets creates target configurations
func (c *PackerConverter) buildTargets() []builder.Target {
	var targets []builder.Target

	// Add container target with platforms from config
	targets = append(targets, builder.Target{
		Type:      "container",
		Platforms: c.globalConfig.Container.DefaultPlatforms,
	})

	// Add AMI target if requested
	if c.options.IncludeAMI {
		// Use convert-specific AMI settings, falling back to general AWS AMI settings
		instanceType := c.globalConfig.Convert.AMIInstanceType
		if instanceType == "" {
			instanceType = c.globalConfig.AWS.AMI.InstanceType
		}

		volumeSize := c.globalConfig.Convert.AMIVolumeSize
		if volumeSize == 0 {
			volumeSize = c.globalConfig.AWS.AMI.VolumeSize
		}

		targets = append(targets, builder.Target{
			Type:         "ami",
			Region:       c.globalConfig.AWS.Region,
			InstanceType: instanceType,
			VolumeSize:   volumeSize,
		})
	}

	return targets
}

// extractBaseImageHCL uses HCL parser to extract base image from variables
func (c *PackerConverter) extractBaseImageHCL(parser *HCLParser) (string, string) {
	varsPath := filepath.Join(c.options.TemplateDir, "variables.pkr.hcl")

	// Try to parse with HCL
	err := parser.ParseVariablesFile(varsPath)
	if err != nil {
		logging.Debug("Failed to parse variables with HCL, using defaults: %v", err)
		return c.globalConfig.Container.DefaultBaseImage, c.globalConfig.Container.DefaultBaseVersion
	}

	// Extract base_image and base_image_version
	baseImage := c.globalConfig.Container.DefaultBaseImage
	baseVersion := c.globalConfig.Container.DefaultBaseVersion

	if v, ok := parser.GetVariable("base_image"); ok && v.Default != "" {
		baseImage = v.Default
	}

	if v, ok := parser.GetVariable("base_image_version"); ok && v.Default != "" {
		baseVersion = v.Default
	}

	return baseImage, baseVersion
}

// parseDockerProvisionersHCL uses HCL parser to extract provisioners from docker.pkr.hcl
func (c *PackerConverter) parseDockerProvisionersHCL(parser *HCLParser) []builder.Provisioner {
	dockerPath := filepath.Join(c.options.TemplateDir, "docker.pkr.hcl")

	builds, err := parser.ParseBuildFile(dockerPath)
	if err != nil {
		logging.Debug("Failed to parse docker.pkr.hcl with HCL: %v", err)
		return nil
	}

	return c.convertHCLProvisioners(builds)
}

// parseAMIProvisionersHCL uses HCL parser to extract provisioners from ami.pkr.hcl
func (c *PackerConverter) parseAMIProvisionersHCL(parser *HCLParser) []builder.Provisioner {
	amiPath := filepath.Join(c.options.TemplateDir, "ami.pkr.hcl")

	builds, err := parser.ParseBuildFile(amiPath)
	if err != nil {
		logging.Debug("Failed to parse ami.pkr.hcl with HCL: %v", err)
		return nil
	}

	return c.convertHCLProvisioners(builds)
}

// convertHCLProvisioners converts HCL provisioners to Warpgate provisioners
func (c *PackerConverter) convertHCLProvisioners(builds []PackerBuild) []builder.Provisioner {
	var provisioners []builder.Provisioner

	for _, build := range builds {
		for _, hclProv := range build.Provisioners {
			var prov builder.Provisioner

			// Set common fields
			prov.Type = hclProv.Type
			prov.Only = hclProv.Only
			prov.Except = hclProv.Except
			prov.User = hclProv.User

			switch hclProv.Type {
			case "shell":
				prov.Inline = hclProv.Inline
				// Convert single script to scripts array
				if hclProv.Script != "" {
					prov.Scripts = []string{hclProv.Script}
				} else if len(hclProv.Scripts) > 0 {
					prov.Scripts = hclProv.Scripts
				}
			case "ansible":
				prov.PlaybookPath = hclProv.PlaybookFile
				prov.GalaxyFile = hclProv.GalaxyFile
				prov.InventoryFile = hclProv.InventoryFile
				prov.AnsibleEnvVars = hclProv.AnsibleEnvVars
				prov.CollectionsPath = hclProv.CollectionsPath
				prov.UseProxy = hclProv.UseProxy

				// Parse extra_arguments into ExtraVars
				prov.ExtraVars = c.parseAnsibleExtraArgs(hclProv.ExtraArguments)
			}

			provisioners = append(provisioners, prov)
		}
	}

	return provisioners
}

// convertHCLPostProcessors converts HCL post-processors to Warpgate post-processors
func (c *PackerConverter) convertHCLPostProcessors(builds []PackerBuild) []builder.PostProcessor {
	var postProcessors []builder.PostProcessor

	for _, build := range builds {
		for _, hclPost := range build.PostProcessors {
			postProc := builder.PostProcessor{
				Type:   hclPost.Type,
				Only:   hclPost.Only,
				Except: hclPost.Except,
			}

			switch hclPost.Type {
			case "manifest":
				postProc.Output = hclPost.Output
				postProc.StripPath = hclPost.StripPath
			case "docker-tag":
				postProc.Repository = hclPost.Repository
				postProc.Tags = hclPost.Tags
				postProc.Force = hclPost.Force
			}

			postProcessors = append(postProcessors, postProc)
		}
	}

	return postProcessors
}

// parseAnsibleExtraArgs parses Ansible extra_arguments array into a map
func (c *PackerConverter) parseAnsibleExtraArgs(args []string) map[string]string {
	extraVars := make(map[string]string)

	for i := 0; i < len(args); i++ {
		// Look for -e or --extra-vars flag
		if args[i] == "-e" || args[i] == "--extra-vars" {
			if i+1 < len(args) {
				i++
				// Parse key=value format
				if parts := strings.SplitN(args[i], "=", 2); len(parts) == 2 {
					extraVars[parts[0]] = parts[1]
				}
			}
		}
	}

	return extraVars
}
