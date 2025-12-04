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

package provisioner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/buildah"
	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// AnsibleProvisioner runs Ansible playbooks inside the container
type AnsibleProvisioner struct {
	builder *buildah.Builder
}

// NewAnsibleProvisioner creates a new Ansible provisioner
func NewAnsibleProvisioner(bldr *buildah.Builder) *AnsibleProvisioner {
	return &AnsibleProvisioner{
		builder: bldr,
	}
}

// Provision runs an Ansible playbook in the container
func (ap *AnsibleProvisioner) Provision(ctx context.Context, config builder.Provisioner) error {
	if config.PlaybookPath == "" {
		return fmt.Errorf("ansible provisioner requires playbook_path")
	}

	logging.Info("Running Ansible playbook: %s", config.PlaybookPath)

	// Set up run options
	runOpts := buildah.RunOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Set working directory if specified
	if config.WorkingDir != "" {
		runOpts.WorkingDir = config.WorkingDir
	}

	// Ensure Ansible is installed
	if err := ap.ensureAnsible(); err != nil {
		return fmt.Errorf("failed to ensure Ansible is installed: %w", err)
	}

	// Install collections if galaxy_file is specified
	if config.GalaxyFile != "" {
		if err := ap.installCollections(config.GalaxyFile); err != nil {
			return fmt.Errorf("failed to install Ansible collections: %w", err)
		}
	}

	// Copy playbook to container
	playbookDest := "/tmp/playbook.yml"
	logging.Debug("Copying playbook to container: %s -> %s", config.PlaybookPath, playbookDest)

	if err := ap.builder.Add(playbookDest, false, buildah.AddAndCopyOptions{}, config.PlaybookPath); err != nil {
		return fmt.Errorf("failed to copy playbook to container: %w", err)
	}

	// Copy inventory if specified
	inventoryArg := ""
	if config.Inventory != "" {
		inventoryDest := "/tmp/inventory"
		logging.Debug("Copying inventory to container: %s -> %s", config.Inventory, inventoryDest)

		if err := ap.builder.Add(inventoryDest, false, buildah.AddAndCopyOptions{}, config.Inventory); err != nil {
			return fmt.Errorf("failed to copy inventory to container: %w", err)
		}
		inventoryArg = fmt.Sprintf("-i %s", inventoryDest)
	}

	// Build extra vars argument
	extraVarsArg := ""
	if len(config.ExtraVars) > 0 {
		var vars []string
		for key, value := range config.ExtraVars {
			vars = append(vars, fmt.Sprintf("%s=%s", key, value))
		}
		extraVarsArg = fmt.Sprintf("--extra-vars '%s'", strings.Join(vars, " "))
	}

	// Build ansible-playbook command
	ansibleCmd := fmt.Sprintf("ansible-playbook %s %s %s", inventoryArg, extraVarsArg, playbookDest)
	cmdArray := []string{"/bin/sh", "-c", ansibleCmd}

	logging.Info("Executing: %s", ansibleCmd)

	// Run ansible-playbook
	if err := ap.builder.Run(cmdArray, runOpts); err != nil {
		return fmt.Errorf("ansible-playbook execution failed: %w", err)
	}

	// Clean up
	cleanupCmd := []string{"/bin/sh", "-c", fmt.Sprintf("rm -f %s", playbookDest)}
	if err := ap.builder.Run(cleanupCmd, runOpts); err != nil {
		logging.Warn("Failed to clean up playbook: %v", err)
	}

	logging.Info("Ansible playbook completed successfully")
	return nil
}

// ensureAnsible checks if Ansible is installed and installs it if needed
func (ap *AnsibleProvisioner) ensureAnsible() error {
	logging.Debug("Checking for Ansible installation")

	runOpts := buildah.RunOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Check if ansible is already installed
	checkCmd := []string{"/bin/sh", "-c", "command -v ansible-playbook"}
	if err := ap.builder.Run(checkCmd, runOpts); err == nil {
		logging.Debug("Ansible is already installed")
		return nil
	}

	logging.Info("Installing Ansible...")

	// Try to detect package manager and install Ansible
	installCmds := []string{
		// Try apt (Debian/Ubuntu)
		"if command -v apt-get >/dev/null 2>&1; then apt-get update && apt-get install -y ansible; exit 0; fi",
		// Try dnf (Fedora/RHEL 8+)
		"if command -v dnf >/dev/null 2>&1; then dnf install -y ansible; exit 0; fi",
		// Try yum (CentOS/RHEL 7)
		"if command -v yum >/dev/null 2>&1; then yum install -y ansible; exit 0; fi",
		// Try apk (Alpine)
		"if command -v apk >/dev/null 2>&1; then apk add --no-cache ansible; exit 0; fi",
		// Try pip as fallback
		"if command -v pip3 >/dev/null 2>&1; then pip3 install ansible; exit 0; fi",
	}

	installCmd := []string{"/bin/sh", "-c", strings.Join(installCmds, "; ")}
	if err := ap.builder.Run(installCmd, runOpts); err != nil {
		return fmt.Errorf("failed to install Ansible: %w", err)
	}

	logging.Info("Ansible installed successfully")
	return nil
}

// installCollections installs Ansible Galaxy collections
func (ap *AnsibleProvisioner) installCollections(galaxyFile string) error {
	logging.Info("Installing Ansible collections from %s", galaxyFile)

	// Copy galaxy file to container
	galaxyDest := "/tmp/requirements.yml"
	if err := ap.builder.Add(galaxyDest, false, buildah.AddAndCopyOptions{}, galaxyFile); err != nil {
		return fmt.Errorf("failed to copy galaxy file to container: %w", err)
	}

	runOpts := buildah.RunOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Install collections
	installCmd := []string{
		"/bin/sh", "-c",
		fmt.Sprintf("ansible-galaxy collection install -r %s --force", galaxyDest),
	}

	if err := ap.builder.Run(installCmd, runOpts); err != nil {
		return fmt.Errorf("failed to install collections: %w", err)
	}

	// Clean up
	cleanupCmd := []string{"/bin/sh", "-c", fmt.Sprintf("rm -f %s", galaxyDest)}
	if err := ap.builder.Run(cleanupCmd, runOpts); err != nil {
		logging.Warn("Failed to clean up galaxy file: %v", err)
	}

	logging.Info("Ansible collections installed successfully")
	return nil
}
