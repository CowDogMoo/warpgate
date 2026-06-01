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
	"strings"
	"testing"
)

func TestNewProxmoxClients_MissingEndpoint(t *testing.T) {
	t.Parallel()
	_, err := NewProxmoxClients(context.Background(), ClientConfig{Node: "pve1", APITokenID: "u", APIToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "Endpoint") {
		t.Fatalf("expected Endpoint required error, got %v", err)
	}
}

func TestNewProxmoxClients_MissingNode(t *testing.T) {
	t.Parallel()
	_, err := NewProxmoxClients(context.Background(), ClientConfig{Endpoint: "https://pve", APITokenID: "u", APIToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "Node") {
		t.Fatalf("expected Node required error, got %v", err)
	}
}

func TestNewProxmoxClients_MissingAuth(t *testing.T) {
	t.Parallel()
	_, err := NewProxmoxClients(context.Background(), ClientConfig{
		Endpoint: "https://pve",
		Node:     "pve1",
	})
	if err == nil || !strings.Contains(err.Error(), "APITokenID") {
		t.Fatalf("expected auth required error, got %v", err)
	}
}

func TestNewProxmoxClients_HappyPath_APIToken(t *testing.T) {
	t.Parallel()
	clients, err := NewProxmoxClients(context.Background(), ClientConfig{
		Endpoint:   "https://pve.example.com:8006/api2/json/",
		Node:       "pve1",
		APITokenID: "user@pve!warpgate",
		APIToken:   "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clients.Node != "pve1" {
		t.Fatalf("expected Node=pve1, got %q", clients.Node)
	}
	if strings.HasSuffix(clients.Endpoint, "/") {
		t.Fatalf("expected endpoint to be right-trimmed, got %q", clients.Endpoint)
	}
	if clients.API == nil {
		t.Fatalf("expected non-nil API client")
	}
	if clients.GetNode() != "pve1" {
		t.Fatalf("GetNode mismatch")
	}
}

func TestNewProxmoxClients_HappyPath_Password(t *testing.T) {
	t.Parallel()
	_, err := NewProxmoxClients(context.Background(), ClientConfig{
		Endpoint: "https://pve",
		Node:     "pve1",
		Username: "root@pam",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatEndpointError(t *testing.T) {
	t.Parallel()
	clients := &ProxmoxClients{Endpoint: "https://pve.example.com"}
	if got := clients.formatEndpointError("read node", nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
	wrapped := clients.formatEndpointError("read node", testErr("boom"))
	if !strings.Contains(wrapped.Error(), "https://pve.example.com") {
		t.Fatalf("expected endpoint in wrapped error, got %v", wrapped)
	}
}

// testErr is a tiny error helper that avoids depending on errors.New in
// tests that just need a sentinel.
type testErr string

func (e testErr) Error() string { return string(e) }
