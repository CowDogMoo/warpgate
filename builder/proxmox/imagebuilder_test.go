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
	"errors"
	"strings"
	"testing"
	"time"

	pveapi "github.com/luthermonson/go-proxmox"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

// fakeOps is a test double for pipelineOps that records the calls it gets
// and can be programmed to fail at specific stages.
type fakeOps struct {
	sourceVMID int
	newVMID    int
	failAt     string // "resolve","allocate","clone","cloudinit","start","provision","template"
	cleanupHit bool
	stages     []string
}

func (f *fakeOps) resolveSource(_ context.Context, _ *builder.Target) (int, error) {
	f.stages = append(f.stages, "resolve")
	if f.failAt == "resolve" {
		return 0, errors.New("resolve fail")
	}
	return f.sourceVMID, nil
}

func (f *fakeOps) allocateVMID(_ context.Context, _ *builder.Target) (int, error) {
	f.stages = append(f.stages, "allocate")
	if f.failAt == "allocate" {
		return 0, errors.New("allocate fail")
	}
	return f.newVMID, nil
}

func (f *fakeOps) clone(_ context.Context, _, _ int, _ string, _ *builder.Target) (*pveapi.VirtualMachine, error) {
	f.stages = append(f.stages, "clone")
	if f.failAt == "clone" {
		return nil, errors.New("clone fail")
	}
	// Return a non-nil VM with a known VMID so cleanup hits the log line.
	return &pveapi.VirtualMachine{VMID: pveapi.StringOrUint64(uint64(f.newVMID))}, nil
}

func (f *fakeOps) configureCloudInit(_ context.Context, _ *pveapi.VirtualMachine, _ *builder.Target) error {
	f.stages = append(f.stages, "cloudinit")
	if f.failAt == "cloudinit" {
		return errors.New("cloudinit fail")
	}
	return nil
}

func (f *fakeOps) startAndWait(_ context.Context, _ *pveapi.VirtualMachine, _ *builder.Target) error {
	f.stages = append(f.stages, "start")
	if f.failAt == "start" {
		return errors.New("start fail")
	}
	return nil
}

func (f *fakeOps) runProvisioners(_ context.Context, _ *pveapi.VirtualMachine, _ builder.Config, _ *builder.Target) error {
	f.stages = append(f.stages, "provision")
	if f.failAt == "provision" {
		return errors.New("provision fail")
	}
	return nil
}

func (f *fakeOps) stopAndTemplate(_ context.Context, _ *pveapi.VirtualMachine) error {
	f.stages = append(f.stages, "template")
	if f.failAt == "template" {
		return errors.New("template fail")
	}
	return nil
}

func (f *fakeOps) cleanup(_ context.Context, _ *pveapi.VirtualMachine) {
	f.cleanupHit = true
}

func newBuilderWithFakeOps(t *testing.T, ops pipelineOps) *ImageBuilder {
	t.Helper()
	b := &ImageBuilder{
		clients:         &ProxmoxClients{Node: "pve1"},
		buildID:         "test-build",
		pipelineFactory: func(*ProxmoxClients) pipelineOps { return ops },
	}
	return b
}

func TestImageBuilder_Build_HappyPath(t *testing.T) {
	t.Parallel()
	ops := &fakeOps{sourceVMID: 9000, newVMID: 9100}
	b := newBuilderWithFakeOps(t, ops)

	cfg := builder.Config{
		Targets: []builder.Target{{
			Type:           "proxmox",
			Node:           "pve1",
			SourceTemplate: 9000,
			TemplateName:   "kali",
		}},
	}

	result, err := b.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if result.TemplateVMID != 9100 {
		t.Fatalf("expected TemplateVMID=9100, got %d", result.TemplateVMID)
	}
	if !strings.HasPrefix(result.TemplateName, "kali-") {
		t.Fatalf("expected template name to start with kali-, got %q", result.TemplateName)
	}
	if result.Node != "pve1" {
		t.Fatalf("expected node pve1, got %q", result.Node)
	}
	wantStages := []string{"resolve", "allocate", "clone", "cloudinit", "start", "provision", "template"}
	if got := strings.Join(ops.stages, ","); got != strings.Join(wantStages, ",") {
		t.Fatalf("unexpected stage order: %s", got)
	}
	if ops.cleanupHit {
		t.Fatal("cleanup should not run on success")
	}
}

