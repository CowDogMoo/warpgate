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

	"github.com/cowdogmoo/warpgate/pkg/builder"
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

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tildeTests := []struct {
		name     string
		input    string
		vars     map[string]string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "Tilde expanded in CLI variable",
			input:    "path: ${REPO_PATH}",
			vars:     map[string]string{"REPO_PATH": "~/ansible-collection"},
			envVars:  nil,
			expected: "path: " + filepath.Join(home, "ansible-collection"),
		},
		{
			name:     "Tilde expanded in environment variable",
			input:    "path: ${REPO_PATH}",
			vars:     nil,
			envVars:  map[string]string{"REPO_PATH": "~/my-repo"},
			expected: "path: " + filepath.Join(home, "my-repo"),
		},
		{
			name:     "Tilde expansion with multiple path segments",
			input:    "path: ${BASE_PATH}/subdir",
			vars:     map[string]string{"BASE_PATH": "~/projects/warpgate"},
			envVars:  nil,
			expected: "path: " + filepath.Join(home, "projects/warpgate") + "/subdir",
		},
		{
			name:     "Non-tilde path unchanged",
			input:    "path: ${ABSOLUTE_PATH}",
			vars:     map[string]string{"ABSOLUTE_PATH": "/opt/data"},
			envVars:  nil,
			expected: "path: /opt/data",
		},
		{
			name:     "Mixed tilde and non-tilde variables",
			input:    "home: ${HOME_PATH}, work: ${WORK_PATH}",
			vars:     map[string]string{"HOME_PATH": "~/personal", "WORK_PATH": "/opt/work"},
			envVars:  nil,
			expected: "home: " + filepath.Join(home, "personal") + ", work: /opt/work",
		},
	}

	tests = append(tests, tildeTests...)

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

	config, err := loader.LoadFromFileWithVars(testFile, nil)
	if err != nil {
		t.Fatalf("LoadFromFileWithVars() error = %v", err)
	}

	expectedImage := "ubuntu:22.04"
	if config.Base.Image != expectedImage {
		t.Errorf("Base.Image = %q, want %q", config.Base.Image, expectedImage)
	}
}

func TestResolveRelativePathsDockerfile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	buildDir := filepath.Join(tmpDir, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("Failed to create build dir: %v", err)
	}

	// Create test files
	dockerfile := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM ubuntu:22.04"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	tests := []struct {
		name               string
		config             builder.Config
		baseDir            string
		expectedDockerfile string
		expectedContext    string
		description        string
	}{
		{
			name: "relative dockerfile paths",
			config: builder.Config{
				Dockerfile: &builder.DockerfileConfig{
					Path:    "build/Dockerfile",
					Context: "build",
				},
			},
			baseDir:            tmpDir,
			expectedDockerfile: filepath.Join(tmpDir, "build", "Dockerfile"),
			expectedContext:    filepath.Join(tmpDir, "build"),
			description:        "Should resolve relative Dockerfile paths to absolute",
		},
		{
			name: "absolute dockerfile paths unchanged",
			config: builder.Config{
				Dockerfile: &builder.DockerfileConfig{
					Path:    dockerfile,
					Context: buildDir,
				},
			},
			baseDir:            tmpDir,
			expectedDockerfile: dockerfile,
			expectedContext:    buildDir,
			description:        "Should not modify absolute paths",
		},
		{
			name: "empty dockerfile context defaults",
			config: builder.Config{
				Dockerfile: &builder.DockerfileConfig{
					Path: "Dockerfile",
				},
			},
			baseDir:            tmpDir,
			expectedDockerfile: filepath.Join(tmpDir, "Dockerfile"),
			expectedContext:    "",
			description:        "Should resolve Dockerfile path even with empty context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader()
			loader.resolveRelativePaths(&tt.config, tt.baseDir)

			if tt.config.Dockerfile != nil {
				if tt.config.Dockerfile.Path != tt.expectedDockerfile {
					t.Errorf("Dockerfile Path = %q, want %q. %s",
						tt.config.Dockerfile.Path, tt.expectedDockerfile, tt.description)
				}
				if tt.expectedContext != "" && tt.config.Dockerfile.Context != tt.expectedContext {
					t.Errorf("Dockerfile Context = %q, want %q. %s",
						tt.config.Dockerfile.Context, tt.expectedContext, tt.description)
				}
			}
		})
	}
}

