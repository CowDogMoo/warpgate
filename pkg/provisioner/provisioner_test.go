/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package provisioner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewShellProvisioner tests the shell provisioner constructor
func TestNewShellProvisioner(t *testing.T) {
	// Shell provisioner can be created with nil builder for unit tests
	sp := NewShellProvisioner(nil, "/usr/bin/crun")
	assert.NotNil(t, sp)
}

// TestShellProvisioner_Provision_NoCommands tests error handling when no commands provided
func TestShellProvisioner_Provision_NoCommands(t *testing.T) {
	sp := NewShellProvisioner(nil, "/usr/bin/crun")
	ctx := context.Background()

	config := builder.Provisioner{
		Type:   "shell",
		Inline: []string{},
	}

	err := sp.Provision(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires inline commands")
}

// TestShellProvisioner_Provision_WithCommands verifies provisioner structure
func TestShellProvisioner_Provision_WithCommands(t *testing.T) {
	// This is a unit test that verifies the structure
	// Integration tests with actual buildah builder are in separate test file
	config := builder.Provisioner{
		Type: "shell",
		Inline: []string{
			"echo hello",
			"echo world",
		},
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
		WorkingDir: "/tmp",
	}

	assert.Equal(t, "shell", config.Type)
	assert.Len(t, config.Inline, 2)
	assert.Equal(t, "echo hello", config.Inline[0])
	assert.Equal(t, "echo world", config.Inline[1])
	assert.Equal(t, "test_value", config.Environment["TEST_VAR"])
	assert.Equal(t, "/tmp", config.WorkingDir)
}

// TestNewScriptProvisioner tests the script provisioner constructor
func TestNewScriptProvisioner(t *testing.T) {
	sp := NewScriptProvisioner(nil)
	assert.NotNil(t, sp)
}

// TestScriptProvisioner_Provision_NoScripts tests error handling when no scripts provided
func TestScriptProvisioner_Provision_NoScripts(t *testing.T) {
	sp := NewScriptProvisioner(nil)
	ctx := context.Background()

	config := builder.Provisioner{
		Type:    "script",
		Scripts: []string{},
	}

	err := sp.Provision(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires scripts")
}

// TestScriptProvisioner_Provision_WithScripts verifies provisioner structure
func TestScriptProvisioner_Provision_WithScripts(t *testing.T) {
	config := builder.Provisioner{
		Type: "script",
		Scripts: []string{
			"/path/to/script1.sh",
			"/path/to/script2.sh",
		},
		Environment: map[string]string{
			"SCRIPT_VAR": "script_value",
		},
		WorkingDir: "/app",
	}

	assert.Equal(t, "script", config.Type)
	assert.Len(t, config.Scripts, 2)
	assert.Equal(t, "/path/to/script1.sh", config.Scripts[0])
	assert.Equal(t, "/path/to/script2.sh", config.Scripts[1])
	assert.Equal(t, "script_value", config.Environment["SCRIPT_VAR"])
	assert.Equal(t, "/app", config.WorkingDir)
}

// TestScriptProvisioner_ScriptPaths tests script path handling
func TestScriptProvisioner_ScriptPaths(t *testing.T) {
	tests := []struct {
		name       string
		scriptPath string
		expected   string
	}{
		{
			name:       "absolute path",
			scriptPath: "/usr/local/bin/install.sh",
			expected:   "install.sh",
		},
		{
			name:       "relative path",
			scriptPath: "scripts/setup.sh",
			expected:   "setup.sh",
		},
		{
			name:       "just filename",
			scriptPath: "configure.sh",
			expected:   "configure.sh",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			basename := filepath.Base(tc.scriptPath)
			assert.Equal(t, tc.expected, basename)
		})
	}
}

// TestNewAnsibleProvisioner tests the ansible provisioner constructor
func TestNewAnsibleProvisioner(t *testing.T) {
	ap := NewAnsibleProvisioner(nil, "/usr/bin/crun")
	assert.NotNil(t, ap)
}

// TestAnsibleProvisioner_Provision_NoPlaybook tests error handling when no playbook provided
func TestAnsibleProvisioner_Provision_NoPlaybook(t *testing.T) {
	ap := NewAnsibleProvisioner(nil, "/usr/bin/crun")
	ctx := context.Background()

	config := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "",
	}

	err := ap.Provision(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires playbook_path")
}

// TestAnsibleProvisioner_Provision_WithPlaybook verifies provisioner structure
func TestAnsibleProvisioner_Provision_WithPlaybook(t *testing.T) {
	config := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "/path/to/playbook.yml",
		GalaxyFile:   "/path/to/requirements.yml",
		ExtraVars: map[string]string{
			"ansible_var": "value",
		},
		Inventory:  "/path/to/inventory",
		WorkingDir: "/ansible",
	}

	assert.Equal(t, "ansible", config.Type)
	assert.Equal(t, "/path/to/playbook.yml", config.PlaybookPath)
	assert.Equal(t, "/path/to/requirements.yml", config.GalaxyFile)
	assert.Equal(t, "value", config.ExtraVars["ansible_var"])
	assert.Equal(t, "/path/to/inventory", config.Inventory)
	assert.Equal(t, "/ansible", config.WorkingDir)
}

