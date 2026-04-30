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

package azure

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

// mustBeType asserts that v has dynamic type T and returns it. Routing the
// assertion through a generic helper avoids a gocritic sloppyTypeAssert false
// positive that fires when the source's static type lives in a different file
// of the same package.
func mustBeType[T any](t *testing.T, v any) T {
	t.Helper()
	out, ok := v.(T)
	require.Truef(t, ok, "expected type %T, got %T", *new(T), v)
	return out
}

func inlineCommands(t *testing.T, c armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification) []string {
	t.Helper()
	switch v := c.(type) {
	case *armvirtualmachineimagebuilder.ImageTemplateShellCustomizer:
		out := make([]string, len(v.Inline))
		for i, p := range v.Inline {
			out[i] = *p
		}
		return out
	case *armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer:
		out := make([]string, len(v.Inline))
		for i, p := range v.Inline {
			out[i] = *p
		}
		return out
	default:
		t.Fatalf("unexpected customizer type %T", c)
		return nil
	}
}

func TestAnsibleCustomizer_LinuxEmitsShellWithPlaybook(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "- hosts: localhost\n  tasks: []\n")

	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
	}
	c, err := ansibleCustomizer(p, 2, "Linux")
	require.NoError(t, err)

	shell := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, c)
	assert.Equal(t, "Shell", *shell.Type)
	assert.Equal(t, "ansible-2", *shell.Name)

	cmds := inlineCommands(t, c)
	joined := strings.Join(cmds, "\n")
	assert.Contains(t, joined, "ansible-playbook /tmp/warpgate-ansible/site.yml")
	assert.Contains(t, joined, "--connection=local")
	expectedB64 := base64.StdEncoding.EncodeToString([]byte("- hosts: localhost\n  tasks: []\n"))
	assert.Contains(t, joined, expectedB64)
}

func TestAnsibleCustomizer_LinuxIncludesGalaxyWhenSet(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")
	galaxy := writeTempFile(t, dir, "requirements.yml", "roles: []")

	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
		GalaxyFile:   galaxy,
	}
	c, err := ansibleCustomizer(p, 0, "Linux")
	require.NoError(t, err)
	joined := strings.Join(inlineCommands(t, c), "\n")
	assert.Contains(t, joined, "ansible-galaxy install -r /tmp/warpgate-ansible/requirements.yml")
	expectedB64 := base64.StdEncoding.EncodeToString([]byte("roles: []"))
	assert.Contains(t, joined, expectedB64)
}

func TestAnsibleCustomizer_LinuxAppendsSortedExtraVars(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")

	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
		ExtraVars: map[string]string{
			"ansible_python_interpreter": "/usr/bin/python3",
			"app_version":                "1.2.3",
			"ansible_connection":         "ssh",
		},
	}
	c, err := ansibleCustomizer(p, 0, "Linux")
	require.NoError(t, err)
	joined := strings.Join(inlineCommands(t, c), "\n")

	assert.Contains(t, joined, "-e 'ansible_python_interpreter=/usr/bin/python3'")
	assert.Contains(t, joined, "-e 'app_version=1.2.3'")
	assert.NotContains(t, joined, "ansible_connection=ssh", "linux ignored extra var should not be passed")

	idxPython := strings.Index(joined, "ansible_python_interpreter=")
	idxApp := strings.Index(joined, "app_version=")
	assert.Less(t, idxPython, idxApp, "extra vars should be sorted lexically")
}

func TestAnsibleCustomizer_LinuxUsesProvidedInventory(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")

	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
		Inventory:    "myhost,",
	}
	c, err := ansibleCustomizer(p, 0, "Linux")
	require.NoError(t, err)
	joined := strings.Join(inlineCommands(t, c), "\n")
	assert.Contains(t, joined, "-i 'myhost,'")
	assert.NotContains(t, joined, "--connection=local")
}

func TestAnsibleCustomizer_WindowsEmitsPowerShell(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")

	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
	}
	c, err := ansibleCustomizer(p, 1, "Windows")
	require.NoError(t, err)
	ps := mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer](t, c)
	assert.Equal(t, "PowerShell", *ps.Type)
	assert.Equal(t, "ansible-1", *ps.Name)
	assert.True(t, *ps.RunElevated)

	joined := strings.Join(inlineCommands(t, c), "\n")
	assert.Contains(t, joined, "python -m pip install ansible pywinrm")
	assert.Contains(t, joined, "ansible-playbook 'C:\\warpgate-ansible\\site.yml'")
	assert.Contains(t, joined, "--connection=local")
}

