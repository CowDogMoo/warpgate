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

package buildkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// TestDetectCollectionRoot tests Ansible collection root detection
func TestDetectCollectionRoot(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create a mock collection structure
	collectionRoot := filepath.Join(tmpDir, "my-collection")
	playbooksDir := filepath.Join(collectionRoot, "playbooks")
	if err := os.MkdirAll(playbooksDir, 0755); err != nil {
		t.Fatalf("Failed to create playbooks dir: %v", err)
	}

	// Create galaxy.yml
	galaxyFile := filepath.Join(collectionRoot, "galaxy.yml")
	if err := os.WriteFile(galaxyFile, []byte("namespace: test\nname: collection\nversion: 1.0.0\n"), 0644); err != nil {
		t.Fatalf("Failed to create galaxy.yml: %v", err)
	}

	// Create a playbook
	playbookPath := filepath.Join(playbooksDir, "test.yml")
	if err := os.WriteFile(playbookPath, []byte("---\n- hosts: localhost\n"), 0644); err != nil {
		t.Fatalf("Failed to create playbook: %v", err)
	}

	tests := []struct {
		name         string
		playbookPath string
		expectRoot   bool
	}{
		{
			name:         "path with /playbooks/ and galaxy.yml",
			playbookPath: playbookPath,
			expectRoot:   true,
		},
		{
			name:         "path without collection structure",
			playbookPath: filepath.Join(tmpDir, "standalone.yml"),
			expectRoot:   false,
		},
		{
			name:         "path with /playbooks/ but no galaxy.yml",
			playbookPath: filepath.Join(tmpDir, "other", "playbooks", "test.yml"),
			expectRoot:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectCollectionRoot(tt.playbookPath)
			if tt.expectRoot && result == "" {
				t.Error("Expected to find collection root, but got empty string")
			}
			if !tt.expectRoot && result != "" {
				t.Errorf("Expected no collection root, but got: %s", result)
			}
			if tt.expectRoot && result != collectionRoot {
				t.Errorf("Expected collection root %s, but got: %s", collectionRoot, result)
			}
		})
	}
}

// TestConvertToLLB tests LLB conversion
func TestConvertToLLB(t *testing.T) {
	tests := []struct {
		name        string
		config      builder.Config
		expectError bool
	}{
		{
			name: "basic configuration with base image",
			config: builder.Config{
				Name:    "test-basic",
				Version: "1.0.0",
				Base: builder.BaseImage{
					Image: "alpine:latest",
				},
				Architectures: []string{"amd64"},
			},
			expectError: false,
		},
		{
			name: "with environment variables",
			config: builder.Config{
				Name:    "test-env",
				Version: "1.0.0",
				Base: builder.BaseImage{
					Image: "alpine:latest",
					Env: map[string]string{
						"MY_VAR":  "value1",
						"MY_VAR2": "value2",
					},
				},
				Architectures: []string{"amd64"},
			},
			expectError: false,
		},
		{
			name: "with shell provisioner",
			config: builder.Config{
				Name:    "test-shell",
				Version: "1.0.0",
				Base: builder.BaseImage{
					Image: "alpine:latest",
				},
				Provisioners: []builder.Provisioner{
					{
						Type: "shell",
						Inline: []string{
							"apk update",
							"apk add curl",
						},
					},
				},
				Architectures: []string{"amd64"},
			},
			expectError: false,
		},
		{
			name: "with post-changes",
			config: builder.Config{
				Name:    "test-post",
				Version: "1.0.0",
				Base: builder.BaseImage{
					Image: "alpine:latest",
				},
				PostChanges: []string{
					"USER nobody",
					"WORKDIR /app",
				},
				Architectures: []string{"amd64"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			state, err := b.convertToLLB(tt.config)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify state can be marshaled
				def, err := state.Marshal(context.Background())
				if err != nil {
					t.Errorf("Failed to marshal LLB state: %v", err)
				}
				if len(def.Def) == 0 {
					t.Error("Expected non-empty LLB definition")
				}
			}
		})
	}
}

