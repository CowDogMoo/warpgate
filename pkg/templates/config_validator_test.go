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

package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/builder"
)

func TestValidator_HasUnresolvedVariable(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Path with $ literal",
			path:     "/path/with/$VAR/file",
			expected: true,
		},
		{
			name:     "Path with ${} syntax",
			path:     "${HOME}/file",
			expected: true,
		},
		{
			name:     "Suspicious absolute path /playbooks",
			path:     "/playbooks/ansible/play.yml",
			expected: true,
		},
		{
			name:     "Suspicious absolute path /requirements",
			path:     "/requirements.yml",
			expected: true,
		},
		{
			name:     "Valid /home path",
			path:     "/home/user/playbooks/play.yml",
			expected: false,
		},
		{
			name:     "Valid /opt path",
			path:     "/opt/ansible/playbooks/play.yml",
			expected: false,
		},
		{
			name:     "Valid /usr path",
			path:     "/usr/local/bin/script.sh",
			expected: false,
		},
		{
			name:     "Valid /tmp path",
			path:     "/tmp/test-script.sh",
			expected: false,
		},
		{
			name:     "Relative path",
			path:     "playbooks/play.yml",
			expected: false,
		},
		{
			name:     "Valid /etc path",
			path:     "/etc/ansible/playbook.yml",
			expected: false,
		},
		{
			name:     "Valid /var path",
			path:     "/var/lib/playbooks/play.yml",
			expected: false,
		},
		{
			name:     "Valid /root path",
			path:     "/root/playbooks/play.yml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.hasUnresolvedVariable(tt.path)
			if result != tt.expected {
				t.Errorf("hasUnresolvedVariable(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestValidator_ValidateWithOptions_SyntaxOnly(t *testing.T) {
	validator := NewValidator()
	tmpDir := t.TempDir()

	// Create a config with a non-existent playbook file
	config := &builder.Config{
		Name: "test",
		Base: builder.BaseImage{
			Image: "ubuntu:22.04",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: "/nonexistent/playbook.yml",
			},
		},
		Targets: []builder.Target{
			{
				Type:      "container",
				Platforms: []string{"linux/amd64"},
			},
		},
	}

	tests := []struct {
		name        string
		options     ValidationOptions
		shouldError bool
		description string
	}{
		{
			name:        "Syntax-only mode should pass with missing files",
			options:     ValidationOptions{SyntaxOnly: true},
			shouldError: false,
			description: "Syntax-only validation should skip file existence checks",
		},
		{
			name:        "Full validation should fail with missing files",
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: true,
			description: "Full validation should error on missing files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateWithOptions(config, tt.options)
			if tt.shouldError && err == nil {
				t.Errorf("ValidateWithOptions() expected error but got none. %s", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ValidateWithOptions() unexpected error: %v. %s", err, tt.description)
			}
		})
	}

	// Test with existing file
	existingPlaybook := filepath.Join(tmpDir, "playbook.yml")
	if err := os.WriteFile(existingPlaybook, []byte("---\n"), 0644); err != nil {
		t.Fatalf("Failed to create test playbook: %v", err)
	}

	configWithExistingFile := &builder.Config{
		Name: "test",
		Base: builder.BaseImage{
			Image: "ubuntu:22.04",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: existingPlaybook,
			},
		},
		Targets: []builder.Target{
			{
				Type:      "container",
				Platforms: []string{"linux/amd64"},
			},
		},
	}

	// Both modes should pass with existing file
	if err := validator.ValidateWithOptions(configWithExistingFile, ValidationOptions{SyntaxOnly: true}); err != nil {
		t.Errorf("ValidateWithOptions(syntax-only) with existing file error: %v", err)
	}
	if err := validator.ValidateWithOptions(configWithExistingFile, ValidationOptions{SyntaxOnly: false}); err != nil {
		t.Errorf("ValidateWithOptions(full) with existing file error: %v", err)
	}
}

