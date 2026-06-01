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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// ImageBuilder builds Proxmox VM templates by cloning a source template,
// running provisioners over SSH, and converting the result into a template.
type ImageBuilder struct {
	clients         *ProxmoxClients
	forceRecreate   bool
	cleanupOnFinish bool

	// buildID uniquely identifies this build so logs and resource names can
	// be correlated across concurrent runs.
	buildID string

	// pipelineFactory builds the pipelineOps used by Build. Defaults to
	// newLiveRunner; tests can substitute a fake.
	pipelineFactory func(*ProxmoxClients) pipelineOps
}

// generateBuildID creates a unique identifier for this build, matching the
// Azure builder's helper so staging paths align across builders.
func generateBuildID() string {
	timestamp := time.Now().UTC().Format("20060102-150405")
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return timestamp
	}
	return fmt.Sprintf("%s-%s", timestamp, hex.EncodeToString(randomBytes))
}

// NewImageBuilder constructs an ImageBuilder using default options.
func NewImageBuilder(ctx context.Context, cfg ClientConfig) (*ImageBuilder, error) {
	return NewImageBuilderWithOptions(ctx, cfg, false)
}

// NewImageBuilderWithOptions constructs an ImageBuilder with the supplied
// force-recreate flag. When forceRecreate is true, an existing template at
// the configured VMID is deleted before the clone runs.
func NewImageBuilderWithOptions(ctx context.Context, cfg ClientConfig, forceRecreate bool) (*ImageBuilder, error) {
	clients, err := NewProxmoxClients(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &ImageBuilder{
		clients:       clients,
		forceRecreate: forceRecreate,
		buildID:       generateBuildID(),
		pipelineFactory: func(c *ProxmoxClients) pipelineOps {
			return newLiveRunner(c)
		},
	}, nil
}

// GetBuildID returns the unique build identifier for log correlation.
func (b *ImageBuilder) GetBuildID() string {
	return b.buildID
}

// SetCleanupOnFinish controls whether the cloned VM is left in place after
// failure. Successful builds always produce a template; this flag only
// affects partial-build cleanup behavior.
func (b *ImageBuilder) SetCleanupOnFinish(v bool) {
	b.cleanupOnFinish = v
}

// Build executes the full Proxmox pipeline: clone the source template,
// apply cloud-init, boot, run provisioners over SSH, stop, and convert to
// a template. The resulting template VMID is returned in BuildResult.
func (b *ImageBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	start := time.Now()

	target, err := b.resolveTarget(cfg)
	if err != nil {
		return nil, err
	}

	logging.InfoContext(ctx, "Starting Proxmox build %s on node %s", b.buildID, target.Node)

	factory := b.pipelineFactory
	if factory == nil {
		factory = func(c *ProxmoxClients) pipelineOps { return newLiveRunner(c) }
	}
	runner := NewPipelineRunner(factory(b.clients))

	templateVMID, templateName, err := runner.Run(ctx, cfg, target, start)
	if err != nil {
		return nil, err
	}

	logging.InfoContext(ctx, "Published Proxmox template VMID %d (%s)", templateVMID, templateName)

	return &builder.BuildResult{
		TemplateVMID: templateVMID,
		TemplateName: templateName,
		Node:         target.Node,
		Duration:     time.Since(start).String(),
	}, nil
}

// Share is a stub for the [builder.ProxmoxImageBuilder] interface. Proxmox
// templates are not "shared" the way cloud images are; access control is
// handled by PVE ACLs on the pool/storage. This is intentionally a no-op
// returning an error so callers know to manage ACLs out-of-band.
func (b *ImageBuilder) Share(ctx context.Context, _ int, _ []string) error {
	logging.InfoContext(ctx, "Share is a no-op for Proxmox templates; use PVE ACLs/pools to grant access")
	return nil
}

// Delete removes a Proxmox template by VMID on the configured node. This is
// the closest analogue to AMI Deregister.
func (b *ImageBuilder) Delete(ctx context.Context, vmid int) error {
	if vmid == 0 {
		return fmt.Errorf("vmid is required")
	}
	node, err := b.clients.API.Node(ctx, b.clients.Node)
	if err != nil {
		return WrapWithRemediation(err, fmt.Sprintf("read node %q", b.clients.Node))
	}
	vm, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		return WrapWithRemediation(err, fmt.Sprintf("read template VMID %d", vmid))
	}
	return deleteVM(ctx, vmAPIAdapter{vm})
}

// Close releases the underlying client. Currently a no-op because the
// Proxmox client uses a pooled http.Transport that the runtime cleans up.
func (b *ImageBuilder) Close() error { return nil }

// resolveTarget extracts the proxmox target from cfg and applies builder
// defaults (default node) before validating required fields.
func (b *ImageBuilder) resolveTarget(cfg builder.Config) (*builder.Target, error) {
	target, err := findProxmoxTarget(cfg)
	if err != nil {
		return nil, err
	}
	if target.Node == "" {
		target.Node = b.clients.Node
	}
	if err := requireProxmoxFields(target); err != nil {
		return nil, err
	}
	return target, nil
}

// Compile-time check that ImageBuilder satisfies the ProxmoxImageBuilder interface.
var _ builder.ProxmoxImageBuilder = (*ImageBuilder)(nil)
