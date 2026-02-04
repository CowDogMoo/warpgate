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

package ami

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	ibtypes "github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewComponentGenerator(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()
	gen := NewComponentGenerator(clients)
	assert.NotNil(t, gen)
	assert.Equal(t, clients, gen.clients)
}

func TestGenerateComponent_Success(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	expectedARN := "arn:aws:imagebuilder:us-east-1:123456789012:component/test-shell/1.0.0/1"
	mocks.imageBuilder.CreateComponentFunc = func(ctx context.Context, params *imagebuilder.CreateComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateComponentOutput, error) {
		return &imagebuilder.CreateComponentOutput{
			ComponentBuildVersionArn: aws.String(expectedARN),
		}, nil
	}

	gen := NewComponentGenerator(clients)
	provisioner := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"echo hello"},
	}

	arn, err := gen.GenerateComponent(context.Background(), provisioner, "test", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, expectedARN, *arn)
}

func TestGenerateComponent_AlreadyExists(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	expectedARN := "arn:aws:imagebuilder:us-east-1:123456789012:component/test-shell/1.0.0/1"
	mocks.imageBuilder.CreateComponentFunc = func(ctx context.Context, params *imagebuilder.CreateComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateComponentOutput, error) {
		return nil, fmt.Errorf("component already exists")
	}
	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{
					Arn:     aws.String(expectedARN),
					Version: aws.String("1.0.0"),
				},
			},
		}, nil
	}

	gen := NewComponentGenerator(clients)
	provisioner := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"echo hello"},
	}

	arn, err := gen.GenerateComponent(context.Background(), provisioner, "test", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, expectedARN, *arn)
}

func TestGenerateComponent_AlreadyExistsButGetARNFails(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.CreateComponentFunc = func(ctx context.Context, params *imagebuilder.CreateComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateComponentOutput, error) {
		return nil, fmt.Errorf("component already exists")
	}
	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return nil, fmt.Errorf("list failed")
	}

	gen := NewComponentGenerator(clients)
	provisioner := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"echo hello"},
	}

	_, err := gen.GenerateComponent(context.Background(), provisioner, "test", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "component exists but failed to get ARN")
}

func TestGenerateComponent_CreateError(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.CreateComponentFunc = func(ctx context.Context, params *imagebuilder.CreateComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.CreateComponentOutput, error) {
		return nil, fmt.Errorf("access denied")
	}

	gen := NewComponentGenerator(clients)
	provisioner := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"echo hello"},
	}

	_, err := gen.GenerateComponent(context.Background(), provisioner, "test", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create component")
}

func TestGenerateComponent_InvalidDocument(t *testing.T) {
	t.Parallel()
	clients, _ := newMockAWSClients()

	gen := NewComponentGenerator(clients)
	provisioner := builder.Provisioner{
		Type: "unknown_type",
	}

	_, err := gen.GenerateComponent(context.Background(), provisioner, "test", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create component document")
}

func TestCreateScriptComponent_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "install.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:    "script",
		Scripts: []string{scriptPath},
	}

	doc, err := gen.createScriptComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "ScriptProvisioner")
	assert.Contains(t, doc, "ExecuteBash")
	assert.Contains(t, doc, "echo hello")
}

func TestCreateScriptComponent_MultipleScripts(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	scriptPath1 := filepath.Join(tmpDir, "script1.sh")
	scriptPath2 := filepath.Join(tmpDir, "script2.sh")
	require.NoError(t, os.WriteFile(scriptPath1, []byte("echo script1"), 0644))
	require.NoError(t, os.WriteFile(scriptPath2, []byte("echo script2"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:    "script",
		Scripts: []string{scriptPath1, scriptPath2},
	}

	doc, err := gen.createScriptComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "echo script1")
	assert.Contains(t, doc, "echo script2")
}

func TestCreateScriptComponent_NoScripts(t *testing.T) {
	t.Parallel()
	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:    "script",
		Scripts: []string{},
	}

	_, err := gen.createScriptComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "script provisioner has no scripts")
}

func TestCreateScriptComponent_NonexistentFile(t *testing.T) {
	t.Parallel()
	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:    "script",
		Scripts: []string{"/nonexistent/path/script.sh"},
	}

	_, err := gen.createScriptComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read script")
}

func TestCreateAnsibleComponent_NoPlaybook(t *testing.T) {
	t.Parallel()
	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type: "ansible",
	}

	_, err := gen.createAnsibleComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ansible provisioner has no playbook path")
}

