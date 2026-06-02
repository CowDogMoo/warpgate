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

	pveapi "github.com/luthermonson/go-proxmox"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// sshRunnerFactory builds an SSHRunner for the host the cloned VM landed on.
// Pulled out so tests can stub it without requiring a real SSH server.
type sshRunnerFactory func(ctx context.Context, host string, target *builder.Target) (SSHRunner, error)

// liveRunner is the production implementation of [pipelineOps]. It calls
// into a live Proxmox cluster via [pveapi.Client].
type liveRunner struct {
	clients          *ProxmoxClients
	sshRunnerFactory sshRunnerFactory
	ipResolver       func(ctx context.Context, vm *pveapi.VirtualMachine) (string, error)
}

// newLiveRunner constructs the default pipelineOps backed by the provided
// Proxmox clients.
func newLiveRunner(clients *ProxmoxClients) *liveRunner {
	return &liveRunner{
		clients:          clients,
		sshRunnerFactory: defaultSSHRunnerFactory,
		ipResolver:       resolveVMIPViaAgent,
	}
}

// resolveSource looks up the source template VMID for the build.
func (r *liveRunner) resolveSource(ctx context.Context, target *builder.Target) (int, error) {
	node, err := r.clients.API.Node(ctx, target.Node)
	if err != nil {
		return 0, WrapWithRemediation(err, fmt.Sprintf("read node %q", target.Node))
	}
	return resolveSourceTemplate(ctx, node, target.SourceTemplate, target.SourceTemplateName)
}

// allocateVMID returns the configured VMID or asks the cluster for the next
// available value when target.NewVMID is zero.
func (r *liveRunner) allocateVMID(ctx context.Context, target *builder.Target) (int, error) {
	if target.NewVMID != 0 {
		return target.NewVMID, nil
	}
	return nextVMID(ctx, r.clients.API)
}

// clone copies the source template into newVMID on the configured node.
func (r *liveRunner) clone(ctx context.Context, sourceVMID, newVMID int, name string, target *builder.Target) (*pveapi.VirtualMachine, error) {
	node, err := r.clients.API.Node(ctx, target.Node)
	if err != nil {
		return nil, WrapWithRemediation(err, fmt.Sprintf("read node %q", target.Node))
	}
	src, err := node.VirtualMachine(ctx, sourceVMID)
	if err != nil {
		return nil, WrapWithRemediation(err, fmt.Sprintf("read source VMID %d", sourceVMID))
	}
	// Default to full clones; linked clones only work when source and clone
	// live on the same storage and are awkward to template afterwards.
	fullClone := !target.LinkedClone
	id, err := cloneTemplate(ctx, src, newVMID, name, target.Storage, target.Pool, fullClone)
	if err != nil {
		return nil, err
	}
	vm, err := node.VirtualMachine(ctx, id)
	if err != nil {
		return nil, WrapWithRemediation(err, fmt.Sprintf("read cloned VMID %d", id))
	}
	return vm, nil
}

// configureCloudInit applies cloud-init user/password/ssh-key and network
// options to the cloned VM. When the source template has no cloud-init
// drive configured, callers should pre-bake one or run with cloud-init
// disabled by leaving target.CloudInitUser empty.
func (r *liveRunner) configureCloudInit(ctx context.Context, vm *pveapi.VirtualMachine, target *builder.Target) error {
	opts := []pveapi.VirtualMachineOption{}
	if target.CloudInitUser != "" {
		opts = append(opts, pveapi.VirtualMachineOption{Name: "ciuser", Value: target.CloudInitUser})
	}
	if target.CloudInitPassword != "" {
		opts = append(opts, pveapi.VirtualMachineOption{Name: "cipassword", Value: target.CloudInitPassword})
	}
	if target.CloudInitSSHKey != "" {
		opts = append(opts, pveapi.VirtualMachineOption{Name: "sshkeys", Value: target.CloudInitSSHKey})
	}
	if target.CloudInitIPConfig != "" {
		opts = append(opts, pveapi.VirtualMachineOption{Name: "ipconfig0", Value: target.CloudInitIPConfig})
	}
	if target.CloudInitNameserver != "" {
		opts = append(opts, pveapi.VirtualMachineOption{Name: "nameserver", Value: target.CloudInitNameserver})
	}
	if len(opts) == 0 {
		return nil
	}
	task, err := vm.Config(ctx, opts...)
	if err != nil {
		return WrapWithRemediation(err, "apply cloud-init config")
	}
	return WrapWithRemediation(waitForTask(ctx, task), "wait for cloud-init config")
}

