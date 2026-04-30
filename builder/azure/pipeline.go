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

package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// pipelineOps is the subset of pipelineRunner used by ImageBuilder.Build. It
// exists so tests can inject a fake without exercising the AIB SDK pollers.
type pipelineOps interface {
	submit(ctx context.Context, name string, tpl *armvirtualmachineimagebuilder.ImageTemplate) error
	run(ctx context.Context, name string) error
	readArtifact(ctx context.Context, name string) (string, error)
	describeLastRun(ctx context.Context, name string) (string, error)
	deleteTemplate(ctx context.Context, name string)
}

// pipelineRunner orchestrates an AIB image template through its lifecycle:
// CreateOrUpdate → Run → Read RunOutput. Errors at any stage are wrapped
// with enough context to point operators at the right Azure portal blade.
type pipelineRunner struct {
	clients       *AzureClients
	resourceGroup string
}

// newPipelineRunner is the default factory used by ImageBuilder. Tests can
// substitute a fake by overriding ImageBuilder.runnerFactory.
func newPipelineRunner(clients *AzureClients, resourceGroup string) pipelineOps {
	return &pipelineRunner{clients: clients, resourceGroup: resourceGroup}
}

// submit creates (or replaces) the image template resource in AIB.
// The poller blocks until provisioning finishes — this only creates the
// template definition, not the build itself.
func (p *pipelineRunner) submit(ctx context.Context, name string, tpl *armvirtualmachineimagebuilder.ImageTemplate) error {
	logging.InfoContext(ctx, "Submitting AIB image template %q", name)
	poller, err := p.clients.ImageTemplates.BeginCreateOrUpdate(ctx, p.resourceGroup, name, *tpl, nil)
	if err != nil {
		return fmt.Errorf("create image template %q: %w", name, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("provision image template %q: %w", name, err)
	}
	return nil
}

// run executes the image template build (the actual VM creation,
// customization, and capture). The poller resolves only after the build
// completes (success, failure, or cancellation).
func (p *pipelineRunner) run(ctx context.Context, name string) error {
	logging.InfoContext(ctx, "Running AIB image template %q", name)
	poller, err := p.clients.ImageTemplates.BeginRun(ctx, p.resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("start image template run %q: %w", name, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("image template run %q: %w", name, err)
	}
	return nil
}

// readArtifact reads the gallery image version artifact ID produced by the
// completed run. The RunOutputName matches the constant used in
// buildSharedImageDistributor.
func (p *pipelineRunner) readArtifact(ctx context.Context, name string) (string, error) {
	resp, err := p.clients.ImageTemplates.GetRunOutput(ctx, p.resourceGroup, name, runOutputName, nil)
	if err != nil {
		return "", fmt.Errorf("get run output for %q: %w", name, err)
	}
	if resp.Properties == nil || resp.Properties.ArtifactID == nil {
		return "", fmt.Errorf("run output for %q has no artifact ID", name)
	}
	return *resp.Properties.ArtifactID, nil
}

// describeLastRun fetches the most recent run state from the image template,
// used for surfacing diagnostic context on failures.
func (p *pipelineRunner) describeLastRun(ctx context.Context, name string) (string, error) {
	resp, err := p.clients.ImageTemplates.Get(ctx, p.resourceGroup, name, nil)
	if err != nil {
		return "", err
	}
	if resp.Properties == nil || resp.Properties.LastRunStatus == nil {
		return "", nil
	}
	state := ""
	if resp.Properties.LastRunStatus.RunState != nil {
		state = string(*resp.Properties.LastRunStatus.RunState)
	}
	msg := ""
	if resp.Properties.LastRunStatus.Message != nil {
		msg = *resp.Properties.LastRunStatus.Message
	}
	return fmt.Sprintf("state=%s message=%s", state, msg), nil
}

// deleteTemplate removes the image template resource. Errors are logged but
// not fatal — the template is just metadata once the artifact is published.
func (p *pipelineRunner) deleteTemplate(ctx context.Context, name string) {
	poller, err := p.clients.ImageTemplates.BeginDelete(ctx, p.resourceGroup, name, nil)
	if err != nil {
		logging.WarnContext(ctx, "Failed to start delete of image template %q: %v", name, err)
		return
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		logging.WarnContext(ctx, "Failed to delete image template %q: %v", name, err)
	}
}
