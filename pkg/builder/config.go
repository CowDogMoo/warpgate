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

// Version is the current version of warpgate
var Version = "1.0.0"

// Config represents a template configuration for building images
type Config struct {
	// Metadata about the template
	Metadata Metadata `yaml:"metadata" json:"metadata"`

	// Name of the image
	Name string `yaml:"name" json:"name"`

	// Version of the template
	Version string `yaml:"version" json:"version"`

	// Base image configuration
	Base BaseImage `yaml:"base" json:"base"`

	// Provisioners to run during the build
	Provisioners []Provisioner `yaml:"provisioners" json:"provisioners"`

	// Post-processors to run after the build
	PostProcessors []PostProcessor `yaml:"post_processors,omitempty" json:"post_processors,omitempty"`

	// Build targets (container, AMI, etc.)
	Targets []Target `yaml:"targets" json:"targets"`

	// Architecture-specific overrides (optional)
	// Allows specifying different configurations per architecture
	ArchOverrides map[string]ArchOverride `yaml:"arch_overrides,omitempty" json:"arch_overrides,omitempty"`

	// Runtime overrides (not in YAML, set by CLI flags)
	Architectures []string `yaml:"-" json:"-"` // Architectures to build for
	Registry      string   `yaml:"-" json:"-"` // Registry to push to (overrides target registry)
}

// ArchOverride allows architecture-specific configuration
type ArchOverride struct {
	// Override base image for this architecture
	Base *BaseImage `yaml:"base,omitempty" json:"base,omitempty"`

	// Additional or replacement provisioners for this architecture
	Provisioners []Provisioner `yaml:"provisioners,omitempty" json:"provisioners,omitempty"`

	// Whether to append provisioners or replace them entirely
	AppendProvisioners bool `yaml:"append_provisioners,omitempty" json:"append_provisioners,omitempty"`
}