// startAndWait powers on the VM and waits for the QEMU guest agent.
func (r *liveRunner) startAndWait(ctx context.Context, vm *pveapi.VirtualMachine, target *builder.Target) error {
	return startAndWait(ctx, vm, target.AgentTimeoutSeconds)
}

// runProvisioners resolves the VM IP, opens an SSH connection, and executes
// provisioners. If target has no provisioners and no SSH config, the build
// proceeds straight to template conversion (useful for templating an
// already-baked source).
func (r *liveRunner) runProvisioners(ctx context.Context, vm *pveapi.VirtualMachine, cfg builder.Config, target *builder.Target) error {
	if len(cfg.Provisioners) == 0 {
		logging.InfoContext(ctx, "No provisioners to run; skipping provisioning step")
		return nil
	}
	host, err := r.ipResolver(ctx, vm)
	if err != nil {
		return WrapWithRemediation(err, "resolve VM IP via guest agent")
	}
	logging.InfoContext(ctx, "Connecting to VM at %s for provisioning", host)
	runner, err := r.sshRunnerFactory(ctx, host, target)
	if err != nil {
		return fmt.Errorf("open SSH session to %s: %w", host, err)
	}
	defer func() {
		if cerr := runner.Close(); cerr != nil {
			logging.WarnContext(ctx, "close SSH session: %v", cerr)
		}
	}()
	return runProvisioners(ctx, runner, cfg.Provisioners)
}

// stopAndTemplate stops the VM and flips it into a template.
func (r *liveRunner) stopAndTemplate(ctx context.Context, vm *pveapi.VirtualMachine) error {
	if err := stopVM(ctx, vm); err != nil {
		return err
	}
	return convertToTemplate(ctx, vm)
}

// cleanup attempts to delete a partially built VM. Errors are logged but
// not returned because the caller is already in a failure path.
func (r *liveRunner) cleanup(ctx context.Context, vm *pveapi.VirtualMachine) {
	if vm == nil {
		return
	}
	// Best-effort stop first so Delete doesn't fail on a running VM.
	if task, err := vm.Stop(ctx); err == nil {
		_ = waitForTask(ctx, task)
	}
	if err := deleteVM(ctx, vm); err != nil {
		logging.WarnContext(ctx, "cleanup: delete VMID %d: %v", vm.VMID, err)
	}
}

// resolveVMIPViaAgent asks the QEMU guest agent for the VM's IPv4 address
// on the first non-loopback interface. The guest agent must be installed
// in the source template and started in cloud-init.
func resolveVMIPViaAgent(ctx context.Context, vm *pveapi.VirtualMachine) (string, error) {
	ifaces, err := vm.AgentGetNetworkIFaces(ctx)
	if err != nil {
		return "", err
	}
	return pickAgentIP(ifaces)
}

// pickAgentIP is the pure portion of resolveVMIPViaAgent. It walks the
// supplied interfaces and returns the first IPv4 address that is neither
// blank nor loopback. Extracted as a separate function so the address
// selection logic can be unit-tested without an HTTP server.
func pickAgentIP(ifaces []*pveapi.AgentNetworkIface) (string, error) {
	for _, iface := range ifaces {
		if iface == nil {
			continue
		}
		if iface.Name == "lo" || iface.Name == "loopback" {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr == nil {
				continue
			}
			if addr.IPAddressType != "ipv4" {
				continue
			}
			if addr.IPAddress == "" || addr.IPAddress == "127.0.0.1" {
				continue
			}
			return addr.IPAddress, nil
		}
	}
	return "", fmt.Errorf("no non-loopback IPv4 address reported by guest agent")
}

// defaultSSHRunnerFactory builds the production SSHRunner that talks to
// the cloned VM over an actual SSH connection.
func defaultSSHRunnerFactory(ctx context.Context, host string, target *builder.Target) (SSHRunner, error) {
	conn := resolveSSHConnection(host, target)
	return newSSHRunner(ctx, conn, defaultSSHDialDeadline, nil)
}