func TestValidator_ValidateProvisioners(t *testing.T) {
	validator := NewValidator()
	tmpDir := t.TempDir()

	// Create test files
	testScript := filepath.Join(tmpDir, "test-script.sh")
	if err := os.WriteFile(testScript, []byte("#!/bin/bash\necho test\n"), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	testPlaybook := filepath.Join(tmpDir, "playbook.yml")
	if err := os.WriteFile(testPlaybook, []byte("---\n"), 0644); err != nil {
		t.Fatalf("Failed to create test playbook: %v", err)
	}

	tests := []struct {
		name        string
		config      *builder.Config
		options     ValidationOptions
		shouldError bool
		description string
	}{
		{
			name: "Shell provisioner with inline commands",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{
						Type:   "shell",
						Inline: []string{"echo test"},
					},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: false,
			description: "Valid shell provisioner should pass",
		},
		{
			name: "Shell provisioner without inline commands",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{Type: "shell"},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: true,
			description: "Shell provisioner without inline should fail",
		},
		{
			name: "Script provisioner with existing script",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{
						Type:    "script",
						Scripts: []string{testScript},
					},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: false,
			description: "Script provisioner with existing file should pass",
		},
		{
			name: "Script provisioner with missing script in full mode",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{
						Type:    "script",
						Scripts: []string{"/nonexistent/script.sh"},
					},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: true,
			description: "Script provisioner with missing file should fail in full mode",
		},
		{
			name: "Script provisioner with missing script in syntax-only mode",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{
						Type:    "script",
						Scripts: []string{"/nonexistent/script.sh"},
					},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: true},
			shouldError: false,
			description: "Script provisioner with missing file should pass in syntax-only mode",
		},
		{
			name: "Ansible provisioner with galaxy file",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{
						Type:         "ansible",
						PlaybookPath: testPlaybook,
						GalaxyFile:   testPlaybook, // reuse same file for simplicity
					},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: false,
			description: "Ansible provisioner with existing galaxy file should pass",
		},
		{
			name: "Ansible provisioner with missing galaxy file in full mode",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{
						Type:         "ansible",
						PlaybookPath: testPlaybook,
						GalaxyFile:   "/nonexistent/requirements.yml",
					},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: true,
			description: "Ansible with missing galaxy file should fail in full mode",
		},
		{
			name: "Ansible provisioner with missing galaxy file in syntax-only mode",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Provisioners: []builder.Provisioner{
					{
						Type:         "ansible",
						PlaybookPath: testPlaybook,
						GalaxyFile:   "/nonexistent/requirements.yml",
					},
				},
				Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
			},
			options:     ValidationOptions{SyntaxOnly: true},
			shouldError: false,
			description: "Ansible with missing galaxy file should pass in syntax-only mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateWithOptions(tt.config, tt.options)
			if tt.shouldError && err == nil {
				t.Errorf("ValidateWithOptions() expected error but got none. %s", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ValidateWithOptions() unexpected error: %v. %s", err, tt.description)
			}
		})
	}
}

func TestValidator_Validate_BackwardCompatibility(t *testing.T) {
	validator := NewValidator()
	tmpDir := t.TempDir()

	testPlaybook := filepath.Join(tmpDir, "playbook.yml")
	if err := os.WriteFile(testPlaybook, []byte("---\n"), 0644); err != nil {
		t.Fatalf("Failed to create test playbook: %v", err)
	}

	config := &builder.Config{
		Name: "test",
		Base: builder.BaseImage{
			Image: "ubuntu:22.04",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: testPlaybook,
			},
		},
		Targets: []builder.Target{
			{
				Type:      "container",
				Platforms: []string{"linux/amd64"},
			},
		},
	}

	// Test that the old Validate() method still works (defaults to full validation)
	if err := validator.Validate(config); err != nil {
		t.Errorf("Validate() (backward compatibility) error: %v", err)
	}

	// Test with missing file - should fail with old method
	configWithMissingFile := &builder.Config{
		Name: "test",
		Base: builder.BaseImage{Image: "ubuntu:22.04"},
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: "/nonexistent/playbook.yml",
			},
		},
		Targets: []builder.Target{{Type: "container", Platforms: []string{"linux/amd64"}}},
	}

	if err := validator.Validate(configWithMissingFile); err == nil {
		t.Error("Validate() (backward compatibility) should fail with missing file")
	}
}