func TestAnsibleCustomizer_WindowsDetectedFromExtraVars(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")

	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
		ExtraVars: map[string]string{
			"ansible_shell_type": "powershell",
		},
	}
	c, err := ansibleCustomizer(p, 0, "Linux")
	require.NoError(t, err)
	mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer](t, c)
}

func TestAnsibleCustomizer_WindowsSkipsIgnoredExtraVars(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")

	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
		ExtraVars: map[string]string{
			"ansible_connection":          "aws_ssm",
			"ansible_shell_type":          "powershell",
			"ansible_aws_ssm_bucket_name": "bucket",
			"ansible_aws_ssm_region":      "us-west-2",
			"my_var":                      "kept",
		},
	}
	c, err := ansibleCustomizer(p, 0, "Windows")
	require.NoError(t, err)
	joined := strings.Join(inlineCommands(t, c), "\n")
	assert.Contains(t, joined, "-e 'my_var=kept'")
	assert.NotContains(t, joined, "ansible_connection=aws_ssm")
	assert.NotContains(t, joined, "ansible_shell_type=")
	assert.NotContains(t, joined, "ansible_aws_ssm_bucket_name=")
	assert.NotContains(t, joined, "ansible_aws_ssm_region=")
}

func TestAnsibleCustomizer_RequiresPlaybookPath(t *testing.T) {
	p := &builder.Provisioner{Type: "ansible"}
	_, err := ansibleCustomizer(p, 5, "Linux")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provisioner[5]")
	assert.Contains(t, err.Error(), "playbook_path")
}

func TestAnsibleCustomizer_PlaybookReadError(t *testing.T) {
	p := &builder.Provisioner{Type: "ansible", PlaybookPath: "/no/such/playbook.yml"}
	_, err := ansibleCustomizer(p, 0, "Linux")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read playbook")
}

func TestAnsibleCustomizer_GalaxyReadError(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")
	p := &builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbook,
		GalaxyFile:   "/no/such/requirements.yml",
	}
	_, err := ansibleCustomizer(p, 0, "Linux")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read galaxy file")
}

func TestBuildCustomizers_AnsibleLinux(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")

	provisioners := []builder.Provisioner{
		{Type: "ansible", PlaybookPath: playbook},
	}
	customs, err := buildCustomizers(provisioners, "Linux", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, customs, 1)
	mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, customs[0])
}

func TestBuildCustomizers_AnsibleWindows(t *testing.T) {
	dir := t.TempDir()
	playbook := writeTempFile(t, dir, "site.yml", "playbook")

	provisioners := []builder.Provisioner{
		{Type: "ansible", PlaybookPath: playbook},
	}
	customs, err := buildCustomizers(provisioners, "Windows", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, customs, 1)
	mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer](t, customs[0])
}

func TestShellEscapeSingleQuote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"it's", `it'\''s`},
		{"a 'b' c", `a '\''b'\'' c`},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, shellEscapeSingleQuote(tt.in), "input %q", tt.in)
	}
}

func TestPowerShellEscape(t *testing.T) {
	assert.Equal(t, "C:\\path\\file.yml", powerShellEscape("C:\\path\\file.yml"))
	assert.Equal(t, "it''s", powerShellEscape("it's"))
}

func TestIsWindowsAnsibleTarget(t *testing.T) {
	tests := []struct {
		name   string
		osType string
		extra  map[string]string
		want   bool
	}{
		{"linux default", "Linux", nil, false},
		{"empty osType linux default", "", nil, false},
		{"windows osType", "Windows", nil, true},
		{"windows lowercase", "windows", nil, true},
		{"shell_type powershell", "Linux", map[string]string{"ansible_shell_type": "powershell"}, true},
		{"shell_type cmd", "Linux", map[string]string{"ansible_shell_type": "cmd"}, true},
		{"shell_type bash", "Linux", map[string]string{"ansible_shell_type": "bash"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &builder.Provisioner{Type: "ansible", ExtraVars: tt.extra}
			assert.Equal(t, tt.want, isWindowsAnsibleTarget(p, tt.osType))
		})
	}
}