// TestApplyShellProvisioner tests shell provisioner LLB conversion
func TestApplyShellProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		provisioner builder.Provisioner
		expectError bool
	}{
		{
			name: "single command",
			provisioner: builder.Provisioner{
				Type: "shell",
				Inline: []string{
					"echo 'hello world'",
				},
			},
			expectError: false,
		},
		{
			name: "multiple commands",
			provisioner: builder.Provisioner{
				Type: "shell",
				Inline: []string{
					"apt-get update",
					"apt-get install -y curl",
					"apt-get clean",
				},
			},
			expectError: false,
		},
		{
			name: "empty inline",
			provisioner: builder.Provisioner{
				Type:   "shell",
				Inline: []string{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			platform := specs.Platform{
				OS:           "linux",
				Architecture: "amd64",
			}
			state := llb.Image("alpine:latest", llb.Platform(platform))

			newState, err := b.applyShellProvisioner(state, tt.provisioner)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify state can be marshaled
				def, err := newState.Marshal(context.Background())
				if err != nil {
					t.Errorf("Failed to marshal LLB state: %v", err)
				}
				if len(def.Def) == 0 {
					t.Error("Expected non-empty LLB definition")
				}
			}
		})
	}
}

// TestApplyPostChanges tests post-changes LLB conversion
func TestApplyPostChanges(t *testing.T) {
	tests := []struct {
		name        string
		postChanges []string
	}{
		{
			name: "with multiple changes",
			postChanges: []string{
				"USER appuser",
				"WORKDIR /app",
			},
		},
		{
			name:        "empty changes",
			postChanges: []string{},
		},
		{
			name: "with ENV changes",
			postChanges: []string{
				"ENV KEY=value",
				"ENV KEY2 value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			platform := specs.Platform{
				OS:           "linux",
				Architecture: "amd64",
			}
			state := llb.Image("alpine:latest", llb.Platform(platform))

			newState := b.applyPostChanges(state, tt.postChanges)

			// Verify state can be marshaled
			def, err := newState.Marshal(context.Background())
			if err != nil {
				t.Errorf("Failed to marshal LLB state: %v", err)
			}
			if len(def.Def) == 0 {
				t.Error("Expected non-empty LLB definition")
			}
		})
	}
}

// TestDetectBuildxBuilder tests buildx builder detection parsing
func TestDetectBuildxBuilder(t *testing.T) {
	// This test requires docker buildx to be installed and running
	// Skip if not available
	t.Skip("Requires running docker buildx builder - integration test")
}

// TestNewBuildKitBuilder tests BuildKit builder initialization
func TestNewBuildKitBuilder(t *testing.T) {
	// This test requires docker buildx to be installed and a builder running
	// Skip if not available
	t.Skip("Requires running docker buildx builder - integration test")
}

// TestBuild tests the complete build process
func TestBuild(t *testing.T) {
	// This test requires docker buildx to be installed and a builder running
	// Skip if not available
	t.Skip("Requires running docker buildx builder - integration test")
}

