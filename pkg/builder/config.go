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

package builder

// Version is the current version of warpgate, set at build time via ldflags.
// Build with: go build -ldflags "-X 'github.com/cowdogmoo/warpgate/pkg/builder.Version=v1.2.3'"
var Version = "dev"

// Config represents a template configuration for building images
type Config struct {
	// Metadata is the metadata about the template
	Metadata Metadata `yaml:"metadata" json:"metadata"`

	// Name is the name of the image
	Name string `yaml:"name" json:"name"`

	// Version is the version of the template
	Version string `yaml:"version" json:"version"`

	// Base is the base image configuration
	Base BaseImage `yaml:"base" json:"base"`

	// Provisioners is the list of provisioners to run during the build
	Provisioners []Provisioner `yaml:"provisioners" json:"provisioners"`

	// PostChanges applies Dockerfile-style instructions after provisioners complete
	// Useful for setting USER/WORKDIR after creating users during provisioning
	PostChanges []string `yaml:"post_changes,omitempty" json:"post_changes,omitempty"`

	// PostProcessors is the list of post-processors to run after the build
	PostProcessors []PostProcessor `yaml:"post_processors,omitempty" json:"post_processors,omitempty"`

	// Targets is the list of build targets (container, AMI, etc.)
	Targets []Target `yaml:"targets" json:"targets"`

	// ArchOverrides allows specifying different configurations per architecture
	ArchOverrides map[string]ArchOverride `yaml:"arch_overrides,omitempty" json:"arch_overrides,omitempty"`

	// Runtime overrides (not in YAML, set by CLI flags)
	Architectures []string `yaml:"-" json:"-"` // Architectures to build for
	Registry      string   `yaml:"-" json:"-"` // Registry to push to (overrides target registry)
}

// ArchOverride allows architecture-specific configuration
type ArchOverride struct {
	// Base is the override base image for this architecture
	Base *BaseImage `yaml:"base,omitempty" json:"base,omitempty"`

	// Provisioners is the list of additional or replacement provisioners for this architecture
	Provisioners []Provisioner `yaml:"provisioners,omitempty" json:"provisioners,omitempty"`

	// AppendProvisioners indicates whether to append provisioners or replace them entirely
	AppendProvisioners bool `yaml:"append_provisioners,omitempty" json:"append_provisioners,omitempty"`
}

// Metadata contains template metadata
type Metadata struct {
	// Name of the template (human-readable identifier)
	Name string `yaml:"name" json:"name"`

	// Version of the template (semantic versioning recommended, e.g., "1.0.0")
	Version string `yaml:"version" json:"version"`

	// Description provides a brief summary of what this template builds
	Description string `yaml:"description" json:"description"`

	// Author is the creator or maintainer of this template (name and/or email)
	Author string `yaml:"author" json:"author"`

	// License specifies the template's license (e.g., "MIT", "Apache-2.0")
	License string `yaml:"license" json:"license"`

	// Tags are keywords for categorizing and searching templates
	Tags []string `yaml:"tags" json:"tags"`

	// Requires specifies version requirements for warpgate and other dependencies
	Requires Requirements `yaml:"requires" json:"requires"`

	// Changelog documents version history and changes (optional)
	Changelog []ChangelogEntry `yaml:"changelog,omitempty" json:"changelog,omitempty"`

	// Extra allows storing arbitrary key-value metadata (optional)
	Extra map[string]string `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// Requirements specifies version requirements
type Requirements struct {
	// Warpgate specifies the minimum warpgate version required (e.g., ">=1.0.0")
	Warpgate string `yaml:"warpgate" json:"warpgate"`
}

// ChangelogEntry represents a changelog entry
type ChangelogEntry struct {
	// Version number for this changelog entry (e.g., "1.0.0")
	Version string `yaml:"version" json:"version"`

	// Date of the release in ISO 8601 format (e.g., "2025-01-15")
	Date string `yaml:"date" json:"date"`

	// Changes is a list of changes made in this version
	Changes []string `yaml:"changes" json:"changes"`
}

// BaseImage specifies the base image to start from
type BaseImage struct {
	// Image is the base container image reference (e.g., "ubuntu:22.04", "alpine:latest")
	Image string `yaml:"image" json:"image"`

	// Platform specifies the target platform (e.g., "linux/amd64", "linux/arm64")
	Platform string `yaml:"platform,omitempty" json:"platform,omitempty"`

	// Pull forces pulling the latest version of the base image (default: false)
	Pull bool `yaml:"pull,omitempty" json:"pull,omitempty"`

	// Auth provides authentication credentials for pulling from private registries
	Auth *ImageAuth `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Env sets environment variables in the base image
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Docker-specific options

	// Privileged runs the container with extended privileges (use with caution)
	Privileged bool `yaml:"privileged,omitempty" json:"privileged,omitempty"`

	// Volumes mounts host directories into the container (format: "host:container")
	Volumes map[string]string `yaml:"volumes,omitempty" json:"volumes,omitempty"`

	// RunCommand overrides the default container run command
	RunCommand []string `yaml:"run_command,omitempty" json:"run_command,omitempty"`

	// Changes applies Dockerfile-style instructions (ENV, USER, WORKDIR, ENTRYPOINT, CMD)
	Changes []string `yaml:"changes,omitempty" json:"changes,omitempty"`
}

