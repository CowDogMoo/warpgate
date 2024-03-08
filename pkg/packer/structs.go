package packer

// BlueprintPacker represents a Packer template associated with a blueprint.
//
// **Attributes:**
//
// Name: Name of the Packer template.
// Container: Container configuration for the template.
// Base: Base image configuration for the template.
// Tag: Tag configuration for the generated image.
// Systemd: Indicates if systemd is used in the container.
type BlueprintPacker struct {
	Name      string             `yaml:"name"`
	Container BlueprintContainer `yaml:"container"`
	Base      BlueprintBase      `yaml:"base"`
	Tag       BlueprintTag       `yaml:"tag"`
	Systemd   bool               `yaml:"systemd"`
}

// BlueprintBase represents the base image configuration for a Packer template.
//
// **Attributes:**
//
// Name: Name of the base image.
// Version: Version of the base image.
type BlueprintBase struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// BlueprintTag represents the tag configuration for the image built by Packer.
//
// **Attributes:**
//
// Name: Name of the tag.
// Version: Version of the tag.
type BlueprintTag struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// BlueprintContainer represents the container configuration for a Packer template.
//
// **Attributes:**
//
// Workdir: Working directory in the container.
// User: User to run commands as in the container.
// Entrypoint: Entrypoint for the container.
// Registry: Container registry configuration.
type BlueprintContainer struct {
	Workdir    string            `yaml:"container.workdir"`
	User       string            `yaml:"container.user"`
	Entrypoint string            `yaml:"container.entrypoint"`
	Registry   BlueprintRegistry `yaml:"container.registry"`
}

// BlueprintRegistry represents the container registry configuration for a Packer template.
//
// **Attributes:**
//
// Server: Server URL of the container registry.
// Username: Username for authentication with the registry.
// Credential: Credential (e.g., password or token) for authentication with the registry.
type BlueprintRegistry struct {
	Server     string `yaml:"registry.server"`
	Username   string `yaml:"registry.username"`
	Credential string `yaml:"registry.credential"`
}
