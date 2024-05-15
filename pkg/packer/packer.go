package packer

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// BlueprintAMI represents the AMI configuration for a Packer template.
//
// **Attributes:**
//
// InstanceType: Instance type to use for the AMI build.
// Region: AWS region to build the AMI in.
// SSHUser: SSH user to use for the AMI build.
type BlueprintAMI struct {
	InstanceType string `mapstructure:"instance_type"`
	Region       string `mapstructure:"region"`
	SSHUser      string `mapstructure:"ssh_user"`
}

// BlueprintPacker represents a Packer template associated with a blueprint.
//
// **Attributes:**
//
// AMI: Optional AMI configuration.
// Base: Base image configuration for the template.
// Container: Container configuration for the template.
// ImageHashes: Hashes of the image layers for the container.
// Name: Name of the Packer template.
// Systemd: Indicates if systemd is used in the container.
// Tag: Tag configuration for the generated image.
// User: User to run commands as in the container.
type BlueprintPacker struct {
	AMI         BlueprintAMI       `mapstructure:"ami,omitempty"`
	Base        BlueprintBase      `mapstructure:"base"`
	Container   BlueprintContainer `mapstructure:"container"`
	ImageHashes map[string]string  `mapstructure:"image_hashes"`
	Systemd     bool               `mapstructure:"systemd"`
	Tag         BlueprintTag       `mapstructure:"tag"`
	User        string             `mapstructure:"user"`
}

// BlueprintBase represents the base image configuration for a Packer template.
//
// **Attributes:**
//
// Name: Name of the base image.
// Version: Version of the base image.
type BlueprintBase struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

// BlueprintTag represents the tag configuration for the image built by Packer.
//
// **Attributes:**
//
// Name: Name of the tag.
// Version: Version of the tag.
type BlueprintTag struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

// BlueprintContainer represents the container configuration for a Packer template.
//
// **Attributes:**
//
// Entrypoint: Entrypoint for the container.
// Registry: Container registry configuration.
// Workdir: Working directory in the container.
type BlueprintContainer struct {
	Entrypoint string            `mapstructure:"entrypoint"`
	Registry   BlueprintRegistry `mapstructure:"registry"`
	Workdir    string            `mapstructure:"workdir"`
}

// BlueprintRegistry represents the container registry configuration for a Packer template.
//
// **Attributes:**
//
// Credential: Credential (e.g., password or token) for authentication with the registry.
// Server: Server URL of the container registry.
// Username: Username for authentication with the registry.
type BlueprintRegistry struct {
	Credential string `mapstructure:"credential"`
	Server     string `mapstructure:"server"`
	Username   string `mapstructure:"username"`
}

// LoadPackerTemplates loads Packer templates from the configuration file.
//
// **Returns:**
//
// []BlueprintPacker: A slice of Packer templates.
// error: An error if any issue occurs while loading the Packer templates.
func LoadPackerTemplates() ([]BlueprintPacker, error) {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return nil, fmt.Errorf("no config file used by viper")
	}
	fmt.Printf("Config file used by viper: %s\n", configFile)
	// Unmarshalling existing packer templates
	var packerTemplates []BlueprintPacker
	if err := viper.UnmarshalKey("packer_templates", &packerTemplates); err != nil {
		return nil, fmt.Errorf("failed to unmarshal packer templates: %v", err)
	}

	if len(packerTemplates) == 0 {
		return nil, fmt.Errorf("no packer templates found")
	}

	// Check and load AMI settings if available
	for i, tmpl := range packerTemplates {
		var amiConfig BlueprintAMI
		if err := viper.UnmarshalKey(fmt.Sprintf("packer_templates.%d.ami", i), &amiConfig); err == nil {
			tmpl.AMI = amiConfig
			packerTemplates[i] = tmpl // Update the templates slice with the AMI settings
		}
	}

	return packerTemplates, nil
}

// ParseImageHashes extracts the image hashes from the output of a Packer build
// command and updates the provided Packer blueprint with the new hashes.
//
// **Parameters:**
//
// output: The output from the Packer build command.
func (p *BlueprintPacker) ParseImageHashes(output string) {
	if strings.Contains(output, "Imported Docker image: sha256:") {
		parts := strings.Split(output, " ")
		for i, part := range parts {
			if part == "sha256:" && i+1 < len(parts) {
				hash := parts[i+1]
				if p.ImageHashes == nil {
					p.ImageHashes = make(map[string]string)
				}
				p.ImageHashes["docker"] = hash
			}
		}
	}
}
