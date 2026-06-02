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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

// generateTestPrivateKeyPEM returns a freshly-generated ed25519 private key
// in OpenSSH PEM form. Used by tests that need a valid key the ssh package
// will accept.
func generateTestPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "warpgate-test")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return string(pem.EncodeToMemory(block))
}

func TestBuildSSHClientConfig_RequiresUser(t *testing.T) {
	t.Parallel()
	_, err := buildSSHClientConfig(SSHConnection{Password: "pw"})
	if err == nil || !strings.Contains(err.Error(), "user is required") {
		t.Fatalf("expected user-required error, got %v", err)
	}
}

func TestBuildSSHClientConfig_RequiresAuth(t *testing.T) {
	t.Parallel()
	_, err := buildSSHClientConfig(SSHConnection{User: "root"})
	if err == nil || !strings.Contains(err.Error(), "private key or password") {
		t.Fatalf("expected auth-required error, got %v", err)
	}
}

func TestBuildSSHClientConfig_PasswordOnly(t *testing.T) {
	t.Parallel()
	cfg, err := buildSSHClientConfig(SSHConnection{User: "root", Password: "secret"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.User != "root" {
		t.Fatalf("user = %q, want %q", cfg.User, "root")
	}
	if len(cfg.Auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(cfg.Auth))
	}
}

func TestBuildSSHClientConfig_PrivateKeyInvalid(t *testing.T) {
	t.Parallel()
	_, err := buildSSHClientConfig(SSHConnection{User: "root", PrivateKey: "not a key"})
	if err == nil || !strings.Contains(err.Error(), "parse private key") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestBuildSSHClientConfig_PrivateKeyValid(t *testing.T) {
	t.Parallel()
	key := generateTestPrivateKeyPEM(t)
	cfg, err := buildSSHClientConfig(SSHConnection{User: "root", PrivateKey: key})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(cfg.Auth) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(cfg.Auth))
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                    "''",
		"plain":               "'plain'",
		"with space":          "'with space'",
		"with'quote":          `'with'\''quote'`,
		"multi 'quotes' here": `'multi '\''quotes'\'' here'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithEnvExports_EmptyReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := withEnvExports(nil); got != "" {
		t.Fatalf("expected empty for nil env, got %q", got)
	}
	if got := withEnvExports(map[string]string{}); got != "" {
		t.Fatalf("expected empty for empty env, got %q", got)
	}
}

func TestWithEnvExports_SortedAndQuoted(t *testing.T) {
	t.Parallel()
	got := withEnvExports(map[string]string{
		"BAR": "hello world",
		"AAA": "v",
		"FOO": "it's me",
	})
	want := `export AAA='v'; export BAR='hello world'; export FOO='it'\''s me'; `
	if got != want {
		t.Fatalf("withEnvExports mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestResolveSSHConnection_FallsBackToCloudInitUser(t *testing.T) {
	t.Parallel()
	conn := resolveSSHConnection("10.0.0.5", &builder.Target{
		CloudInitUser: "kali",
		SSHPort:       2222,
		SSHPrivateKey: "key",
		SSHPassword:   "pw",
	})
	if conn.User != "kali" {
		t.Fatalf("user = %q, want %q", conn.User, "kali")
	}
	if conn.Port != 2222 {
		t.Fatalf("port = %d, want 2222", conn.Port)
	}
	if conn.PrivateKey != "key" || conn.Password != "pw" {
		t.Fatalf("auth fields not propagated: %+v", conn)
	}
}

func TestResolveSSHConnection_PrefersExplicitUser(t *testing.T) {
	t.Parallel()
	conn := resolveSSHConnection("10.0.0.5", &builder.Target{
		CloudInitUser: "kali",
		SSHUsername:   "deploy",
		SSHPrivateKey: "k",
	})
	if conn.User != "deploy" {
		t.Fatalf("user = %q, want %q", conn.User, "deploy")
	}
}

func TestNewSSHRunner_DialErrorPropagates(t *testing.T) {
	t.Parallel()
	dial := func(context.Context, string, string, *ssh.ClientConfig) (*ssh.Client, error) {
		return nil, errors.New("connection refused")
	}
	_, err := newSSHRunner(
		context.Background(),
		SSHConnection{Host: "h", Port: 22, User: "root", Password: "pw"},
		50*time.Millisecond,
		dial,
	)
	if err == nil {
		t.Fatal("expected dial error after deadline")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected wrapped connection refused, got %v", err)
	}
}

func TestNewSSHRunner_RetriesUntilSuccess(t *testing.T) {
	t.Parallel()
	calls := 0
	dial := func(context.Context, string, string, *ssh.ClientConfig) (*ssh.Client, error) {
		calls++
		if calls < 2 {
			return nil, errors.New("not ready")
		}
		// Return a non-nil client we never use. nil works for the test
		// since we only check it's returned, not exercised.
		return &ssh.Client{}, nil
	}
	// Bump the retry interval down via a much shorter deadline window;
	// the test uses the default 5s retry which would be too slow, so we
	// give the deadline enough room for one tick.
	r, err := newSSHRunner(
		context.Background(),
		SSHConnection{Host: "h", Port: 22, User: "root", Password: "pw"},
		10*time.Second,
		dial,
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r == nil {
		t.Fatal("expected runner")
	}
	if calls < 2 {
		t.Fatalf("expected at least 2 dial attempts, got %d", calls)
	}
}

func TestNewSSHRunner_ContextCancellationStops(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dial := func(context.Context, string, string, *ssh.ClientConfig) (*ssh.Client, error) {
		return nil, errors.New("not ready")
	}
	_, err := newSSHRunner(
		ctx,
		SSHConnection{Host: "h", Port: 22, User: "root", Password: "pw"},
		time.Hour,
		dial,
	)
	if err == nil {
		t.Fatal("expected ctx cancel error")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Fatalf("expected context error, got %v", err)
	}
}

func TestSortStringsInPlace(t *testing.T) {
	t.Parallel()
	s := []string{"c", "a", "b", "a"}
	sortStrings(s)
	want := []string{"a", "a", "b", "c"}
	for i := range want {
		if s[i] != want[i] {
			t.Fatalf("sortStrings: got %v, want %v", s, want)
		}
	}
}

func TestSSHRunner_CloseNilClient(t *testing.T) {
	t.Parallel()
	r := &sshRunner{}
	if err := r.Close(); err != nil {
		t.Fatalf("Close on nil client should be no-op, got %v", err)
	}
}