func TestCreateLinuxAnsibleComponent_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all\n  tasks: []"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
	}

	doc, err := gen.createLinuxAnsibleComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "AnsibleProvisioner")
	assert.Contains(t, doc, "ExecuteBash")
	assert.Contains(t, doc, "ansible-playbook")
	assert.Contains(t, doc, "--connection=local")
}

func TestCreateLinuxAnsibleComponent_WithGalaxyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	galaxyPath := filepath.Join(tmpDir, "requirements.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))
	require.NoError(t, os.WriteFile(galaxyPath, []byte("---\nroles:\n  - geerlingguy.docker"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
		GalaxyFile:   galaxyPath,
	}

	doc, err := gen.createLinuxAnsibleComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "ansible-galaxy install")
	assert.Contains(t, doc, "base64 -d")
}

func TestCreateLinuxAnsibleComponent_WithExtraVarsAndInventory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
		ExtraVars:    map[string]string{"env": "prod", "role": "web"},
		Inventory:    "/etc/ansible/hosts",
	}

	doc, err := gen.createLinuxAnsibleComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "-e '")
	assert.Contains(t, doc, "-i /etc/ansible/hosts")
	// Should NOT contain the default connection=local since inventory is provided
	assert.NotContains(t, doc, "--connection=local")
}

func TestCreateLinuxAnsibleComponent_NonexistentPlaybook(t *testing.T) {
	t.Parallel()
	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "/nonexistent/playbook.yml",
	}

	_, err := gen.createLinuxAnsibleComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read playbook")
}

func TestCreateLinuxAnsibleComponent_NonexistentGalaxyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
		GalaxyFile:   "/nonexistent/requirements.yml",
	}

	_, err := gen.createLinuxAnsibleComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read galaxy file")
}

func TestCreateAnsibleComponent_WindowsPlatform(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
		ExtraVars: map[string]string{
			"ansible_shell_type": "powershell",
		},
	}

	doc, err := gen.createAnsibleComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "ExecutePowerShell")
	assert.Contains(t, doc, "Ansible provisioner component for Windows")
}

func TestCreateWindowsAnsibleComponent_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "windows-playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
	}

	doc, err := gen.createWindowsAnsibleComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "ExecutePowerShell")
	assert.Contains(t, doc, "ansible-playbook")
	assert.Contains(t, doc, "--connection=local")
	assert.Contains(t, doc, "chocolatey")
}

func TestCreateWindowsAnsibleComponent_WithGalaxyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	galaxyPath := filepath.Join(tmpDir, "requirements.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))
	require.NoError(t, os.WriteFile(galaxyPath, []byte("---\nroles:\n  - test_role"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
		GalaxyFile:   galaxyPath,
	}

	doc, err := gen.createWindowsAnsibleComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "ansible-galaxy install")
	assert.Contains(t, doc, "FromBase64String")
}

func TestCreateWindowsAnsibleComponent_WithExtraVars(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
		ExtraVars: map[string]string{
			"env":                         "prod",
			"ansible_connection":          "aws_ssm",
			"ansible_shell_type":          "powershell",
			"ansible_aws_ssm_bucket_name": "my-bucket",
			"ansible_aws_ssm_region":      "us-east-1",
		},
	}

	doc, err := gen.createWindowsAnsibleComponent(provisioner)
	require.NoError(t, err)
	// Connection-related vars should be filtered out
	assert.NotContains(t, doc, "ansible_connection")
	assert.NotContains(t, doc, "ansible_shell_type")
	assert.NotContains(t, doc, "ansible_aws_ssm_bucket_name")
	assert.NotContains(t, doc, "ansible_aws_ssm_region")
	// Normal extra vars should be present
	assert.Contains(t, doc, "env=prod")
}

func TestCreateWindowsAnsibleComponent_NonexistentPlaybook(t *testing.T) {
	t.Parallel()
	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "/nonexistent/playbook.yml",
	}

	_, err := gen.createWindowsAnsibleComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read playbook")
}

func TestCreateWindowsAnsibleComponent_NonexistentGalaxyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
		GalaxyFile:   "/nonexistent/requirements.yml",
	}

	_, err := gen.createWindowsAnsibleComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read galaxy file")
}

func TestShellComponent_WithEnvironment(t *testing.T) {
	t.Parallel()
	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"echo hello"},
		Environment: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
	}

	doc, err := gen.createShellComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "export")
}

