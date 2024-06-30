package packer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
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

// ImageHash represents the hash of an image built by Packer.
//
// **Attributes:**
//
// Arch: Architecture of the image.
// OS: Operating system of the image.
// Hash: Hash of the image.
type ImageHash struct {
	Arch string `mapstructure:"arch"`
	OS   string `mapstructure:"os"`
	Hash string
}

// Container represents the container configuration for a Packer template.
//
// **Attributes:**
//
// BaseImageValues: Name and version of the base image.
// DockerClient: Docker client to use for the container.
// Entrypoint: Entrypoint for the container.
// ImageHashes: Hashes of the images built by Packer.
// ImageRegistry: Container registry configuration.
// OperatingSystem: Operating system of the container.
// Workdir: Working directory in the container.
type Container struct {
	BaseImageValues ImageValues            `mapstructure:"base_image_values"`
	DockerClient    string                 `mapstructure:"docker_client"`
	Entrypoint      string                 `mapstructure:"entrypoint"`
	ImageHashes     []ImageHash            `mapstructure:"image_hashes"`
	ImageRegistry   ContainerImageRegistry `mapstructure:"registry"`
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

// PackerTemplate represents a collection of Packer templates consumed by a blueprint.
//
// **Attributes:**
//
// Container: Configuration for container images.
// ImageValues: Name and version of the image.
// User: User responsible for provisioning the blueprint, usually high-privilege (e.g., root or Administrator).
// AMI: Optional AMI configuration.
// Tag: Tag configuration for the image built by Packer.
type PackerTemplates struct {
	Container   Container   `mapstructure:"container,omitempty"`
	ImageValues ImageValues `mapstructure:"image_values"`
	User        string      `mapstructure:"user"`
	AMI         AMI         `mapstructure:"ami,omitempty"`
	Tag         Tag         `mapstructure:"tag"`
}

// Tag represents the tag configuration for the image built by Packer.
//
// **Attributes:**
//
// Name: Name of the tag.
// Version: Version of the tag.
type Tag struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
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
func (p *PackerTemplates) ParseAMIDetails(output string) string {
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

// ParseImageHashes extracts the image hashes from the output of a Packer build
// command and updates the provided PackerTemplates struct with the new hashes.
//
// **Parameters:**
//
// output: The output from the Packer build command.
//
// **Returns:**
//
// []ImageHash: A slice of ImageHash structs parsed from the build output.
func (p *PackerTemplates) ParseImageHashes(output string) []ImageHash {
	if p.Container.ImageHashes == nil {
		p.Container.ImageHashes = []ImageHash{}
	}

	imageHashesConfig := viper.Get("container.image_hashes").([]interface{})

	// Regular expression to match and remove ANSI escape sequences
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	cleanOutput := re.ReplaceAllString(output, "")

	lines := strings.Split(cleanOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Imported Docker image: sha256:") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				hash := strings.TrimPrefix(parts[len(parts)-1], "sha256:")
				archParts := strings.Split(parts[1], ".")
				if len(archParts) > 1 {
					arch := strings.TrimSuffix(archParts[1], ":")
					for _, ihConfig := range imageHashesConfig {
						ih := ihConfig.(map[string]interface{})
						if ih["arch"].(string) == arch {
							imageHash := ImageHash{
								Arch: arch,
								OS:   ih["os"].(string),
								Hash: hash,
							}
							// Ensure that the hash is not empty
							if imageHash.Hash != "" {
								p.Container.ImageHashes = append(p.Container.ImageHashes, imageHash)
								fmt.Printf("Updated ImageHashes: %v\n", p.Container.ImageHashes)
							} else {
								fmt.Printf("Warning: Skipping empty hash for arch: %s, os: %s\n", arch, ih["os"].(string))
							}
							break
						}
					}
				}
			}
		}
	}
	return p.Container.ImageHashes
}
