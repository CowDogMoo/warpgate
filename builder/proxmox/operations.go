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
	"time"

	pveapi "github.com/luthermonson/go-proxmox"
)

// defaultTaskWaitTimeout bounds how long we wait on a single Proxmox task
// (clone, start, stop, etc.) before considering it stuck. Matches PVE's
// observed worst-case for a full clone of a multi-GB template.
const defaultTaskWaitTimeout = 30 * time.Minute

// nodeAPI is the subset of *pveapi.Node operations we use. Defined as an
// interface so tests can inject a fake without spinning up an HTTP server.
type nodeAPI interface {
	NewVirtualMachine(ctx context.Context, vmid int, options ...pveapi.VirtualMachineOption) (*pveapi.Task, error)
	VirtualMachine(ctx context.Context, vmid int) (*pveapi.VirtualMachine, error)
	VirtualMachines(ctx context.Context) (pveapi.VirtualMachines, error)
}

// vmAPI is the subset of *pveapi.VirtualMachine operations the builder calls.
// Pulled out so tests can substitute a fake without contacting Proxmox.
type vmAPI interface {
	Clone(ctx context.Context, params *pveapi.VirtualMachineCloneOptions) (int, *pveapi.Task, error)
	Config(ctx context.Context, options ...pveapi.VirtualMachineOption) (*pveapi.Task, error)
	Start(ctx context.Context) (*pveapi.Task, error)
	Stop(ctx context.Context) (*pveapi.Task, error)
	Shutdown(ctx context.Context) (*pveapi.Task, error)
	Delete(ctx context.Context) (*pveapi.Task, error)
	ConvertToTemplate(ctx context.Context) (*pveapi.Task, error)
	WaitForAgent(ctx context.Context, seconds int) error
}

// resolveSourceTemplate returns the VMID of the source template. If t.SourceTemplate
// is set, it is returned as-is; otherwise t.SourceTemplateName is matched
// against the VM list on the node.
func resolveSourceTemplate(ctx context.Context, node nodeAPI, sourceVMID int, sourceName string) (int, error) {
	if sourceVMID != 0 {
		return sourceVMID, nil
	}
	if sourceName == "" {
		return 0, fmt.Errorf("source template VMID or name is required")
	}
	vms, err := node.VirtualMachines(ctx)
	if err != nil {
		return 0, fmt.Errorf("list VMs: %w", err)
	}
	for _, vm := range vms {
		if vm.Name == sourceName {
			return int(vm.VMID), nil
		}
	}
	return 0, fmt.Errorf("source template %q not found on node", sourceName)
}

// nextVMID returns a free VMID using Proxmox's cluster-wide /cluster/nextid
// endpoint. The Proxmox client takes a uint64; we narrow back to int because
// PVE VMIDs are bounded well below 2^31.
func nextVMID(ctx context.Context, client *pveapi.Client) (int, error) {
	type nextIDResp struct {
		Data string `json:"data"`
	}
	var resp nextIDResp
	if err := client.Get(ctx, "/cluster/nextid", &resp); err != nil {
		return 0, fmt.Errorf("allocate next vmid: %w", err)
	}
	var id int
	if _, err := fmt.Sscanf(resp.Data, "%d", &id); err != nil {
		return 0, fmt.Errorf("parse next vmid %q: %w", resp.Data, err)
	}
	return id, nil
}

// waitForTask blocks until the Proxmox task completes (or the context is
// cancelled). The poll interval is 1s; the upper bound is defaultTaskWaitTimeout.
func waitForTask(ctx context.Context, t *pveapi.Task) error {
	if t == nil {
		return nil
	}
	return t.Wait(ctx, time.Second, defaultTaskWaitTimeout)
}

// cloneTemplate clones the source template into newID with the supplied
// human-readable name. The clone is full by default; callers can override
// via params if a linked clone is desired.
func cloneTemplate(ctx context.Context, src vmAPI, newID int, name, storage, pool string, fullClone bool) (int, error) {
	full := pveapi.IntOrBool(false)
	if fullClone {
		full = pveapi.IntOrBool(true)
	}
	params := &pveapi.VirtualMachineCloneOptions{
		NewID:   newID,
		Name:    name,
		Storage: storage,
		Pool:    pool,
		Full:    full,
	}
	id, task, err := src.Clone(ctx, params)
	if err != nil {
		return 0, WrapWithRemediation(err, "clone template")
	}
	if err := waitForTask(ctx, task); err != nil {
		return 0, WrapWithRemediation(err, "wait for clone task")
	}
	return id, nil
}

// startAndWait starts vm and waits for the guest agent. agentSeconds bounds
// the agent wait; zero falls back to 300s (5 min), which is comfortable for
// cloud-init plus first boot.
func startAndWait(ctx context.Context, vm vmAPI, agentSeconds int) error {
	task, err := vm.Start(ctx)
	if err != nil {
		return WrapWithRemediation(err, "start VM")
	}
	if err := waitForTask(ctx, task); err != nil {
		return WrapWithRemediation(err, "wait for start task")
	}
	if agentSeconds <= 0 {
		agentSeconds = 300
	}
	if err := vm.WaitForAgent(ctx, agentSeconds); err != nil {
		return WrapWithRemediation(err, "wait for guest agent")
	}
	return nil
}

// stopVM attempts a graceful shutdown first; if the context deadline is
// short or shutdown fails, we fall back to a hard stop. Either way the VM
// is left in a stopped state ready for templating.
func stopVM(ctx context.Context, vm vmAPI) error {
	if task, err := vm.Shutdown(ctx); err == nil {
		if werr := waitForTask(ctx, task); werr == nil {
			return nil
		}
	}
	task, err := vm.Stop(ctx)
	if err != nil {
		return WrapWithRemediation(err, "stop VM")
	}
	return WrapWithRemediation(waitForTask(ctx, task), "wait for stop task")
}

// convertToTemplate flips the cloned VM into a Proxmox template so it can be
// cloned by subsequent builds.
func convertToTemplate(ctx context.Context, vm vmAPI) error {
	task, err := vm.ConvertToTemplate(ctx)
	if err != nil {
		return WrapWithRemediation(err, "convert to template")
	}
	return WrapWithRemediation(waitForTask(ctx, task), "wait for template conversion")
}

// deleteVM removes the VM. Used to clean up partial builds on failure.
func deleteVM(ctx context.Context, vm vmAPI) error {
	task, err := vm.Delete(ctx)
	if err != nil {
		return WrapWithRemediation(err, "delete VM")
	}
	return WrapWithRemediation(waitForTask(ctx, task), "wait for delete task")
}
