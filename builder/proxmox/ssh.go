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
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

const (
	defaultSSHPort         = 22
	defaultSSHDialDeadline = 5 * time.Minute
	sshDialRetryInterval   = 5 * time.Second
	sshConnectTimeout      = 15 * time.Second
)

// runCmdFunc executes command on the remote host, optionally piping stdin
// in and stdout/stderr out. It is the single seam between the testable
// SSHRunner logic and the live golang.org/x/crypto/ssh plumbing. Tests
// inject a fake runCmdFunc; production wires sshRunCmd around an ssh.Client.
type runCmdFunc func(ctx context.Context, command string, stdin io.Reader, stdout, stderr io.Writer) error

// sshRunner is the production SSHRunner.
type sshRunner struct {
	addr   string
	user   string
	runCmd runCmdFunc
	closer io.Closer
}

// dialSSHFunc is the indirection used to inject a fake dialer in tests.
type dialSSHFunc func(ctx context.Context, network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error)

// newSSHRunner dials addr with a bounded retry window. Newly cloned VMs
// often need a handful of cloud-init cycles before sshd starts listening,
// so we retry until dialDeadline elapses or ctx is cancelled.
func newSSHRunner(ctx context.Context, conn SSHConnection, dialDeadline time.Duration, dial dialSSHFunc) (*sshRunner, error) {
	cfg, err := buildSSHClientConfig(conn)
	if err != nil {
		return nil, err
	}
	port := conn.Port
	if port == 0 {
		port = defaultSSHPort
	}
	addr := net.JoinHostPort(conn.Host, strconv.Itoa(port))
	if dial == nil {
		dial = dialSSH
	}
	if dialDeadline <= 0 {
		dialDeadline = defaultSSHDialDeadline
	}

	deadline := time.Now().Add(dialDeadline)
	var lastErr error
	for {
		client, err := dial(ctx, "tcp", addr, cfg)
		if err == nil {
			return &sshRunner{
				addr:   addr,
				user:   conn.User,
				runCmd: sshRunCmd(client),
				closer: client,
			}, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, ctx.Err())
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("dial %s after %s: %w", addr, dialDeadline, lastErr)
		}
		logging.DebugContext(ctx, "ssh dial %s not ready: %v; retrying in %s", addr, err, sshDialRetryInterval)
		select {
		case <-time.After(sshDialRetryInterval):
		case <-ctx.Done():
			return nil, fmt.Errorf("dial %s: %w", addr, ctx.Err())
		}
	}
}

