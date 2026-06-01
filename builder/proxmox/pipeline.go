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

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// pipelineOps captures the operations the pipeline runner needs from a
// Proxmox cluster. It exists so tests can inject a fake runner without
// going through a live PVE node.
type pipelineOps interface {
	// resolveSource resolves the source template VMID for the build.
	resolveSource(ctx context.Context, target *builder.Target) (int, error)

	// allocateVMID returns an unused VMID for the new VM. When the target
	// pins a specific VMID, that value is returned.
	allocateVMID(ctx context.Context, target *builder.Target) (int, error)

	// clone copies the source template into newVMID with the supplied name
	// and storage. Returns the new VMID and the cloned VM handle.
	clone(ctx context.Context, sourceVMID, newVMID int, name string, target *builder.Target) (*pveapi.VirtualMachine, error)

	// configureCloudInit applies cloud-init user-data and network settings.
	configureCloudInit(ctx context.Context, vm *pveapi.VirtualMachine, target *builder.Target) error

	// startAndWait powers the VM on and waits for the guest agent.
	startAndWait(ctx context.Context, vm *pveapi.VirtualMachine, target *builder.Target) error

	// runProvisioners executes provisioners against the running VM.
	runProvisioners(ctx context.Context, vm *pveapi.VirtualMachine, cfg builder.Config, target *builder.Target) error

	// stopAndTemplate stops the VM and converts it to a Proxmox template.
	stopAndTemplate(ctx context.Context, vm *pveapi.VirtualMachine) error

	// cleanup removes a partially built VM after a failure.
	cleanup(ctx context.Context, vm *pveapi.VirtualMachine)
}

// PipelineRunner orchestrates the clone → configure → provision → template
// flow. It owns no state across builds; one builder may run many pipelines.
type PipelineRunner struct {
	ops pipelineOps
}

// NewPipelineRunner constructs a runner with the supplied pipeline ops.
func NewPipelineRunner(ops pipelineOps) *PipelineRunner {
	return &PipelineRunner{ops: ops}
}

// Run executes the full pipeline and returns the resulting template VMID
// and the human-readable name of the produced template.
func (r *PipelineRunner) Run(ctx context.Context, cfg builder.Config, target *builder.Target, start time.Time) (int, string, error) {
	sourceVMID, err := r.ops.resolveSource(ctx, target)
	if err != nil {
		return 0, "", fmt.Errorf("resolve source template: %w", err)
	}
	logging.InfoContext(ctx, "Using source template VMID %d on node %s", sourceVMID, target.Node)

	newVMID, err := r.ops.allocateVMID(ctx, target)
	if err != nil {
		return 0, "", fmt.Errorf("allocate VMID: %w", err)
	}

	name := buildResourceName(target.TemplateName, start)
	logging.InfoContext(ctx, "Cloning template VMID %d → %d (%s)", sourceVMID, newVMID, name)

	vm, err := r.ops.clone(ctx, sourceVMID, newVMID, name, target)
	if err != nil {
		return 0, "", err
	}

	cleanupOnError := func(stage string, runErr error) error {
		logging.WarnContext(ctx, "build failed at %s; cleaning up VMID %d", stage, newVMID)
		r.ops.cleanup(ctx, vm)
		return runErr
	}

	if err := r.ops.configureCloudInit(ctx, vm, target); err != nil {
		return 0, "", cleanupOnError("configure cloud-init", err)
	}

	if err := r.ops.startAndWait(ctx, vm, target); err != nil {
		return 0, "", cleanupOnError("start VM / wait for agent", err)
	}

	if err := r.ops.runProvisioners(ctx, vm, cfg, target); err != nil {
		return 0, "", cleanupOnError("run provisioners", err)
	}

	if err := r.ops.stopAndTemplate(ctx, vm); err != nil {
		return 0, "", cleanupOnError("convert to template", err)
	}

	return newVMID, name, nil
}
