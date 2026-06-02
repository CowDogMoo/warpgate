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
