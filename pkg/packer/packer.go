package packer

import (
	"fmt"
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
// Entrypoint: Entrypoint for the container.
// Registry: Container registry configuration.
// Workdir: Working directory in the container.
type Container struct {
	BaseImageValues ImageValues            `mapstructure:"base_image_values"`
	Entrypoint      string                 `mapstructure:"entrypoint"`
	ImageHashes     map[string]string      `mapstructure:"image_hashes"`
	Registry        ContainerImageRegistry `mapstructure:"registry"`
	Workdir         string                 `mapstructure:"workdir"`
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
// Base: Base image configuration for the template.
// Container: Container configuration for the template.
// ImageHashes: Hashes of the image layers for the container.
// Name: Name of the Packer template.
// Systemd: Indicates if systemd is used in the container.
// Tag: Tag configuration for the generated image.
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
func (p *PackerTemplate) ParseImageHashes(output string) {
	if p.Container.ImageHashes == nil {
		p.Container.ImageHashes = make(map[string]string)
	}

	if strings.Contains(output, "Imported Docker image: sha256:") {
		parts := strings.Split(output, " ")
		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "sha256:" {
				hash := parts[i+1]
				if i > 0 {
					archParts := strings.Split(parts[i-1], ".")
					if len(archParts) > 1 {
						arch := strings.TrimSuffix(archParts[1], ":")
						p.Container.ImageHashes[arch] = hash
						fmt.Printf("Updated ImageHashes: %v\n", p.Container.ImageHashes)
					}
				}
			}
		}
	}
}