// TestLoadTLSConfig tests TLS configuration loading
func TestLoadTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create temporary test certificates
	caCertPath := filepath.Join(tmpDir, "ca.pem")
	clientCertPath := filepath.Join(tmpDir, "client.pem")

	// Create mock CA cert
	caCert := []byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`)
	if err := os.WriteFile(caCertPath, caCert, 0644); err != nil {
		t.Fatalf("Failed to write CA cert: %v", err)
	}

	// Create mock client cert (simplified for testing)
	clientCert := caCert // In real world, this would be different
	if err := os.WriteFile(clientCertPath, clientCert, 0644); err != nil {
		t.Fatalf("Failed to write client cert: %v", err)
	}

	tests := []struct {
		name        string
		config      globalconfig.BuildKitConfig
		expectError bool
		description string
	}{
		{
			name: "with valid CA certificate",
			config: globalconfig.BuildKitConfig{
				TLSEnabled: true,
				TLSCACert:  caCertPath,
			},
			expectError: false,
			description: "Should load TLS config with CA cert",
		},
		{
			name: "with missing client certificate key file",
			config: globalconfig.BuildKitConfig{
				TLSEnabled: true,
				TLSCert:    clientCertPath,
				TLSKey:     "/nonexistent/client-key.pem",
			},
			expectError: true,
			description: "Should error with missing client key file",
		},
		{
			name: "with missing CA certificate file",
			config: globalconfig.BuildKitConfig{
				TLSEnabled: true,
				TLSCACert:  "/nonexistent/ca.pem",
			},
			expectError: true,
			description: "Should error with missing CA cert file",
		},
		{
			name: "with missing client certificate file",
			config: globalconfig.BuildKitConfig{
				TLSEnabled: true,
				TLSCert:    "/nonexistent/client.pem",
				TLSKey:     "/nonexistent/client-key.pem",
			},
			expectError: true,
			description: "Should error with missing client cert file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := loadTLSConfig(tt.config)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none. %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v. %s", err, tt.description)
			}
			if !tt.expectError && tlsConfig == nil {
				t.Error("Expected TLS config but got nil")
			}
		})
	}
}

// TestParseCacheAttrs tests parsing of cache attribute strings
func TestParseCacheAttrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "basic cache spec",
			input: "type=registry,ref=user/app:cache",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
			},
		},
		{
			name:  "cache spec with mode",
			input: "type=registry,ref=user/app:cache,mode=max",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
				"mode": "max",
			},
		},
		{
			name:  "cache spec with whitespace",
			input: "type=registry, ref=user/app:cache , mode=max",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
				"mode": "max",
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "single attribute",
			input: "type=local",
			expected: map[string]string{
				"type": "local",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCacheAttrs(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d attributes, got %d", len(tt.expected), len(result))
			}
			for key, expectedValue := range tt.expected {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("Missing expected key: %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("For key %s: expected %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// TestParsePlatform tests platform string parsing
func TestParsePlatform(t *testing.T) {
	tests := []struct {
		name         string
		platformStr  string
		expectedOS   string
		expectedArch string
		expectError  bool
	}{
		{
			name:         "linux/amd64",
			platformStr:  "linux/amd64",
			expectedOS:   "linux",
			expectedArch: "amd64",
			expectError:  false,
		},
		{
			name:         "linux/arm64",
			platformStr:  "linux/arm64",
			expectedOS:   "linux",
			expectedArch: "arm64",
			expectError:  false,
		},
		{
			name:         "windows/amd64",
			platformStr:  "windows/amd64",
			expectedOS:   "windows",
			expectedArch: "amd64",
			expectError:  false,
		},
		{
			name:        "invalid format - missing architecture",
			platformStr: "linux",
			expectError: true,
		},
		{
			name:        "invalid format - too many parts",
			platformStr: "linux/amd64/extra",
			expectError: true,
		},
		{
			name:        "empty string",
			platformStr: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os, arch, err := parsePlatform(tt.platformStr)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError {
				if os != tt.expectedOS {
					t.Errorf("Expected OS %q, got %q", tt.expectedOS, os)
				}
				if arch != tt.expectedArch {
					t.Errorf("Expected arch %q, got %q", tt.expectedArch, arch)
				}
			}
		})
	}
}

// TestCalculateBuildContext tests build context calculation
func TestCalculateBuildContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	subDir1 := filepath.Join(tmpDir, "subdir1")
	subDir2 := filepath.Join(tmpDir, "subdir2")
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("Failed to create subdir1: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("Failed to create subdir2: %v", err)
	}

	// Create test files
	playbook1 := filepath.Join(subDir1, "playbook.yml")
	playbook2 := filepath.Join(subDir2, "playbook.yml")
	script1 := filepath.Join(subDir1, "setup.sh")

	for _, file := range []string{playbook1, playbook2, script1} {
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	tests := []struct {
		name            string
		config          builder.Config
		expectedContext string
		description     string
	}{
		{
			name: "single ansible playbook",
			config: builder.Config{
				Provisioners: []builder.Provisioner{
					{
						Type:         "ansible",
						PlaybookPath: playbook1,
					},
				},
			},
			expectedContext: subDir1,
			description:     "Context should be the directory containing the playbook",
		},
		{
			name: "multiple files in same directory",
			config: builder.Config{
				Provisioners: []builder.Provisioner{
					{
						Type:         "ansible",
						PlaybookPath: playbook1,
					},
					{
						Type:    "script",
						Scripts: []string{script1},
					},
				},
			},
			expectedContext: subDir1,
			description:     "Context should be the common directory",
		},
		{
			name: "files in different subdirectories",
			config: builder.Config{
				Provisioners: []builder.Provisioner{
					{
						Type:         "ansible",
						PlaybookPath: playbook1,
					},
					{
						Type:         "ansible",
						PlaybookPath: playbook2,
					},
				},
			},
			expectedContext: tmpDir,
			description:     "Context should be the common parent directory",
		},
		{
			name:            "no provisioners",
			config:          builder.Config{},
			expectedContext: ".",
			description:     "Context should be current directory when no files referenced",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			contextDir, err := b.calculateBuildContext(tt.config)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectedContext != "." && contextDir != tt.expectedContext {
				t.Errorf("Expected context %q, got %q. %s", tt.expectedContext, contextDir, tt.description)
			}
		})
	}
}

// TestFindCommonParent tests finding common parent directories
func TestFindCommonParent(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected string
	}{
		{
			name:     "same directory",
			path1:    "/home/user/project",
			path2:    "/home/user/project",
			expected: "/home/user/project",
		},
		{
			name:     "sibling directories",
			path1:    "/home/user/project1",
			path2:    "/home/user/project2",
			expected: "/home/user",
		},
		{
			name:     "nested directories",
			path1:    "/home/user/project",
			path2:    "/home/user/project/subdir",
			expected: "/home/user/project",
		},
		{
			name:     "completely different paths",
			path1:    "/home/user1/project",
			path2:    "/opt/application",
			expected: "/",
		},
		{
			name:     "root level",
			path1:    "/home",
			path2:    "/opt",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findCommonParent(tt.path1, tt.path2)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestMakeRelativePath tests path relativization
func TestMakeRelativePath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		contextDir  string
		targetPath  string
		expectError bool
		description string
	}{
		{
			name:        "path in context",
			contextDir:  tmpDir,
			targetPath:  filepath.Join(tmpDir, "subdir", "file.txt"),
			expectError: false,
			description: "Should make path relative to context",
		},
		{
			name:        "path same as context",
			contextDir:  tmpDir,
			targetPath:  tmpDir,
			expectError: false,
			description: "Should handle path equal to context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{contextDir: tt.contextDir}
			relPath, err := b.makeRelativePath(tt.targetPath)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none. %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v. %s", err, tt.description)
			}
			if !tt.expectError && filepath.IsAbs(relPath) {
				t.Errorf("Expected relative path but got absolute: %s", relPath)
			}
		})
	}
}

// TestBuildExportAttributes tests export attribute building
func TestBuildExportAttributes(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		labels    map[string]string
		expected  map[string]string
	}{
		{
			name:      "basic image name",
			imageName: "myapp:latest",
			labels:    nil,
			expected: map[string]string{
				"name": "myapp:latest",
			},
		},
		{
			name:      "with labels",
			imageName: "myapp:v1.0.0",
			labels: map[string]string{
				"version":     "1.0.0",
				"maintainer":  "test@example.com",
				"description": "Test application",
			},
			expected: map[string]string{
				"name":              "myapp:v1.0.0",
				"label:version":     "1.0.0",
				"label:maintainer":  "test@example.com",
				"label:description": "Test application",
			},
		},
		{
			name:      "registry with image",
			imageName: "ghcr.io/org/myapp:latest",
			labels:    map[string]string{"env": "production"},
			expected: map[string]string{
				"name":      "ghcr.io/org/myapp:latest",
				"label:env": "production",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExportAttributes(tt.imageName, tt.labels)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d attributes, got %d", len(tt.expected), len(result))
			}
			for key, expectedValue := range tt.expected {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("Missing expected key: %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("For key %s: expected %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// TestExpandContainerVars tests container variable expansion
func TestExpandContainerVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		env      map[string]string
		expected string
	}{
		{
			name:  "expand single variable",
			input: "/opt/bin:$PATH",
			env: map[string]string{
				"PATH": "/usr/bin:/bin",
			},
			expected: "/opt/bin:/usr/bin:/bin",
		},
		{
			name:  "expand multiple variables",
			input: "$HOME/bin:$PATH",
			env: map[string]string{
				"HOME": "/home/user",
				"PATH": "/usr/bin",
			},
			expected: "/home/user/bin:/usr/bin",
		},
		{
			name:     "no variables to expand",
			input:    "/usr/local/bin",
			env:      map[string]string{},
			expected: "/usr/local/bin",
		},
		{
			name:  "variable not in env",
			input: "$UNKNOWN/path",
			env: map[string]string{
				"PATH": "/usr/bin",
			},
			expected: "$UNKNOWN/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			result := b.expandContainerVars(tt.input, tt.env)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestGetPlatformString tests platform string extraction
func TestGetPlatformString(t *testing.T) {
	tests := []struct {
		name     string
		config   builder.Config
		expected string
	}{
		{
			name: "with base platform",
			config: builder.Config{
				Base: builder.BaseImage{
					Platform: "linux/amd64",
				},
			},
			expected: "linux/amd64",
		},
		{
			name: "with architectures",
			config: builder.Config{
				Architectures: []string{"arm64"},
			},
			expected: "linux/arm64",
		},
		{
			name: "base platform takes precedence",
			config: builder.Config{
				Base: builder.BaseImage{
					Platform: "linux/amd64",
				},
				Architectures: []string{"arm64"},
			},
			expected: "linux/amd64",
		},
		{
			name:     "no platform specified",
			config:   builder.Config{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPlatformString(tt.config)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