func TestLoadFromFileWithDockerfile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM ubuntu:22.04"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Create test config file
	testFile := filepath.Join(tmpDir, "config.yaml")
	content := `metadata:
  name: dockerfile-template
  version: 1.0.0
  description: Dockerfile-based template
  author: test
  license: MIT
  tags: []
  requires:
    warpgate: '>=1.0.0'
name: test-app
version: latest
dockerfile:
  path: Dockerfile
  context: .
  target: production
  args:
    VERSION: 1.0.0
    BUILD_DATE: ${BUILD_DATE}
targets:
  - type: container
    platforms:
      - linux/amd64
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set environment variable for build date
	buildDate := "2025-01-15"
	if err := os.Setenv("BUILD_DATE", buildDate); err != nil {
		t.Fatalf("Failed to set BUILD_DATE: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("BUILD_DATE"); err != nil {
			t.Logf("Failed to unset BUILD_DATE: %v", err)
		}
	}()

	loader := NewLoader()
	config, err := loader.LoadFromFileWithVars(testFile, nil)
	if err != nil {
		t.Fatalf("LoadFromFileWithVars() error = %v", err)
	}

	// Verify Dockerfile configuration was loaded
	if config.Dockerfile == nil {
		t.Fatal("Expected Dockerfile config but got nil")
	}

	// Verify paths are resolved to absolute
	if !filepath.IsAbs(config.Dockerfile.Path) {
		t.Errorf("Expected absolute Dockerfile path, got %q", config.Dockerfile.Path)
	}
	if !filepath.IsAbs(config.Dockerfile.Context) {
		t.Errorf("Expected absolute context path, got %q", config.Dockerfile.Context)
	}

	// Verify values
	if config.Dockerfile.Target != "production" {
		t.Errorf("Expected target 'production', got %q", config.Dockerfile.Target)
	}
	if config.Dockerfile.Args["VERSION"] != "1.0.0" {
		t.Errorf("Expected VERSION '1.0.0', got %q", config.Dockerfile.Args["VERSION"])
	}
	if config.Dockerfile.Args["BUILD_DATE"] != buildDate {
		t.Errorf("Expected BUILD_DATE %q, got %q", buildDate, config.Dockerfile.Args["BUILD_DATE"])
	}
}

func TestResolveRelativePathsMixed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	ansibleDir := filepath.Join(tmpDir, "ansible")
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(ansibleDir, 0755); err != nil {
		t.Fatalf("Failed to create ansible dir: %v", err)
	}
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("Failed to create scripts dir: %v", err)
	}

	// Create test files
	playbookPath := filepath.Join(ansibleDir, "playbook.yml")
	scriptPath := filepath.Join(scriptsDir, "setup.sh")
	if err := os.WriteFile(playbookPath, []byte("---\n"), 0644); err != nil {
		t.Fatalf("Failed to create playbook: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\n"), 0644); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	config := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: "ansible/playbook.yml",
			},
			{
				Type:    "script",
				Scripts: []string{"scripts/setup.sh"},
			},
		},
	}

	loader := NewLoader()
	loader.resolveRelativePaths(&config, tmpDir)

	// Verify ansible paths are resolved
	if config.Provisioners[0].PlaybookPath != playbookPath {
		t.Errorf("PlaybookPath = %q, want %q", config.Provisioners[0].PlaybookPath, playbookPath)
	}

	// Verify script paths are resolved
	if config.Provisioners[1].Scripts[0] != scriptPath {
		t.Errorf("Script path = %q, want %q", config.Provisioners[1].Scripts[0], scriptPath)
	}
}
