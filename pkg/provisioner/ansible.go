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
	"path/filepath"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// AnsibleProvisioner runs Ansible playbooks inside the container
type AnsibleProvisioner struct {
	builder *buildah.Builder
	runtime string // OCI runtime path (e.g., /usr/bin/crun, /usr/bin/runc)
}

// NewAnsibleProvisioner creates a new Ansible provisioner
func NewAnsibleProvisioner(bldr *buildah.Builder, runtime string) *AnsibleProvisioner {
	return &AnsibleProvisioner{
		builder: bldr,
		runtime: runtime,
	}
}

// getRunOptions returns standard run options with OCI isolation and necessary capabilities
// In nested container environments, falls back to chroot isolation for compatibility
func (ap *AnsibleProvisioner) getRunOptions() (buildah.RunOptions, error) {
	isolation := buildah.IsolationOCI
	runtime := ap.runtime
	if runtime == "" {
		// Auto-detect available runtime (crun preferred, runc fallback)
		runtime = globalconfig.DetectOCIRuntime()
		if runtime == "" {
			return buildah.RunOptions{}, fmt.Errorf("no OCI runtime found (tried crun, runc)")
		}
	}

	return buildah.RunOptions{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Isolation: isolation,
		Runtime:   runtime,
		// Use host networking to allow package managers to resolve hostnames
		// while avoiding netavark nftables issues in nested containers
		NamespaceOptions: define.NamespaceOptions{
			{Name: string(specs.NetworkNamespace), Host: true},
		},
		// Add capabilities needed for apt-get and package managers to drop privileges
		// Only applies to OCI isolation (chroot doesn't support them)
		AddCapabilities: []string{
			"CAP_SETUID",
			"CAP_SETGID",
			"CAP_SETFCAP",
			"CAP_CHOWN",
			"CAP_DAC_OVERRIDE",
			"CAP_FOWNER",
		},
	}, nil
}

// Provision runs an Ansible playbook in the container
func (ap *AnsibleProvisioner) Provision(ctx context.Context, config builder.Provisioner) error {
	if config.PlaybookPath == "" {
		return fmt.Errorf("ansible provisioner requires playbook_path")
	}

	logging.Info("Running Ansible playbook: %s", config.PlaybookPath)

	// Set up run options
	runOpts, err := ap.setupRunOptions(config.WorkingDir)
	if err != nil {
		return err
	}

	// Ensure Ansible is installed and install galaxy dependencies
	if err := ap.prepareAnsible(config.GalaxyFile); err != nil {
		return err
	}

	// Copy playbook to container
	playbookDest := "/tmp/playbook.yml"
	if err := ap.copyPlaybook(config.PlaybookPath, playbookDest); err != nil {
		return err
	}

	// Set up inventory
	inventoryArg, err := ap.setupInventory(config.Inventory)
	if err != nil {
		return err
	}

	// Build and execute ansible-playbook command
	if err := ap.executePlaybook(playbookDest, inventoryArg, config.ExtraVars, runOpts); err != nil {
		return err
	}

	// Clean up
	ap.cleanupPlaybook(playbookDest, runOpts)

	logging.Info("Ansible playbook completed successfully")
	return nil
}

// setupRunOptions configures run options for command execution
func (ap *AnsibleProvisioner) setupRunOptions(workingDir string) (buildah.RunOptions, error) {
	runOpts, err := ap.getRunOptions()
	if err != nil {
		return buildah.RunOptions{}, err
	}

	if workingDir != "" {
		runOpts.WorkingDir = workingDir
	}

	return runOpts, nil
}

// prepareAnsible ensures Ansible is installed and installs galaxy dependencies
func (ap *AnsibleProvisioner) prepareAnsible(galaxyFile string) error {
	if err := ap.ensureAnsible(); err != nil {
		return fmt.Errorf("failed to ensure Ansible is installed: %w", err)
	}

	if galaxyFile == "" {
		return nil
	}

	if err := ap.installCollections(galaxyFile); err != nil {
		return fmt.Errorf("failed to install Ansible collections: %w", err)
	}

	if err := ap.installRoles(galaxyFile); err != nil {
		return fmt.Errorf("failed to install Ansible roles: %w", err)
	}

	if err := ap.installLocalCollection(galaxyFile); err != nil {
		return fmt.Errorf("failed to install local collection: %w", err)
	}

	return nil
}

// copyPlaybook copies the playbook file to the container
func (ap *AnsibleProvisioner) copyPlaybook(playbookPath, playbookDest string) error {
	logging.Debug("Copying playbook to container: %s -> %s", playbookPath, playbookDest)
	if err := ap.builder.Add(playbookDest, false, buildah.AddAndCopyOptions{}, playbookPath); err != nil {
		return fmt.Errorf("failed to copy playbook to container: %w", err)
	}
	return nil
}

// setupInventory copies inventory file to container if specified and returns inventory argument
func (ap *AnsibleProvisioner) setupInventory(inventory string) (string, error) {
	if inventory == "" {
		return "", nil
	}

	inventoryDest := "/tmp/inventory"
	logging.Debug("Copying inventory to container: %s -> %s", inventory, inventoryDest)

	if err := ap.builder.Add(inventoryDest, false, buildah.AddAndCopyOptions{}, inventory); err != nil {
		return "", fmt.Errorf("failed to copy inventory to container: %w", err)
	}

	return fmt.Sprintf("-i %s", inventoryDest), nil
}

