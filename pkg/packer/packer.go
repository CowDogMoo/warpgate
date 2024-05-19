package packer

import (
	"fmt"
	"regexp"
	"strings"
)

// AMI represents the AMI configuration for a Packer template.
//
// **Attributes:**
//
// InstanceType: Instance type to use for the AMI build.
// Region: AWS region to build the AMI in.
// SSHUser: SSH user to use for the AMI build.
type AMI struct {
	InstanceType string `mapstructure:"instance_type"`
	Region       string `mapstructure:"region"`
	SSHUser      string `mapstructure:"ssh_user"`
}

// Container represents the container configuration for a Packer template.
//
// **Attributes:**
//
// BaseImageValues: Name and version of the base image.
// Entrypoint: Entrypoint for the container.
// ImageHashes: Hashes of the images built by Packer.
// Registry: Container registry configuration.
// Workdir: Working directory in the container.
// DockerClient: Docker client to use for the container.
type Container struct {
	BaseImageValues ImageValues            `mapstructure:"base_image_values"`
	Entrypoint      string                 `mapstructure:"entrypoint"`
	ImageHashes     map[string]string      `mapstructure:"image_hashes"`
	Registry        ContainerImageRegistry `mapstructure:"registry"`
	Workdir         string                 `mapstructure:"workdir"`
	DockerClient    string                 `mapstructure:"docker_client"`
}

// ContainerImageRegistry represents the container registry configuration for a Packer template.
//
// **Attributes:**
//
// Credential: Credential (e.g., password or token) for authentication with the registry.
// Server: Server URL of the container registry.
// Username: Username for authentication with the registry.
type ContainerImageRegistry struct {
	Credential string `mapstructure:"credential"`
	Server     string `mapstructure:"server"`
	Username   string `mapstructure:"username"`
}

// ImageValues provides the name and version of an image.
//
// **Attributes:**
//
// Name: Name of the base image.
// Version: Version of the base image.
type ImageValues struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

// PackerTemplate represents a Packer template associated with a blueprint.
//
// **Attributes:**
//
// AMI: Optional AMI configuration.
// Container: Container configuration for the template.
// ImageValues: Name and version of the image.
// Systemd: Indicates if systemd is used in the container.
// User: User to run commands as in the container.
type PackerTemplate struct {
	AMI         AMI         `mapstructure:"ami,omitempty"`
	Container   Container   `mapstructure:"container,omitempty"`
	ImageValues ImageValues `mapstructure:"image_values"`
	Systemd     bool        `mapstructure:"systemd"`
	User        string      `mapstructure:"user"`
}

// ParseImageHashes extracts the image hashes from the output of a Packer build
// command and updates the provided Packer blueprint with the new hashes.
//
// **Parameters:**
//
// output: The output from the Packer build command.
//
// **Returns:**
//
// map[string]string: A map of image hashes parsed from the build output.
func (p *PackerTemplate) ParseImageHashes(output string) map[string]string {
	if p.Container.ImageHashes == nil {
		p.Container.ImageHashes = make(map[string]string)
	}

	// Regular expression to match and remove ANSI escape sequences
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	cleanOutput := re.ReplaceAllString(output, "")

	lines := strings.Split(cleanOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Imported Docker image: sha256:") {
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				hash := strings.TrimPrefix(parts[len(parts)-1], "sha256:")
				archParts := strings.Split(parts[1], ".")
				if len(archParts) > 1 {
					arch := strings.TrimSuffix(archParts[1], ":")
					p.Container.ImageHashes[arch] = hash
					fmt.Printf("Updated ImageHashes: %v\n", p.Container.ImageHashes)
				}
			}
		}
	}

	return p.Container.ImageHashes
}

// ParseAMIDetails extracts the AMI ID from the output of a Packer build command.
//
// **Parameters:**
//
// output: The output from the Packer build command.
//
// **Returns:**
//
// string: The AMI ID if found in the output.
func (p *PackerTemplate) ParseAMIDetails(output string) string {
	if strings.Contains(output, "AMI:") {
		parts := strings.Split(output, " ")
		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "AMI:" {
				return parts[i+1]
			}
		}
	}
	return ""
}