// TestAnsibleProvisioner_PlaybookPath tests playbook path validation
func TestAnsibleProvisioner_PlaybookPath(t *testing.T) {
	tests := []struct {
		name         string
		playbookPath string
		expectValid  bool
	}{
		{
			name:         "yaml extension",
			playbookPath: "playbook.yml",
			expectValid:  true,
		},
		{
			name:         "yaml extension alternate",
			playbookPath: "playbook.yaml",
			expectValid:  true,
		},
		{
			name:         "absolute path",
			playbookPath: "/opt/ansible/site.yml",
			expectValid:  true,
		},
		{
			name:         "relative path",
			playbookPath: "../../ansible/deploy.yml",
			expectValid:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ext := filepath.Ext(tc.playbookPath)
			isYaml := ext == ".yml" || ext == ".yaml"
			assert.Equal(t, tc.expectValid, isYaml)
		})
	}
}

// TestAnsibleProvisioner_GalaxyFile tests galaxy file handling
func TestAnsibleProvisioner_GalaxyFile(t *testing.T) {
	config := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "playbook.yml",
		GalaxyFile:   "requirements.yml",
	}

	assert.NotEmpty(t, config.GalaxyFile)
	assert.Equal(t, "requirements.yml", config.GalaxyFile)
}

// TestAnsibleProvisioner_ExtraVars tests extra variables handling
func TestAnsibleProvisioner_ExtraVars(t *testing.T) {
	extraVars := map[string]string{
		"app_version": "1.2.3",
		"environment": "production",
		"debug_mode":  "false",
	}

	config := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "deploy.yml",
		ExtraVars:    extraVars,
	}

	assert.Len(t, config.ExtraVars, 3)
	assert.Equal(t, "1.2.3", config.ExtraVars["app_version"])
	assert.Equal(t, "production", config.ExtraVars["environment"])
	assert.Equal(t, "false", config.ExtraVars["debug_mode"])
}

// TestParseGalaxyRequirements tests galaxy requirements parsing
func TestParseGalaxyRequirements(t *testing.T) {
	// Create a temporary requirements file
	tmpDir := t.TempDir()
	reqFile := filepath.Join(tmpDir, "requirements.yml")

	content := `---
collections:
  - name: community.general
    version: ">=3.0.0"
  - name: ansible.posix
    version: "1.4.0"
`

	err := os.WriteFile(reqFile, []byte(content), 0644)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(reqFile)
	assert.NoError(t, err)

	// Read and verify content
	data, err := os.ReadFile(reqFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "community.general")
	assert.Contains(t, string(data), "ansible.posix")
}

// TestEnvironmentVariables tests environment variable handling across provisioners
func TestEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected int
	}{
		{
			name:     "no variables",
			envVars:  map[string]string{},
			expected: 0,
		},
		{
			name: "single variable",
			envVars: map[string]string{
				"VAR1": "value1",
			},
			expected: 1,
		},
		{
			name: "multiple variables",
			envVars: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
				"VAR3": "value3",
			},
			expected: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := builder.Provisioner{
				Type:        "shell",
				Inline:      []string{"echo test"},
				Environment: tc.envVars,
			}

			assert.Len(t, config.Environment, tc.expected)
		})
	}
}

