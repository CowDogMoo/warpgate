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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
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
			err := validator.ValidateWithOptions(context.Background(), config, tt.options)
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
	if err := validator.ValidateWithOptions(context.Background(), configWithExistingFile, ValidationOptions{SyntaxOnly: true}); err != nil {
		t.Errorf("ValidateWithOptions(syntax-only) with existing file error: %v", err)
	}
	if err := validator.ValidateWithOptions(context.Background(), configWithExistingFile, ValidationOptions{SyntaxOnly: false}); err != nil {
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
			err := validator.ValidateWithOptions(context.Background(), tt.config, tt.options)
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
	if err := validator.Validate(context.Background(), config); err != nil {
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

	if err := validator.Validate(context.Background(), configWithMissingFile); err == nil {
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
			err := validator.Validate(context.Background(), tt.config)
			if tt.shouldError && err == nil {
				t.Errorf("Validate() expected error containing %q but got none", tt.errorMsg)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestValidator_AMIFilters(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		config      *builder.Config
		shouldError bool
		errContains string
	}{
		{
			name: "ami_filters with AMI target passes",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{
					AMIFilters: &builder.AMIFilterConfig{
						Owners:  []string{"679593333241"},
						Filters: map[string]string{"name": "kali-*"},
					},
				},
				Targets: []builder.Target{
					{Type: "ami", Region: "us-east-1", AMIName: "test-ami"},
				},
			},
			shouldError: false,
		},
		{
			name: "both image and ami_filters errors",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{
					Image: "ami-123",
					AMIFilters: &builder.AMIFilterConfig{
						Owners:  []string{"679593333241"},
						Filters: map[string]string{"name": "kali-*"},
					},
				},
				Targets: []builder.Target{
					{Type: "ami", Region: "us-east-1", AMIName: "test-ami"},
				},
			},
			shouldError: true,
			errContains: "mutually exclusive",
		},
		{
			name: "ami_filters without AMI target errors",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{
					AMIFilters: &builder.AMIFilterConfig{
						Owners:  []string{"679593333241"},
						Filters: map[string]string{"name": "kali-*"},
					},
				},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
			shouldError: true,
			errContains: "requires at least one AMI target",
		},
		{
			name: "ami_filters with empty owners errors",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{
					AMIFilters: &builder.AMIFilterConfig{
						Owners:  []string{},
						Filters: map[string]string{"name": "kali-*"},
					},
				},
				Targets: []builder.Target{
					{Type: "ami", Region: "us-east-1", AMIName: "test-ami"},
				},
			},
			shouldError: true,
			errContains: "at least one owner",
		},
		{
			name: "ami_filters with empty filters errors",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{
					AMIFilters: &builder.AMIFilterConfig{
						Owners:  []string{"679593333241"},
						Filters: map[string]string{},
					},
				},
				Targets: []builder.Target{
					{Type: "ami", Region: "us-east-1", AMIName: "test-ami"},
				},
			},
			shouldError: true,
			errContains: "at least one filter",
		},
		{
			name: "neither image nor ami_filters errors",
			config: &builder.Config{
				Name: "test",
				Base: builder.BaseImage{},
				Targets: []builder.Target{
					{Type: "ami", Region: "us-east-1", AMIName: "test-ami"},
				},
			},
			shouldError: true,
			errContains: "config.base.image is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateWithOptions(context.Background(), tt.config, ValidationOptions{SyntaxOnly: true})
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q but got none", tt.errContains)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q but got: %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
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
			err := validator.ValidateWithOptions(context.Background(), tt.config, tt.options)
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
			err := validator.Validate(context.Background(), tt.config)
			if tt.shouldError && err == nil {
				t.Errorf("Validate() expected error but got none. %s", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Validate() unexpected error: %v. %s", err, tt.description)
			}
		})
	}
}

