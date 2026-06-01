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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pveapi "github.com/luthermonson/go-proxmox"
)

// fakeVM is a hand-rolled implementation of vmAPI for testing the operation
// helpers without contacting Proxmox.
type fakeVM struct {
	cloneErr      error
	configErr     error
	startErr      error
	stopErr       error
	shutdownErr   error
	deleteErr     error
	convertErr    error
	waitAgentErr  error
	calls         []string
	clonedNewID   int
	shutdownFails bool
}

func (f *fakeVM) Clone(_ context.Context, params *pveapi.VirtualMachineCloneOptions) (int, *pveapi.Task, error) {
	f.calls = append(f.calls, "Clone")
	if f.cloneErr != nil {
		return 0, nil, f.cloneErr
	}
	id := f.clonedNewID
	if id == 0 {
		id = params.NewID
	}
	return id, nil, nil
}
func (f *fakeVM) Config(_ context.Context, _ ...pveapi.VirtualMachineOption) (*pveapi.Task, error) {
	f.calls = append(f.calls, "Config")
	return nil, f.configErr
}
func (f *fakeVM) Start(_ context.Context) (*pveapi.Task, error) {
	f.calls = append(f.calls, "Start")
	return nil, f.startErr
}
func (f *fakeVM) Stop(_ context.Context) (*pveapi.Task, error) {
	f.calls = append(f.calls, "Stop")
	return nil, f.stopErr
}
func (f *fakeVM) Shutdown(_ context.Context) (*pveapi.Task, error) {
	f.calls = append(f.calls, "Shutdown")
	if f.shutdownFails {
		return nil, errors.New("shutdown not allowed")
	}
	return nil, f.shutdownErr
}
func (f *fakeVM) Delete(_ context.Context) (*pveapi.Task, error) {
	f.calls = append(f.calls, "Delete")
	return nil, f.deleteErr
}
func (f *fakeVM) ConvertToTemplate(_ context.Context) (*pveapi.Task, error) {
	f.calls = append(f.calls, "ConvertToTemplate")
	return nil, f.convertErr
}
func (f *fakeVM) WaitForAgent(_ context.Context, _ int) error {
	f.calls = append(f.calls, "WaitForAgent")
	return f.waitAgentErr
}

// fakeNode implements nodeAPI for resolveSourceTemplate tests.
type fakeNode struct {
	vms     pveapi.VirtualMachines
	listErr error
}

func (f *fakeNode) VirtualMachines(_ context.Context) (pveapi.VirtualMachines, error) {
	return f.vms, f.listErr
}

