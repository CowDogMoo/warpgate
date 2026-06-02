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
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
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

func TestSSHRunner_CloseNilCloser(t *testing.T) {
	t.Parallel()
	r := &sshRunner{}
	if err := r.Close(); err != nil {
		t.Fatalf("Close with nil closer should be no-op, got %v", err)
	}
}

// fakeCloser captures Close calls so we can assert sshRunner.Close
// delegates to the underlying ssh client.
type fakeCloser struct {
	closed bool
	err    error
}

func (f *fakeCloser) Close() error {
	f.closed = true
	return f.err
}

func TestSSHRunner_CloseDelegates(t *testing.T) {
	t.Parallel()
	fc := &fakeCloser{}
	r := &sshRunner{closer: fc}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !fc.closed {
		t.Fatal("expected closer to be invoked")
	}
}

// fakeCmd captures one invocation of a runCmdFunc so tests can assert on
// the command line, stdin payload, and emitted stdout/stderr.
type fakeCmd struct {
	command  string
	stdinBuf []byte
	stdout   string
	stderr   string
	err      error
}

func (f *fakeCmd) run(_ context.Context, command string, stdin io.Reader, stdout, stderr io.Writer) error {
	f.command = command
	if stdin != nil {
		var b bytes.Buffer
		if _, err := io.Copy(&b, stdin); err != nil {
			return err
		}
		f.stdinBuf = b.Bytes()
	}
	if f.stdout != "" {
		_, _ = stdout.Write([]byte(f.stdout))
	}
	if f.stderr != "" {
		_, _ = stderr.Write([]byte(f.stderr))
	}
	return f.err
}

// chainCmd wires up a sequence of fakeCmd responses so tests can model
// multi-step calls like UploadFile (cat then chmod).
type chainCmd struct {
	calls []*fakeCmd
	idx   int
}

func (c *chainCmd) run(ctx context.Context, command string, stdin io.Reader, stdout, stderr io.Writer) error {
	if c.idx >= len(c.calls) {
		return errors.New("chainCmd: no more scripted responses")
	}
	fc := c.calls[c.idx]
	c.idx++
	return fc.run(ctx, command, stdin, stdout, stderr)
}