func TestValidator_RequiredFields(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		config      *builder.Config
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Missing config name",
			config: &builder.Config{
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
			},
			shouldError: true,
			errorMsg:    "config.name is required",
		},
		{
			name: "Missing base image",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{},
			},
			shouldError: true,
			errorMsg:    "config.base.image is required",
		},
		{
			name: "Missing targets",
			config: &builder.Config{
				Name:    "test",
				Base:    builder.BaseImage{Image: "ubuntu:22.04"},
				Targets: []builder.Target{},
			},
			shouldError: true,
			errorMsg:    "at least one target is required",
		},
		{
			name: "Valid minimal config",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.config)
			if tt.shouldError && err == nil {
				t.Errorf("Validate() expected error containing %q but got none", tt.errorMsg)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestValidator_ValidateDockerfile(t *testing.T) {
	validator := NewValidator()
	tmpDir := t.TempDir()

	// Create test Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM ubuntu:22.04"), 0644); err != nil {
		t.Fatalf("Failed to create test Dockerfile: %v", err)
	}

	tests := []struct {
		name        string
		config      *builder.Config
		options     ValidationOptions
		shouldError bool
		description string
	}{
		{
			name: "valid dockerfile config with existing file",
			config: &builder.Config{
				Name: "test",
				Dockerfile: &builder.DockerfileConfig{
					Path: dockerfilePath,
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: false,
			description: "Should pass with existing Dockerfile",
		},
		{
			name: "dockerfile config with missing file in full mode",
			config: &builder.Config{
				Name: "test",
				Dockerfile: &builder.DockerfileConfig{
					Path: "/nonexistent/Dockerfile",
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: true,
			description: "Should fail with missing Dockerfile in full validation",
		},
		{
			name: "dockerfile config with missing file in syntax-only mode",
			config: &builder.Config{
				Name: "test",
				Dockerfile: &builder.DockerfileConfig{
					Path: "/nonexistent/Dockerfile",
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			options:     ValidationOptions{SyntaxOnly: true},
			shouldError: false,
			description: "Should pass with missing Dockerfile in syntax-only mode",
		},
		{
			name: "dockerfile with default path",
			config: &builder.Config{
				Name: "test",
				Dockerfile: &builder.DockerfileConfig{
					Path: "",
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			options:     ValidationOptions{SyntaxOnly: true},
			shouldError: false,
			description: "Should handle default Dockerfile path",
		},
		{
			name: "dockerfile with build args and target",
			config: &builder.Config{
				Name: "test",
				Dockerfile: &builder.DockerfileConfig{
					Path: dockerfilePath,
					Args: map[string]string{
						"VERSION": "1.0.0",
					},
					Target: "production",
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			options:     ValidationOptions{SyntaxOnly: false},
			shouldError: false,
			description: "Should pass with build args and target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateWithOptions(tt.config, tt.options)
			if tt.shouldError && err == nil {
				t.Errorf("ValidateWithOptions() expected error but got none. %s", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ValidateWithOptions() unexpected error: %v. %s", err, tt.description)
			}
		})
	}
}

func TestValidator_DockerfileVsProvisioners(t *testing.T) {
	validator := NewValidator()
	tmpDir := t.TempDir()

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM ubuntu:22.04"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	tests := []struct {
		name        string
		config      *builder.Config
		shouldError bool
		description string
	}{
		{
			name: "dockerfile mode skips base image validation",
			config: &builder.Config{
				Name: "test",
				Dockerfile: &builder.DockerfileConfig{
					Path: dockerfilePath,
				},
				// Base is empty - should not error in Dockerfile mode
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			shouldError: false,
			description: "Dockerfile mode should not require base image",
		},
		{
			name: "provisioner mode requires base image",
			config: &builder.Config{
				Name: "test",
				// No Dockerfile config - should require base image
				Provisioners: []builder.Provisioner{
					{
						Type:   "shell",
						Inline: []string{"echo test"},
					},
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			shouldError: true,
			description: "Provisioner mode should require base image",
		},
		{
			name: "dockerfile with provisioners should work",
			config: &builder.Config{
				Name: "test",
				Dockerfile: &builder.DockerfileConfig{
					Path: dockerfilePath,
				},
				Provisioners: []builder.Provisioner{
					{
						Type:   "shell",
						Inline: []string{"echo test"},
					},
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			shouldError: false,
			description: "Can have both Dockerfile and provisioners",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.config)
			if tt.shouldError && err == nil {
				t.Errorf("Validate() expected error but got none. %s", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Validate() unexpected error: %v. %s", err, tt.description)
			}
		})
	}
}