func TestGetComponentARN_Success(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	expectedARN := "arn:aws:imagebuilder:us-east-1:123456789012:component/test/1.0.0/1"
	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{Arn: aws.String(expectedARN), Version: aws.String("1.0.0")},
				{Arn: aws.String("other-arn"), Version: aws.String("2.0.0")},
			},
		}, nil
	}

	gen := NewComponentGenerator(clients)
	arn, err := gen.getComponentARN(context.Background(), "test", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, expectedARN, *arn)
}

func TestGetComponentARN_VersionNotFound(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return &imagebuilder.ListComponentsOutput{
			ComponentVersionList: []ibtypes.ComponentVersion{
				{Arn: aws.String("some-arn"), Version: aws.String("2.0.0")},
			},
		}, nil
	}

	gen := NewComponentGenerator(clients)
	_, err := gen.getComponentARN(context.Background(), "test", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetComponentARN_ListError(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.ListComponentsFunc = func(ctx context.Context, params *imagebuilder.ListComponentsInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.ListComponentsOutput, error) {
		return nil, fmt.Errorf("access denied")
	}

	gen := NewComponentGenerator(clients)
	_, err := gen.getComponentARN(context.Background(), "test", "1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list components")
}

func TestComponentGeneratorDeleteComponent_Success(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		assert.Equal(t, "arn:test", *params.ComponentBuildVersionArn)
		return &imagebuilder.DeleteComponentOutput{}, nil
	}

	gen := NewComponentGenerator(clients)
	err := gen.DeleteComponent(context.Background(), "arn:test")
	assert.NoError(t, err)
}

func TestComponentGeneratorDeleteComponent_Error(t *testing.T) {
	t.Parallel()
	clients, mocks := newMockAWSClients()

	mocks.imageBuilder.DeleteComponentFunc = func(ctx context.Context, params *imagebuilder.DeleteComponentInput, optFns ...func(*imagebuilder.Options)) (*imagebuilder.DeleteComponentOutput, error) {
		return nil, fmt.Errorf("not found")
	}

	gen := NewComponentGenerator(clients)
	err := gen.DeleteComponent(context.Background(), "arn:test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete component")
}

func TestProcessPowerShellScript(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "removes BOM",
			input:    append([]byte{0xEF, 0xBB, 0xBF}, []byte("Write-Host 'Hello'")...),
			expected: "Write-Host 'Hello'",
		},
		{
			name:     "normalizes CRLF to LF",
			input:    []byte("line1\r\nline2\r\n"),
			expected: "line1\nline2\n",
		},
		{
			name:     "plain script unchanged",
			input:    []byte("Write-Host 'test'"),
			expected: "Write-Host 'test'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processPowerShellScript(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatePowerShellSyntax(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		script  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "balanced script",
			script:  "if ($true) { Write-Host 'hello' }",
			wantErr: false,
		},
		{
			name:    "unbalanced braces - unclosed",
			script:  "if ($true) { Write-Host 'hello'",
			wantErr: true,
			errMsg:  "unbalanced braces",
		},
		{
			name:    "unbalanced braces - extra closing",
			script:  "Write-Host 'hello' }",
			wantErr: true,
			errMsg:  "unbalanced braces",
		},
		{
			name:    "unbalanced parentheses - unclosed",
			script:  "Get-Process (",
			wantErr: true,
			errMsg:  "unbalanced parentheses",
		},
		{
			name:    "unbalanced parentheses - extra closing",
			script:  "Get-Process )",
			wantErr: true,
			errMsg:  "unbalanced parentheses",
		},
		{
			name:    "nested balanced",
			script:  "if (Test-Path ($env:TEMP)) { Write-Host (Get-Date) }",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePowerShellSyntax(tt.script)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckCharacterBalance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		script  string
		open    rune
		close   rune
		label   string
		wantErr bool
	}{
		{
			name:    "balanced braces",
			script:  "{ { } }",
			open:    '{',
			close:   '}',
			label:   "braces",
			wantErr: false,
		},
		{
			name:    "extra close",
			script:  "}",
			open:    '{',
			close:   '}',
			label:   "braces",
			wantErr: true,
		},
		{
			name:    "extra open",
			script:  "{",
			open:    '{',
			close:   '}',
			label:   "braces",
			wantErr: true,
		},
		{
			name:    "empty string",
			script:  "",
			open:    '{',
			close:   '}',
			label:   "braces",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkCharacterBalance(tt.script, tt.open, tt.close, tt.label)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNeedsReboot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{
			name:   "contains Restart-Computer",
			script: "Install-WindowsFeature\nRestart-Computer -Force",
			want:   true,
		},
		{
			name:   "contains shutdown /r",
			script: "shutdown /r /t 0",
			want:   true,
		},
		{
			name:   "contains REQUIRES_REBOOT tag",
			script: "# REQUIRES_REBOOT\nSome-Command",
			want:   true,
		},
		{
			name:   "no reboot needed",
			script: "Write-Host 'Hello World'",
			want:   false,
		},
		{
			name:   "case insensitive",
			script: "restart-computer",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := needsReboot(tt.script)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMarshalComponentDocument(t *testing.T) {
	t.Parallel()

	doc := map[string]interface{}{
		"schemaVersion": 1.0,
		"name":          "TestComponent",
		"description":   "Test component",
	}

	result, err := marshalComponentDocument(doc)
	require.NoError(t, err)
	assert.Contains(t, result, "TestComponent")
	assert.Contains(t, result, "schemaVersion")
}

func TestCreateComponentDocument_AllTypes(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "test.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("echo test"), 0644))

	psScriptPath := filepath.Join(tmpDir, "test.ps1")
	require.NoError(t, os.WriteFile(psScriptPath, []byte("Write-Host 'test'"), 0644))

	playbookPath := filepath.Join(tmpDir, "playbook.yml")
	require.NoError(t, os.WriteFile(playbookPath, []byte("---\n- hosts: all"), 0644))

	tests := []struct {
		name        string
		provisioner builder.Provisioner
		wantErr     bool
		contains    string
	}{
		{
			name: "shell",
			provisioner: builder.Provisioner{
				Type:   "shell",
				Inline: []string{"echo hello"},
			},
			contains: "ShellProvisioner",
		},
		{
			name: "script",
			provisioner: builder.Provisioner{
				Type:    "script",
				Scripts: []string{scriptPath},
			},
			contains: "ScriptProvisioner",
		},
		{
			name: "ansible",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: playbookPath,
			},
			contains: "AnsibleProvisioner",
		},
		{
			name: "powershell",
			provisioner: builder.Provisioner{
				Type:      "powershell",
				PSScripts: []string{psScriptPath},
			},
			contains: "PowerShellProvisioner",
		},
		{
			name: "unsupported type",
			provisioner: builder.Provisioner{
				Type: "chef",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &ComponentGenerator{}
			doc, err := gen.createComponentDocument(tt.provisioner)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, doc, tt.contains)
			}
		})
	}
}