func TestValidator_ValidateSource(t *testing.T) {
	validator := NewValidator()

	baseConfig := func(sources []builder.Source) *builder.Config {
		return &builder.Config{
			Name: "test",
			Base: builder.BaseImage{Image: "ubuntu:22.04"},
			Provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo test"}},
			},
			Targets: []builder.Target{
				{Type: "container", Platforms: []string{"linux/amd64"}},
			},
			Sources: sources,
		}
	}

	tests := []struct {
		name        string
		sources     []builder.Source
		shouldError bool
		errContains string
	}{
		{
			name: "Valid git source",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo.git",
					},
				},
			},
			shouldError: false,
		},
		{
			name: "Valid git source with ref and depth",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo.git",
						Ref:        "main",
						Depth:      1,
					},
				},
			},
			shouldError: false,
		},
		{
			name: "Valid SSH git source",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "git@github.com:org/repo.git",
					},
				},
			},
			shouldError: false,
		},
		{
			name: "Missing source name",
			sources: []builder.Source{
				{
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo.git",
					},
				},
			},
			shouldError: true,
			errContains: "name is required",
		},
		{
			name: "Duplicate source names",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo1.git",
					},
				},
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo2.git",
					},
				},
			},
			shouldError: true,
			errContains: "duplicate source name",
		},
		{
			name: "Invalid source name characters",
			sources: []builder.Source{
				{
					Name: "my source!",
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo.git",
					},
				},
			},
			shouldError: true,
			errContains: "invalid characters",
		},
		{
			name: "Missing source type",
			sources: []builder.Source{
				{
					Name: "my-source",
				},
			},
			shouldError: true,
			errContains: "must specify a source type",
		},
		{
			name: "Missing git repository",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git:  &builder.GitSource{},
				},
			},
			shouldError: true,
			errContains: "repository is required",
		},
		{
			name: "Invalid git repository URL",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "not-a-valid-url",
					},
				},
			},
			shouldError: true,
			errContains: "valid git URL",
		},
		{
			name: "Negative depth",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo.git",
						Depth:      -1,
					},
				},
			},
			shouldError: true,
			errContains: "non-negative",
		},
		{
			name: "Valid source with token auth",
			sources: []builder.Source{
				{
					Name: "my-source",
					Git: &builder.GitSource{
						Repository: "https://github.com/org/repo.git",
						Auth: &builder.GitAuth{
							Token: "${GITHUB_TOKEN}",
						},
					},
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig(tt.sources)
			err := validator.ValidateWithOptions(context.Background(), config, ValidationOptions{SyntaxOnly: true})

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q but got none", tt.errContains)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q but got: %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidator_AzureTargetRejectsInvalidOSTypeWhenExplicit(t *testing.T) {
	validator := NewValidator()
	cfg := &builder.Config{
		Name: "azure-test",
		Base: builder.BaseImage{Image: "ubuntu:22.04"},
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"echo hi"}},
		},
		Targets: []builder.Target{
			{
				Type:                   "azure",
				ResourceGroup:          "rg-1",
				Location:               "eastus",
				Gallery:                "gallery1",
				GalleryImageDefinition: "def1",
				IdentityID:             "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami",
				OSType:                 "Solaris",
				VMSize:                 "Standard_D2s_v3",
				SourceImage:            &builder.AzureSourceImage{GalleryImageVersionID: "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/galleries/g/images/i/versions/1.0.0"},
			},
		},
	}

	err := validator.ValidateWithOptions(context.Background(), cfg, ValidationOptions{SyntaxOnly: true})
	if err == nil || !contains(err.Error(), "must be \"Linux\" or \"Windows\"") {
		t.Fatalf("expected invalid os_type error, got %v", err)
	}
}

// azureValidTarget returns a fully-populated Azure target that should pass
// validation. Tests mutate a copy of this fixture to exercise individual
// failure modes.
func azureValidTarget() builder.Target {
	return builder.Target{
		Type:                   "azure",
		ResourceGroup:          "rg-1",
		Location:               "eastus",
		Gallery:                "gallery1",
		GalleryImageDefinition: "def1",
		IdentityID:             "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami",
		OSType:                 "Linux",
		VMSize:                 "Standard_D2s_v3",
		SourceImage: &builder.AzureSourceImage{
			Marketplace: &builder.AzureMarketplaceImage{
				Publisher: "Canonical",
				Offer:     "0001-com-ubuntu-server-jammy",
				SKU:       "22_04-lts-gen2",
				Version:   "latest",
			},
		},
	}
}

func azureBaseConfig(target builder.Target) *builder.Config {
	return &builder.Config{
		Name: "azure-test",
		Base: builder.BaseImage{Image: "ubuntu:22.04"},
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"echo hi"}},
		},
		Targets: []builder.Target{target},
	}
}

