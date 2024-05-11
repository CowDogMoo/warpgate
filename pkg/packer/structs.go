/*
Copyright © 2024-present, Jayson Grace <jayson.e.grace@gmail.com>

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
package packer

// BlueprintAMI represents the AMI configuration for a Packer template.
//
// **Attributes:**
//
// AMITag: Tag to apply to the AMI.
// InstanceType: Instance type to use for the AMI build.
// Region: AWS region to build the AMI in.
// SourceARN: ARN of the source AMI to use as a base.
type BlueprintAMI struct {
	AMITag       string `mapstructure:"ami_tag"`
	InstanceType string `mapstructure:"instance_type"`
	Region       string `mapstructure:"region"`
	SourceARN    string `mapstructure:"source_arn"`
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
type BlueprintPacker struct {
	AMI         BlueprintAMI       `mapstructure:"ami,omitempty"`
	Base        BlueprintBase      `mapstructure:"base"`
	Container   BlueprintContainer `mapstructure:"container"`
	ImageHashes map[string]string  `mapstructure:"image_hashes"`
	Name        string             `mapstructure:"name"`
	Systemd     bool               `mapstructure:"systemd"`
	Tag         BlueprintTag       `mapstructure:"tag"`
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
// User: User to run commands as in the container.
// Workdir: Working directory in the container.
type BlueprintContainer struct {
	Entrypoint string            `mapstructure:"entrypoint"`
	Registry   BlueprintRegistry `mapstructure:"registry"`
	User       string            `mapstructure:"user"`
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
