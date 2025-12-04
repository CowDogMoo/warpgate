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

	"github.com/cowdogmoo/warpgate/pkg/builder"
)

// Validator validates template configurations
type Validator struct{}

// NewValidator creates a new config validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate checks if a configuration is valid
func (v *Validator) Validate(config *builder.Config) error {
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
		if _, err := os.Stat(prov.PlaybookPath); err != nil {
			return fmt.Errorf("provisioner[%d]: playbook file not found: %s", index, prov.PlaybookPath)
		}

	case "script":
		if len(prov.Scripts) == 0 {
			return fmt.Errorf("provisioner[%d]: script provisioner requires 'scripts'", index)
		}
		for _, script := range prov.Scripts {
			if _, err := os.Stat(script); err != nil {
				return fmt.Errorf("provisioner[%d]: script file not found: %s", index, script)
			}
		}

	case "powershell":
		if len(prov.PSScripts) == 0 {
			return fmt.Errorf("provisioner[%d]: powershell provisioner requires 'ps_scripts'", index)
		}
		for _, script := range prov.PSScripts {
			if _, err := os.Stat(script); err != nil {
				return fmt.Errorf("provisioner[%d]: PowerShell script file not found: %s", index, script)
			}
		}

	case "":
		return fmt.Errorf("provisioner[%d]: type is required", index)

	default:
		return fmt.Errorf("provisioner[%d]: unknown provisioner type: %s", index, prov.Type)
	}

	return nil
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
