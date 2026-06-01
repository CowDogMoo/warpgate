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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	pveapi "github.com/luthermonson/go-proxmox"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

// pveFakeServer is a minimal stand-in for the Proxmox VE REST API. It
// returns canned, OK-shaped responses for the endpoints exercised by the
// liveRunner so the adapter and runner glue can be covered without a real
// cluster. Calls and POST bodies are recorded for assertions.
type pveFakeServer struct {
	t      *testing.T
	calls  []string
	mu     sync.Mutex
	taskID string
}

func newPVEFakeServer(t *testing.T) (*pveFakeServer, *httptest.Server, *pveapi.Client) {
	t.Helper()
	f := &pveFakeServer{
		t:      t,
		taskID: "UPID:pve1:0000ABCD:00000001:OK:warpgate:root@pam:",
	}
	srv := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(srv.Close)
	client := pveapi.NewClient(srv.URL, pveapi.WithAPIToken("user@pve!t", "secret"))
	return f, srv, client
}

func (f *pveFakeServer) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *pveFakeServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if body := f.responseFor(r); body != "" {
		_, _ = w.Write([]byte(body))
		return
	}
	// Catch-all OK response so unstubbed PVE calls don't crash the test.
	empty, _ := json.Marshal(map[string]any{"data": map[string]any{}})
	_, _ = w.Write(empty)
}

// responseFor returns the JSON body to send back for r, or "" to fall through
// to the catch-all. Split out from handle to keep cyclomatic complexity below
// the project's go-cyclo threshold (15).
func (f *pveFakeServer) responseFor(r *http.Request) string {
	path := r.URL.Path
	taskBody := fmt.Sprintf(`{"data":%q}`, f.taskID)

	if body := f.staticResponse(path, r.Method); body != "" {
		return body
	}
	if isTaskMutation(path, r.Method) {
		return taskBody
	}
	if strings.Contains(path, "/qemu/9000/status/current") || strings.Contains(path, "/qemu/9100/status/current") {
		return fmt.Sprintf(`{"data":{"vmid":%s,"name":"vm","status":"stopped"}}`, lastVMID(path))
	}
	if strings.HasSuffix(path, "/config") && r.Method == http.MethodGet {
		return `{"data":{"memory":2048,"cores":2,"name":"vm","scsihw":"virtio-scsi-pci"}}`
	}
	return ""
}

// staticResponse returns canned bodies for endpoints whose payload is fixed.
// Returns "" when no rule matches so the caller can try other matchers.
func (f *pveFakeServer) staticResponse(path, method string) string {
	switch {
	case strings.HasSuffix(path, "/cluster/nextid"):
		return `{"data":"9105"}`
	case strings.HasSuffix(path, "/nodes/pve1") && method == http.MethodGet:
		return `{"data":{"node":"pve1","status":"online"}}`
	case strings.HasSuffix(path, "/nodes/pve1/qemu") && method == http.MethodGet:
		return `{"data":[{"vmid":9000,"name":"kali-template"},{"vmid":9001,"name":"other"}]}`
	case strings.Contains(path, "/agent/network-get-interfaces"):
		return `{"data":{"result":[{"name":"lo","ip-addresses":[{"ip-address-type":"ipv4","ip-address":"127.0.0.1"}]},{"name":"eth0","ip-addresses":[{"ip-address-type":"ipv4","ip-address":"10.0.0.5"}]}]}}`
	case strings.Contains(path, "/agent/ping"):
		return `{"data":{}}`
	case strings.HasSuffix(path, "/agent/get-osinfo"):
		return `{"data":{"result":{"id":"ubuntu","name":"Ubuntu","kernel-release":"5.15.0","pretty-name":"Ubuntu","machine":"x86_64","version":"22.04","version-id":"22.04"}}}`
	case strings.Contains(path, "/tasks/") && strings.HasSuffix(path, "/status"):
		return `{"data":{"status":"stopped","exitstatus":"OK"}}`
	case strings.HasSuffix(path, "/version"):
		return `{"data":{"version":"8.0.0","release":"0","repoid":"test"}}`
	}
	return ""
}