// ImageAuth contains authentication information for pulling images
type ImageAuth struct {
	// Username is the username for registry authentication
	Username string `yaml:"username,omitempty" json:"username,omitempty"`

	// Password is the password or access token for registry authentication (use environment variables for security)
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Registry is the registry URL (e.g., "ghcr.io", "docker.io", "quay.io")
	Registry string `yaml:"registry,omitempty" json:"registry,omitempty"`
}

// Provisioner represents a provisioning step
type Provisioner struct {
	// Type is the type of provisioner: "shell", "ansible", "script", or "powershell"
	Type string `yaml:"type" json:"type"`

	// Conditionals - restrict provisioner to specific sources

	// Only is the list of build sources to restrict execution to (e.g., ["docker.amd64", "docker.arm64"])
	Only []string `yaml:"only,omitempty" json:"only,omitempty"`

	// Except is the list of build sources to skip execution for
	Except []string `yaml:"except,omitempty" json:"except,omitempty"`

	// Shell provisioner fields

	// Inline contains shell commands to execute line by line
	Inline []string `yaml:"inline,omitempty" json:"inline,omitempty"`

	// Environment is the map of environment variables to set during shell command execution
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`

	// Ansible provisioner fields

	// PlaybookPath is the path to the Ansible playbook file
	PlaybookPath string `yaml:"playbook_path,omitempty" json:"playbook_path,omitempty"`

	// GalaxyFile is the path to requirements.yml for installing Ansible Galaxy roles
	GalaxyFile string `yaml:"galaxy_file,omitempty" json:"galaxy_file,omitempty"`

	// ExtraVars provides additional variables to pass to Ansible
	ExtraVars map[string]string `yaml:"extra_vars,omitempty" json:"extra_vars,omitempty"`

	// Inventory is an inline inventory string for Ansible
	Inventory string `yaml:"inventory,omitempty" json:"inventory,omitempty"`

	// InventoryFile is the path to an Ansible inventory file
	InventoryFile string `yaml:"inventory_file,omitempty" json:"inventory_file,omitempty"`

	// AnsibleEnvVars sets environment variables for Ansible execution (format: "KEY=value")
	AnsibleEnvVars []string `yaml:"ansible_env_vars,omitempty" json:"ansible_env_vars,omitempty"`

	// UseProxy enables using a proxy for Ansible connections
	UseProxy bool `yaml:"use_proxy,omitempty" json:"use_proxy,omitempty"`

	// CollectionsPath specifies the path to Ansible collections
	CollectionsPath string `yaml:"collections_path,omitempty" json:"collections_path,omitempty"`

	// Script provisioner fields

	// Scripts is a list of script file paths to execute
	Scripts []string `yaml:"scripts,omitempty" json:"scripts,omitempty"`

	// PowerShell provisioner fields

	// PSScripts is a list of PowerShell script file paths to execute
	PSScripts []string `yaml:"ps_scripts,omitempty" json:"ps_scripts,omitempty"`

	// Common fields

	// WorkingDir sets the working directory for provisioner execution
	WorkingDir string `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`

	// User specifies which user to run the provisioner as
	User string `yaml:"user,omitempty" json:"user,omitempty"`
}

// PostProcessor represents a post-processing step after the build
type PostProcessor struct {
	// Type is the type of post-processor: "manifest", "docker-tag", "docker-push", "compress", or "checksum"
	Type string `yaml:"type" json:"type"`

	// Manifest post-processor fields

	// Output specifies the path where the manifest file should be written
	Output string `yaml:"output,omitempty" json:"output,omitempty"`

	// StripPath indicates whether to remove directory paths from artifact names in the manifest
	StripPath bool `yaml:"strip_path,omitempty" json:"strip_path,omitempty"`

	// Docker-tag post-processor fields

	// Repository is the target repository name (e.g., "ghcr.io/org/name")
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"`

	// Tags is a list of tags to apply to the image (e.g., ["latest", "v1.0.0"])
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// Force indicates whether to overwrite existing tags if they already exist
	Force bool `yaml:"force,omitempty" json:"force,omitempty"`

	// Docker-push post-processor fields

	// LoginServer is the registry login server for ECR/ACR authentication
	LoginServer string `yaml:"login_server,omitempty" json:"login_server,omitempty"`

	// LoginUsername is the username for registry authentication
	LoginUsername string `yaml:"login_username,omitempty" json:"login_username,omitempty"`

	// LoginPassword is the password or token for registry authentication (use environment variables)
	LoginPassword string `yaml:"login_password,omitempty" json:"login_password,omitempty"`

	// Compress post-processor fields

	// Format specifies the compression format: "tar.gz", "zip", "tar.bz2", etc.
	Format string `yaml:"format,omitempty" json:"format,omitempty"`

	// CompressionLevel sets the compression level from 1 (fast) to 9 (best compression)
	CompressionLevel int `yaml:"compression_level,omitempty" json:"compression_level,omitempty"`

	// Checksum post-processor fields

	// ChecksumTypes specifies hash algorithms to use: "md5", "sha1", "sha256", "sha512"
	ChecksumTypes []string `yaml:"checksum_types,omitempty" json:"checksum_types,omitempty"`

	// Common fields

	// Only is the list of build sources to restrict execution to
	Only []string `yaml:"only,omitempty" json:"only,omitempty"`

	// Except is the list of build sources to skip execution for
	Except []string `yaml:"except,omitempty" json:"except,omitempty"`

	// KeepInputArtifact indicates whether to preserve the original artifact after post-processing
	KeepInputArtifact bool `yaml:"keep_input_artifact,omitempty" json:"keep_input_artifact,omitempty"`
}

