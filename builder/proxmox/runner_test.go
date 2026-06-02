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

	pveapi "github.com/luthermonson/go-proxmox"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

// findOption returns the value paired with name in opts, or "" when name
// isn't present. Keeps the assertion helpers below readable.
func findOption(opts []pveapi.VirtualMachineOption, name string) string {
	for _, o := range opts {
		if o.Name == name {
			if s, ok := o.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

func TestBuildCloudInitOptions_EmptyTargetReturnsNoOptions(t *testing.T) {
	t.Parallel()
	if got := buildCloudInitOptions(&builder.Target{}); len(got) != 0 {
		t.Fatalf("expected empty options for empty target, got %+v", got)
	}
}

func TestBuildCloudInitOptions_SSHKeyIsURLEncoded(t *testing.T) {
	t.Parallel()
	opts := buildCloudInitOptions(&builder.Target{
		CloudInitSSHKey: "ssh-ed25519 AAAAC3Nz key@host",
	})
	got := findOption(opts, "sshkeys")
	// PVE's validator requires the Python urllib.parse.quote(s, safe='')
	// style: spaces as %20, not '+'. Assert the actual encoding rather
	// than re-implementing it.
	want := "ssh-ed25519%20AAAAC3Nz%20key%40host"
	if got != want {
		t.Fatalf("sshkeys = %q, want %q", got, want)
	}
}

func TestBuildCloudInitOptions_SSHKeyEncodingHasNoPlus(t *testing.T) {
	t.Parallel()
	// Regression guard: net/url.QueryEscape emits '+' for spaces, which
	// PVE's sshkeys validator rejects. EncodeSSHKeys must never emit '+'.
	opts := buildCloudInitOptions(&builder.Target{
		CloudInitSSHKey: "ssh-rsa AAAA name with spaces",
	})
	got := findOption(opts, "sshkeys")
	if strings.ContainsRune(got, '+') {
		t.Fatalf("sshkeys must not contain '+'; got %q", got)
	}
}

func TestBuildCloudInitOptions_MultipleKeysSeparatorEscaped(t *testing.T) {
	t.Parallel()
	opts := buildCloudInitOptions(&builder.Target{
		CloudInitSSHKey: "ssh-ed25519 AAA one\nssh-ed25519 BBB two",
	})
	got := findOption(opts, "sshkeys")
	// Newlines between keys must be present (as %0A) so PVE installs
	// both. Embedded spaces in comments must be %20.
	if !strings.Contains(got, "%0A") {
		t.Fatalf("expected encoded newline between keys, got %q", got)
	}
	if strings.Contains(got, "+") {
		t.Fatalf("sshkeys must not contain '+'; got %q", got)
	}
}

func TestBuildCloudInitOptions_PassesThroughOtherFieldsRaw(t *testing.T) {
	t.Parallel()
	opts := buildCloudInitOptions(&builder.Target{
		CloudInitUser:       "ansible",
		CloudInitPassword:   "p@ssword!",
		CloudInitIPConfig:   "ip=dhcp",
		CloudInitNameserver: "1.1.1.1",
	})
	cases := map[string]string{
		"ciuser":     "ansible",
		"cipassword": "p@ssword!",
		"ipconfig0":  "ip=dhcp",
		"nameserver": "1.1.1.1",
	}
	for k, want := range cases {
		if got := findOption(opts, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestBuildCloudInitOptions_SkipsEmptyFields(t *testing.T) {
	t.Parallel()
	opts := buildCloudInitOptions(&builder.Target{CloudInitUser: "only-this"})
	if len(opts) != 1 {
		t.Fatalf("expected exactly 1 option, got %d (%+v)", len(opts), opts)
	}
	if findOption(opts, "ciuser") != "only-this" {
		t.Fatalf("unexpected ciuser value: %+v", opts)
	}
}

func TestDefaultSSHRunnerFactory_RequiresAuth(t *testing.T) {
	t.Parallel()
	// No private key and no password — the factory must fail before any
	// network I/O so misconfigured templates surface immediately.
	_, err := defaultSSHRunnerFactory(context.Background(), "10.0.0.1", &builder.Target{SSHUsername: "ansible"})
	if err == nil {
		t.Fatal("expected error when no auth is configured")
	}
	if !strings.Contains(err.Error(), "private key or password") {
		t.Fatalf("expected auth-required error, got %v", err)
	}
}

func TestDefaultSSHRunnerFactory_FallsBackToCloudInitUser(t *testing.T) {
	t.Parallel()
	// When SSHUsername is unset, the runner should fall back to
	// CloudInitUser. The factory will fail at auth (no key/password) but
	// the user check fires first, so we assert the auth error rather than
	// a missing-user error.
	_, err := defaultSSHRunnerFactory(context.Background(), "10.0.0.1", &builder.Target{CloudInitUser: "kali"})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if strings.Contains(err.Error(), "user is required") {
		t.Fatalf("expected user fallback to CloudInitUser, got %v", err)
	}
}

func TestResolveVMIPViaAgent_ReturnsFirstNonLoopbackV4(t *testing.T) {
	t.Parallel()
	ifaces := []*pveapi.AgentNetworkIface{
		{
			Name: "lo",
			IPAddresses: []*pveapi.AgentNetworkIPAddress{
				{IPAddressType: "ipv4", IPAddress: "127.0.0.1"},
			},
		},
		nil, // skipped
		{
			Name: "eth0",
			IPAddresses: []*pveapi.AgentNetworkIPAddress{
				nil, // skipped
				{IPAddressType: "ipv6", IPAddress: "fe80::1"},
				{IPAddressType: "ipv4", IPAddress: ""},          // skipped empty
				{IPAddressType: "ipv4", IPAddress: "127.0.0.1"}, // skipped loopback
				{IPAddressType: "ipv4", IPAddress: "10.0.0.5"},
			},
		},
	}
	got, err := pickAgentIP(ifaces)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "10.0.0.5" {
		t.Fatalf("expected 10.0.0.5, got %q", got)
	}
}

func TestResolveVMIPViaAgent_NoV4Found(t *testing.T) {
	t.Parallel()
	ifaces := []*pveapi.AgentNetworkIface{
		{
			Name: "eth0",
			IPAddresses: []*pveapi.AgentNetworkIPAddress{
				{IPAddressType: "ipv6", IPAddress: "fe80::1"},
			},
		},
	}
	_, err := pickAgentIP(ifaces)
	if err == nil || !strings.Contains(err.Error(), "no non-loopback") {
		t.Fatalf("expected no-IP error, got %v", err)
	}
}

// pickAgentIP is the pure portion of resolveVMIPViaAgent extracted for tests;
// see resolveVMIPViaAgent for the live call.

func TestNewLiveRunner(t *testing.T) {
	t.Parallel()
	r := newLiveRunner(&ProxmoxClients{Node: "pve1"})
	if r == nil {
		t.Fatal("expected non-nil live runner")
	}
	if r.sshRunnerFactory == nil {
		t.Fatal("expected default sshRunnerFactory")
	}
	if r.ipResolver == nil {
		t.Fatal("expected default ipResolver")
	}
}

func TestLiveRunner_RunProvisioners_NoneSkipsRunner(t *testing.T) {
	t.Parallel()
	// When the config has no provisioners, the live runner should short
	// circuit without calling the SSH factory or IP resolver. We assert
	// this by wiring stubs that would fail if invoked.
	r := &liveRunner{
		clients: &ProxmoxClients{Node: "pve1"},
		sshRunnerFactory: func(context.Context, string, *builder.Target) (SSHRunner, error) {
			return nil, errors.New("factory should not be called")
		},
		ipResolver: func(context.Context, *pveapi.VirtualMachine) (string, error) {
			return "", errors.New("resolver should not be called")
		},
	}
	err := r.runProvisioners(context.Background(), &pveapi.VirtualMachine{}, builder.Config{}, &builder.Target{})
	if err != nil {
		t.Fatalf("expected no error when no provisioners, got %v", err)
	}
}

func TestLiveRunner_RunProvisioners_PropagatesIPError(t *testing.T) {
	t.Parallel()
	r := &liveRunner{
		clients: &ProxmoxClients{Node: "pve1"},
		ipResolver: func(context.Context, *pveapi.VirtualMachine) (string, error) {
			return "", errors.New("no ip yet")
		},
	}
	cfg := builder.Config{Provisioners: []builder.Provisioner{{Type: "shell", Inline: []string{"true"}}}}
	err := r.runProvisioners(context.Background(), &pveapi.VirtualMachine{}, cfg, &builder.Target{})
	if err == nil || !strings.Contains(err.Error(), "guest agent") {
		t.Fatalf("expected guest agent error, got %v", err)
	}
}

func TestLiveRunner_RunProvisioners_FactoryError(t *testing.T) {
	t.Parallel()
	r := &liveRunner{
		clients: &ProxmoxClients{Node: "pve1"},
		ipResolver: func(context.Context, *pveapi.VirtualMachine) (string, error) {
			return "10.0.0.1", nil
		},
		sshRunnerFactory: func(context.Context, string, *builder.Target) (SSHRunner, error) {
			return nil, errors.New("factory fail")
		},
	}
	cfg := builder.Config{Provisioners: []builder.Provisioner{{Type: "shell", Inline: []string{"true"}}}}
	err := r.runProvisioners(context.Background(), &pveapi.VirtualMachine{}, cfg, &builder.Target{})
	if err == nil || !strings.Contains(err.Error(), "SSH session") {
		t.Fatalf("expected SSH session error, got %v", err)
	}
}

func TestLiveRunner_AllocateVMID_UsesConfiguredVMID(t *testing.T) {
	t.Parallel()
	r := &liveRunner{clients: &ProxmoxClients{Node: "pve1"}}
	got, err := r.allocateVMID(context.Background(), &builder.Target{NewVMID: 9123})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 9123 {
		t.Fatalf("expected configured VMID 9123, got %d", got)
	}
}

func TestLiveRunner_RunProvisioners_RunsWithFakeRunner(t *testing.T) {
	t.Parallel()
	captured := &fakeRunner{}
	r := &liveRunner{
		clients: &ProxmoxClients{Node: "pve1"},
		ipResolver: func(context.Context, *pveapi.VirtualMachine) (string, error) {
			return "10.0.0.1", nil
		},
		sshRunnerFactory: func(context.Context, string, *builder.Target) (SSHRunner, error) {
			return captured, nil
		},
	}
	cfg := builder.Config{Provisioners: []builder.Provisioner{{Type: "shell", Inline: []string{"true"}}}}
	if err := r.runProvisioners(context.Background(), &pveapi.VirtualMachine{}, cfg, &builder.Target{}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !captured.closed {
		t.Fatal("expected SSH runner to be closed via defer")
	}
	if len(captured.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(captured.commands))
	}
}
