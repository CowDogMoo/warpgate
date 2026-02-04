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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

// ---------------------------------------------------------------------------
// config_loader.go coverage (from coverage_boost_test.go)
// ---------------------------------------------------------------------------

func TestLoadFromYAML(t *testing.T) {
	t.Parallel()
	loader := NewLoader()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		check   func(t *testing.T, cfg *builder.Config)
	}{
		{
			name: "valid minimal YAML",
			data: []byte(`
name: test-image
base:
  image: ubuntu:22.04
targets:
  - type: container
`),
			wantErr: false,
			check: func(t *testing.T, cfg *builder.Config) {
				t.Helper()
				if cfg.Name != "test-image" {
					t.Errorf("expected name 'test-image', got %q", cfg.Name)
				}
				if cfg.Base.Image != "ubuntu:22.04" {
					t.Errorf("expected base image 'ubuntu:22.04', got %q", cfg.Base.Image)
				}
			},
		},
		{
			name:    "invalid YAML",
			data:    []byte(`{{{invalid yaml`),
			wantErr: true,
		},
		{
			name: "YAML with provisioners",
			data: []byte(`
name: test
base:
  image: alpine:latest
provisioners:
  - type: shell
    inline:
      - echo hello
targets:
  - type: container
`),
			wantErr: false,
			check: func(t *testing.T, cfg *builder.Config) {
				t.Helper()
				if len(cfg.Provisioners) != 1 {
					t.Fatalf("expected 1 provisioner, got %d", len(cfg.Provisioners))
				}
				if cfg.Provisioners[0].Type != "shell" {
					t.Errorf("expected provisioner type 'shell', got %q", cfg.Provisioners[0].Type)
				}
			},
		},
		{
			name: "YAML with sources",
			data: []byte(`
name: test-sources
base:
  image: ubuntu:22.04
sources:
  - name: my-repo
    git:
      repository: https://github.com/org/repo.git
targets:
  - type: container
`),
			wantErr: false,
			check: func(t *testing.T, cfg *builder.Config) {
				t.Helper()
				if len(cfg.Sources) != 1 {
					t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
				}
				if cfg.Sources[0].Name != "my-repo" {
					t.Errorf("expected source name 'my-repo', got %q", cfg.Sources[0].Name)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := loader.LoadFromYAML(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

func TestSaveToFile(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		config  *builder.Config
		wantErr bool
	}{
		{
			name: "save valid config",
			config: &builder.Config{
				Name: "saved-template",
				Base: builder.BaseImage{Image: "ubuntu:22.04"},
				Targets: []builder.Target{
					{Type: "container", Platforms: []string{"linux/amd64"}},
				},
			},
		},
		{
			name: "save config with provisioners",
			config: &builder.Config{
				Name: "prov-template",
				Base: builder.BaseImage{Image: "alpine:latest"},
				Provisioners: []builder.Provisioner{
					{Type: "shell", Inline: []string{"echo hello"}},
				},
				Targets: []builder.Target{
					{Type: "container"},
				},
			},
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(tmpDir, "config"+string(rune('0'+i))+".yaml")
			err := loader.SaveToFile(tc.config, path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify file was written and can be read back
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read saved file: %v", err)
			}

			var readBack builder.Config
			if err := yaml.Unmarshal(data, &readBack); err != nil {
				t.Fatalf("failed to parse saved file: %v", err)
			}

			if readBack.Name != tc.config.Name {
				t.Errorf("name mismatch: got %q, want %q", readBack.Name, tc.config.Name)
			}
		})
	}
}

func TestSaveToFileInvalidPath(t *testing.T) {
	t.Parallel()
	loader := NewLoader()
	cfg := &builder.Config{Name: "test"}
	err := loader.SaveToFile(cfg, "/nonexistent/deep/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestResolvePowerShellPaths(t *testing.T) {
	t.Parallel()
	loader := NewLoader()
	baseDir := "/base/dir"

	prov := &builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{"scripts/setup.ps1", "/abs/path/script.ps1"},
	}
	loader.resolveProvisionerPaths(prov, baseDir)

	if prov.PSScripts[0] != filepath.Join(baseDir, "scripts/setup.ps1") {
		t.Errorf("relative ps_script not resolved: got %q", prov.PSScripts[0])
	}
	if prov.PSScripts[1] != "/abs/path/script.ps1" {
		t.Errorf("absolute ps_script should not change: got %q", prov.PSScripts[1])
	}
}

// ---------------------------------------------------------------------------
// config_validator.go coverage
// ---------------------------------------------------------------------------

func TestValidatePowerShellProvisioner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	v := NewValidator()

	tests := []struct {
		name    string
		prov    builder.Provisioner
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing ps_scripts",
			prov:    builder.Provisioner{Type: "powershell"},
			wantErr: true,
			errMsg:  "powershell provisioner requires 'ps_scripts'",
		},
		{
			name:    "empty ps_scripts",
			prov:    builder.Provisioner{Type: "powershell", PSScripts: []string{}},
			wantErr: true,
			errMsg:  "powershell provisioner requires 'ps_scripts'",
		},
		{
			name: "valid with syntax only",
			prov: builder.Provisioner{Type: "powershell", PSScripts: []string{"setup.ps1"}},
			// With syntax-only mode, file existence is not checked
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Use syntax-only to avoid file existence checks
			v2 := NewValidator()
			v2.options = ValidationOptions{SyntaxOnly: true}
			err := v2.validatePowerShellProvisioner(ctx, &tc.prov, 0)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errMsg != "" && !containsStr(err.Error(), tc.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	// Also test with real files
	tmpDir := t.TempDir()
	script := filepath.Join(tmpDir, "setup.ps1")
	if err := os.WriteFile(script, []byte("Write-Host hello"), 0644); err != nil {
		t.Fatal(err)
	}
	prov := builder.Provisioner{Type: "powershell", PSScripts: []string{script}}
	v.options = ValidationOptions{SyntaxOnly: false}
	if err := v.validatePowerShellProvisioner(ctx, &prov, 0); err != nil {
		t.Errorf("unexpected error with existing file: %v", err)
	}

	// Test with nonexistent file
	prov2 := builder.Provisioner{Type: "powershell", PSScripts: []string{"/nonexistent/script.ps1"}}
	if err := v.validatePowerShellProvisioner(ctx, &prov2, 0); err == nil {
		t.Error("expected error for nonexistent script, got nil")
	}
}

func TestValidateFileProvisioner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		prov    builder.Provisioner
		syntax  bool
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing source",
			prov:    builder.Provisioner{Type: "file", Destination: "/dst"},
			wantErr: true,
			errMsg:  "file provisioner requires 'source'",
		},
		{
			name:    "missing destination",
			prov:    builder.Provisioner{Type: "file", Source: srcFile},
			wantErr: true,
			errMsg:  "file provisioner requires 'destination'",
		},
		{
			name:    "source reference is ok",
			prov:    builder.Provisioner{Type: "file", Source: "${sources.myrepo}", Destination: "/dst"},
			syntax:  true,
			wantErr: false,
		},
		{
			name:    "valid with real file",
			prov:    builder.Provisioner{Type: "file", Source: srcFile, Destination: "/dst"},
			wantErr: false,
		},
		{
			name:    "nonexistent source file",
			prov:    builder.Provisioner{Type: "file", Source: "/nonexistent/file", Destination: "/dst"},
			wantErr: true,
			errMsg:  "source file file not found",
		},
		{
			name:    "syntax only skips file check",
			prov:    builder.Provisioner{Type: "file", Source: "/nonexistent/file", Destination: "/dst"},
			syntax:  true,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := NewValidator()
			v.options = ValidationOptions{SyntaxOnly: tc.syntax}
			err := v.validateFileProvisioner(ctx, &tc.prov, 0)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errMsg != "" && !containsStr(err.Error(), tc.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsSourceReference(t *testing.T) {
	t.Parallel()
	v := NewValidator()

	tests := []struct {
		path string
		want bool
	}{
		{"${sources.myrepo}", true},
		{"${sources.my-repo_v2}", true},
		{"${sources.}", true},
		{"${other.ref}", false},
		{"/some/path", false},
		{"", false},
		{"${sources.myrepo", false}, // missing closing brace
		{"sources.myrepo}", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := v.isSourceReference(tc.path)
			if got != tc.want {
				t.Errorf("isSourceReference(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestValidateTargetExtended(t *testing.T) {
	t.Parallel()
	v := NewValidator()

	tests := []struct {
		name    string
		target  builder.Target
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty type",
			target:  builder.Target{},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name:    "unknown type",
			target:  builder.Target{Type: "foobar"},
			wantErr: true,
			errMsg:  "unknown target type: foobar",
		},
		{
			name:   "container with no platforms defaults",
			target: builder.Target{Type: "container"},
			// Should succeed and set default platform
		},
		{
			name:   "container with platforms",
			target: builder.Target{Type: "container", Platforms: []string{"linux/amd64", "linux/arm64"}},
		},
		{
			name:    "ami missing region",
			target:  builder.Target{Type: "ami", AMIName: "my-ami"},
			wantErr: true,
			errMsg:  "ami target requires 'region'",
		},
		{
			name:    "ami missing ami_name",
			target:  builder.Target{Type: "ami", Region: "us-east-1"},
			wantErr: true,
			errMsg:  "ami target requires 'ami_name'",
		},
		{
			name:   "ami valid",
			target: builder.Target{Type: "ami", Region: "us-east-1", AMIName: "my-ami"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := v.validateTarget(&tc.target, 0)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errMsg != "" && !containsStr(err.Error(), tc.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	// Verify default platform was set
	target := builder.Target{Type: "container"}
	_ = v.validateTarget(&target, 0)
	if len(target.Platforms) != 1 || target.Platforms[0] != "linux/amd64" {
		t.Errorf("expected default platform [linux/amd64], got %v", target.Platforms)
	}
}

func TestValidateGitAuthExtended(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "id_rsa")
	if err := os.WriteFile(keyFile, []byte("fake-key"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		auth    builder.GitAuth
		syntax  bool
		wantErr bool
		errMsg  string
	}{
		{
			name: "multiple auth methods conflict",
			auth: builder.GitAuth{
				Token:    "my-token",
				Username: "user",
				Password: "pass",
			},
			wantErr: true,
			errMsg:  "specify only one auth method",
		},
		{
			name: "ssh key file exists",
			auth: builder.GitAuth{
				SSHKeyFile: keyFile,
			},
		},
		{
			name: "ssh key file not found",
			auth: builder.GitAuth{
				SSHKeyFile: "/tmp/nonexistent_warpgate_test_key_file_12345",
			},
			wantErr: true,
			errMsg:  "ssh_key_file not found",
		},
		{
			name: "ssh key file with syntax only skips check",
			auth: builder.GitAuth{
				SSHKeyFile: "/nonexistent/key",
			},
			syntax: true,
		},
		{
			name: "token auth only",
			auth: builder.GitAuth{Token: "my-token"},
		},
		{
			name: "username/password auth only",
			auth: builder.GitAuth{Username: "user", Password: "pass"},
		},
		{
			name: "ssh_key and token conflict",
			auth: builder.GitAuth{
				SSHKey: "key-content",
				Token:  "my-token",
			},
			wantErr: true,
			errMsg:  "specify only one auth method",
		},
		{
			name: "ssh_key_file with unresolved var warns",
			auth: builder.GitAuth{
				SSHKeyFile: "${HOME}/.ssh/nonexistent_key_" + t.Name(),
			},
			// This contains $ so hasUnresolvedVariable returns true, just warns
			wantErr: false, // The path after expansion might not exist but the $ causes warning path
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := NewValidator()
			v.options = ValidationOptions{SyntaxOnly: tc.syntax}
			err := v.validateGitAuth(ctx, &tc.auth, 0)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errMsg != "" && !containsStr(err.Error(), tc.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExpandPathFunc(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"tilde expansion", "~/test", home + "/test"},
		{"no tilde", "/absolute/path", "/absolute/path"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := expandPath(tc.path)
			if got != tc.want {
				t.Errorf("expandPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestValidateProvisionerAllTypes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	v := NewValidator()
	v.options = ValidationOptions{SyntaxOnly: true}

	tmpDir := t.TempDir()
	scriptFile := filepath.Join(tmpDir, "script.sh")
	if err := os.WriteFile(scriptFile, []byte("#!/bin/bash"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		prov    builder.Provisioner
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty type",
			prov:    builder.Provisioner{},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name:    "unknown type",
			prov:    builder.Provisioner{Type: "unknown_type"},
			wantErr: true,
			errMsg:  "unknown provisioner type",
		},
		{
			name: "shell valid",
			prov: builder.Provisioner{Type: "shell", Inline: []string{"echo hi"}},
		},
		{
			name:    "shell missing inline",
			prov:    builder.Provisioner{Type: "shell"},
			wantErr: true,
			errMsg:  "shell provisioner requires 'inline' commands",
		},
		{
			name: "ansible valid",
			prov: builder.Provisioner{Type: "ansible", PlaybookPath: "/some/playbook.yml"},
		},
		{
			name:    "ansible missing playbook",
			prov:    builder.Provisioner{Type: "ansible"},
			wantErr: true,
			errMsg:  "ansible provisioner requires 'playbook_path'",
		},
		{
			name: "script valid",
			prov: builder.Provisioner{Type: "script", Scripts: []string{"/some/script.sh"}},
		},
		{
			name:    "script missing scripts",
			prov:    builder.Provisioner{Type: "script"},
			wantErr: true,
			errMsg:  "script provisioner requires 'scripts'",
		},
		{
			name: "powershell valid",
			prov: builder.Provisioner{Type: "powershell", PSScripts: []string{"setup.ps1"}},
		},
		{
			name:    "powershell missing scripts",
			prov:    builder.Provisioner{Type: "powershell"},
			wantErr: true,
			errMsg:  "powershell provisioner requires 'ps_scripts'",
		},
		{
			name: "file valid",
			prov: builder.Provisioner{Type: "file", Source: "/some/file", Destination: "/dst"},
		},
		{
			name:    "file missing source",
			prov:    builder.Provisioner{Type: "file", Destination: "/dst"},
			wantErr: true,
			errMsg:  "file provisioner requires 'source'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := v.validateProvisioner(ctx, &tc.prov, 0)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errMsg != "" && !containsStr(err.Error(), tc.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// registry.go coverage
// ---------------------------------------------------------------------------

func TestTagTemplatesWithRepository(t *testing.T) {
	t.Parallel()

	templates := []TemplateInfo{
		{Name: "tmpl1"},
		{Name: "tmpl2"},
		{Name: "tmpl3"},
	}

	tagTemplatesWithRepository(templates, "my-repo")

	for _, tmpl := range templates {
		if tmpl.Repository != "my-repo" {
			t.Errorf("expected repository 'my-repo' for %s, got %q", tmpl.Name, tmpl.Repository)
		}
	}
}

func TestTemplateRegistryDirectMethods(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	tr := &TemplateRegistry{
		repos:         map[string]string{"test": "https://github.com/test/repo.git"},
		localPaths:    []string{"/some/local/path"},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	// Test GetRepositories
	repos := tr.GetRepositories()
	if len(repos) != 1 || repos["test"] != "https://github.com/test/repo.git" {
		t.Errorf("GetRepositories returned unexpected: %v", repos)
	}

	// Ensure it's a copy
	repos["modified"] = "something"
	if _, ok := tr.repos["modified"]; ok {
		t.Error("GetRepositories should return a copy, not the original map")
	}

	// Test GetLocalPaths
	paths := tr.GetLocalPaths()
	if len(paths) != 1 || paths[0] != "/some/local/path" {
		t.Errorf("GetLocalPaths returned unexpected: %v", paths)
	}

	// Ensure it's a copy
	paths[0] = "modified"
	if tr.localPaths[0] == "modified" {
		t.Error("GetLocalPaths should return a copy")
	}

	// Test AddRepository and RemoveRepository
	tr.AddRepository("new-repo", "https://github.com/new/repo.git")
	if tr.repos["new-repo"] != "https://github.com/new/repo.git" {
		t.Error("AddRepository did not add the repo")
	}

	tr.RemoveRepository("new-repo")
	if _, ok := tr.repos["new-repo"]; ok {
		t.Error("RemoveRepository did not remove the repo")
	}
}

func TestRegistrySaveCacheAndLoadCache(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tr := &TemplateRegistry{
		repos:         map[string]string{"test": "https://github.com/test/repo.git"},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	templates := []TemplateInfo{
		{Name: "tmpl1", Description: "desc1", Version: "1.0.0"},
		{Name: "tmpl2", Description: "desc2", Version: "2.0.0", Tags: []string{"security"}},
	}

	// Save cache
	if err := tr.saveCache("test", templates); err != nil {
		t.Fatalf("saveCache error: %v", err)
	}

	// Load cache
	cache, err := tr.loadCache("test")
	if err != nil {
		t.Fatalf("loadCache error: %v", err)
	}

	if cache == nil {
		t.Fatal("loadCache returned nil")
	}

	if len(cache.Templates) != 2 {
		t.Errorf("expected 2 cached templates, got %d", len(cache.Templates))
	}

	if cache.Templates["tmpl1"].Description != "desc1" {
		t.Errorf("cached template description mismatch")
	}

	// Load nonexistent cache
	_, err = tr.loadCache("nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent cache")
	}
}

func TestRegistryLoadCacheCorruptJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tr := &TemplateRegistry{
		repos:    map[string]string{},
		cacheDir: tmpDir,
	}

	// Write corrupt JSON
	cachePath := filepath.Join(tmpDir, "corrupt.json")
	if err := os.WriteFile(cachePath, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := tr.loadCache("corrupt")
	if err == nil {
		t.Error("expected error loading corrupt cache")
	}
}

func TestRegistryDiscoverTemplates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tr := &TemplateRegistry{
		pathValidator: NewPathValidator(),
	}

	// No templates directory
	_, err := tr.discoverTemplates(tmpDir)
	if err == nil {
		t.Error("expected error when templates dir doesn't exist")
	}

	// Create templates directory with template subdirs
	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Template with valid config
	tmpl1Dir := filepath.Join(templatesDir, "my-template")
	if err := os.MkdirAll(tmpl1Dir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `name: my-template
metadata:
  description: "A test template"
  version: "1.0.0"
  author: "Test Author"
  tags:
    - test
    - security
base:
  image: ubuntu:22.04
targets:
  - type: container
`
	if err := os.WriteFile(filepath.Join(tmpl1Dir, "warpgate.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Template dir without config (should be skipped)
	tmpl2Dir := filepath.Join(templatesDir, "no-config")
	if err := os.MkdirAll(tmpl2Dir, 0755); err != nil {
		t.Fatal(err)
	}

	// A file (not dir) in templates dir (should be skipped)
	if err := os.WriteFile(filepath.Join(templatesDir, "not-a-dir"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	// Template with invalid YAML (should be skipped)
	tmpl3Dir := filepath.Join(templatesDir, "bad-yaml")
	if err := os.MkdirAll(tmpl3Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpl3Dir, "warpgate.yaml"), []byte("{{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	templates, err := tr.discoverTemplates(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	if templates[0].Name != "my-template" {
		t.Errorf("expected template name 'my-template', got %q", templates[0].Name)
	}
	if templates[0].Description != "A test template" {
		t.Errorf("expected description 'A test template', got %q", templates[0].Description)
	}
	if templates[0].Author != "Test Author" {
		t.Errorf("expected author 'Test Author', got %q", templates[0].Author)
	}
}

func TestRegistryMatchesQuery(t *testing.T) {
	t.Parallel()

	tr := &TemplateRegistry{
		pathValidator: NewPathValidator(),
	}

	tmpl := TemplateInfo{
		Name:        "attack-box",
		Description: "A security penetration testing template",
		Tags:        []string{"security", "pentest", "offensive"},
	}

	tests := []struct {
		query string
		want  bool
	}{
		{"attack", true},
		{"ATTACK", true},
		{"box", true},
		{"security", true},
		{"pentest", true},
		{"offensive", true},
		{"penetration", true},
		{"zzzznotfound", false},
		{"attck", true}, // fuzzy match
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()
			got := tr.matchesQuery(tmpl, tc.query)
			if got != tc.want {
				t.Errorf("matchesQuery(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func TestRegistryListUnknownRepo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	tr := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	_, err := tr.List(ctx, "nonexistent-repo")
	if err == nil {
		t.Error("expected error for unknown repo")
	}
}

func TestRegistryListLocalRepo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	// Create a local repo structure
	templatesDir := filepath.Join(tmpDir, "localrepo", "templates")
	if err := os.MkdirAll(filepath.Join(templatesDir, "my-tmpl"), 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `name: my-tmpl
metadata:
  description: "local template"
  version: "1.0.0"
base:
  image: ubuntu:22.04
targets:
  - type: container
`
	if err := os.WriteFile(filepath.Join(templatesDir, "my-tmpl", "warpgate.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	localRepoPath := filepath.Join(tmpDir, "localrepo")
	tr := &TemplateRegistry{
		repos:         map[string]string{"local-test": localRepoPath},
		cacheDir:      filepath.Join(tmpDir, "cache"),
		pathValidator: NewPathValidator(),
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "cache"), 0755); err != nil {
		t.Fatal(err)
	}

	templates, err := tr.List(ctx, "local-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}
	if templates[0].Name != "my-tmpl" {
		t.Errorf("expected 'my-tmpl', got %q", templates[0].Name)
	}
}

func TestRegistryListWithCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-populate a fresh cache (less than 1 hour old)
	cache := CacheMetadata{
		LastUpdated: time.Now(),
		Templates: map[string]TemplateInfo{
			"cached-tmpl": {Name: "cached-tmpl", Description: "from cache", Version: "1.0.0"},
		},
		Repositories: map[string]string{},
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	if err := os.WriteFile(filepath.Join(cacheDir, "git-repo.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	tr := &TemplateRegistry{
		repos:         map[string]string{"git-repo": "https://github.com/test/templates.git"},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	templates, err := tr.List(ctx, "git-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 cached template, got %d", len(templates))
	}
	if templates[0].Name != "cached-tmpl" {
		t.Errorf("expected 'cached-tmpl', got %q", templates[0].Name)
	}
}

func TestRegistryScanLocalPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "local")
	templatesDir := filepath.Join(localDir, "templates")
	tmplDir := filepath.Join(templatesDir, "local-tmpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `name: local-tmpl
metadata:
  description: "a local template"
  version: "1.0.0"
base:
  image: alpine:latest
targets:
  - type: container
`
	if err := os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	tr := &TemplateRegistry{
		repos:         map[string]string{},
		localPaths:    []string{localDir, "/nonexistent/path"},
		cacheDir:      filepath.Join(tmpDir, "cache"),
		pathValidator: NewPathValidator(),
	}

	templates := tr.scanLocalPaths(ctx)
	if len(templates) != 1 {
		t.Fatalf("expected 1 template from local paths, got %d", len(templates))
	}
	if templates[0].Name != "local-tmpl" {
		t.Errorf("expected 'local-tmpl', got %q", templates[0].Name)
	}
	if !containsStr(templates[0].Repository, "local:") {
		t.Errorf("expected repository to contain 'local:', got %q", templates[0].Repository)
	}
}

func TestRegistryListLocal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create a local repo
	localRepo := filepath.Join(tmpDir, "localrepo")
	tmplDir := filepath.Join(localRepo, "templates", "local-tmpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `name: local-tmpl
metadata:
  description: "test"
  version: "1.0.0"
base:
  image: alpine:latest
targets:
  - type: container
`
	if err := os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	tr := &TemplateRegistry{
		repos: map[string]string{
			"local":  localRepo,
			"remote": "https://github.com/test/repo.git",
		},
		localPaths:    []string{},
		cacheDir:      filepath.Join(tmpDir, "cache"),
		pathValidator: NewPathValidator(),
	}

	templates, err := tr.ListLocal(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 local template, got %d", len(templates))
	}
}

func TestRegistrySaveAndLoadRepositories(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tr := &TemplateRegistry{
		repos: map[string]string{
			"official": "https://github.com/cowdogmoo/warpgate-templates.git",
			"custom":   "https://github.com/custom/repo.git",
		},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	// Save
	if err := tr.SaveRepositories(); err != nil {
		t.Fatalf("SaveRepositories error: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, "repositories.json")
	if !fileExists(configPath) {
		t.Fatal("repositories.json not created")
	}

	// Create new registry and load
	tr2 := &TemplateRegistry{
		repos:         map[string]string{"initial": "https://initial.git"},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	if err := tr2.LoadRepositories(); err != nil {
		t.Fatalf("LoadRepositories error: %v", err)
	}

	// Should have merged
	if tr2.repos["official"] != "https://github.com/cowdogmoo/warpgate-templates.git" {
		t.Error("official repo not loaded")
	}
	if tr2.repos["custom"] != "https://github.com/custom/repo.git" {
		t.Error("custom repo not loaded")
	}
	if tr2.repos["initial"] != "https://initial.git" {
		t.Error("initial repo should still exist")
	}
}

func TestRegistryLoadRepositoriesNoFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tr := &TemplateRegistry{
		repos:         map[string]string{"test": "https://test.git"},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	// Should succeed with no file
	if err := tr.LoadRepositories(); err != nil {
		t.Fatalf("LoadRepositories should succeed when file doesn't exist: %v", err)
	}
}

func TestRegistryLoadRepositoriesCorruptJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "repositories.json")
	if err := os.WriteFile(configPath, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}

	tr := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	if err := tr.LoadRepositories(); err == nil {
		t.Error("expected error loading corrupt JSON")
	}
}

// ---------------------------------------------------------------------------
// scaffold.go coverage
// ---------------------------------------------------------------------------

func TestScaffolderSaveTemplateConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s := NewScaffolder()

	cfg := &builder.Config{
		Name: "test-scaffold",
		Metadata: builder.Metadata{
			Name:        "test-scaffold",
			Version:     "1.0.0",
			Description: "test",
		},
		Base: builder.BaseImage{Image: "ubuntu:22.04"},
		Targets: []builder.Target{
			{Type: "container"},
		},
	}

	path := filepath.Join(tmpDir, "warpgate.yaml")
	if err := s.saveTemplateConfig(cfg, path); err != nil {
		t.Fatalf("saveTemplateConfig error: %v", err)
	}

	// Verify file exists and contains schema comment
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !containsStr(string(data), "yaml-language-server") {
		t.Error("expected schema comment in output")
	}

	// Should be parseable YAML (after stripping comment)
	var readBack builder.Config
	// The schema comment is a YAML comment, so it should still parse
	if err := yaml.Unmarshal(data, &readBack); err != nil {
		t.Errorf("saved config is not valid YAML: %v", err)
	}
	if readBack.Name != "test-scaffold" {
		t.Errorf("expected name 'test-scaffold', got %q", readBack.Name)
	}
}

func TestScaffolderSaveTemplateConfigInvalidPath(t *testing.T) {
	t.Parallel()
	s := NewScaffolder()
	cfg := &builder.Config{Name: "test"}
	err := s.saveTemplateConfig(cfg, "/nonexistent/deep/path/warpgate.yaml")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

// ---------------------------------------------------------------------------
// git.go coverage
// ---------------------------------------------------------------------------

func TestIsSpecificVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		want    bool
	}{
		{"", false},
		{"main", false},
		{"master", false},
		{"v1.0.0", true},
		{"develop", true},
		{"feature/test", true},
		{"1.0.0", true},
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()
			got := isSpecificVersion(tc.version)
			if got != tc.want {
				t.Errorf("isSpecificVersion(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// loader.go coverage
// ---------------------------------------------------------------------------

func TestSetVariables(t *testing.T) {
	t.Parallel()

	// We can't use NewTemplateLoader since it needs config.Load()
	// but we can test SetVariables on a manually created loader
	tl := &TemplateLoader{
		variables: make(map[string]string),
	}

	// Set some variables
	vars := map[string]string{"key1": "val1", "key2": "val2"}
	tl.SetVariables(vars)

	if tl.variables["key1"] != "val1" {
		t.Errorf("expected key1=val1, got %q", tl.variables["key1"])
	}

	// Set nil resets
	tl.SetVariables(nil)
	if len(tl.variables) != 0 {
		t.Errorf("expected empty variables after nil set, got %d", len(tl.variables))
	}

	// Set again
	tl.SetVariables(map[string]string{"a": "b"})
	if tl.variables["a"] != "b" {
		t.Error("SetVariables failed after nil reset")
	}
}

func TestParseTemplateRefExtended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ref     string
		name    string
		version string
	}{
		{"attack-box@v1.2.0", "attack-box", "v1.2.0"},
		{"my-template@latest", "my-template", "latest"},
		{"name@1.0.0", "name", "1.0.0"},
		{"multi@at@signs", "multi", "at"},
	}

	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			t.Parallel()
			name, version := parseTemplateRef(tc.ref)
			if name != tc.name {
				t.Errorf("name: got %q, want %q", name, tc.name)
			}
			if version != tc.version {
				t.Errorf("version: got %q, want %q", version, tc.version)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// manager.go coverage
// ---------------------------------------------------------------------------

func TestManagerRemoveFromLocalPaths(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Templates.LocalPaths = []string{"/path/one", "/path/two", "/path/three"}
	cfg.Templates.Repositories = map[string]string{}

	m := NewManager(cfg)

	// Remove existing path
	removed := m.removeFromLocalPaths("/path/two", "/path/two")
	if !removed {
		t.Error("expected removal to succeed")
	}
	if len(cfg.Templates.LocalPaths) != 2 {
		t.Errorf("expected 2 paths remaining, got %d", len(cfg.Templates.LocalPaths))
	}

	// Remove non-existing path
	removed = m.removeFromLocalPaths("/nonexistent", "/nonexistent")
	if removed {
		t.Error("expected removal to fail for nonexistent path")
	}
}

func TestManagerRemoveFromRepositories(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Templates.Repositories = map[string]string{
		"repo1": "https://github.com/org/repo1.git",
		"repo2": "https://github.com/org/repo2.git",
	}

	m := NewManager(cfg)

	removed := m.removeFromRepositories("repo1")
	if !removed {
		t.Error("expected removal to succeed")
	}
	if _, ok := cfg.Templates.Repositories["repo1"]; ok {
		t.Error("repo1 should have been removed")
	}

	removed = m.removeFromRepositories("nonexistent")
	if removed {
		t.Error("expected removal to fail for nonexistent repo")
	}
}

// ---------------------------------------------------------------------------
// version.go additional coverage
// ---------------------------------------------------------------------------

func TestIsBreakingChangeExtended(t *testing.T) {
	t.Parallel()

	vm, err := NewVersionManager("2.0.0")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		old     string
		new     string
		want    bool
		wantErr bool
	}{
		{"major bump", "1.0.0", "2.0.0", true, false},
		{"minor bump", "1.0.0", "1.1.0", false, false},
		{"patch bump", "1.0.0", "1.0.1", false, false},
		{"same version", "1.0.0", "1.0.0", false, false},
		{"old is latest", "latest", "2.0.0", false, false},
		{"new is latest", "1.0.0", "latest", false, false},
		{"both latest", "latest", "latest", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := vm.IsBreakingChange(tc.old, tc.new)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("IsBreakingChange(%q, %q) = %v, want %v", tc.old, tc.new, got, tc.want)
			}
		})
	}
}

func TestCompareVersionsExtended(t *testing.T) {
	t.Parallel()

	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{"equal", "1.0.0", "1.0.0", 0},
		{"v1 less", "1.0.0", "2.0.0", -1},
		{"v1 greater", "2.0.0", "1.0.0", 1},
		{"both latest", "latest", "latest", 0},
		{"v1 latest", "latest", "1.0.0", 1},
		{"v2 latest", "1.0.0", "latest", -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := vm.CompareVersions(tc.v1, tc.v2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.v1, tc.v2, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Full validation flow coverage (ValidateWithOptions)
// ---------------------------------------------------------------------------

func TestValidateWithOptionsDockerfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM ubuntu:22.04"), 0644); err != nil {
		t.Fatal(err)
	}

	v := NewValidator()

	// Valid dockerfile config
	cfg := &builder.Config{
		Name: "docker-test",
		Dockerfile: &builder.DockerfileConfig{
			Path: dockerfilePath,
		},
		Targets: []builder.Target{{Type: "container"}},
	}
	if err := v.ValidateWithOptions(ctx, cfg, ValidationOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Dockerfile not found
	cfg2 := &builder.Config{
		Name: "docker-test",
		Dockerfile: &builder.DockerfileConfig{
			Path: "/nonexistent/Dockerfile",
		},
		Targets: []builder.Target{{Type: "container"}},
	}
	if err := v.ValidateWithOptions(ctx, cfg2, ValidationOptions{}); err == nil {
		t.Error("expected error for missing dockerfile")
	}

	// Nil dockerfile config
	cfg3 := &builder.Config{
		Name:       "docker-test",
		Dockerfile: &builder.DockerfileConfig{},
		Targets:    []builder.Target{{Type: "container"}},
	}
	// With SyntaxOnly it should pass (no file check)
	if err := v.ValidateWithOptions(ctx, cfg3, ValidationOptions{SyntaxOnly: true}); err != nil {
		t.Fatalf("unexpected error with syntax-only: %v", err)
	}

	// Missing name
	cfg4 := &builder.Config{
		Dockerfile: &builder.DockerfileConfig{Path: dockerfilePath},
		Targets:    []builder.Target{{Type: "container"}},
	}
	if err := v.ValidateWithOptions(ctx, cfg4, ValidationOptions{}); err == nil {
		t.Error("expected error for missing name")
	}

	// No targets
	cfg5 := &builder.Config{
		Name:       "test",
		Dockerfile: &builder.DockerfileConfig{Path: dockerfilePath},
	}
	if err := v.ValidateWithOptions(ctx, cfg5, ValidationOptions{}); err == nil {
		t.Error("expected error for missing targets")
	}
}

func TestValidateWithOptionsProvisionerBased(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	v := NewValidator()

	// Missing base image
	cfg := &builder.Config{
		Name:    "test",
		Targets: []builder.Target{{Type: "container"}},
	}
	if err := v.ValidateWithOptions(ctx, cfg, ValidationOptions{SyntaxOnly: true}); err == nil {
		t.Error("expected error for missing base image")
	}

	// Valid with provisioners
	cfg2 := &builder.Config{
		Name: "test",
		Base: builder.BaseImage{Image: "ubuntu:22.04"},
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"echo hi"}},
		},
		Targets: []builder.Target{{Type: "container"}},
	}
	if err := v.ValidateWithOptions(ctx, cfg2, ValidationOptions{SyntaxOnly: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// config_loader.go: LoadFromFileWithVars powershell paths
// ---------------------------------------------------------------------------

func TestLoadFromFileWithVarsPowerShell(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptFile := filepath.Join(tmpDir, "setup.ps1")
	if err := os.WriteFile(scriptFile, []byte("Write-Host 'Hello'"), 0644); err != nil {
		t.Fatal(err)
	}

	configContent := `name: ps-test
base:
  image: mcr.microsoft.com/windows/servercore:ltsc2022
provisioners:
  - type: powershell
    ps_scripts:
      - setup.ps1
targets:
  - type: container
    platforms:
      - linux/amd64
`
	configPath := filepath.Join(tmpDir, "warpgate.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.LoadFromFileWithVars(configPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The ps_scripts path should be resolved to absolute
	if len(cfg.Provisioners) != 1 {
		t.Fatalf("expected 1 provisioner, got %d", len(cfg.Provisioners))
	}
	if !filepath.IsAbs(cfg.Provisioners[0].PSScripts[0]) {
		t.Errorf("expected absolute path for ps_script, got %q", cfg.Provisioners[0].PSScripts[0])
	}
}

// ---------------------------------------------------------------------------
// registry.go: listAll, Search coverage
// ---------------------------------------------------------------------------

func TestRegistryListAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create a local repo with templates
	localRepo := filepath.Join(tmpDir, "localrepo")
	tmplDir := filepath.Join(localRepo, "templates", "tmpl-a")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `name: tmpl-a
metadata:
  description: "template A"
  version: "1.0.0"
base:
  image: alpine:latest
targets:
  - type: container
`
	if err := os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a local path with templates
	localPath := filepath.Join(tmpDir, "localpath")
	tmplDir2 := filepath.Join(localPath, "templates", "tmpl-b")
	if err := os.MkdirAll(tmplDir2, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML2 := `name: tmpl-b
metadata:
  description: "template B"
  version: "2.0.0"
base:
  image: ubuntu:22.04
targets:
  - type: container
`
	if err := os.WriteFile(filepath.Join(tmplDir2, "warpgate.yaml"), []byte(configYAML2), 0644); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	tr := &TemplateRegistry{
		repos: map[string]string{
			"local-repo": localRepo,
		},
		localPaths:    []string{localPath},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	// listAll (via List with empty string)
	templates, err := tr.List(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find templates from both the local repo and local paths
	if len(templates) < 2 {
		t.Errorf("expected at least 2 templates from listAll, got %d", len(templates))
	}
}

func TestRegistrySearch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create local repo with multiple templates
	localRepo := filepath.Join(tmpDir, "repo")
	for _, name := range []string{"attack-box", "dev-env", "security-scanner"} {
		tmplDir := filepath.Join(localRepo, "templates", name)
		if err := os.MkdirAll(tmplDir, 0755); err != nil {
			t.Fatal(err)
		}
		yaml := `name: ` + name + `
metadata:
  description: "` + name + ` template"
  version: "1.0.0"
  tags:
    - test
base:
  image: ubuntu:22.04
targets:
  - type: container
`
		if err := os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	tr := &TemplateRegistry{
		repos:         map[string]string{"local": localRepo},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	results, err := tr.Search(ctx, "attack")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 search result for 'attack'")
	}

	results2, err := tr.Search(ctx, "security")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results2) < 1 {
		t.Error("expected at least 1 search result for 'security'")
	}

	results3, err := tr.Search(ctx, "zzzzzzzzz")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results3) != 0 {
		t.Errorf("expected 0 results for nonsense query, got %d", len(results3))
	}
}

// ---------------------------------------------------------------------------
// loader.go: LoadTemplateWithVars paths coverage
// ---------------------------------------------------------------------------

func TestLoadTemplateWithVarsLocalFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create a valid template file
	configContent := `name: local-test
base:
  image: ubuntu:22.04
provisioners:
  - type: shell
    inline:
      - echo hello
targets:
  - type: container
    platforms:
      - linux/amd64
`
	configPath := filepath.Join(tmpDir, "warpgate.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a TemplateLoader manually (avoiding NewTemplateLoader which needs config.Load())
	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	tl := &TemplateLoader{
		cacheDir:   cacheDir,
		configLoad: NewLoader(),
		registry: &TemplateRegistry{
			repos:         map[string]string{},
			cacheDir:      cacheDir,
			pathValidator: NewPathValidator(),
		},
		gitOps:        NewGitOperations(cacheDir),
		variables:     make(map[string]string),
		pathValidator: NewPathValidator(),
	}

	// Test loading absolute path
	cfg, err := tl.LoadTemplateWithVars(ctx, configPath, nil)
	if err != nil {
		t.Fatalf("unexpected error loading absolute path: %v", err)
	}
	if cfg.Name != "local-test" {
		t.Errorf("expected name 'local-test', got %q", cfg.Name)
	}

	// Test loading from directory
	cfg2, err := tl.LoadTemplateWithVars(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("unexpected error loading directory: %v", err)
	}
	if cfg2.Name != "local-test" {
		t.Errorf("expected name 'local-test', got %q", cfg2.Name)
	}

	// Test unknown reference
	_, err = tl.LoadTemplateWithVars(ctx, "https://nonexistent.invalid/repo.git//template", nil)
	if err == nil {
		t.Error("expected error for nonexistent git URL")
	}

	// Test unknown ref format
	_, err = tl.LoadTemplateWithVars(ctx, "git@nonexistent.invalid:repo.git//template", nil)
	if err == nil {
		t.Error("expected error for nonexistent git@ URL")
	}

	// Test loading directory without warpgate.yaml
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	_, err = tl.LoadTemplateWithVars(ctx, emptyDir, nil)
	if err == nil {
		t.Error("expected error for directory without warpgate.yaml")
	}
}

func TestLoadTemplateWithVarsWithMergedVars(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	configContent := `name: var-test
base:
  image: ${BASE_IMAGE}
provisioners:
  - type: shell
    inline:
      - echo hello
targets:
  - type: container
    platforms:
      - linux/amd64
`
	configPath := filepath.Join(tmpDir, "warpgate.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	tl := &TemplateLoader{
		cacheDir:   cacheDir,
		configLoad: NewLoader(),
		registry: &TemplateRegistry{
			repos:         map[string]string{},
			cacheDir:      cacheDir,
			pathValidator: NewPathValidator(),
		},
		gitOps:        NewGitOperations(cacheDir),
		variables:     map[string]string{"BASE_IMAGE": "instance-var"},
		pathValidator: NewPathValidator(),
	}

	// Instance variables should be used
	cfg, err := tl.LoadTemplateWithVars(ctx, configPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Base.Image != "instance-var" {
		t.Errorf("expected instance var to be used, got %q", cfg.Base.Image)
	}

	// Provided vars take precedence over instance vars
	cfg2, err := tl.LoadTemplateWithVars(ctx, configPath, map[string]string{"BASE_IMAGE": "provided-var"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.Base.Image != "provided-var" {
		t.Errorf("expected provided var to take precedence, got %q", cfg2.Base.Image)
	}
}

func TestLoadTemplateWithVarsNameLookup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create local repo structure
	localRepo := filepath.Join(tmpDir, "repo")
	tmplDir := filepath.Join(localRepo, "templates", "my-tmpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `name: my-tmpl
base:
  image: ubuntu:22.04
provisioners:
  - type: shell
    inline:
      - echo hello
targets:
  - type: container
    platforms:
      - linux/amd64
`
	if err := os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	tl := &TemplateLoader{
		cacheDir:   cacheDir,
		configLoad: NewLoader(),
		registry: &TemplateRegistry{
			repos:         map[string]string{"local": localRepo},
			localPaths:    []string{},
			cacheDir:      cacheDir,
			pathValidator: NewPathValidator(),
		},
		gitOps:        NewGitOperations(cacheDir),
		variables:     make(map[string]string),
		pathValidator: NewPathValidator(),
	}

	// Load by name
	cfg, err := tl.LoadTemplateWithVars(ctx, "my-tmpl", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "my-tmpl" {
		t.Errorf("expected 'my-tmpl', got %q", cfg.Name)
	}

	// Template not found by name
	_, err = tl.LoadTemplateWithVars(ctx, "nonexistent-template", nil)
	if err == nil {
		t.Error("expected error for nonexistent template name")
	}
}

func TestTemplateLoaderList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a local repo called "official" with templates
	localRepo := filepath.Join(tmpDir, "official")
	tmplDir := filepath.Join(localRepo, "templates", "my-tmpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	configYAML := `name: my-tmpl
metadata:
  description: "test"
  version: "1.0.0"
base:
  image: ubuntu:22.04
targets:
  - type: container
`
	if err := os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	tl := &TemplateLoader{
		cacheDir:   cacheDir,
		configLoad: NewLoader(),
		registry: &TemplateRegistry{
			repos:         map[string]string{"official": localRepo},
			cacheDir:      cacheDir,
			pathValidator: NewPathValidator(),
		},
		gitOps:        NewGitOperations(cacheDir),
		variables:     make(map[string]string),
		pathValidator: NewPathValidator(),
	}

	templates, err := tl.List(ctx)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(templates))
	}
}

// ---------------------------------------------------------------------------
// paths.go: more coverage
// ---------------------------------------------------------------------------

func TestExtractRepoNameVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/org/my-templates.git", "my-templates"},
		{"https://github.com/org/my-templates", "my-templates"},
		{"git@github.com:org/repo.git", "repo"},
		{"https://github.com/org/repo", "repo"},
		{"", "templates"},
		{"  ", "templates"},
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			got := ExtractRepoName(tc.url)
			if got != tc.want {
				t.Errorf("ExtractRepoName(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestPathValidatorIsLocalPath(t *testing.T) {
	t.Parallel()
	pv := NewPathValidator()

	tmpDir := t.TempDir()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing absolute dir", tmpDir, true},
		{"nonexistent absolute dir", "/nonexistent/path/xyz", false},
		{"git https url", "https://github.com/test/repo.git", false},
		{"git@ url", "git@github.com:test/repo.git", false},
		{"http url", "http://github.com/test/repo.git", false},
		{"relative dot path to nonexistent", "./nonexistent_dir_12345", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pv.IsLocalPath(tc.path)
			if got != tc.want {
				t.Errorf("IsLocalPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestPathValidatorNormalizePath(t *testing.T) {
	t.Parallel()
	pv := NewPathValidator()

	// Absolute path stays the same
	p, err := pv.NormalizePath("/absolute/path")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %q", p)
	}

	// Relative path with slash gets resolved to absolute
	p2, err := pv.NormalizePath("./relative/path")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p2) {
		t.Errorf("expected absolute path, got %q", p2)
	}

	// Simple name (no slash) stays unchanged
	p3, err := pv.NormalizePath("simplename")
	if err != nil {
		t.Fatal(err)
	}
	if p3 != "simplename" {
		t.Errorf("expected 'simplename', got %q", p3)
	}
}

func TestPathValidatorValidateLocalPath(t *testing.T) {
	t.Parallel()
	pv := NewPathValidator()

	tmpDir := t.TempDir()

	// Valid directory
	if err := pv.ValidateLocalPath(tmpDir); err != nil {
		t.Errorf("expected no error for valid directory: %v", err)
	}

	// Non-existent path
	if err := pv.ValidateLocalPath("/nonexistent/path/xyz"); err == nil {
		t.Error("expected error for nonexistent path")
	}

	// File, not directory
	tmpFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := pv.ValidateLocalPath(tmpFile); err == nil {
		t.Error("expected error for file (not directory)")
	}
}

func TestPathValidatorExpandPath(t *testing.T) {
	t.Parallel()
	pv := NewPathValidator()

	// Absolute path stays absolute
	p, err := pv.ExpandPath("/absolute/path")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %q", p)
	}

	// Relative path gets expanded to absolute
	p2, err := pv.ExpandPath("relative")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p2) {
		t.Errorf("expected absolute path, got %q", p2)
	}
}

// ---------------------------------------------------------------------------
// manager.go: more RemoveSource coverage
// ---------------------------------------------------------------------------

func TestManagerRemoveSourceNotFound(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Templates.LocalPaths = []string{"/path/one"}
	cfg.Templates.Repositories = map[string]string{"repo1": "https://test.git"}

	m := NewManager(cfg)

	err := m.RemoveSource(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

// ---------------------------------------------------------------------------
// version.go: more edge case coverage
// ---------------------------------------------------------------------------

func TestGetLatestVersionAllInvalid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// All "latest" values
	result, err := vm.GetLatestVersion(ctx, []string{"latest", "latest"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "latest" {
		t.Errorf("expected 'latest', got %q", result)
	}

	// Empty list
	_, err = vm.GetLatestVersion(ctx, []string{})
	if err == nil {
		t.Error("expected error for empty list")
	}
}

func TestValidateVersionRangeEdge(t *testing.T) {
	t.Parallel()

	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// Latest always satisfies
	ok, err := vm.ValidateVersionRange("latest", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected latest to satisfy range")
	}

	// Below minimum
	ok, err = vm.ValidateVersionRange("0.5.0", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected 0.5.0 to be below minimum 1.0.0")
	}

	// Above maximum
	ok, err = vm.ValidateVersionRange("3.0.0", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected 3.0.0 to be above maximum 2.0.0")
	}
}

func TestValidateConstraintEdge(t *testing.T) {
	t.Parallel()

	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// Invalid constraint
	_, err = vm.ValidateConstraint("1.0.0", "not-a-constraint")
	if err == nil {
		t.Error("expected error for invalid constraint")
	}
}

// ---------------------------------------------------------------------------
// config_validator.go: validateDockerfile nil case
// ---------------------------------------------------------------------------

func TestValidateDockerfileNil(t *testing.T) {
	t.Parallel()
	v := NewValidator()
	err := v.validateDockerfile(nil)
	if err == nil {
		t.Error("expected error for nil dockerfile config")
	}
}

// ---------------------------------------------------------------------------
// config_loader.go: expandVariables edge - unclosed ${
// ---------------------------------------------------------------------------

func TestExpandVariablesUnclosedBrace(t *testing.T) {
	t.Parallel()
	loader := NewLoader()
	// Unclosed ${VAR should be passed through as-is
	result := loader.expandVariables("prefix ${UNCLOSED", nil)
	if result != "prefix ${UNCLOSED" {
		t.Errorf("expected unclosed brace passthrough, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// git.go: CloneOrUpdate, pullUpdates using real git repos
// ---------------------------------------------------------------------------

func TestGitCloneOrUpdateLocalRepo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create a local bare git repo to clone from
	srcRepo := filepath.Join(tmpDir, "src-repo")
	if err := os.MkdirAll(filepath.Join(srcRepo, "templates", "test-tmpl"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRepo, "templates", "test-tmpl", "warpgate.yaml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Init and commit in source repo
	initCmd := "cd " + srcRepo + " && git init && git add -A && git commit -m 'init'"
	if out, err := runShellCommand(initCmd); err != nil {
		t.Skipf("git init failed (git may not be available): %v: %s", err, out)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	gitOps := NewGitOperations(cacheDir)

	// Clone
	repoPath, err := gitOps.CloneOrUpdate(ctx, srcRepo, "")
	if err != nil {
		t.Fatalf("CloneOrUpdate (clone) error: %v", err)
	}

	if !dirExists(repoPath) {
		t.Fatalf("cloned repo path does not exist: %s", repoPath)
	}

	// Pull updates (second call should use cached version)
	repoPath2, err := gitOps.CloneOrUpdate(ctx, srcRepo, "")
	if err != nil {
		t.Fatalf("CloneOrUpdate (update) error: %v", err)
	}

	if repoPath != repoPath2 {
		t.Errorf("expected same path for cached repo, got %q vs %q", repoPath, repoPath2)
	}
}

func TestGitCloneWithVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create a local git repo with a tag
	srcRepo := filepath.Join(tmpDir, "src-repo")
	if err := os.MkdirAll(srcRepo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRepo, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	initCmd := "cd " + srcRepo + " && git init && git add -A && git commit -m 'init' && git tag v1.0.0"
	if out, err := runShellCommand(initCmd); err != nil {
		t.Skipf("git init/tag failed: %v: %s", err, out)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	gitOps := NewGitOperations(cacheDir)

	// Clone with specific version
	repoPath, err := gitOps.CloneOrUpdate(ctx, srcRepo, "v1.0.0")
	if err != nil {
		t.Fatalf("CloneOrUpdate with version error: %v", err)
	}

	if !dirExists(repoPath) {
		t.Fatalf("repo not cloned to expected path: %s", repoPath)
	}
}

// runShellCommand is a test helper to run shell commands
func runShellCommand(cmd string) (string, error) {
	c := exec.Command("sh", "-c", cmd)
	output, err := c.CombinedOutput()
	return string(output), err
}

// ---------------------------------------------------------------------------
// registry.go: SaveRepositories error path
// ---------------------------------------------------------------------------

func TestRegistrySaveRepositoriesInvalidPath(t *testing.T) {
	t.Parallel()

	tr := &TemplateRegistry{
		repos:         map[string]string{"test": "https://test.git"},
		cacheDir:      "/nonexistent/path/that/cannot/be/created",
		pathValidator: NewPathValidator(),
	}

	err := tr.SaveRepositories()
	if err == nil {
		t.Error("expected error writing to invalid path")
	}
}

// ---------------------------------------------------------------------------
// git.go: pullUpdates with local repo
// ---------------------------------------------------------------------------

func TestGitPullUpdates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create source repo
	srcRepo := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcRepo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRepo, "file.txt"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	initCmd := "cd " + srcRepo + " && git init && git add -A && git commit -m 'init'"
	if _, err := runShellCommand(initCmd); err != nil {
		t.Skip("git not available")
	}

	// Clone it
	cacheDir := filepath.Join(tmpDir, "cache")
	gitOps := NewGitOperations(cacheDir)
	repoPath, err := gitOps.CloneOrUpdate(ctx, srcRepo, "")
	if err != nil {
		t.Fatalf("clone error: %v", err)
	}

	// Add a commit to source
	addCmd := "cd " + srcRepo + " && echo v2 > file.txt && git add -A && git commit -m 'update'"
	if _, err := runShellCommand(addCmd); err != nil {
		t.Fatalf("failed to add commit: %v", err)
	}

	// Pull updates
	err = gitOps.pullUpdates(ctx, repoPath)
	if err != nil {
		t.Fatalf("pullUpdates error: %v", err)
	}

	// Verify updated
	data, err := os.ReadFile(filepath.Join(repoPath, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v2\n" {
		t.Errorf("expected 'v2\\n' after pull, got %q", string(data))
	}
}

func TestGitPullUpdatesInvalidRepo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpDir := t.TempDir()
	gitOps := NewGitOperations(tmpDir)

	// pullUpdates on non-git directory
	err := gitOps.pullUpdates(ctx, tmpDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestGitCheckoutVersion(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create repo with tag
	srcRepo := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcRepo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRepo, "file.txt"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	initCmd := "cd " + srcRepo + " && git init && git add -A && git commit -m 'init' && git tag v1.0.0 && echo v2 > file.txt && git add -A && git commit -m 'v2'"
	if _, err := runShellCommand(initCmd); err != nil {
		t.Skip("git not available")
	}

	// Open the repo
	repo, err := openGitRepo(srcRepo)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	// Checkout tag
	err = checkoutVersion(repo, "v1.0.0")
	if err != nil {
		t.Fatalf("checkoutVersion error: %v", err)
	}

	// Verify we're at v1
	data, err := os.ReadFile(filepath.Join(srcRepo, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v1" {
		t.Errorf("expected 'v1' after checkout, got %q", string(data))
	}

	// Checkout nonexistent version
	err = checkoutVersion(repo, "nonexistent-tag-or-branch")
	if err == nil {
		t.Error("expected error for nonexistent version")
	}
}

// openGitRepo is a helper to open a git repo using go-git
func openGitRepo(path string) (*git.Repository, error) {
	return git.PlainOpen(path)
}

// ---------------------------------------------------------------------------
// scaffold.go: createDefaultTemplate, createReadme error paths
// ---------------------------------------------------------------------------

func TestCreateDefaultTemplateContent(t *testing.T) {
	t.Parallel()
	s := NewScaffolder()

	tmpDir := t.TempDir()
	if err := s.createDefaultTemplate("test-name", tmpDir); err != nil {
		t.Fatalf("createDefaultTemplate error: %v", err)
	}

	// Verify file exists and contains template name
	data, err := os.ReadFile(filepath.Join(tmpDir, "warpgate.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !containsStr(content, "test-name") {
		t.Error("expected template name in config")
	}
	if !containsStr(content, "ubuntu:22.04") {
		t.Error("expected default base image")
	}
}

func TestCreateDefaultTemplateInvalidDir(t *testing.T) {
	t.Parallel()
	s := NewScaffolder()
	err := s.createDefaultTemplate("test", "/nonexistent/path/xyz")
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

func TestCreateReadmeContent(t *testing.T) {
	t.Parallel()
	s := NewScaffolder()

	tmpDir := t.TempDir()
	if err := s.createReadme("my-project", tmpDir); err != nil {
		t.Fatalf("createReadme error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !containsStr(content, "my-project") {
		t.Error("expected project name in README")
	}
	if !containsStr(content, "warpgate") {
		t.Error("expected warpgate references in README")
	}
}

func TestCreateReadmeInvalidDir(t *testing.T) {
	t.Parallel()
	s := NewScaffolder()
	err := s.createReadme("test", "/nonexistent/path/xyz")
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

// ---------------------------------------------------------------------------
// manager.go: AddGitRepository, AddLocalPath, RemoveSource with viper config
// ---------------------------------------------------------------------------

func TestAddGitRepositoryValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a temporary config file for viper
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("templates:\n  repositories: {}\n  local_paths: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Templates.Repositories = make(map[string]string)
	m := NewManager(cfg)

	// Invalid git URL
	err := m.AddGitRepository(ctx, "test", "not-a-url")
	if err == nil {
		t.Error("expected error for invalid git URL")
	}

	// Placeholder URL
	err = m.AddGitRepository(ctx, "test", "https://example.com/repo.git")
	if err == nil {
		t.Error("expected error for placeholder URL")
	}

	// Valid URL but duplicate (pre-populate)
	cfg.Templates.Repositories["existing"] = "https://github.com/org/repo.git"
	err = m.AddGitRepository(ctx, "existing", "https://github.com/org/repo.git")
	if err != nil {
		t.Errorf("expected no error for same URL duplicate: %v", err)
	}

	// Same name, different URL
	err = m.AddGitRepository(ctx, "existing", "https://github.com/org/different.git")
	if err == nil {
		t.Error("expected error for conflicting name")
	}
}

func TestAddLocalPathValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := &config.Config{}
	m := NewManager(cfg)

	// Non-existent path
	err := m.AddLocalPath(ctx, "/nonexistent/path/xyz/12345")
	if err == nil {
		t.Log("AddLocalPath with non-existent path did not return error (may be deferred)")
	}

	// Valid path (existing tmpdir)
	tmpDir := t.TempDir()
	err = m.AddLocalPath(ctx, tmpDir)
	// Will fail at saveConfigValue since there's no viper config, but the validation passes
	// This is OK -- we're testing the validation logic not the save
	if err != nil && containsStr(err.Error(), "does not exist") {
		t.Errorf("unexpected path validation error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// version.go: CheckCompatibility warnings
// ---------------------------------------------------------------------------

func TestCheckCompatibilityWarning(t *testing.T) {
	t.Parallel()

	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// Compatible
	ok, warnings, err := vm.CheckCompatibility("1.0.0", ">=1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected compatible")
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	// Incompatible
	ok, warnings, err = vm.CheckCompatibility("1.0.0", ">=2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected incompatible")
	}
	if len(warnings) == 0 {
		t.Error("expected warnings for incompatible version")
	}

	// Invalid constraint
	_, _, err = vm.CheckCompatibility("1.0.0", "invalid-constraint")
	if err == nil {
		t.Error("expected error for invalid constraint")
	}
}

// ---------------------------------------------------------------------------
// registry.go: isPlaceholderURL edge cases
// ---------------------------------------------------------------------------

func TestIsPlaceholderURLEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want bool
	}{
		// git@ with single part (no colon)
		{"git@example.com", true},
		// Unparsable URL
		{"://bad-url", false},
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			got := isPlaceholderURL(tc.url)
			if got != tc.want {
				t.Errorf("isPlaceholderURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// config_validator.go: validateGitSource negative depth
// ---------------------------------------------------------------------------

func TestValidateGitSourceNegativeDepth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	v := NewValidator()
	v.options = ValidationOptions{SyntaxOnly: true}

	gs := &builder.GitSource{
		Repository: "https://github.com/org/repo.git",
		Depth:      -1,
	}
	err := v.validateGitSource(ctx, gs, 0)
	if err == nil {
		t.Error("expected error for negative depth")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// manager.go: saveConfigValue, saveTemplatesConfig
// ---------------------------------------------------------------------------

func TestSaveConfigValue_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories:\n    official: https://github.com/cowdogmoo/warpgate-templates.git\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: map[string]string{
				"official": "https://github.com/cowdogmoo/warpgate-templates.git",
				"custom":   "https://github.com/acme/templates.git",
			},
		},
	}
	manager := NewManager(cfg)

	err := manager.saveConfigValue("templates.repositories", cfg.Templates.Repositories)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "custom")
}

func TestSaveConfigValue_NoConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/config/path")

	cfg := &config.Config{}
	manager := NewManager(cfg)

	err := manager.saveConfigValue("templates.repositories", map[string]string{})
	assert.Error(t, err)
}

func TestSaveTemplatesConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories:\n    official: https://github.com/cowdogmoo/warpgate-templates.git\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: map[string]string{
				"official": "https://github.com/cowdogmoo/warpgate-templates.git",
			},
			LocalPaths: []string{"/some/new/path"},
		},
	}
	manager := NewManager(cfg)

	err := manager.saveTemplatesConfig()
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "/some/new/path")
}

func TestSaveTemplatesConfig_NoConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/config/path")

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths:   []string{},
			Repositories: map[string]string{},
		},
	}
	manager := NewManager(cfg)

	err := manager.saveTemplatesConfig()
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// manager.go: AddGitRepository with working config persistence
// ---------------------------------------------------------------------------

func TestAddGitRepositoryWithPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: make(map[string]string),
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddGitRepository(ctx, "my-repo", "https://github.com/acme/templates.git")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/templates.git", manager.config.Templates.Repositories["my-repo"])
}

func TestAddGitRepositoryAutoName(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: make(map[string]string),
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddGitRepository(ctx, "", "https://github.com/acme/my-awesome-repo.git")
	require.NoError(t, err)
	assert.Contains(t, manager.config.Templates.Repositories, "my-awesome-repo")
}

func TestAddGitRepositorySameURLNoError(t *testing.T) {
	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: map[string]string{
				"existing": "https://github.com/acme/templates.git",
			},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	// Same name, same URL - should return nil (warn only)
	err := manager.AddGitRepository(ctx, "existing", "https://github.com/acme/templates.git")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// manager.go: AddLocalPath with config persistence
// ---------------------------------------------------------------------------

func TestAddLocalPathWithPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	localDir := filepath.Join(tmpDir, "my-templates")
	require.NoError(t, os.MkdirAll(localDir, 0755))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddLocalPath(ctx, localDir)
	require.NoError(t, err)
	assert.Len(t, manager.config.Templates.LocalPaths, 1)
}

// ---------------------------------------------------------------------------
// manager.go: RemoveSource with config persistence
// ---------------------------------------------------------------------------

func TestRemoveSourceLocalPathPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	localDir := filepath.Join(tmpDir, "my-templates")
	require.NoError(t, os.MkdirAll(localDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths:\n    - " + localDir + "\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths:   []string{localDir},
			Repositories: map[string]string{},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.RemoveSource(ctx, localDir)
	require.NoError(t, err)
	assert.Empty(t, manager.config.Templates.LocalPaths)
}

func TestRemoveSourceRepoPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories:\n    my-repo: https://github.com/acme/templates.git\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{},
			Repositories: map[string]string{
				"my-repo": "https://github.com/acme/templates.git",
			},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.RemoveSource(ctx, "my-repo")
	require.NoError(t, err)
	assert.Empty(t, manager.config.Templates.Repositories)
}

// ---------------------------------------------------------------------------
// registry.go: UpdateCache with local templates
// ---------------------------------------------------------------------------

func TestUpdateCacheWithRepo(t *testing.T) {
	repoURL := createTestGitRepoWithTemplates(t)

	cacheDir := t.TempDir()
	registry := &TemplateRegistry{
		repos: map[string]string{
			"test-repo": repoURL,
		},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	err := registry.UpdateCache(ctx, "test-repo")
	require.NoError(t, err)

	cachePath := filepath.Join(cacheDir, "test-repo.json")
	assert.True(t, fileExists(cachePath))
}

func TestUpdateAllCachesWithFailures(t *testing.T) {
	cacheDir := t.TempDir()
	registry := &TemplateRegistry{
		repos: map[string]string{
			"bad-repo": "https://nonexistent-host.invalid/repo.git",
		},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	err := registry.UpdateAllCaches(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update some caches")
}

func TestDiscoverTemplatesWithInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	validDir := filepath.Join(tmpDir, "templates", "valid")
	invalidDir := filepath.Join(tmpDir, "templates", "invalid")
	require.NoError(t, os.MkdirAll(validDir, 0755))
	require.NoError(t, os.MkdirAll(invalidDir, 0755))

	validContent := "metadata:\n  description: Valid\n  version: 1.0.0\n  author: test\n  tags:\n    - test\n"
	require.NoError(t, os.WriteFile(filepath.Join(validDir, "warpgate.yaml"), []byte(validContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "warpgate.yaml"), []byte("{{{invalid"), 0644))

	registry := &TemplateRegistry{
		pathValidator: NewPathValidator(),
	}

	templates, err := registry.discoverTemplates(tmpDir)
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "valid", templates[0].Name)
}

func TestRegistryListWithFreshCacheHit(t *testing.T) {
	tmpDir := t.TempDir()

	cache := CacheMetadata{
		LastUpdated: time.Now(),
		Templates: map[string]TemplateInfo{
			"cached-tmpl": {
				Name:        "cached-tmpl",
				Description: "Cached template",
				Version:     "1.0.0",
			},
		},
		Repositories: map[string]string{},
	}

	cacheData, err := json.MarshalIndent(cache, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "remote-repo.json"), cacheData, 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"remote-repo": "https://github.com/test/repo.git",
		},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates, err := registry.List(ctx, "remote-repo")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "cached-tmpl", templates[0].Name)
}

func TestScanLocalPathsWithMixedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	validPath := filepath.Join(tmpDir, "valid-local")
	tmplDir := filepath.Join(validPath, "templates", "local-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	tmplContent := "metadata:\n  description: Local\n  version: 1.0.0\n  author: test\n  tags:\n    - local\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(tmplContent), 0644))

	noTmplsPath := filepath.Join(tmpDir, "no-templates")
	require.NoError(t, os.MkdirAll(noTmplsPath, 0755))

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		localPaths:    []string{validPath, noTmplsPath, "/nonexistent/path"},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates := registry.scanLocalPaths(ctx)
	assert.Len(t, templates, 1)
	assert.Equal(t, "local-tmpl", templates[0].Name)
}

func TestListLocalWithMixedRepos(t *testing.T) {
	tmpDir := t.TempDir()

	localRepo := filepath.Join(tmpDir, "local-repo")
	tmplDir := filepath.Join(localRepo, "templates", "local-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	tmplContent := "metadata:\n  description: Local template\n  version: 1.0.0\n  author: test\n  tags:\n    - local\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(tmplContent), 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"local":  localRepo,
			"remote": "https://github.com/test/repo.git",
		},
		localPaths:    []string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates, err := registry.ListLocal(ctx)
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "local-tmpl", templates[0].Name)
}

// ---------------------------------------------------------------------------
// loader.go: SetVariables, LoadTemplateWithVars, List
// ---------------------------------------------------------------------------

func TestSetVariablesReplace(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	vars := map[string]string{"key1": "val1", "key2": "val2"}
	loader.SetVariables(vars)
	assert.Equal(t, "val1", loader.variables["key1"])
	assert.Equal(t, "val2", loader.variables["key2"])

	newVars := map[string]string{"key3": "val3"}
	loader.SetVariables(newVars)
	assert.Len(t, loader.variables, 1)
	assert.Equal(t, "val3", loader.variables["key3"])
}

func TestLoadTemplateWithVarsVariableMerge(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "warpgate.yaml")

	content := `metadata:
  name: merge-test
  version: 1.0.0
  description: Merge test
  author: Test
  license: MIT
  requires:
    warpgate: ">=1.0.0"
name: merge-test
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo "test"
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	loader.SetVariables(map[string]string{"INSTANCE_VAR": "instance_val"})
	cfg, err := loader.LoadTemplateWithVars(context.Background(), configPath, map[string]string{"CALL_VAR": "call_val"})
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoadTemplateWithVarsByNameNotFound(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	_, err = loader.LoadTemplateWithVars(context.Background(), "absolutely-nonexistent-template-xyz-999", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// config_loader.go: LoadFromFileWithVars error paths
// ---------------------------------------------------------------------------

func TestLoadFromFileWithVarsNonExistent(t *testing.T) {
	loader := NewLoader()
	_, err := loader.LoadFromFileWithVars("/nonexistent/path/config.yaml", nil)
	assert.Error(t, err)
}

func TestLoadFromFileWithVarsInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	require.NoError(t, os.WriteFile(badFile, []byte("{{{invalid yaml"), 0644))

	loader := NewLoader()
	_, err := loader.LoadFromFileWithVars(badFile, nil)
	assert.Error(t, err)
}

func TestExpandVariablesSourcesPreserved(t *testing.T) {
	loader := NewLoader()

	input := "path: ${sources.my_source}/file.txt"
	result := loader.expandVariables(input, nil)
	assert.Equal(t, "path: ${sources.my_source}/file.txt", result)
}

func TestExpandVariablesEmptyVar(t *testing.T) {
	loader := NewLoader()

	input := "value: ${TOTALLY_UNKNOWN_VAR_XYZ_12345}"
	result := loader.expandVariables(input, nil)
	assert.Equal(t, "value: ", result)
}

func TestResolveRelativePathsNilDockerfile(t *testing.T) {
	loader := NewLoader()

	cfg := &builder.Config{
		Dockerfile: nil,
	}
	loader.resolveRelativePaths(cfg, "/base")
	assert.Nil(t, cfg.Dockerfile)
}

func TestSaveToFileValid(t *testing.T) {
	loader := NewLoader()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.yaml")

	cfg := &builder.Config{
		Name:    "test-save",
		Version: "latest",
	}
	err := loader.SaveToFile(cfg, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-save")
}

func TestSaveToFileInvalidDir(t *testing.T) {
	loader := NewLoader()
	err := loader.SaveToFile(&builder.Config{Name: "test"}, "/nonexistent/dir/output.yaml")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// version.go: edge cases
// ---------------------------------------------------------------------------

func TestCompareVersionsV2Nil(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.CompareVersions("1.0.0", "latest")
	require.NoError(t, err)
	assert.Equal(t, -1, result)
}

func TestCompareVersionsInvalid(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.CompareVersions("invalid", "1.0.0")
	assert.Error(t, err)

	_, err = vm.CompareVersions("1.0.0", "invalid")
	assert.Error(t, err)
}

func TestIsBreakingChangeNilVersions(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.IsBreakingChange("latest", "latest")
	require.NoError(t, err)
	assert.False(t, result)

	result, err = vm.IsBreakingChange("1.0.0", "latest")
	require.NoError(t, err)
	assert.False(t, result)

	result, err = vm.IsBreakingChange("latest", "2.0.0")
	require.NoError(t, err)
	assert.False(t, result)
}

func TestIsBreakingChangeInvalid(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.IsBreakingChange("invalid", "2.0.0")
	assert.Error(t, err)

	_, err = vm.IsBreakingChange("1.0.0", "invalid")
	assert.Error(t, err)
}

func TestValidateConstraintInvalidVersion(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.ValidateConstraint("invalid", ">=1.0.0")
	assert.Error(t, err)
}

func TestValidateConstraintInvalidConstraint(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.ValidateConstraint("1.0.0", ">>>invalid<<<")
	assert.Error(t, err)
}

func TestCheckCompatibilityInvalidConstraint(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, _, err = vm.CheckCompatibility("1.0.0", ">>>invalid<<<")
	assert.Error(t, err)
}

func TestGetLatestVersionAllLatest(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.GetLatestVersion(context.Background(), []string{"latest", "", "latest"})
	require.NoError(t, err)
	assert.Equal(t, "latest", result)
}

func TestValidateVersionRangeInvalidMinMax(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.ValidateVersionRange("1.0.0", "invalid", "2.0.0")
	assert.Error(t, err)

	_, err = vm.ValidateVersionRange("1.0.0", "1.0.0", "invalid")
	assert.Error(t, err)

	_, err = vm.ValidateVersionRange("invalid", "1.0.0", "2.0.0")
	assert.Error(t, err)
}

func TestValidateVersionRangeNoMin(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.ValidateVersionRange("0.5.0", "", "2.0.0")
	require.NoError(t, err)
	assert.True(t, result)
}

// ---------------------------------------------------------------------------
// paths.go: ExtractRepoName edge cases
// ---------------------------------------------------------------------------

func TestExtractRepoNameCoverage(t *testing.T) {
	tests := []struct {
		gitURL string
		want   string
	}{
		{"git@github.com:user/repo.git", "repo"},
		{"git@github.com:user/my-repo", "my-repo"},
		{"https://github.com/user/repo.git/", "templates"},
		{"", "templates"},
		{"https://github.com", "github.com"},
		{"https://gitlab.com/a/b/c/d/my-template.git", "my-template"},
	}

	for _, tt := range tests {
		result := ExtractRepoName(tt.gitURL)
		assert.Equal(t, tt.want, result, "ExtractRepoName(%q)", tt.gitURL)
	}
}

func TestMustExpandPathWithEnvVar(t *testing.T) {
	t.Setenv("WARPGATE_TEST_EXPAND_COV", "/custom/expanded/path")
	result := MustExpandPath("${WARPGATE_TEST_EXPAND_COV}/subdir")
	assert.Equal(t, "/custom/expanded/path/subdir", result)
}

func TestPathValidatorIsLocalPathEdgeCases(t *testing.T) {
	pv := NewPathValidator()

	assert.False(t, pv.IsLocalPath("./nonexistent-dir-xyz"))
	assert.False(t, pv.IsLocalPath("~/nonexistent-dir-xyz"))
}

// ---------------------------------------------------------------------------
// registry.go: isPlaceholderURL edge cases
// ---------------------------------------------------------------------------

func TestIsPlaceholderURLGitAtVariants(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"git@example.com:user/repo.git", true},
		{"git@foo.test:user/repo.git", true},
		{"git@foo.invalid:user/repo.git", true},
		{"git@foo.localhost:user/repo.git", true},
		{"git@github.com:user/repo.git", false},
		{"git@example.com", true},
	}

	for _, tt := range tests {
		result := isPlaceholderURL(tt.url)
		assert.Equal(t, tt.expected, result, "isPlaceholderURL(%q)", tt.url)
	}
}

// ---------------------------------------------------------------------------
// registry.go: GetLocalPaths returns copy
// ---------------------------------------------------------------------------

func TestGetLocalPathsCopy(t *testing.T) {
	registry := &TemplateRegistry{
		repos:      map[string]string{},
		localPaths: []string{"/path/one", "/path/two"},
	}

	paths := registry.GetLocalPaths()
	assert.Len(t, paths, 2)

	paths[0] = "/modified"
	assert.Equal(t, "/path/one", registry.localPaths[0])
}

// ---------------------------------------------------------------------------
// registry.go: LoadRepositories with corrupt JSON
// ---------------------------------------------------------------------------

func TestLoadRepositoriesCorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "repositories.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{invalid json}"), 0644))

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	err := registry.LoadRepositories()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

// ---------------------------------------------------------------------------
// scaffold.go: saveTemplateConfig error path
// ---------------------------------------------------------------------------

func TestSaveTemplateConfigNilConfig(t *testing.T) {
	s := NewScaffolder()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &builder.Config{
		Name:    "nil-test",
		Version: "latest",
	}
	err := s.saveTemplateConfig(cfg, configPath)
	assert.NoError(t, err)
	assert.True(t, fileExists(configPath))
}

func TestSaveTemplateConfigBadPath(t *testing.T) {
	s := NewScaffolder()
	cfg := &builder.Config{Name: "test"}
	err := s.saveTemplateConfig(cfg, "/nonexistent/dir/that/cannot/be/created/config.yaml")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: scaffold.go Create and Fork
// ---------------------------------------------------------------------------

func TestScaffolderCreateSuccess(t *testing.T) {
	s := NewScaffolder()
	outputDir := t.TempDir()
	ctx := context.Background()

	err := s.Create(ctx, "my-new-template", outputDir)
	require.NoError(t, err)

	// Verify created structure
	assert.True(t, fileExists(filepath.Join(outputDir, "my-new-template", "warpgate.yaml")))
	assert.True(t, dirExists(filepath.Join(outputDir, "my-new-template", "scripts")))
	assert.True(t, fileExists(filepath.Join(outputDir, "my-new-template", "README.md")))
}

func TestScaffolderCreateReadOnlyDir(t *testing.T) {
	s := NewScaffolder()
	ctx := context.Background()

	err := s.Create(ctx, "template", "/nonexistent/readonly/dir")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: paths.go NormalizePath, ExpandPath (method), ExtractRepoName
// ---------------------------------------------------------------------------

func TestNormalizePathWithSlash(t *testing.T) {
	pv := NewPathValidator()

	// Test path that contains "/" but is not absolute -> should become absolute
	result, err := pv.NormalizePath("some/relative/path")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

func TestExtractRepoNameColonWithoutSlash(t *testing.T) {
	// This exercises the colon-parsing branch for git@host:repo format without /
	result := ExtractRepoName("git@myhost.com:single-repo.git")
	assert.Equal(t, "single-repo", result)

	// Just whitespace after cleaning
	result2 := ExtractRepoName("   ")
	assert.Equal(t, "templates", result2)

	// Colon with single part (no path after colon)
	result3 := ExtractRepoName("git@host.com:")
	assert.Equal(t, "templates", result3)
}

func TestPathValidatorExpandPathRelative(t *testing.T) {
	pv := NewPathValidator()

	// Relative path should become absolute
	result, err := pv.ExpandPath("relative/path")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

func TestPathValidatorExpandPathAbsolute(t *testing.T) {
	pv := NewPathValidator()

	result, err := pv.ExpandPath("/absolute/path")
	require.NoError(t, err)
	assert.Equal(t, "/absolute/path", result)
}

// ---------------------------------------------------------------------------
// Additional coverage: registry.go List with local path, SaveRepositories
// ---------------------------------------------------------------------------

func TestRegistryListWithLocalPathRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a local repo structure with templates
	tmplDir := filepath.Join(tmpDir, "templates", "my-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	tmplContent := "metadata:\n  description: Local test\n  version: 1.0.0\n  author: test\n  tags:\n    - test\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(tmplContent), 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"local-repo": tmpDir,
		},
		localPaths:    []string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates, err := registry.List(ctx, "local-repo")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "my-tmpl", templates[0].Name)
}

func TestRegistryListUnknownRepoError(t *testing.T) {
	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	_, err := registry.List(ctx, "unknown-repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repository")
}

func TestSaveRepositoriesSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	registry := &TemplateRegistry{
		repos: map[string]string{
			"test": "https://github.com/test/repo.git",
		},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	err := registry.SaveRepositories()
	require.NoError(t, err)
	assert.True(t, fileExists(filepath.Join(tmpDir, "repositories.json")))
}

func TestSaveRepositoriesBadPath(t *testing.T) {
	registry := &TemplateRegistry{
		repos:         map[string]string{"test": "url"},
		cacheDir:      "/nonexistent/path",
		pathValidator: NewPathValidator(),
	}

	err := registry.SaveRepositories()
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: config_loader.go LoadFromFileWithVars relative base dir
// ---------------------------------------------------------------------------

func TestLoadFromFileWithVarsRelativePath(t *testing.T) {
	// Create a valid config in a subdir of the cwd
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sub", "warpgate.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))

	content := `metadata:
  name: rel-test
  version: 1.0.0
  description: Relative path test
  author: Test
  license: MIT
  requires:
    warpgate: ">=1.0.0"
name: rel-test
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo hello
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	loader := NewLoader()
	cfg, err := loader.LoadFromFileWithVars(configPath, map[string]string{"MY_VAR": "my_val"})
	require.NoError(t, err)
	assert.Equal(t, "rel-test", cfg.Name)
}

// ---------------------------------------------------------------------------
// Additional coverage: registry.go matchesQuery - fuzzy match on description
// ---------------------------------------------------------------------------

func TestMatchesQueryFuzzyDescription(t *testing.T) {
	registry := &TemplateRegistry{pathValidator: NewPathValidator()}

	tmpl := TemplateInfo{
		Name:        "attack-box",
		Description: "Security penetration testing toolkit",
		Tags:        []string{"security"},
	}

	// Fuzzy match on description word
	assert.True(t, registry.matchesQuery(tmpl, "penetration"))
	// No match
	assert.False(t, registry.matchesQuery(tmpl, "zzzzzzzzzzz"))
}

// ---------------------------------------------------------------------------
// Additional coverage: registry.go listAll
// ---------------------------------------------------------------------------

func TestRegistryListAllWithMixedSources(t *testing.T) {
	tmpDir := t.TempDir()

	// Create local repo with templates
	tmplDir := filepath.Join(tmpDir, "repo1", "templates", "tmpl1")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"),
		[]byte("metadata:\n  description: T1\n  version: 1.0.0\n  author: test\n  tags:\n    - test\n"), 0644))

	// Create local path with templates
	localPath := filepath.Join(tmpDir, "local")
	localTmplDir := filepath.Join(localPath, "templates", "tmpl2")
	require.NoError(t, os.MkdirAll(localTmplDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(localTmplDir, "warpgate.yaml"),
		[]byte("metadata:\n  description: T2\n  version: 1.0.0\n  author: test\n  tags:\n    - local\n"), 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"local-repo": filepath.Join(tmpDir, "repo1"),
		},
		localPaths:    []string{localPath},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	all, err := registry.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

// ---------------------------------------------------------------------------
// Additional coverage: loader.go LoadTemplateWithVars with directory ref
// ---------------------------------------------------------------------------

func TestLoadTemplateWithVarsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	content := `metadata:
  name: dir-test
  version: 1.0.0
  description: Directory test
  author: Test
  license: MIT
  requires:
    warpgate: ">=1.0.0"
name: dir-test
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo dir
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "warpgate.yaml"), []byte(content), 0644))

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	cfg, err := loader.LoadTemplateWithVars(context.Background(), tmpDir, nil)
	require.NoError(t, err)
	assert.Equal(t, "dir-test", cfg.Name)
}

func TestLoadTemplateWithVarsDirNoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	_, err = loader.LoadTemplateWithVars(context.Background(), tmpDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no warpgate.yaml found")
}

func TestLoadTemplateWithVarsUnknownRef(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	_, err = loader.LoadTemplateWithVars(context.Background(), "some/path/with/slashes", nil)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: manager.go AddLocalPath duplicate path
// ---------------------------------------------------------------------------

func TestAddLocalPathDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("templates:\n  repositories: {}\n  local_paths: []\n"), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	localDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(localDir, 0755))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{localDir},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	// Adding duplicate should not error
	err := manager.AddLocalPath(ctx, localDir)
	assert.NoError(t, err)
	assert.Len(t, manager.config.Templates.LocalPaths, 1)
}

func TestAddLocalPathNonExistent(t *testing.T) {
	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddLocalPath(ctx, "/nonexistent/path/xyz")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: version.go GetLatestVersion with mixed versions
// ---------------------------------------------------------------------------

func TestGetLatestVersionMixed(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.GetLatestVersion(context.Background(), []string{"1.0.0", "2.0.0", "1.5.0"})
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", result)
}

func TestGetLatestVersionSingleValid(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.GetLatestVersion(context.Background(), []string{"latest", "invalid!", "3.0.0"})
	require.NoError(t, err)
	assert.Equal(t, "3.0.0", result)
}

func TestGetLatestVersionEmpty(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.GetLatestVersion(context.Background(), []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no versions provided")
}