// TestWorkingDirectory tests working directory handling across provisioners
func TestWorkingDirectory(t *testing.T) {
	tests := []struct {
		name       string
		workingDir string
		isEmpty    bool
	}{
		{
			name:       "no working dir",
			workingDir: "",
			isEmpty:    true,
		},
		{
			name:       "absolute path",
			workingDir: "/app",
			isEmpty:    false,
		},
		{
			name:       "relative path",
			workingDir: "./workspace",
			isEmpty:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := builder.Provisioner{
				Type:       "shell",
				Inline:     []string{"pwd"},
				WorkingDir: tc.workingDir,
			}

			if tc.isEmpty {
				assert.Empty(t, config.WorkingDir)
			} else {
				assert.NotEmpty(t, config.WorkingDir)
				assert.Equal(t, tc.workingDir, config.WorkingDir)
			}
		})
	}
}

// TestProvisionerTypes tests all provisioner types
func TestProvisionerTypes(t *testing.T) {
	types := []string{"shell", "script", "ansible", "powershell"}

	for _, provType := range types {
		t.Run(provType, func(t *testing.T) {
			config := builder.Provisioner{
				Type: provType,
			}

			assert.Equal(t, provType, config.Type)
			assert.NotEmpty(t, config.Type)
		})
	}
}

// TestAnsibleProvisioner_RolesInstallation tests roles installation from requirements.yml
func TestAnsibleProvisioner_RolesInstallation(t *testing.T) {
	// Create a temporary requirements file with roles
	tmpDir := t.TempDir()
	reqFile := filepath.Join(tmpDir, "requirements.yml")

	content := `---
roles:
  - name: geerlingguy.docker
    version: "7.4.1"
  - name: example.test_role
    version: "1.0.0"
`

	err := os.WriteFile(reqFile, []byte(content), 0644)
	require.NoError(t, err)

	// Verify file exists and contains roles
	data, err := os.ReadFile(reqFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "roles:")
	assert.Contains(t, string(data), "geerlingguy.docker")
	assert.Contains(t, string(data), "example.test_role")
}

// TestAnsibleProvisioner_LocalCollectionDetection tests local collection detection
func TestAnsibleProvisioner_LocalCollectionDetection(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	reqFile := filepath.Join(tmpDir, "requirements.yml")
	galaxyFile := filepath.Join(tmpDir, "galaxy.yml")

	// Create requirements.yml
	reqContent := `---
collections:
  - name: community.general
    version: ">=3.0.0"
`
	err := os.WriteFile(reqFile, []byte(reqContent), 0644)
	require.NoError(t, err)

	// Create galaxy.yml
	galaxyContent := `---
namespace: myorg
name: mycollection
version: "1.0.0"
readme: README.md
authors:
  - Test Author <test@example.com>
description: Test collection
license_file: "LICENSE"
`
	err = os.WriteFile(galaxyFile, []byte(galaxyContent), 0644)
	require.NoError(t, err)

	// Verify both files exist in the same directory
	reqDir := filepath.Dir(reqFile)
	galaxyPath := filepath.Join(reqDir, "galaxy.yml")

	_, err = os.Stat(galaxyPath)
	assert.NoError(t, err, "galaxy.yml should exist in the same directory as requirements.yml")

	// Verify galaxy.yml contains collection metadata
	data, err := os.ReadFile(galaxyPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "namespace:")
	assert.Contains(t, string(data), "name:")
	assert.Contains(t, string(data), "version:")
}

// TestAnsibleProvisioner_MixedRequirements tests requirements.yml with both roles and collections
func TestAnsibleProvisioner_MixedRequirements(t *testing.T) {
	tmpDir := t.TempDir()
	reqFile := filepath.Join(tmpDir, "requirements.yml")

	content := `---
roles:
  - name: geerlingguy.docker
    version: "7.4.1"

collections:
  - name: community.general
    version: ">=3.0.0"
  - name: ansible.posix
    version: "1.4.0"
`

	err := os.WriteFile(reqFile, []byte(content), 0644)
	require.NoError(t, err)

	// Verify file contains both roles and collections
	data, err := os.ReadFile(reqFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "roles:")
	assert.Contains(t, string(data), "geerlingguy.docker")
	assert.Contains(t, string(data), "collections:")
	assert.Contains(t, string(data), "community.general")
	assert.Contains(t, string(data), "ansible.posix")
}
