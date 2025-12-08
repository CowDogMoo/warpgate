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

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
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

	if config.Base.Image == "" {
		return fmt.Errorf("config.base.image is required")
	}

	// Validate provisioners
	for i, prov := range config.Provisioners {
		if err := v.validateProvisioner(&prov, i); err != nil {
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

// validateProvisioner validates a single provisioner
func (v *Validator) validateProvisioner(prov *builder.Provisioner, index int) error {
	switch prov.Type {
	case "shell":
		if len(prov.Inline) == 0 {
			return fmt.Errorf("provisioner[%d]: shell provisioner requires 'inline' commands", index)
		}

	case "ansible":
		if prov.PlaybookPath == "" {
			return fmt.Errorf("provisioner[%d]: ansible provisioner requires 'playbook_path'", index)
		}

		// Only check file existence if not in syntax-only mode
		if !v.options.SyntaxOnly {
			if _, err := os.Stat(prov.PlaybookPath); err != nil {
				// If file doesn't exist, check if it might be due to unresolved variables
				if v.hasUnresolvedVariable(prov.PlaybookPath) {
					logging.Warn("provisioner[%d]: playbook_path may contain unresolved environment variables: %s", index, prov.PlaybookPath)
				}
				return fmt.Errorf("provisioner[%d]: playbook file not found: %s", index, prov.PlaybookPath)
			}
		} else {
			// In syntax-only mode, warn about potential unresolved variables
			if v.hasUnresolvedVariable(prov.PlaybookPath) {
				logging.Warn("provisioner[%d]: playbook_path may contain unresolved environment variables: %s", index, prov.PlaybookPath)
			}
		}

		// Check galaxy file if specified
		if prov.GalaxyFile != "" {
			if !v.options.SyntaxOnly {
				if _, err := os.Stat(prov.GalaxyFile); err != nil {
					if v.hasUnresolvedVariable(prov.GalaxyFile) {
						logging.Warn("provisioner[%d]: galaxy_file may contain unresolved environment variables: %s", index, prov.GalaxyFile)
					}
					return fmt.Errorf("provisioner[%d]: galaxy file not found: %s", index, prov.GalaxyFile)
				}
			} else {
				if v.hasUnresolvedVariable(prov.GalaxyFile) {
					logging.Warn("provisioner[%d]: galaxy_file may contain unresolved environment variables: %s", index, prov.GalaxyFile)
				}
			}
		}

	case "script":
		if len(prov.Scripts) == 0 {
			return fmt.Errorf("provisioner[%d]: script provisioner requires 'scripts'", index)
		}

		for _, script := range prov.Scripts {
			if !v.options.SyntaxOnly {
				if _, err := os.Stat(script); err != nil {
					if v.hasUnresolvedVariable(script) {
						logging.Warn("provisioner[%d]: script may contain unresolved environment variables: %s", index, script)
					}
					return fmt.Errorf("provisioner[%d]: script file not found: %s", index, script)
				}
			} else {
				if v.hasUnresolvedVariable(script) {
					logging.Warn("provisioner[%d]: script may contain unresolved environment variables: %s", index, script)
				}
			}
		}

	case "powershell":
		if len(prov.PSScripts) == 0 {
			return fmt.Errorf("provisioner[%d]: powershell provisioner requires 'ps_scripts'", index)
		}

		for _, script := range prov.PSScripts {
			if !v.options.SyntaxOnly {
				if _, err := os.Stat(script); err != nil {
					if v.hasUnresolvedVariable(script) {
						logging.Warn("provisioner[%d]: PowerShell script may contain unresolved environment variables: %s", index, script)
					}
					return fmt.Errorf("provisioner[%d]: PowerShell script file not found: %s", index, script)
				}
			} else {
				if v.hasUnresolvedVariable(script) {
					logging.Warn("provisioner[%d]: PowerShell script may contain unresolved environment variables: %s", index, script)
				}
			}
		}

	case "":
		return fmt.Errorf("provisioner[%d]: type is required", index)

	default:
		return fmt.Errorf("provisioner[%d]: unknown provisioner type: %s", index, prov.Type)
	}

	return nil
}

// hasUnresolvedVariable checks if a path contains unresolved environment variables
// This detects common patterns like:
// - Paths starting with / that seem incomplete (e.g., "/playbooks/..." instead of "/home/user/...")
// - Paths containing $ or ${ that weren't expanded
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
