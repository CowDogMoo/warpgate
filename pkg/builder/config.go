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

	// Build targets (container, AMI, etc.)
	Targets []Target `yaml:"targets" json:"targets"`

	// Runtime overrides (not in YAML, set by CLI flags)
	Architectures []string `yaml:"-" json:"-"` // Architectures to build for
	Registry      string   `yaml:"-" json:"-"` // Registry to push to (overrides target registry)
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

	// Shell provisioner fields
	Inline      []string          `yaml:"inline,omitempty" json:"inline,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`

	// Ansible provisioner fields
	PlaybookPath string            `yaml:"playbook_path,omitempty" json:"playbook_path,omitempty"`
	GalaxyFile   string            `yaml:"galaxy_file,omitempty" json:"galaxy_file,omitempty"`
	ExtraVars    map[string]string `yaml:"extra_vars,omitempty" json:"extra_vars,omitempty"`
	Inventory    string            `yaml:"inventory,omitempty" json:"inventory,omitempty"`

	// Script provisioner fields
	Scripts []string `yaml:"scripts,omitempty" json:"scripts,omitempty"`

	// PowerShell provisioner fields
	PSScripts []string `yaml:"ps_scripts,omitempty" json:"ps_scripts,omitempty"`

	// Common fields
	WorkingDir string `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
	User       string `yaml:"user,omitempty" json:"user,omitempty"`
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

	// AMI ID for AMI builds
	AMIID string `json:"ami_id,omitempty"`

	// Region where AMI was created
	Region string `json:"region,omitempty"`

	// Build duration
	Duration string `json:"duration"`

	// Any warnings or notes
	Notes []string `json:"notes,omitempty"`
}
