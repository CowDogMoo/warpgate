/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

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

package proxmox

import (
	"fmt"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

// requireProxmoxFields enforces the minimum target fields needed for a
// Proxmox build. This is a backstop for the validator so calling Build
// directly (e.g., from tests) still fails loudly.
func requireProxmoxFields(t *builder.Target) error {
	if t == nil {
		return fmt.Errorf("proxmox target: target is nil")
	}
	if t.Node == "" {
		return fmt.Errorf("proxmox target: node is required")
	}
	if t.SourceTemplate == 0 && t.SourceTemplateName == "" {
		return fmt.Errorf("proxmox target: source_template (VMID) or source_template_name is required")
	}
	if t.SourceTemplate != 0 && t.SourceTemplate < 100 {
		return fmt.Errorf("proxmox target: source_template VMID must be >= 100 (PVE reserves 1-99)")
	}
	if t.TemplateName == "" {
		return fmt.Errorf("proxmox target: template_name is required")
	}
	return nil
}

// findProxmoxTarget returns the first proxmox target in the config or an error.
func findProxmoxTarget(cfg builder.Config) (*builder.Target, error) {
	for i := range cfg.Targets {
		if cfg.Targets[i].Type == "proxmox" {
			return &cfg.Targets[i], nil
		}
	}
	return nil, fmt.Errorf("no proxmox target found in configuration")
}

// ValidatePrerequisites verifies basic preconditions before kicking off a
// build. It returns a descriptive error if any required value is missing.
func ValidatePrerequisites(endpoint, node string, sourceTemplate int) error {
	if endpoint == "" {
		return fmt.Errorf("proxmox endpoint is required")
	}
	if node == "" {
		return fmt.Errorf("proxmox node is required")
	}
	if sourceTemplate != 0 && sourceTemplate < 100 {
		return fmt.Errorf("source template VMID must be >= 100 (got %d)", sourceTemplate)
	}
	return nil
}