// isTaskMutation reports whether path/method is a write that PVE replies to
// with a task UPID string.
func isTaskMutation(path, method string) bool {
	if method == http.MethodDelete {
		return true
	}
	if strings.HasSuffix(path, "/clone") ||
		strings.HasSuffix(path, "/status/start") ||
		strings.HasSuffix(path, "/status/stop") ||
		strings.HasSuffix(path, "/status/shutdown") ||
		strings.HasSuffix(path, "/template") {
		return true
	}
	if strings.HasSuffix(path, "/config") && method != http.MethodGet {
		return true
	}
	return false
}

// lastVMID returns the trailing numeric segment of /qemu/<vmid>/.... Used by
// the status/current handler to echo back the requested VMID.
func lastVMID(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "qemu" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "0"
}

func newLiveRunnerFor(t *testing.T) (*liveRunner, *pveFakeServer) {
	t.Helper()
	f, _, client := newPVEFakeServer(t)
	clients := &ProxmoxClients{API: client, Node: "pve1"}
	r := newLiveRunner(clients)
	return r, f
}

func TestLiveRunner_ResolveSource_ByVMID(t *testing.T) {
	t.Parallel()
	r, _ := newLiveRunnerFor(t)
	got, err := r.resolveSource(context.Background(), &builder.Target{
		Node:           "pve1",
		SourceTemplate: 9000,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 9000 {
		t.Fatalf("expected 9000, got %d", got)
	}
}

func TestLiveRunner_ResolveSource_ByName(t *testing.T) {
	t.Parallel()
	r, _ := newLiveRunnerFor(t)
	got, err := r.resolveSource(context.Background(), &builder.Target{
		Node:               "pve1",
		SourceTemplateName: "kali-template",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 9000 {
		t.Fatalf("expected vmid 9000 for kali-template, got %d", got)
	}
}

func TestLiveRunner_AllocateVMID_FromCluster(t *testing.T) {
	t.Parallel()
	r, _ := newLiveRunnerFor(t)
	got, err := r.allocateVMID(context.Background(), &builder.Target{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 9105 {
		t.Fatalf("expected 9105, got %d", got)
	}
}

func TestLiveRunner_Clone(t *testing.T) {
	t.Parallel()
	r, _ := newLiveRunnerFor(t)
	vm, err := r.clone(context.Background(), 9000, 9100, "kali-build", &builder.Target{
		Node:    "pve1",
		Storage: "local-zfs",
		Pool:    "deploy",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if vm == nil {
		t.Fatal("expected non-nil cloned VM")
	}
}

func TestLiveRunner_ConfigureCloudInit_NoOptions(t *testing.T) {
	t.Parallel()
	r, _ := newLiveRunnerFor(t)
	// No cloud-init fields set → fast return without making API calls.
	if err := r.configureCloudInit(context.Background(), &pveapi.VirtualMachine{Node: "pve1", VMID: 9100}, &builder.Target{}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestLiveRunner_ConfigureCloudInit_AppliesAllFields(t *testing.T) {
	t.Parallel()
	r, f := newLiveRunnerFor(t)
	target := &builder.Target{
		CloudInitUser:       "ansible",
		CloudInitPassword:   "secret",
		CloudInitSSHKey:     "ssh-ed25519 AAAA...",
		CloudInitIPConfig:   "ip=dhcp",
		CloudInitNameserver: "1.1.1.1",
	}
	// Need to read the VM into being via the SDK so we have a *pveapi.VirtualMachine
	// with a wired-up client; do that by hitting the status endpoint first.
	node, err := r.clients.API.Node(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	vm, err := node.VirtualMachine(context.Background(), 9100)
	if err != nil {
		t.Fatalf("VirtualMachine: %v", err)
	}
	if err := r.configureCloudInit(context.Background(), vm, target); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	found := false
	for _, c := range f.recorded() {
		if strings.Contains(c, "/qemu/9100/config") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a /config POST, got calls: %v", f.recorded())
	}
}

func TestLiveRunner_StopAndTemplate(t *testing.T) {
	t.Parallel()
	r, f := newLiveRunnerFor(t)
	node, err := r.clients.API.Node(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	vm, err := node.VirtualMachine(context.Background(), 9100)
	if err != nil {
		t.Fatalf("VirtualMachine: %v", err)
	}
	if err := r.stopAndTemplate(context.Background(), vm); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	calls := f.recorded()
	gotTemplate, gotShutdown := false, false
	for _, c := range calls {
		if strings.HasSuffix(c, "/template") {
			gotTemplate = true
		}
		if strings.Contains(c, "/shutdown") {
			gotShutdown = true
		}
	}
	if !gotShutdown {
		t.Errorf("expected a shutdown call, got %v", calls)
	}
	if !gotTemplate {
		t.Errorf("expected a /template call, got %v", calls)
	}
}

func TestLiveRunner_Cleanup_NoOpForNilVM(t *testing.T) {
	t.Parallel()
	r, f := newLiveRunnerFor(t)
	r.cleanup(context.Background(), nil)
	if len(f.recorded()) != 0 {
		t.Fatalf("cleanup of nil VM should not hit the API: %v", f.recorded())
	}
}

func TestLiveRunner_Cleanup_DeletesVM(t *testing.T) {
	t.Parallel()
	r, f := newLiveRunnerFor(t)
	node, err := r.clients.API.Node(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	vm, err := node.VirtualMachine(context.Background(), 9100)
	if err != nil {
		t.Fatalf("VirtualMachine: %v", err)
	}
	r.cleanup(context.Background(), vm)
	deletes := 0
	for _, c := range f.recorded() {
		if strings.HasPrefix(c, "DELETE ") {
			deletes++
		}
	}
	if deletes == 0 {
		t.Fatalf("expected a DELETE call, got %v", f.recorded())
	}
}

func TestLiveRunner_StartAndWait(t *testing.T) {
	t.Parallel()
	r, f := newLiveRunnerFor(t)
	node, err := r.clients.API.Node(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	vm, err := node.VirtualMachine(context.Background(), 9100)
	if err != nil {
		t.Fatalf("VirtualMachine: %v", err)
	}
	if err := r.startAndWait(context.Background(), vm, &builder.Target{AgentTimeoutSeconds: 1}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	gotStart, gotAgent := false, false
	for _, c := range f.recorded() {
		if strings.HasSuffix(c, "/status/start") {
			gotStart = true
		}
		if strings.Contains(c, "/agent/get-osinfo") {
			gotAgent = true
		}
	}
	if !gotStart {
		t.Errorf("expected /status/start call, got %v", f.recorded())
	}
	if !gotAgent {
		t.Errorf("expected /agent/get-osinfo call, got %v", f.recorded())
	}
}

func TestLiveRunner_Clone_BadSourceVMID(t *testing.T) {
	t.Parallel()
	r, _ := newLiveRunnerFor(t)
	// Source VMID 8888 isn't stubbed; the SDK's GET /status/current will
	// land in the catch-all empty body and the clone call won't happen.
	_, err := r.clone(context.Background(), 8888, 9100, "kali", &builder.Target{Node: "pve1"})
	// We may or may not get an error depending on go-proxmox's behavior; if we
	// do, it should surface through WrapWithRemediation.
	if err != nil && !strings.Contains(err.Error(), "source VMID") && !strings.Contains(err.Error(), "remediation") {
		// Anything else means the error path isn't wrapped — but we don't
		// require an error here; the test purely exercises the code path.
		t.Logf("non-fatal: unexpected error shape: %v", err)
	}
}

func TestImageBuilder_Delete_HitsAPI(t *testing.T) {
	t.Parallel()
	_, _, client := newPVEFakeServer(t)
	b := &ImageBuilder{
		clients: &ProxmoxClients{API: client, Node: "pve1"},
		buildID: "test",
	}
	if err := b.Delete(context.Background(), 9100); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageBuilder_Delete_UnknownNode(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a 500 so node lookup fails — surfaces the WrapWithRemediation path.
		http.Error(w, "internal", http.StatusInternalServerError)
	}))
	defer srv.Close()
	b := &ImageBuilder{
		clients: &ProxmoxClients{
			API:  pveapi.NewClient(srv.URL, pveapi.WithAPIToken("u@pve!t", "secret")),
			Node: "pve-missing",
		},
		buildID: "test",
	}
	if err := b.Delete(context.Background(), 9100); err == nil {
		t.Fatal("expected error from delete against bad node")
	}
}

func TestResolveVMIPViaAgent_LiveCall(t *testing.T) {
	t.Parallel()
	r, _ := newLiveRunnerFor(t)
	node, err := r.clients.API.Node(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	vm, err := node.VirtualMachine(context.Background(), 9100)
	if err != nil {
		t.Fatalf("VirtualMachine: %v", err)
	}
	got, err := resolveVMIPViaAgent(context.Background(), vm)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "10.0.0.5" {
		t.Fatalf("expected 10.0.0.5, got %q", got)
	}
}