// Metadata contains template metadata
type Metadata struct {
	Name        string            `yaml:"name" json:"name"`
	Version     string            `yaml:"version" json:"version"`
	Description string            `yaml:"description" json:"description"`
	Author      string            `yaml:"author" json:"author"`
	License     string            `yaml:"license" json:"license"`
	Tags        []string          `yaml:"tags" json:"tags"`
	Requires    Requirements      `yaml:"requires" json:"requires"`
	Changelog   []ChangelogEntry  `yaml:"changelog,omitempty" json:"changelog,omitempty"`
	Extra       map[string]string `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// Requirements specifies version requirements
type Requirements struct {
	Warpgate string `yaml:"warpgate" json:"warpgate"`
}

// ChangelogEntry represents a changelog entry
type ChangelogEntry struct {
	Version string   `yaml:"version" json:"version"`
	Date    string   `yaml:"date" json:"date"`
	Changes []string `yaml:"changes" json:"changes"`
}

// BaseImage specifies the base image to start from
type BaseImage struct {
	Image    string            `yaml:"image" json:"image"`
	Platform string            `yaml:"platform,omitempty" json:"platform,omitempty"`
	Pull     bool              `yaml:"pull,omitempty" json:"pull,omitempty"`
	Auth     *ImageAuth        `yaml:"auth,omitempty" json:"auth,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Docker-specific options
	Privileged bool              `yaml:"privileged,omitempty" json:"privileged,omitempty"`
	Volumes    map[string]string `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	RunCommand []string          `yaml:"run_command,omitempty" json:"run_command,omitempty"`
	Changes    []string          `yaml:"changes,omitempty" json:"changes,omitempty"` // Dockerfile instructions (ENV, USER, WORKDIR, ENTRYPOINT, CMD)
}

// ImageAuth contains authentication information for pulling images
type ImageAuth struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Registry string `yaml:"registry,omitempty" json:"registry,omitempty"`
}

// Provisioner represents a provisioning step
type Provisioner struct {
	// Type of provisioner (shell, ansible, script, powershell)
	Type string `yaml:"type" json:"type"`

	// Conditionals - restrict provisioner to specific sources
	Only   []string `yaml:"only,omitempty" json:"only,omitempty"`     // Only run for these sources (e.g., ["docker.amd64", "docker.arm64"])
	Except []string `yaml:"except,omitempty" json:"except,omitempty"` // Skip these sources

	// Shell provisioner fields
	Inline      []string          `yaml:"inline,omitempty" json:"inline,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`

	// Ansible provisioner fields
	PlaybookPath    string            `yaml:"playbook_path,omitempty" json:"playbook_path,omitempty"`
	GalaxyFile      string            `yaml:"galaxy_file,omitempty" json:"galaxy_file,omitempty"`
	ExtraVars       map[string]string `yaml:"extra_vars,omitempty" json:"extra_vars,omitempty"`
	Inventory       string            `yaml:"inventory,omitempty" json:"inventory,omitempty"`
	InventoryFile   string            `yaml:"inventory_file,omitempty" json:"inventory_file,omitempty"`
	AnsibleEnvVars  []string          `yaml:"ansible_env_vars,omitempty" json:"ansible_env_vars,omitempty"`
	UseProxy        bool              `yaml:"use_proxy,omitempty" json:"use_proxy,omitempty"`
	CollectionsPath string            `yaml:"collections_path,omitempty" json:"collections_path,omitempty"`

	// Script provisioner fields
	Scripts []string `yaml:"scripts,omitempty" json:"scripts,omitempty"`

	// PowerShell provisioner fields
	PSScripts []string `yaml:"ps_scripts,omitempty" json:"ps_scripts,omitempty"`

	// Common fields
	WorkingDir string `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
	User       string `yaml:"user,omitempty" json:"user,omitempty"`
}

// PostProcessor represents a post-processing step after the build
type PostProcessor struct {
	// Type of post-processor (manifest, docker-tag, docker-push, compress, checksum)
	Type string `yaml:"type" json:"type"`

	// Manifest post-processor fields
	Output    string `yaml:"output,omitempty" json:"output,omitempty"`         // Output path for manifest
	StripPath bool   `yaml:"strip_path,omitempty" json:"strip_path,omitempty"` // Strip path from artifact names

	// Docker-tag post-processor fields
	Repository string   `yaml:"repository,omitempty" json:"repository,omitempty"` // Repository name (e.g., "ghcr.io/org/name")
	Tags       []string `yaml:"tags,omitempty" json:"tags,omitempty"`             // Tags to apply (e.g., ["latest", "v1.0.0"])
	Force      bool     `yaml:"force,omitempty" json:"force,omitempty"`           // Force tag even if exists

	// Docker-push post-processor fields
	LoginServer   string `yaml:"login_server,omitempty" json:"login_server,omitempty"`     // ECR/ACR login server
	LoginUsername string `yaml:"login_username,omitempty" json:"login_username,omitempty"` // Registry username
	LoginPassword string `yaml:"login_password,omitempty" json:"login_password,omitempty"` // Registry password

	// Compress post-processor fields
	Format           string `yaml:"format,omitempty" json:"format,omitempty"`                       // Compression format (tar.gz, zip, etc.)
	CompressionLevel int    `yaml:"compression_level,omitempty" json:"compression_level,omitempty"` // 1-9

	// Checksum post-processor fields
	ChecksumTypes []string `yaml:"checksum_types,omitempty" json:"checksum_types,omitempty"` // Hash types (md5, sha1, sha256, sha512)

	// Common fields
	Only              []string `yaml:"only,omitempty" json:"only,omitempty"`                               // Only run for these sources
	Except            []string `yaml:"except,omitempty" json:"except,omitempty"`                           // Skip these sources
	KeepInputArtifact bool     `yaml:"keep_input_artifact,omitempty" json:"keep_input_artifact,omitempty"` // Keep original artifact
}

// Target represents a build target
type Target struct {
	// Type of target (container, ami)
	Type string `yaml:"type" json:"type"`

	// Container-specific fields
	Platforms []string `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	Registry  string   `yaml:"registry,omitempty" json:"registry,omitempty"`
	Tags      []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Push      bool     `yaml:"push,omitempty" json:"push,omitempty"`

	// AMI-specific fields
	Region       string            `yaml:"region,omitempty" json:"region,omitempty"`
	InstanceType string            `yaml:"instance_type,omitempty" json:"instance_type,omitempty"`
	AMIName      string            `yaml:"ami_name,omitempty" json:"ami_name,omitempty"`
	AMITags      map[string]string `yaml:"ami_tags,omitempty" json:"ami_tags,omitempty"`
	SubnetID     string            `yaml:"subnet_id,omitempty" json:"subnet_id,omitempty"`
	VolumeSize   int               `yaml:"volume_size,omitempty" json:"volume_size,omitempty"`
}

// BuildResult represents the result of a build operation
type BuildResult struct {
	// Image reference for container builds
	ImageRef string `json:"image_ref,omitempty"`

	// Image digest for container builds (used for multi-arch manifests)
	Digest string `json:"digest,omitempty"`

	// Architecture of the built image (e.g., "amd64", "arm64")
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
