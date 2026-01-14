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

package templates

import (
	"fmt"
	"os"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// ValidationOptions contains options for validation
type ValidationOptions struct {
	// SyntaxOnly validates only syntax and structure, skipping file existence checks
	SyntaxOnly bool
}

// Validator validates template configurations
type Validator struct {
	options ValidationOptions
}

// NewValidator creates a new config validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate checks if a configuration is valid with default options
func (v *Validator) Validate(config *builder.Config) error {
	return v.ValidateWithOptions(config, ValidationOptions{})
}

// ValidateWithOptions checks if a configuration is valid with custom validation options
func (v *Validator) ValidateWithOptions(config *builder.Config, options ValidationOptions) error {
	v.options = options
	if config.Name == "" {
		return fmt.Errorf("config.name is required")
	}

	// Check if this is a Dockerfile-based build
	if config.IsDockerfileBased() {
		// Validate Dockerfile configuration
		if err := v.validateDockerfile(config.Dockerfile); err != nil {
			return err
		}
	} else {
		// Validate base image for provisioner-based builds
		if config.Base.Image == "" {
			return fmt.Errorf("config.base.image is required (or use dockerfile mode)")
		}

		// Validate provisioners
		for i, prov := range config.Provisioners {
			if err := v.validateProvisioner(&prov, i); err != nil {
				return err
			}
		}
	}

	// Validate sources
	sourceNames := make(map[string]bool)
	for i, source := range config.Sources {
		if err := v.validateSource(&source, i, sourceNames); err != nil {
			return err
		}
	}

	// Validate targets
	if len(config.Targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}

	for i, target := range config.Targets {
		if err := v.validateTarget(&target, i); err != nil {
			return err
		}
	}

	return nil
}

// validateDockerfile validates Dockerfile configuration
func (v *Validator) validateDockerfile(df *builder.DockerfileConfig) error {
	if df == nil {
		return fmt.Errorf("dockerfile configuration is nil")
	}

	dockerfilePath := df.GetDockerfilePath()
	if !v.options.SyntaxOnly {
		if _, err := os.Stat(dockerfilePath); err != nil {
			return fmt.Errorf("dockerfile not found: %s", dockerfilePath)
		}
	}

	return nil
}

// validateProvisioner validates a single provisioner
func (v *Validator) validateProvisioner(prov *builder.Provisioner, index int) error {
	switch prov.Type {
	case "shell":
		return v.validateShellProvisioner(prov, index)
	case "ansible":
		return v.validateAnsibleProvisioner(prov, index)
	case "script":
		return v.validateScriptProvisioner(prov, index)
	case "powershell":
		return v.validatePowerShellProvisioner(prov, index)
	case "file":
		return v.validateFileProvisioner(prov, index)
	case "":
		return fmt.Errorf("provisioner[%d]: type is required", index)
	default:
		return fmt.Errorf("provisioner[%d]: unknown provisioner type: %s", index, prov.Type)
	}
}

// validateShellProvisioner validates a shell provisioner
func (v *Validator) validateShellProvisioner(prov *builder.Provisioner, index int) error {
	if len(prov.Inline) == 0 {
		return fmt.Errorf("provisioner[%d]: shell provisioner requires 'inline' commands", index)
	}
	return nil
}

// validateAnsibleProvisioner validates an ansible provisioner
func (v *Validator) validateAnsibleProvisioner(prov *builder.Provisioner, index int) error {
	if prov.PlaybookPath == "" {
		return fmt.Errorf("provisioner[%d]: ansible provisioner requires 'playbook_path'", index)
	}

	if err := v.validateFilePath(prov.PlaybookPath, index, "playbook"); err != nil {
		return err
	}

	if prov.GalaxyFile != "" {
		if err := v.validateFilePath(prov.GalaxyFile, index, "galaxy"); err != nil {
			return err
		}
	}

	return nil
}

// validateScriptProvisioner validates a script provisioner
func (v *Validator) validateScriptProvisioner(prov *builder.Provisioner, index int) error {
	if len(prov.Scripts) == 0 {
		return fmt.Errorf("provisioner[%d]: script provisioner requires 'scripts'", index)
	}

	for _, script := range prov.Scripts {
		if err := v.validateFilePath(script, index, "script"); err != nil {
			return err
		}
	}

	return nil
}

// validatePowerShellProvisioner validates a powershell provisioner
func (v *Validator) validatePowerShellProvisioner(prov *builder.Provisioner, index int) error {
	if len(prov.PSScripts) == 0 {
		return fmt.Errorf("provisioner[%d]: powershell provisioner requires 'ps_scripts'", index)
	}

	for _, script := range prov.PSScripts {
		if err := v.validateFilePath(script, index, "PowerShell script"); err != nil {
			return err
		}
	}

	return nil
}

// validateFileProvisioner validates a file provisioner
func (v *Validator) validateFileProvisioner(prov *builder.Provisioner, index int) error {
	if prov.Source == "" {
		return fmt.Errorf("provisioner[%d]: file provisioner requires 'source'", index)
	}

	if prov.Destination == "" {
		return fmt.Errorf("provisioner[%d]: file provisioner requires 'destination'", index)
	}

	// Check if source is a ${sources.*} reference
	if v.isSourceReference(prov.Source) {
		// This is a reference to a fetched source - it will be resolved at build time
		// Skip file existence validation
		return nil
	}

	// Validate source file exists
	if err := v.validateFilePath(prov.Source, index, "source file"); err != nil {
		return err
	}

	return nil
}

// isSourceReference checks if a path is a ${sources.*} reference
func (v *Validator) isSourceReference(path string) bool {
	return strings.HasPrefix(path, "${sources.") && strings.HasSuffix(path, "}")
}

// validateFilePath checks if a file exists and warns about unresolved variables
func (v *Validator) validateFilePath(path string, index int, fileType string) error {
	if !v.options.SyntaxOnly {
		if _, err := os.Stat(path); err != nil {
			if v.hasUnresolvedVariable(path) {
				logging.Warn("provisioner[%d]: %s may contain unresolved environment variables: %s", index, fileType, path)
			}
			return fmt.Errorf("provisioner[%d]: %s file not found: %s", index, fileType, path)
		}
	} else if v.hasUnresolvedVariable(path) {
		logging.Warn("provisioner[%d]: %s may contain unresolved environment variables: %s", index, fileType, path)
	}
	return nil
}

// hasUnresolvedVariable checks if a path contains unresolved environment variables.
func (v *Validator) hasUnresolvedVariable(path string) bool {
	// Check for literal $ or ${ in the path
	if strings.Contains(path, "$") {
		return true
	}

	// Check for suspicious absolute paths that start with / but seem incomplete
	// e.g., "/playbooks/..." instead of "/home/user/..." or "/opt/..."
	if strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "/home/") &&
		!strings.HasPrefix(path, "/usr/") && !strings.HasPrefix(path, "/opt/") &&
		!strings.HasPrefix(path, "/var/") && !strings.HasPrefix(path, "/etc/") &&
		!strings.HasPrefix(path, "/tmp/") && !strings.HasPrefix(path, "/root/") {
		return true
	}

	return false
}