// buildExtraVarsArg builds the extra vars argument for ansible-playbook
func buildExtraVarsArg(extraVars map[string]string) string {
	if len(extraVars) == 0 {
		return ""
	}

	vars := make([]string, 0, len(extraVars))
	for key, value := range extraVars {
		vars = append(vars, fmt.Sprintf("%s=%s", key, value))
	}
	return fmt.Sprintf("--extra-vars '%s'", strings.Join(vars, " "))
}

// executePlaybook builds and executes the ansible-playbook command
func (ap *AnsibleProvisioner) executePlaybook(playbookDest, inventoryArg string, extraVars map[string]string, runOpts buildah.RunOptions) error {
	// Build extra vars argument
	extraVarsArg := buildExtraVarsArg(extraVars)

	// Set default inventory if none specified
	connectionArg := ""
	if inventoryArg == "" {
		inventoryArg = "-i localhost,"
		connectionArg = "-c local"
	}

	ansibleCmd := fmt.Sprintf("ansible-playbook %s %s %s %s", connectionArg, inventoryArg, extraVarsArg, playbookDest)
	cmdArray := []string{"/bin/sh", "-c", ansibleCmd}

	logging.Info("Executing: %s", ansibleCmd)

	if err := ap.builder.Run(cmdArray, runOpts); err != nil {
		return fmt.Errorf("ansible-playbook execution failed: %w", err)
	}

	return nil
}

// cleanupPlaybook removes the playbook file from the container
func (ap *AnsibleProvisioner) cleanupPlaybook(playbookDest string, runOpts buildah.RunOptions) {
	cleanupCmd := []string{"/bin/sh", "-c", fmt.Sprintf("rm -f %s", playbookDest)}
	if err := ap.builder.Run(cleanupCmd, runOpts); err != nil {
		logging.Warn("Failed to clean up playbook: %v", err)
	}
}

// ensureAnsible checks if Ansible is installed and installs it if needed
func (ap *AnsibleProvisioner) ensureAnsible() error {
	logging.Debug("Checking for Ansible installation")

	runOpts, err := ap.getRunOptions()
	if err != nil {
		return err
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

	runOpts, err := ap.getRunOptions()
	if err != nil {
		return err
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

// installRoles installs Ansible Galaxy roles from requirements.yml
func (ap *AnsibleProvisioner) installRoles(galaxyFile string) error {
	logging.Info("Installing Ansible roles from %s", galaxyFile)

	// Copy galaxy file to container
	galaxyDest := "/tmp/requirements.yml"
	if err := ap.builder.Add(galaxyDest, false, buildah.AddAndCopyOptions{}, galaxyFile); err != nil {
		return fmt.Errorf("failed to copy galaxy file to container: %w", err)
	}

	runOpts, err := ap.getRunOptions()
	if err != nil {
		return err
	}

	// Install roles
	installCmd := []string{
		"/bin/sh", "-c",
		fmt.Sprintf("ansible-galaxy role install -r %s --force", galaxyDest),
	}

	if err := ap.builder.Run(installCmd, runOpts); err != nil {
		return fmt.Errorf("failed to install roles: %w", err)
	}

	// Clean up
	cleanupCmd := []string{"/bin/sh", "-c", fmt.Sprintf("rm -f %s", galaxyDest)}
	if err := ap.builder.Run(cleanupCmd, runOpts); err != nil {
		logging.Warn("Failed to clean up galaxy file: %v", err)
	}

	logging.Info("Ansible roles installed successfully")
	return nil
}

// installLocalCollection checks if a galaxy.yml exists in the same directory as
// the requirements file, and if so, builds and installs that collection
func (ap *AnsibleProvisioner) installLocalCollection(requirementsPath string) error {
	// Get the directory containing requirements.yml
	requirementsDir := filepath.Dir(requirementsPath)

	// Check if galaxy.yml exists in that directory
	galaxyYmlPath := filepath.Join(requirementsDir, "galaxy.yml")
	if _, err := os.Stat(galaxyYmlPath); os.IsNotExist(err) {
		logging.Debug("No galaxy.yml found at %s, skipping local collection install", galaxyYmlPath)
		return nil
	}

	logging.Info("Found galaxy.yml at %s, building and installing local collection", galaxyYmlPath)

	// Copy the entire collection directory to /tmp/collection in the container
	collectionDest := "/tmp/collection"
	if err := ap.builder.Add(collectionDest, false, buildah.AddAndCopyOptions{}, requirementsDir); err != nil {
		return fmt.Errorf("failed to copy collection directory to container: %w", err)
	}

	runOpts, err := ap.getRunOptions()
	if err != nil {
		return err
	}

	// Build and install the collection
	installCmd := []string{
		"/bin/sh", "-c",
		fmt.Sprintf("cd %s && ansible-galaxy collection build --force && ansible-galaxy collection install *.tar.gz --force", collectionDest),
	}

	if err := ap.builder.Run(installCmd, runOpts); err != nil {
		return fmt.Errorf("failed to build and install local collection: %w", err)
	}

	// Clean up
	cleanupCmd := []string{"/bin/sh", "-c", fmt.Sprintf("rm -rf %s", collectionDest)}
	if err := ap.builder.Run(cleanupCmd, runOpts); err != nil {
		logging.Warn("Failed to clean up collection directory: %v", err)
	}

	logging.Info("Local collection installed successfully")
	return nil
}
