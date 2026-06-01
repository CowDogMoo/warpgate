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
	"context"
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// SSHRunner executes provisioner steps over SSH against a running VM. The
// builder uses an SSHRunner so tests can inject a fake that captures the
// commands run instead of opening real connections.
type SSHRunner interface {
	// Run executes a single command on the remote host. stdout/stderr is
	// streamed into the returned string for callers that want to log it.
	Run(ctx context.Context, command string, env map[string]string) (string, error)

	// UploadFile copies a local file (or directory) to a destination path
	// on the remote host with the given mode.
	UploadFile(ctx context.Context, source, destination, mode string) error

	// Close releases the underlying SSH session.
	Close() error
}

// SSHConnection bundles connection parameters for an SSHRunner.
type SSHConnection struct {
	// Host is the IP or DNS name of the VM.
	Host string

	// Port is the SSH port; 22 when zero.
	Port int

	// User is the SSH user.
	User string

	// Password is the SSH password. Either Password or PrivateKey must be set.
	Password string

	// PrivateKey is the SSH private key (PEM-encoded).
	PrivateKey string
}

// runProvisioners executes the configured warpgate provisioners against the
// VM using the supplied SSH runner. Provisioners run in order; the first
// failure aborts the build. Only types meaningful for Proxmox builds are
// supported: shell, file, ansible (local), script.
func runProvisioners(ctx context.Context, runner SSHRunner, provisioners []builder.Provisioner) error {
	for i, p := range provisioners {
		if err := runProvisioner(ctx, runner, i, p); err != nil {
			return fmt.Errorf("provisioner[%d] (%s): %w", i, p.Type, err)
		}
	}
	return nil
}

// runProvisioner dispatches a single provisioner to its handler.
func runProvisioner(ctx context.Context, runner SSHRunner, idx int, p builder.Provisioner) error {
	switch p.Type {
	case "shell":
		return runShellProvisioner(ctx, runner, p)
	case "file":
		return runFileProvisioner(ctx, runner, p)
	case "script":
		return runScriptProvisioner(ctx, runner, p)
	case "ansible":
		logging.WarnContext(ctx, "provisioner[%d]: ansible is not yet supported for proxmox builds; skipping", idx)
		return nil
	case "powershell":
		return runPowerShellProvisioner(ctx, runner, p)
	default:
		return fmt.Errorf("unsupported provisioner type %q", p.Type)
	}
}

// runShellProvisioner concatenates inline shell commands and runs them in a
// single sh session so `set -e` semantics carry across the script.
func runShellProvisioner(ctx context.Context, runner SSHRunner, p builder.Provisioner) error {
	if len(p.Inline) == 0 {
		return nil
	}
	cmd := strings.Join(append([]string{"set -e"}, p.Inline...), "\n")
	if p.WorkingDir != "" {
		cmd = fmt.Sprintf("cd %q && %s", p.WorkingDir, cmd)
	}
	if p.User != "" {
		// Use sudo -u so env vars propagate correctly via sudo -E.
		cmd = fmt.Sprintf("sudo -E -u %s -- sh -c %q", p.User, cmd)
	}
	out, err := runner.Run(ctx, cmd, p.Environment)
	if out != "" {
		logging.DebugContext(ctx, "shell output: %s", out)
	}
	return err
}

// runFileProvisioner uploads source to destination with mode.
func runFileProvisioner(ctx context.Context, runner SSHRunner, p builder.Provisioner) error {
	if p.Source == "" || p.Destination == "" {
		return fmt.Errorf("file provisioner requires source and destination")
	}
	return runner.UploadFile(ctx, p.Source, p.Destination, p.Mode)
}

// runScriptProvisioner uploads each script and executes it on the remote
// host. Each script runs as a separate command so failures are localized.
func runScriptProvisioner(ctx context.Context, runner SSHRunner, p builder.Provisioner) error {
	for _, script := range p.Scripts {
		remote := fmt.Sprintf("/tmp/warpgate-%s", filepathBase(script))
		if err := runner.UploadFile(ctx, script, remote, "0755"); err != nil {
			return fmt.Errorf("upload %s: %w", script, err)
		}
		out, err := runner.Run(ctx, remote, p.Environment)
		if out != "" {
			logging.DebugContext(ctx, "script %s output: %s", script, out)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// runPowerShellProvisioner runs PS1 scripts via powershell.exe over OpenSSH.
// Only useful on Windows source templates with the OpenSSH server enabled.
func runPowerShellProvisioner(ctx context.Context, runner SSHRunner, p builder.Provisioner) error {
	policy := p.ExecutionPolicy
	if policy == "" {
		policy = "Bypass"
	}
	for _, script := range p.PSScripts {
		remote := fmt.Sprintf("C:\\Windows\\Temp\\warpgate-%s", filepathBase(script))
		if err := runner.UploadFile(ctx, script, remote, ""); err != nil {
			return fmt.Errorf("upload %s: %w", script, err)
		}
		cmd := fmt.Sprintf("powershell.exe -ExecutionPolicy %s -File %s", policy, remote)
		if _, err := runner.Run(ctx, cmd, p.Environment); err != nil {
			return err
		}
	}
	return nil
}

// filepathBase returns the trailing element of a path without importing
// path/filepath, since provisioner sources may use either separator and
// path.Base would mangle Windows paths.
func filepathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