func TestValidator_AzureTargetValidation(t *testing.T) {
	validSubnet := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet"
	galleryRef := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/galleries/g/images/i/versions/1.0.0"

	tests := []struct {
		name        string
		mutate      func(t *builder.Target)
		shouldError bool
		errContains string
	}{
		{
			name:        "valid marketplace target",
			mutate:      func(t *builder.Target) {},
			shouldError: false,
		},
		{
			name: "valid gallery reference target",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{GalleryImageVersionID: galleryRef}
			},
			shouldError: false,
		},
		{
			name: "valid Windows os_type",
			mutate: func(t *builder.Target) {
				t.OSType = "Windows"
			},
			shouldError: false,
		},
		{
			name: "valid networking with subnet and proxy size",
			mutate: func(t *builder.Target) {
				t.SubnetID = validSubnet
				t.ProxyVMSize = "Standard_D2s_v3"
			},
			shouldError: false,
		},
		{
			name: "valid share_with entries",
			mutate: func(t *builder.Target) {
				t.ShareWith = []string{"sub-aaa", "sub-bbb"}
			},
			shouldError: false,
		},
		{
			name:        "missing resource_group",
			mutate:      func(t *builder.Target) { t.ResourceGroup = "" },
			shouldError: true,
			errContains: "requires 'resource_group'",
		},
		{
			name:        "missing location",
			mutate:      func(t *builder.Target) { t.Location = "" },
			shouldError: true,
			errContains: "requires 'location'",
		},
		{
			name:        "missing gallery",
			mutate:      func(t *builder.Target) { t.Gallery = "" },
			shouldError: true,
			errContains: "requires 'gallery'",
		},
		{
			name:        "missing gallery_image_definition",
			mutate:      func(t *builder.Target) { t.GalleryImageDefinition = "" },
			shouldError: true,
			errContains: "requires 'gallery_image_definition'",
		},
		{
			name:        "missing identity_id",
			mutate:      func(t *builder.Target) { t.IdentityID = "" },
			shouldError: true,
			errContains: "requires 'identity_id'",
		},
		{
			name:        "missing os_type",
			mutate:      func(t *builder.Target) { t.OSType = "" },
			shouldError: true,
			errContains: "requires 'os_type'",
		},
		{
			name:        "invalid os_type",
			mutate:      func(t *builder.Target) { t.OSType = "BSD" },
			shouldError: true,
			errContains: "must be \"Linux\" or \"Windows\"",
		},
		{
			name:        "missing source_image",
			mutate:      func(t *builder.Target) { t.SourceImage = nil },
			shouldError: true,
			errContains: "requires 'source_image'",
		},
		{
			name: "both marketplace and gallery_image_version_id set",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "Canonical",
						Offer:     "0001-com-ubuntu-server-jammy",
						SKU:       "22_04-lts-gen2",
					},
					GalleryImageVersionID: galleryRef,
				}
			},
			shouldError: true,
			errContains: "must set exactly one of 'marketplace' or 'gallery_image_version_id'",
		},
		{
			name: "neither marketplace nor gallery reference set",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{}
			},
			shouldError: true,
			errContains: "must set exactly one of 'marketplace' or 'gallery_image_version_id'",
		},
		{
			name: "marketplace missing publisher",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Offer: "offer", SKU: "sku",
					},
				}
			},
			shouldError: true,
			errContains: "marketplace source requires publisher, offer, and sku",
		},
		{
			name: "marketplace missing offer",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "pub", SKU: "sku",
					},
				}
			},
			shouldError: true,
			errContains: "marketplace source requires publisher, offer, and sku",
		},
		{
			name: "marketplace missing sku",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "pub", Offer: "offer",
					},
				}
			},
			shouldError: true,
			errContains: "marketplace source requires publisher, offer, and sku",
		},
		{
			name: "marketplace plan with valid fields",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "kali-linux",
						Offer:     "kali",
						SKU:       "kali",
						Plan: &builder.AzurePurchasePlan{
							Name:      "kali",
							Product:   "kali-linux",
							Publisher: "kali-linux",
						},
					},
				}
			},
			shouldError: false,
		},
		{
			name: "marketplace plan missing name",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "pub", Offer: "offer", SKU: "sku",
						Plan: &builder.AzurePurchasePlan{
							Product: "p", Publisher: "pp",
						},
					},
				}
			},
			shouldError: true,
			errContains: "marketplace 'plan' requires name, product, and publisher",
		},
		{
			name: "marketplace plan missing product",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "pub", Offer: "offer", SKU: "sku",
						Plan: &builder.AzurePurchasePlan{
							Name: "n", Publisher: "pp",
						},
					},
				}
			},
			shouldError: true,
			errContains: "marketplace 'plan' requires name, product, and publisher",
		},
		{
			name: "marketplace plan missing publisher",
			mutate: func(t *builder.Target) {
				t.SourceImage = &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "pub", Offer: "offer", SKU: "sku",
						Plan: &builder.AzurePurchasePlan{
							Name: "n", Product: "p",
						},
					},
				}
			},
			shouldError: true,
			errContains: "marketplace 'plan' requires name, product, and publisher",
		},
		{
			name: "blank share_with entry",
			mutate: func(t *builder.Target) {
				t.ShareWith = []string{"sub-aaa", "  "}
			},
			shouldError: true,
			errContains: "share_with[1]' must not be blank",
		},
		{
			name: "empty share_with entry",
			mutate: func(t *builder.Target) {
				t.ShareWith = []string{""}
			},
			shouldError: true,
			errContains: "share_with[0]' must not be blank",
		},
		{
			name: "invalid subnet_id format",
			mutate: func(t *builder.Target) {
				t.SubnetID = "not-an-arm-id"
			},
			shouldError: true,
			errContains: "must be a full resource ID",
		},
		{
			name: "subnet_id missing subnets segment",
			mutate: func(t *builder.Target) {
				t.SubnetID = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet"
			},
			shouldError: true,
			errContains: "must be a full resource ID",
		},
		{
			name: "proxy_vm_size without subnet_id",
			mutate: func(t *builder.Target) {
				t.ProxyVMSize = "Standard_D2s_v3"
			},
			shouldError: true,
			errContains: "'proxy_vm_size' has no effect without 'subnet_id'",
		},
	}

	validator := NewValidator()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			target := azureValidTarget()
			tc.mutate(&target)
			cfg := azureBaseConfig(target)
			err := validator.ValidateWithOptions(context.Background(), cfg, ValidationOptions{SyntaxOnly: true})
			if tc.shouldError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errContains)
				}
				if tc.errContains != "" && !contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tc.errContains, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