func TestResolveSourceTemplate_ByVMID(t *testing.T) {
	t.Parallel()
	got, err := resolveSourceTemplate(context.Background(), &fakeNode{}, 9000, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 9000 {
		t.Fatalf("expected 9000, got %d", got)
	}
}

func TestResolveSourceTemplate_ByName(t *testing.T) {
	t.Parallel()
	n := &fakeNode{vms: pveapi.VirtualMachines{
		&pveapi.VirtualMachine{Name: "other", VMID: 100},
		&pveapi.VirtualMachine{Name: "kali-template", VMID: 9001},
	}}
	got, err := resolveSourceTemplate(context.Background(), n, 0, "kali-template")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 9001 {
		t.Fatalf("expected 9001, got %d", got)
	}
}

func TestResolveSourceTemplate_NameNotFound(t *testing.T) {
	t.Parallel()
	n := &fakeNode{vms: pveapi.VirtualMachines{&pveapi.VirtualMachine{Name: "other", VMID: 100}}}
	if _, err := resolveSourceTemplate(context.Background(), n, 0, "missing"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestResolveSourceTemplate_RequiresOne(t *testing.T) {
	t.Parallel()
	if _, err := resolveSourceTemplate(context.Background(), &fakeNode{}, 0, ""); err == nil {
		t.Fatal("expected error when both empty")
	}
}

func TestResolveSourceTemplate_ListErrorPropagates(t *testing.T) {
	t.Parallel()
	n := &fakeNode{listErr: errors.New("permission denied")}
	if _, err := resolveSourceTemplate(context.Background(), n, 0, "anything"); err == nil {
		t.Fatal("expected list error to propagate")
	}
}

func TestCloneTemplate_FullClone(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{clonedNewID: 9100}
	id, err := cloneTemplate(context.Background(), vm, 9100, "kali", "local-zfs", "deploy", true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if id != 9100 {
		t.Fatalf("expected 9100, got %d", id)
	}
	if len(vm.calls) != 1 || vm.calls[0] != "Clone" {
		t.Fatalf("expected Clone call, got %v", vm.calls)
	}
}

func TestCloneTemplate_LinkedClone(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{clonedNewID: 9101}
	if _, err := cloneTemplate(context.Background(), vm, 9101, "kali", "", "", false); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestCloneTemplate_PropagatesError(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{cloneErr: errors.New("VM is locked (clone)")}
	if _, err := cloneTemplate(context.Background(), vm, 9100, "kali", "", "", true); err == nil {
		t.Fatal("expected clone error")
	}
}

func TestStartAndWait_HappyPath(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{}
	if err := startAndWait(context.Background(), vm, 0); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(vm.calls) != 2 || vm.calls[0] != "Start" || vm.calls[1] != "WaitForAgent" {
		t.Fatalf("unexpected call order: %v", vm.calls)
	}
}

func TestStartAndWait_StartFails(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{startErr: errors.New("no VM resources")}
	if err := startAndWait(context.Background(), vm, 60); err == nil {
		t.Fatal("expected start error")
	}
}

func TestStartAndWait_AgentTimesOut(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{waitAgentErr: errors.New("QEMU guest agent is not running")}
	if err := startAndWait(context.Background(), vm, 1); err == nil {
		t.Fatal("expected agent error")
	}
}

func TestStopVM_ShutdownFallbackToStop(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{shutdownFails: true}
	if err := stopVM(context.Background(), vm); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Both Shutdown and Stop should have been attempted.
	if len(vm.calls) < 2 || vm.calls[0] != "Shutdown" || vm.calls[1] != "Stop" {
		t.Fatalf("expected Shutdown then Stop, got %v", vm.calls)
	}
}

func TestStopVM_ShutdownAlone(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{}
	if err := stopVM(context.Background(), vm); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Shutdown alone is fine; Stop is not called.
	if len(vm.calls) != 1 || vm.calls[0] != "Shutdown" {
		t.Fatalf("expected Shutdown only, got %v", vm.calls)
	}
}

func TestStopVM_StopErrors(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{shutdownFails: true, stopErr: errors.New("VM is locked")}
	if err := stopVM(context.Background(), vm); err == nil {
		t.Fatal("expected stop error")
	}
}

func TestConvertToTemplate(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{}
	if err := convertToTemplate(context.Background(), vm); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	vmErr := &fakeVM{convertErr: errors.New("permission check")}
	if err := convertToTemplate(context.Background(), vmErr); err == nil {
		t.Fatal("expected convert error")
	}
}

func TestDeleteVM(t *testing.T) {
	t.Parallel()
	vm := &fakeVM{}
	if err := deleteVM(context.Background(), vm); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	vmErr := &fakeVM{deleteErr: errors.New("VM is locked")}
	if err := deleteVM(context.Background(), vmErr); err == nil {
		t.Fatal("expected delete error")
	}
}

func TestWaitForTask_NilIsNoop(t *testing.T) {
	t.Parallel()
	if err := waitForTask(context.Background(), nil); err != nil {
		t.Fatalf("expected nil for nil task, got %v", err)
	}
}

func TestNextVMID_ParsesResponse(t *testing.T) {
	t.Parallel()
	// Spin up a tiny HTTP server that mimics PVE's /cluster/nextid endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/cluster/nextid") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"9105"}`))
	}))
	defer srv.Close()

	client := pveapi.NewClient(srv.URL, pveapi.WithAPIToken("u@pve!t", "secret"))
	got, err := nextVMID(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 9105 {
		t.Fatalf("expected 9105, got %d", got)
	}
}

func TestNextVMID_UnparseableResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":"not-a-number"}`))
	}))
	defer srv.Close()
	client := pveapi.NewClient(srv.URL, pveapi.WithAPIToken("u@pve!t", "secret"))
	if _, err := nextVMID(context.Background(), client); err == nil {
		t.Fatal("expected parse error")
	}
}
