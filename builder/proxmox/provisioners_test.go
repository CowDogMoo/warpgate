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

	"github.com/cowdogmoo/warpgate/v3/builder"
)

// fakeRunner is a test double for SSHRunner that records every call.
type fakeRunner struct {
	commands []string
	uploads  []upload
	failRun  bool
	failUp   bool
	closed   bool
}

type upload struct {
	src, dst, mode string
	sudo           bool
}

func (f *fakeRunner) Run(_ context.Context, cmd string, _ map[string]string) (string, error) {
	f.commands = append(f.commands, cmd)
	if f.failRun {
		return "", errors.New("run failed")
	}
	return "ok\n", nil
}

func (f *fakeRunner) UploadFile(_ context.Context, src, dst string, opts UploadOptions) error {
	f.uploads = append(f.uploads, upload{src, dst, opts.Mode, opts.Sudo})
	if f.failUp {
		return errors.New("upload failed")
	}
	return nil
}

func (f *fakeRunner) Close() error {
	f.closed = true
	return nil
}

func TestFilepathBase(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"/tmp/foo.sh":              "foo.sh",
		"C:\\Windows\\Temp\\a.ps1": "a.ps1",
		"plain.txt":                "plain.txt",
		"":                         "",
		"a/b/c/d":                  "d",
	}
	for in, want := range cases {
		if got := filepathBase(in); got != want {
			t.Errorf("filepathBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunProvisioners_ShellHappyPath(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:   "shell",
		Inline: []string{"echo one", "echo two"},
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(r.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(r.commands))
	}
	if !strings.Contains(r.commands[0], "set -e") {
		t.Fatalf("expected set -e prefix, got %q", r.commands[0])
	}
	if !strings.Contains(r.commands[0], "echo one") || !strings.Contains(r.commands[0], "echo two") {
		t.Fatalf("expected commands joined, got %q", r.commands[0])
	}
}

func TestRunProvisioners_ShellWithWorkingDirAndUser(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:       "shell",
		Inline:     []string{"true"},
		WorkingDir: "/opt/work",
		User:       "deploy",
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := r.commands[0]
	// After sudo-wrap the WorkingDir double-quotes are escaped (\"). We
	// just verify the path makes it into the final command unchanged.
	if !strings.Contains(got, "/opt/work") {
		t.Fatalf("expected working dir in command, got %q", got)
	}
	if !strings.Contains(got, "sudo -E -u deploy") {
		t.Fatalf("expected sudo invocation, got %q", got)
	}
}

func TestRunProvisioners_ShellEmptyInlineSkipped(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:   "shell",
		Inline: nil,
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(r.commands) != 0 {
		t.Fatalf("expected no commands when Inline empty, got %d", len(r.commands))
	}
}

func TestRunProvisioners_File(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:        "file",
		Source:      "src.tar",
		Destination: "/opt/dst.tar",
		Mode:        "0644",
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(r.uploads) != 1 || r.uploads[0] != (upload{"src.tar", "/opt/dst.tar", "0644", false}) {
		t.Fatalf("unexpected uploads: %+v", r.uploads)
	}
}

func TestRunProvisioners_FilePropagatesSudo(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:        "file",
		Source:      "src.tar",
		Destination: "/root/dst.tar",
		Mode:        "0600",
		Sudo:        true,
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(r.uploads) != 1 || !r.uploads[0].sudo {
		t.Fatalf("expected sudo upload, got %+v", r.uploads)
	}
}

func TestRunProvisioners_ShellWithSudo(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:   "shell",
		Inline: []string{"echo hi"},
		Sudo:   true,
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(r.commands[0], "sudo -E sh -c") {
		t.Fatalf("expected sudo wrap, got %q", r.commands[0])
	}
}

func TestRunProvisioners_ShellUserBeatsSudo(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:   "shell",
		Inline: []string{"true"},
		User:   "kali",
		Sudo:   true,
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(r.commands[0], "sudo -E -u kali") {
		t.Fatalf("expected user wrap to take precedence, got %q", r.commands[0])
	}
	if strings.Contains(r.commands[0], "sudo -E sh -c") {
		t.Fatalf("plain sudo wrap should not stack on top of user wrap, got %q", r.commands[0])
	}
}

func TestRunProvisioners_FileRequiresSrcDst(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{Type: "file"}})
	if err == nil || !strings.Contains(err.Error(), "source and destination") {
		t.Fatalf("expected src/dst error, got %v", err)
	}
}

func TestRunProvisioners_Script(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:    "script",
		Scripts: []string{"/local/install.sh"},
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(r.uploads) != 1 || !strings.HasSuffix(r.uploads[0].dst, "install.sh") {
		t.Fatalf("expected upload of install.sh, got %+v", r.uploads)
	}
	if len(r.commands) != 1 || !strings.HasSuffix(r.commands[0], "install.sh") {
		t.Fatalf("expected execution of uploaded script, got %+v", r.commands)
	}
}

func TestRunProvisioners_ScriptWithSudo(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:    "script",
		Scripts: []string{"install.sh"},
		Sudo:    true,
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.HasPrefix(r.commands[0], "sudo -E ") {
		t.Fatalf("expected sudo prefix on script execution, got %q", r.commands[0])
	}
}

func TestRunProvisioners_ScriptUploadFailure(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{failUp: true}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:    "script",
		Scripts: []string{"/local/install.sh"},
	}})
	if err == nil {
		t.Fatal("expected upload failure to propagate")
	}
}

func TestRunProvisioners_PowerShell(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:      "powershell",
		PSScripts: []string{"C:\\local\\setup.ps1"},
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(r.uploads) != 1 || !strings.HasSuffix(r.uploads[0].dst, "setup.ps1") {
		t.Fatalf("expected ps1 upload, got %+v", r.uploads)
	}
	if !strings.Contains(r.commands[0], "powershell.exe -ExecutionPolicy Bypass") {
		t.Fatalf("expected default bypass policy, got %q", r.commands[0])
	}
}

func TestRunProvisioners_PowerShellCustomPolicy(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:            "powershell",
		PSScripts:       []string{"setup.ps1"},
		ExecutionPolicy: "RemoteSigned",
	}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(r.commands[0], "RemoteSigned") {
		t.Fatalf("expected RemoteSigned policy, got %q", r.commands[0])
	}
}

func TestRunProvisioners_AnsibleSkippedWithWarn(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{Type: "ansible"}})
	if err != nil {
		t.Fatalf("ansible should be skipped without error, got %v", err)
	}
	if len(r.commands) != 0 {
		t.Fatalf("expected no commands for ansible skip")
	}
}

func TestRunProvisioners_Unsupported(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{Type: "chef"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

func TestRunProvisioners_RunFailurePropagates(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{failRun: true}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{{
		Type:   "shell",
		Inline: []string{"true"},
	}})
	if err == nil {
		t.Fatal("expected run failure to propagate")
	}
}

func TestRunProvisioners_OrderedFailIndex(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	err := runProvisioners(context.Background(), r, []builder.Provisioner{
		{Type: "shell", Inline: []string{"echo a"}},
		{Type: "file"}, // missing src/dst
	})
	if err == nil || !strings.Contains(err.Error(), "provisioner[1]") {
		t.Fatalf("expected provisioner[1] failure, got %v", err)
	}
}
