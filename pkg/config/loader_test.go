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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandVariables(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name     string
		input    string
		vars     map[string]string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "CLI var takes precedence over env var",
			input:    "path: ${TEST_PATH}",
			vars:     map[string]string{"TEST_PATH": "/cli/path"},
			envVars:  map[string]string{"TEST_PATH": "/env/path"},
			expected: "path: /cli/path",
		},
		{
			name:     "Env var used when CLI var not provided",
			input:    "path: ${TEST_PATH}",
			vars:     nil,
			envVars:  map[string]string{"TEST_PATH": "/env/path"},
			expected: "path: /env/path",
		},
		{
			name:     "Multiple variables",
			input:    "path: ${PATH1}/subdir/${PATH2}",
			vars:     map[string]string{"PATH1": "/first", "PATH2": "second"},
			envVars:  nil,
			expected: "path: /first/subdir/second",
		},
		{
			name:     "Mixed CLI and env vars",
			input:    "path: ${CLI_VAR}/${ENV_VAR}",
			vars:     map[string]string{"CLI_VAR": "cli"},
			envVars:  map[string]string{"ENV_VAR": "env"},
			expected: "path: cli/env",
		},
		{
			name:     "Short form variable NOT expanded (left for container)",
			input:    "ENV PATH=/opt/bin:$PATH",
			vars:     map[string]string{"PATH": "/host/path"},
			envVars:  map[string]string{"PATH": "/host/env/path"},
			expected: "ENV PATH=/opt/bin:$PATH",
		},
		{
			name:     "Braced variables expanded, unbraced left alone",
			input:    "ENV PATH=${CUSTOM_PATH}:$PATH",
			vars:     map[string]string{"CUSTOM_PATH": "/opt/myapp"},
			envVars:  map[string]string{"PATH": "/host/path"},
			expected: "ENV PATH=/opt/myapp:$PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tt.envVars {
				if err := os.Setenv(k, v); err != nil {
					t.Fatalf("Failed to set env var %s: %v", k, err)
				}
				defer func(key string) {
					if err := os.Unsetenv(key); err != nil {
						t.Logf("Failed to unset env var %s: %v", key, err)
					}
				}(k)
			}

			result := loader.expandVariables(tt.input, tt.vars)
			if result != tt.expected {
				t.Errorf("expandVariables() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLoadFromFileWithVars(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-config.yaml")

	content := `metadata:
  name: test-template
  version: 1.0.0
  description: Test template
  author: test
  license: MIT
  tags: []
  requires:
    warpgate: '>=1.0.0'
name: test
version: latest
base:
  image: ubuntu:22.04
provisioners:
  - type: ansible
    playbook_path: ${ANSIBLE_PATH}/playbook.yml
    galaxy_file: ${ANSIBLE_PATH}/requirements.yml
targets:
  - type: container
    platforms:
      - linux/amd64
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loader := NewLoader()

	tests := []struct {
		name           string
		vars           map[string]string
		envVars        map[string]string
		expectedPath   string
		expectedGalaxy string
	}{
		{
			name:           "CLI var takes precedence",
			vars:           map[string]string{"ANSIBLE_PATH": "/cli/ansible"},
			envVars:        map[string]string{"ANSIBLE_PATH": "/env/ansible"},
			expectedPath:   "/cli/ansible/playbook.yml",
			expectedGalaxy: "/cli/ansible/requirements.yml",
		},
		{
			name:           "Env var used when no CLI var",
			vars:           nil,
			envVars:        map[string]string{"ANSIBLE_PATH": "/env/ansible"},
			expectedPath:   "/env/ansible/playbook.yml",
			expectedGalaxy: "/env/ansible/requirements.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tt.envVars {
				if err := os.Setenv(k, v); err != nil {
					t.Fatalf("Failed to set env var %s: %v", k, err)
				}
				defer func(key string) {
					if err := os.Unsetenv(key); err != nil {
						t.Logf("Failed to unset env var %s: %v", key, err)
					}
				}(k)
			}

			config, err := loader.LoadFromFileWithVars(testFile, tt.vars)
			if err != nil {
				t.Fatalf("LoadFromFileWithVars() error = %v", err)
			}

			if len(config.Provisioners) == 0 {
				t.Fatal("Expected at least one provisioner")
			}

			provisioner := config.Provisioners[0]
			if provisioner.PlaybookPath != tt.expectedPath {
				t.Errorf("PlaybookPath = %q, want %q", provisioner.PlaybookPath, tt.expectedPath)
			}
			if provisioner.GalaxyFile != tt.expectedGalaxy {
				t.Errorf("GalaxyFile = %q, want %q", provisioner.GalaxyFile, tt.expectedGalaxy)
			}
		})
	}
}

func TestLoadFromFile_BackwardCompatibility(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-config.yaml")

	content := `metadata:
  name: test-template
  version: 1.0.0
  description: Test template
  author: test
  license: MIT
  tags: []
  requires:
    warpgate: '>=1.0.0'
name: test
version: latest
base:
  image: ubuntu:${UBUNTU_VERSION}
provisioners: []
targets:
  - type: container
    platforms:
      - linux/amd64
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set environment variable
	if err := os.Setenv("UBUNTU_VERSION", "22.04"); err != nil {
		t.Fatalf("Failed to set UBUNTU_VERSION: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("UBUNTU_VERSION"); err != nil {
			t.Logf("Failed to unset UBUNTU_VERSION: %v", err)
		}
	}()

	loader := NewLoader()

	// Test that LoadFromFile still works (backward compatibility)
	config, err := loader.LoadFromFile(testFile)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	expectedImage := "ubuntu:22.04"
	if config.Base.Image != expectedImage {
		t.Errorf("Base.Image = %q, want %q", config.Base.Image, expectedImage)
	}
}
