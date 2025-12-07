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

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// ShellProvisioner runs shell commands inside the container
type ShellProvisioner struct {
	builder *buildah.Builder
	runtime string // OCI runtime path (e.g., /usr/bin/crun, /usr/bin/runc)
}

// NewShellProvisioner creates a new shell provisioner
func NewShellProvisioner(bldr *buildah.Builder, runtime string) *ShellProvisioner {
	return &ShellProvisioner{
		builder: bldr,
		runtime: runtime,
	}
}

// Provision runs shell commands in the container
func (sp *ShellProvisioner) Provision(ctx context.Context, config builder.Provisioner) error {
	if len(config.Inline) == 0 {
		return fmt.Errorf("shell provisioner requires inline commands")
	}

	logging.Info("Running %d shell commands", len(config.Inline))

	// Set up run options with OCI isolation for full container capabilities
	runtime := sp.runtime
	if runtime == "" {
		runtime = "/usr/bin/crun" // Default fallback
	}
	runOpts := buildah.RunOptions{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Isolation: buildah.IsolationOCI,
		Runtime:   runtime,
		// Use host networking to allow apt-get to resolve hostnames
		// while avoiding netavark nftables issues in nested containers
		NamespaceOptions: define.NamespaceOptions{
			{Name: string(specs.NetworkNamespace), Host: true},
		},
		// Add capabilities needed for apt-get and package managers to drop privileges
		AddCapabilities: []string{
			"CAP_SETUID",
			"CAP_SETGID",
			"CAP_SETFCAP",
			"CAP_CHOWN",
			"CAP_DAC_OVERRIDE",
			"CAP_FOWNER",
		},
	}

	// Set working directory if specified
	if config.WorkingDir != "" {
		logging.Debug("Setting working directory: %s", config.WorkingDir)
		runOpts.WorkingDir = config.WorkingDir
	}

	// Set environment variables if specified
	if len(config.Environment) > 0 {
		logging.Debug("Setting %d environment variables", len(config.Environment))
		for key, value := range config.Environment {
			sp.builder.SetEnv(key, value)
		}
	}

	// Run each command
	for i, cmd := range config.Inline {
		logging.Info("Executing command %d/%d: %s", i+1, len(config.Inline), cmd)

		// Build command array for shell execution
		cmdArray := []string{"/bin/sh", "-c", cmd}

		// Execute the command
		if err := sp.builder.Run(cmdArray, runOpts); err != nil {
			return fmt.Errorf("command failed: %s: %w", cmd, err)
		}

		logging.Debug("Command completed successfully")
	}

	logging.Info("All shell commands completed successfully")
	return nil
}