// validateTarget validates a single target
func (v *Validator) validateTarget(target *builder.Target, index int) error {
	switch target.Type {
	case "container":
		if len(target.Platforms) == 0 {
			// Default to linux/amd64 if not specified
			target.Platforms = []string{"linux/amd64"}
		}

	case "ami":
		if target.Region == "" {
			return fmt.Errorf("target[%d]: ami target requires 'region'", index)
		}
		if target.AMIName == "" {
			return fmt.Errorf("target[%d]: ami target requires 'ami_name'", index)
		}

	case "":
		return fmt.Errorf("target[%d]: type is required", index)

	default:
		return fmt.Errorf("target[%d]: unknown target type: %s", index, target.Type)
	}

	return nil
}

// validateSource validates a single source definition
func (v *Validator) validateSource(source *builder.Source, index int, seenNames map[string]bool) error {
	// Name is required
	if source.Name == "" {
		return fmt.Errorf("sources[%d]: name is required", index)
	}

	// Check for duplicate names
	if seenNames[source.Name] {
		return fmt.Errorf("sources[%d]: duplicate source name %q", index, source.Name)
	}
	seenNames[source.Name] = true

	// Validate source name format (alphanumeric, hyphens, underscores)
	for _, r := range source.Name {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		isValid := isLower || isUpper || isDigit || r == '-' || r == '_'
		if !isValid {
			return fmt.Errorf("sources[%d]: name %q contains invalid characters (use alphanumeric, hyphens, underscores)", index, source.Name)
		}
	}

	// Must have at least one source type defined
	if source.Git == nil {
		return fmt.Errorf("sources[%d]: must specify a source type (git)", index)
	}

	// Validate git source
	if source.Git != nil {
		if err := v.validateGitSource(source.Git, index); err != nil {
			return err
		}
	}

	return nil
}

// validateGitSource validates a git source configuration
func (v *Validator) validateGitSource(git *builder.GitSource, index int) error {
	if git.Repository == "" {
		return fmt.Errorf("sources[%d].git: repository is required", index)
	}

	// Basic URL validation
	if !strings.HasPrefix(git.Repository, "https://") &&
		!strings.HasPrefix(git.Repository, "http://") &&
		!strings.HasPrefix(git.Repository, "git@") &&
		!strings.HasPrefix(git.Repository, "ssh://") {
		return fmt.Errorf("sources[%d].git: repository must be a valid git URL (https://, git@, ssh://)", index)
	}

	// Validate depth is non-negative
	if git.Depth < 0 {
		return fmt.Errorf("sources[%d].git: depth must be non-negative", index)
	}

	// Validate auth configuration
	if git.Auth != nil {
		if err := v.validateGitAuth(git.Auth, index); err != nil {
			return err
		}
	}

	return nil
}

// validateGitAuth validates git authentication configuration
func (v *Validator) validateGitAuth(auth *builder.GitAuth, index int) error {
	// Check for conflicting auth methods
	authMethods := 0
	if auth.SSHKey != "" || auth.SSHKeyFile != "" {
		authMethods++
	}
	if auth.Token != "" {
		authMethods++
	}
	if auth.Username != "" && auth.Password != "" {
		authMethods++
	}

	if authMethods > 1 {
		return fmt.Errorf("sources[%d].git.auth: specify only one auth method (ssh_key/ssh_key_file, token, or username/password)", index)
	}

	// Validate SSH key file exists if specified
	if auth.SSHKeyFile != "" && !v.options.SyntaxOnly {
		keyPath := expandPath(auth.SSHKeyFile)
		if _, err := os.Stat(keyPath); err != nil {
			if v.hasUnresolvedVariable(auth.SSHKeyFile) {
				logging.Warn("sources[%d].git.auth: ssh_key_file may contain unresolved environment variables: %s", index, auth.SSHKeyFile)
			} else {
				return fmt.Errorf("sources[%d].git.auth: ssh_key_file not found: %s", index, keyPath)
			}
		}
	}

	return nil
}

// expandPath expands ~ and environment variables in a path
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + path[1:]
		}
	}
	return os.ExpandEnv(path)
}