// dialSSH wraps ssh.NewClient with a per-attempt TCP connect timeout so a
// hung SYN doesn't extend the overall retry window.
func dialSSH(ctx context.Context, network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	d := net.Dialer{Timeout: sshConnectTimeout}
	raw, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(raw, addr, cfg)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// buildSSHClientConfig builds an ssh.ClientConfig from conn. Either
// PrivateKey or Password (or both) must be set.
func buildSSHClientConfig(conn SSHConnection) (*ssh.ClientConfig, error) {
	if conn.User == "" {
		return nil, fmt.Errorf("ssh: user is required")
	}
	var auth []ssh.AuthMethod
	if conn.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(conn.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if conn.Password != "" {
		auth = append(auth, ssh.Password(conn.Password))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("ssh: either private key or password is required")
	}
	// Build VMs are ephemeral and freshly created by warpgate — there is
	// no prior known_hosts entry to verify against. Pinning host keys
	// here would require an out-of-band channel to fetch them from the
	// source template, which warpgate doesn't have.
	return &ssh.ClientConfig{
		User:            conn.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sshConnectTimeout,
	}, nil
}

// Run executes command on the remote host with env exported into the shell
// before evaluation. Combined stdout+stderr is returned so callers can log
// it on failure.
func (s *sshRunner) Run(ctx context.Context, command string, env map[string]string) (string, error) {
	full := withEnvExports(env) + command
	var out bytes.Buffer
	if err := s.runCmd(ctx, full, nil, &out, &out); err != nil {
		return out.String(), fmt.Errorf("run %q: %w", command, err)
	}
	return out.String(), nil
}

// UploadFile copies source to destination by piping the file body to a
// remote `cat > destination`. Mode is applied with chmod afterwards. Using
// cat-over-stdin avoids pulling in an SCP/SFTP dep.
func (s *sshRunner) UploadFile(ctx context.Context, source, destination, mode string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source %s: %w", source, err)
	}
	defer func() { _ = f.Close() }()

	var errBuf bytes.Buffer
	cmd := fmt.Sprintf("cat > %s", shellQuote(destination))
	if err := s.runCmd(ctx, cmd, f, io.Discard, &errBuf); err != nil {
		stderr := strings.TrimSpace(errBuf.String())
		if stderr != "" {
			return fmt.Errorf("upload %s -> %s: %w (%s)", source, destination, err, stderr)
		}
		return fmt.Errorf("upload %s -> %s: %w", source, destination, err)
	}

	if mode != "" {
		chmod := fmt.Sprintf("chmod %s %s", shellQuote(mode), shellQuote(destination))
		if _, err := s.Run(ctx, chmod, nil); err != nil {
			return fmt.Errorf("chmod %s %s: %w", mode, destination, err)
		}
	}
	return nil
}

// Close shuts down the underlying ssh client connection.
func (s *sshRunner) Close() error {
	if s.closer == nil {
		return nil
	}
	return s.closer.Close()
}

// sshSession is the subset of *ssh.Session that orchestrateSession needs.
// Pulling it behind an interface lets the orchestration be tested without a
// live SSH connection.
type sshSession interface {
	StdinPipe() (io.WriteCloser, error)
	Start(cmd string) error
	Wait() error
	Signal(sig ssh.Signal) error
	Close() error
}

// sshRunCmd returns a runCmdFunc that opens a fresh ssh.Session on client,
// wires stdout/stderr, and hands the rest off to orchestrateSession.
func sshRunCmd(client *ssh.Client) runCmdFunc {
	return func(ctx context.Context, command string, stdin io.Reader, stdout, stderr io.Writer) error {
		sess, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("open ssh session: %w", err)
		}
		sess.Stdout = stdout
		sess.Stderr = stderr
		return orchestrateSession(ctx, sess, command, stdin)
	}
}

// orchestrateSession starts command on sess, pipes stdin in when supplied,
// waits for completion, and sends SIGKILL on ctx cancellation. stdout and
// stderr must already be wired on sess before the call. The session is
// closed before returning.
func orchestrateSession(ctx context.Context, sess sshSession, command string, stdin io.Reader) error {
	defer func() { _ = sess.Close() }()

	var copyErr chan error
	if stdin != nil {
		pipe, err := sess.StdinPipe()
		if err != nil {
			return fmt.Errorf("open stdin pipe: %w", err)
		}
		copyErr = make(chan error, 1)
		go func() {
			_, err := io.Copy(pipe, stdin)
			_ = pipe.Close()
			copyErr <- err
		}()
	}

	if err := sess.Start(command); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()

	select {
	case err := <-done:
		if copyErr != nil {
			if cerr := <-copyErr; cerr != nil {
				return cerr
			}
		}
		return err
	case <-ctx.Done():
		_ = sess.Signal(ssh.SIGKILL)
		return ctx.Err()
	}
}

// withEnvExports renders a `export K='V'; ...` prefix the remote sh
// evaluates before command. Many sshd builds disable SendEnv/AcceptEnv, so
// passing envs as exports is the only portable channel.
func withEnvExports(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString("export ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(shellQuote(env[k]))
		b.WriteString("; ")
	}
	return b.String()
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes
// via the standard '\” trick. Safe for arbitrary bytes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// resolveSSHConnection builds an SSHConnection from target. SSHUsername
// falls back to CloudInitUser when unset, so callers that wire cloud-init
// once get matching SSH access without repeating themselves.
func resolveSSHConnection(host string, target *builder.Target) SSHConnection {
	conn := SSHConnection{
		Host:       host,
		Port:       target.SSHPort,
		User:       target.SSHUsername,
		Password:   target.SSHPassword,
		PrivateKey: target.SSHPrivateKey,
	}
	if conn.User == "" {
		conn.User = target.CloudInitUser
	}
	return conn
}