func TestSSHRunner_Run_HappyPathReturnsStdout(t *testing.T) {
	t.Parallel()
	fc := &fakeCmd{stdout: "hello\n"}
	r := &sshRunner{runCmd: fc.run}
	out, err := r.Run(context.Background(), "echo hello", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "hello\n" {
		t.Fatalf("out = %q, want %q", out, "hello\n")
	}
	if fc.command != "echo hello" {
		t.Fatalf("command = %q, want plain command (no env)", fc.command)
	}
}

func TestSSHRunner_Run_EnvPrefixedToCommand(t *testing.T) {
	t.Parallel()
	fc := &fakeCmd{stdout: "ok"}
	r := &sshRunner{runCmd: fc.run}
	_, err := r.Run(context.Background(), "cmd", map[string]string{"BAR": "two", "AAA": "one"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "export AAA='one'; export BAR='two'; cmd"
	if fc.command != want {
		t.Fatalf("command = %q, want %q", fc.command, want)
	}
}

func TestSSHRunner_Run_WrapsErrorWithCommand(t *testing.T) {
	t.Parallel()
	fc := &fakeCmd{stdout: "partial", err: errors.New("exit 1")}
	r := &sshRunner{runCmd: fc.run}
	out, err := r.Run(context.Background(), "fail.sh", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `run "fail.sh"`) {
		t.Fatalf("expected wrapped command in error, got %v", err)
	}
	if out != "partial" {
		t.Fatalf("partial output should still be returned, got %q", out)
	}
}

func TestSSHRunner_UploadFile_StreamsAndChmod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	body := []byte("payload\n")
	if err := os.WriteFile(src, body, 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	chain := &chainCmd{calls: []*fakeCmd{{}, {}}}
	r := &sshRunner{runCmd: chain.run}

	if err := r.UploadFile(context.Background(), src, "/opt/dst", "0640"); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if chain.idx != 2 {
		t.Fatalf("expected 2 commands (cat + chmod), got %d", chain.idx)
	}
	if got := chain.calls[0].command; got != "cat > '/opt/dst'" {
		t.Fatalf("first command = %q, want cat redirect", got)
	}
	if string(chain.calls[0].stdinBuf) != string(body) {
		t.Fatalf("stdin = %q, want %q", chain.calls[0].stdinBuf, body)
	}
	if got := chain.calls[1].command; got != "chmod '0640' '/opt/dst'" {
		t.Fatalf("second command = %q, want chmod", got)
	}
}

func TestSSHRunner_UploadFile_NoModeSkipsChmod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	chain := &chainCmd{calls: []*fakeCmd{{}}}
	r := &sshRunner{runCmd: chain.run}

	if err := r.UploadFile(context.Background(), src, "/opt/dst", ""); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if chain.idx != 1 {
		t.Fatalf("expected exactly 1 command (no chmod), got %d", chain.idx)
	}
}

func TestSSHRunner_UploadFile_MissingSourceWraps(t *testing.T) {
	t.Parallel()
	r := &sshRunner{runCmd: (&fakeCmd{}).run}
	err := r.UploadFile(context.Background(), filepath.Join(t.TempDir(), "missing"), "/tmp/dst", "")
	if err == nil {
		t.Fatal("expected error opening missing source")
	}
	if !strings.Contains(err.Error(), "open source") {
		t.Fatalf("expected open source error, got %v", err)
	}
}

func TestSSHRunner_UploadFile_CatFailureIncludesStderr(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	chain := &chainCmd{calls: []*fakeCmd{{
		err:    errors.New("exit 1"),
		stderr: "permission denied\n",
	}}}
	r := &sshRunner{runCmd: chain.run}
	err := r.UploadFile(context.Background(), src, "/opt/dst", "0640")
	if err == nil {
		t.Fatal("expected upload failure")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected stderr in error, got %v", err)
	}
}

func TestSSHRunner_UploadFile_CatFailureBareWithoutStderr(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	chain := &chainCmd{calls: []*fakeCmd{{err: errors.New("eof")}}}
	r := &sshRunner{runCmd: chain.run}
	err := r.UploadFile(context.Background(), src, "/opt/dst", "0640")
	if err == nil {
		t.Fatal("expected upload failure")
	}
	if strings.Contains(err.Error(), "()") {
		t.Fatalf("bare failure should not include empty parens, got %v", err)
	}
}

func TestSSHRunner_UploadFile_ChmodFailureSurfaces(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	chain := &chainCmd{calls: []*fakeCmd{
		{},
		{err: errors.New("operation not permitted")},
	}}
	r := &sshRunner{runCmd: chain.run}
	err := r.UploadFile(context.Background(), src, "/opt/dst", "0640")
	if err == nil {
		t.Fatal("expected chmod error to surface")
	}
	if !strings.Contains(err.Error(), "chmod 0640 /opt/dst") {
		t.Fatalf("expected chmod context in error, got %v", err)
	}
}

func TestDialSSH_ConnectionRefusedFails(t *testing.T) {
	t.Parallel()
	cfg := &ssh.ClientConfig{
		User:            "x",
		Auth:            []ssh.AuthMethod{ssh.Password("y")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         200 * time.Millisecond,
	}
	// 127.0.0.1:1 is privileged and reliably refuses on Linux/macOS test runners.
	_, err := dialSSH(context.Background(), "tcp", "127.0.0.1:1", cfg)
	if err == nil {
		t.Fatal("expected dial failure to an unreachable port")
	}
}

// fakeSession is a stand-in for *ssh.Session that lets us drive
// orchestrateSession's branches without a live SSH connection.
type fakeSession struct {
	stdinPipeErr error
	startErr     error
	waitErr      error
	waitDelay    time.Duration
	startCalled  string
	signaled     ssh.Signal
	closed       bool
	pipeWritten  []byte
	pipeCloseErr error
}

type fakePipe struct {
	parent *fakeSession
	buf    bytes.Buffer
}

func (p *fakePipe) Write(b []byte) (int, error) { return p.buf.Write(b) }
func (p *fakePipe) Close() error {
	p.parent.pipeWritten = p.buf.Bytes()
	return p.parent.pipeCloseErr
}

func (s *fakeSession) StdinPipe() (io.WriteCloser, error) {
	if s.stdinPipeErr != nil {
		return nil, s.stdinPipeErr
	}
	return &fakePipe{parent: s}, nil
}

func (s *fakeSession) Start(cmd string) error {
	s.startCalled = cmd
	return s.startErr
}

func (s *fakeSession) Wait() error {
	if s.waitDelay > 0 {
		time.Sleep(s.waitDelay)
	}
	return s.waitErr
}

func (s *fakeSession) Signal(sig ssh.Signal) error { s.signaled = sig; return nil }
func (s *fakeSession) Close() error                { s.closed = true; return nil }

func TestOrchestrateSession_StartErrorWraps(t *testing.T) {
	t.Parallel()
	sess := &fakeSession{startErr: errors.New("boom")}
	err := orchestrateSession(context.Background(), sess, "cmd", nil)
	if err == nil || !strings.Contains(err.Error(), "start:") {
		t.Fatalf("expected wrapped start error, got %v", err)
	}
	if !sess.closed {
		t.Fatal("expected session to be closed on error path")
	}
}

func TestOrchestrateSession_HappyPathReturnsWaitNil(t *testing.T) {
	t.Parallel()
	sess := &fakeSession{}
	if err := orchestrateSession(context.Background(), sess, "cmd", nil); err != nil {
		t.Fatalf("orchestrateSession: %v", err)
	}
	if sess.startCalled != "cmd" {
		t.Fatalf("Start called with %q, want %q", sess.startCalled, "cmd")
	}
	if !sess.closed {
		t.Fatal("expected session to be closed")
	}
}

func TestOrchestrateSession_WaitErrorPropagates(t *testing.T) {
	t.Parallel()
	sess := &fakeSession{waitErr: errors.New("exit 1")}
	err := orchestrateSession(context.Background(), sess, "cmd", nil)
	if err == nil || err.Error() != "exit 1" {
		t.Fatalf("expected raw wait error, got %v", err)
	}
}

func TestOrchestrateSession_StdinPipedToSession(t *testing.T) {
	t.Parallel()
	sess := &fakeSession{}
	body := strings.NewReader("payload-bytes")
	if err := orchestrateSession(context.Background(), sess, "cat > /dst", body); err != nil {
		t.Fatalf("orchestrateSession: %v", err)
	}
	if string(sess.pipeWritten) != "payload-bytes" {
		t.Fatalf("pipe got %q, want %q", sess.pipeWritten, "payload-bytes")
	}
}

func TestOrchestrateSession_StdinPipeErrorWraps(t *testing.T) {
	t.Parallel()
	sess := &fakeSession{stdinPipeErr: errors.New("ENOMEM")}
	err := orchestrateSession(context.Background(), sess, "cat > /dst", strings.NewReader("x"))
	if err == nil || !strings.Contains(err.Error(), "open stdin pipe") {
		t.Fatalf("expected stdin pipe error, got %v", err)
	}
}

// erroringReader fails on every Read so io.Copy in the stdin goroutine
// returns a non-nil error.
type erroringReader struct{ err error }

func (e erroringReader) Read([]byte) (int, error) { return 0, e.err }

func TestOrchestrateSession_CopyErrorBeatsWaitNil(t *testing.T) {
	t.Parallel()
	sess := &fakeSession{}
	err := orchestrateSession(context.Background(), sess, "cat > /dst", erroringReader{err: errors.New("read fail")})
	if err == nil || !strings.Contains(err.Error(), "read fail") {
		t.Fatalf("expected copy error to surface, got %v", err)
	}
}

func TestOrchestrateSession_ContextCancelSendsSIGKILL(t *testing.T) {
	t.Parallel()
	sess := &fakeSession{waitDelay: 200 * time.Millisecond, waitErr: errors.New("wait")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := orchestrateSession(ctx, sess, "cmd", nil)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled, got %v", err)
	}
	if sess.signaled != ssh.SIGKILL {
		t.Fatalf("expected SIGKILL on cancel, got %q", sess.signaled)
	}
}

// stubDial just hands back a *ssh.Client we ignore — used to drive
// newSSHRunner happy and ctx-during-retry paths without real I/O.
func stubDialSuccess(_ context.Context, _, _ string, _ *ssh.ClientConfig) (*ssh.Client, error) {
	return &ssh.Client{}, nil
}

func TestNewSSHRunner_SuccessFirstAttemptReturnsRunner(t *testing.T) {
	t.Parallel()
	r, err := newSSHRunner(context.Background(), SSHConnection{
		Host: "h", User: "u", Password: "pw",
	}, time.Second, stubDialSuccess)
	if err != nil {
		t.Fatalf("newSSHRunner: %v", err)
	}
	if r == nil || r.runCmd == nil || r.closer == nil {
		t.Fatal("expected fully populated runner")
	}
}

func TestNewSSHRunner_CtxCancelledMidRetryReturns(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	dial := func(context.Context, string, string, *ssh.ClientConfig) (*ssh.Client, error) {
		calls++
		// First call fails so we enter the retry select; cancel ctx so
		// the select's ctx.Done branch fires.
		go cancel()
		return nil, errors.New("not ready")
	}
	_, err := newSSHRunner(ctx, SSHConnection{Host: "h", User: "u", Password: "pw"}, time.Hour, dial)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled, got %v", err)
	}
	if calls == 0 {
		t.Fatal("expected at least one dial attempt")
	}
}

func TestDialSSH_NonSSHListenerFailsHandshake(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	// Accept and immediately close so the SSH handshake fails fast.
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	cfg := &ssh.ClientConfig{
		User:            "x",
		Auth:            []ssh.AuthMethod{ssh.Password("y")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         500 * time.Millisecond,
	}
	_, err = dialSSH(context.Background(), "tcp", ln.Addr().String(), cfg)
	if err == nil {
		t.Fatal("expected ssh handshake failure against non-ssh listener")
	}
}