func TestImageBuilder_Build_FailureTriggersCleanup(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"cloudinit", "start", "provision", "template"} {
		stage := stage
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			ops := &fakeOps{sourceVMID: 9000, newVMID: 9100, failAt: stage}
			b := newBuilderWithFakeOps(t, ops)
			cfg := builder.Config{
				Targets: []builder.Target{{
					Type:           "proxmox",
					Node:           "pve1",
					SourceTemplate: 9000,
					TemplateName:   "kali",
				}},
			}
			if _, err := b.Build(context.Background(), cfg); err == nil {
				t.Fatalf("expected error from failAt=%s", stage)
			}
			if !ops.cleanupHit {
				t.Fatalf("expected cleanup to be invoked after failAt=%s", stage)
			}
		})
	}
}

func TestImageBuilder_Build_NoProxmoxTarget(t *testing.T) {
	t.Parallel()
	b := newBuilderWithFakeOps(t, &fakeOps{})
	cfg := builder.Config{
		Targets: []builder.Target{{Type: "container"}},
	}
	_, err := b.Build(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "no proxmox target") {
		t.Fatalf("expected no-target error, got %v", err)
	}
}

func TestImageBuilder_Build_MissingFields(t *testing.T) {
	t.Parallel()
	b := newBuilderWithFakeOps(t, &fakeOps{})
	cfg := builder.Config{
		Targets: []builder.Target{{
			Type:           "proxmox",
			SourceTemplate: 9000,
			TemplateName:   "kali",
		}}, // missing Node
	}
	// builder.clients.Node fallback should not apply because client config
	// node was set in the helper. Verify defaulting works.
	result, err := b.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected defaulted node to succeed, got %v", err)
	}
	if result.Node != "pve1" {
		t.Fatalf("expected default node pve1, got %q", result.Node)
	}
}

func TestNewImageBuilder(t *testing.T) {
	t.Parallel()
	b, err := NewImageBuilder(context.Background(), ClientConfig{
		Endpoint:   "https://pve.example.com",
		Node:       "pve1",
		APITokenID: "u@pve!t",
		APIToken:   "secret",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if b.GetBuildID() == "" {
		t.Fatal("expected non-empty build ID")
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewImageBuilderWithOptions_Forwards(t *testing.T) {
	t.Parallel()
	b, err := NewImageBuilderWithOptions(context.Background(), ClientConfig{
		Endpoint: "https://pve",
		Node:     "pve1",
		Username: "root@pam",
		Password: "secret",
	}, true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !b.forceRecreate {
		t.Fatal("expected forceRecreate=true")
	}
}

func TestImageBuilder_SetCleanupOnFinish(t *testing.T) {
	t.Parallel()
	b := newBuilderWithFakeOps(t, &fakeOps{})
	b.SetCleanupOnFinish(true)
	if !b.cleanupOnFinish {
		t.Fatal("expected cleanupOnFinish=true after Set")
	}
}

func TestImageBuilder_Share_IsNoOp(t *testing.T) {
	t.Parallel()
	b := newBuilderWithFakeOps(t, &fakeOps{})
	if err := b.Share(context.Background(), 9100, []string{"alice"}); err != nil {
		t.Fatalf("expected Share to be a no-op, got %v", err)
	}
}

func TestImageBuilder_Delete_RequiresVMID(t *testing.T) {
	t.Parallel()
	b := newBuilderWithFakeOps(t, &fakeOps{})
	if err := b.Delete(context.Background(), 0); err == nil {
		t.Fatal("expected vmid required error")
	}
}

func TestGenerateBuildID(t *testing.T) {
	t.Parallel()
	id1 := generateBuildID()
	time.Sleep(2 * time.Millisecond)
	id2 := generateBuildID()
	if id1 == "" || id2 == "" {
		t.Fatal("expected non-empty build IDs")
	}
	// Even at the same second they should differ because of the random suffix.
}