// Target represents a build target
type Target struct {
	// Type is the type of build target: "container" for Docker images or "ami" for AWS AMIs
	Type string `yaml:"type" json:"type"`

	// Container-specific fields

	// Platforms specifies target platforms for multi-arch builds (e.g., ["linux/amd64", "linux/arm64"])
	Platforms []string `yaml:"platforms,omitempty" json:"platforms,omitempty"`

	// Registry is the container registry to push to (e.g., "ghcr.io", "docker.io")
	Registry string `yaml:"registry,omitempty" json:"registry,omitempty"`

	// Tags is a list of image tags to apply (e.g., ["latest", "v1.0.0"])
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// Push indicates whether to automatically push the image to the registry after building
	Push bool `yaml:"push,omitempty" json:"push,omitempty"`

	// AMI-specific fields

	// Region is the AWS region where the AMI will be created (e.g., "us-east-1")
	Region string `yaml:"region,omitempty" json:"region,omitempty"`

	// InstanceType specifies the EC2 instance type for building (e.g., "t3.medium")
	InstanceType string `yaml:"instance_type,omitempty" json:"instance_type,omitempty"`

	// AMIName is the name to assign to the created AMI
	AMIName string `yaml:"ami_name,omitempty" json:"ami_name,omitempty"`

	// AMITags are key-value tags to apply to the AMI
	AMITags map[string]string `yaml:"ami_tags,omitempty" json:"ami_tags,omitempty"`

	// SubnetID is the VPC subnet ID to use for the build instance
	SubnetID string `yaml:"subnet_id,omitempty" json:"subnet_id,omitempty"`

	// VolumeSize is the root volume size in GB for the AMI
	VolumeSize int `yaml:"volume_size,omitempty" json:"volume_size,omitempty"`
}

// BuildResult represents the result of a build operation
type BuildResult struct {
	// ImageRef is the image reference for container builds
	ImageRef string `json:"image_ref,omitempty"`

	// Digest is the image digest for container builds (used for multi-arch manifests)
	Digest string `json:"digest,omitempty"`

	// Architecture is the architecture of the built image (e.g., "amd64", "arm64")
	Architecture string `json:"architecture,omitempty"`

	// Platform of the built image (e.g., "linux/amd64", "linux/arm64")
	Platform string `json:"platform,omitempty"`

	// AMI ID for AMI builds
	AMIID string `json:"ami_id,omitempty"`

	// Region where AMI was created
	Region string `json:"region,omitempty"`

	// Build duration
	Duration string `json:"duration"`

	// Any warnings or notes
	Notes []string `json:"notes,omitempty"`
}