func TestPowerShellComponentWithReboot(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "reboot.ps1")
	require.NoError(t, os.WriteFile(scriptPath, []byte("Install-WindowsFeature\nRestart-Computer -Force"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{scriptPath},
	}

	doc, err := gen.createPowerShellComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "Reboot")
	assert.Contains(t, doc, "RebootAfterScript_0")
}

func TestPowerShellComponentWithCustomExecutionPolicy(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "test.ps1")
	require.NoError(t, os.WriteFile(scriptPath, []byte("Write-Host 'hello'"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:            "powershell",
		PSScripts:       []string{scriptPath},
		ExecutionPolicy: "RemoteSigned",
	}

	doc, err := gen.createPowerShellComponent(provisioner)
	require.NoError(t, err)
	assert.Contains(t, doc, "RemoteSigned")
}

func TestPowerShellComponentWithInvalidSyntax(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "bad.ps1")
	require.NoError(t, os.WriteFile(scriptPath, []byte("if ($true) { Write-Host 'unclosed'"), 0644))

	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{scriptPath},
	}

	_, err := gen.createPowerShellComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax issues")
}

func TestWrapPowerShellWithErrorHandling(t *testing.T) {
	t.Parallel()
	script := "Write-Host 'Hello'"
	result := wrapPowerShellWithErrorHandling(script, "Bypass")

	assert.Contains(t, result, "Bypass")
	assert.Contains(t, result, "$ErrorActionPreference = 'Stop'")
	assert.Contains(t, result, "try {")
	assert.Contains(t, result, "Write-Host 'Hello'")
	assert.Contains(t, result, "catch")
}

func TestCreateShellComponent_EmptyInline(t *testing.T) {
	t.Parallel()
	gen := &ComponentGenerator{}
	provisioner := builder.Provisioner{
		Type:   "shell",
		Inline: []string{},
	}

	_, err := gen.createShellComponent(provisioner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shell provisioner has no inline commands")
}

func TestCreateScriptComponent_ViaComponentDocument(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "install.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\napt-get update"), 0644))

	gen := &ComponentGenerator{}
	doc, err := gen.createComponentDocument(builder.Provisioner{
		Type:    "script",
		Scripts: []string{scriptPath},
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(doc, "ScriptProvisioner"))
}
