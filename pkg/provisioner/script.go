/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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

	"github.com/containers/buildah"
	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// ScriptProvisioner runs script files inside the container
type ScriptProvisioner struct {
	builder *buildah.Builder
}

// NewScriptProvisioner creates a new script provisioner
func NewScriptProvisioner(bldr *buildah.Builder) *ScriptProvisioner {
	return &ScriptProvisioner{
		builder: bldr,
	}
}

// Provision copies and executes script files in the container
func (sp *ScriptProvisioner) Provision(ctx context.Context, config builder.Provisioner) error {
	if len(config.Scripts) == 0 {
		return fmt.Errorf("script provisioner requires scripts")
	}

	logging.Info("Running %d script(s)", len(config.Scripts))

	// Set up run options
	runOpts := buildah.RunOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Set working directory if specified
	if config.WorkingDir != "" {
		runOpts.WorkingDir = config.WorkingDir
	}

	// Set environment variables if specified
	if len(config.Environment) > 0 {
		for key, value := range config.Environment {
			sp.builder.SetEnv(key, value)
		}
	}

	// Process each script
	for i, scriptPath := range config.Scripts {
		logging.Info("Processing script %d/%d: %s", i+1, len(config.Scripts), scriptPath)

		// Determine destination path in container
		scriptName := filepath.Base(scriptPath)
		destPath := filepath.Join("/tmp", scriptName)

		logging.Debug("Copying script to container: %s -> %s", scriptPath, destPath)

		// Add script to container
		if err := sp.builder.Add(destPath, false, buildah.AddAndCopyOptions{}, scriptPath); err != nil {
			return fmt.Errorf("failed to copy script to container: %w", err)
		}

		// Make script executable
		chmodCmd := []string{"/bin/sh", "-c", fmt.Sprintf("chmod +x %s", destPath)}
		if err := sp.builder.Run(chmodCmd, runOpts); err != nil {
			return fmt.Errorf("failed to make script executable: %w", err)
		}

		logging.Info("Executing script: %s", scriptName)

		// Execute the script
		execCmd := []string{"/bin/sh", "-c", destPath}
		if err := sp.builder.Run(execCmd, runOpts); err != nil {
			return fmt.Errorf("script execution failed: %s: %w", scriptName, err)
		}

		// Clean up script file
		cleanupCmd := []string{"/bin/sh", "-c", fmt.Sprintf("rm -f %s", destPath)}
		if err := sp.builder.Run(cleanupCmd, runOpts); err != nil {
			logging.Warn("Failed to clean up script file: %v", err)
		}

		logging.Debug("Script completed successfully: %s", scriptName)
	}

	logging.Info("All scripts completed successfully")
	return nil
}
