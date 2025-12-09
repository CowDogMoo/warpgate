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

package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/builder"
)

// TestGenerateDockerfile tests the Dockerfile generation functionality
func TestGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name          string
		config        builder.Config
		expectedParts []string
		notExpected   []string
	}{
		{
			name: "basic configuration with base image",
			config: builder.Config{
				Name:    "test-basic",
				Version: "1.0.0",
				Base: builder.BaseImage{
					Image: "ubuntu:22.04",
				},
			},
			expectedParts: []string{
				"FROM ubuntu:22.04",
			},
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
			},
			expectedParts: []string{
				"FROM alpine:latest",
				"ENV MY_VAR=value1",
				"ENV MY_VAR2=value2",
			},
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
			},
			expectedParts: []string{
				"FROM alpine:latest",
				"RUN apk update",
				"apk add curl",
			},
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
					"ENTRYPOINT [\"/bin/sh\"]",
				},
			},
			expectedParts: []string{
				"FROM alpine:latest",
				"# Post-build changes",
				"USER nobody",
				"WORKDIR /app",
				"ENTRYPOINT [\"/bin/sh\"]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{buildxAvailable: true}
			dockerfile, err := b.generateDockerfile(tt.config)
			if err != nil {
				t.Fatalf("generateDockerfile failed: %v", err)
			}

			// Check expected parts are present
			for _, expected := range tt.expectedParts {
				if !strings.Contains(dockerfile, expected) {
					t.Errorf("Expected Dockerfile to contain %q, but it didn't.\nGenerated Dockerfile:\n%s", expected, dockerfile)
				}
			}

			// Check parts that should not be present
			for _, notExpected := range tt.notExpected {
				if strings.Contains(dockerfile, notExpected) {
					t.Errorf("Expected Dockerfile NOT to contain %q, but it did.\nGenerated Dockerfile:\n%s", notExpected, dockerfile)
				}
			}
		})
	}
}

// TestAddShellProvisioner tests shell provisioner generation
func TestAddShellProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		provisioner builder.Provisioner
		expected    []string
		notExpected []string
	}{
		{
			name: "single command",
			provisioner: builder.Provisioner{
				Type: "shell",
				Inline: []string{
					"echo 'hello world'",
				},
			},
			expected: []string{
				"RUN echo 'hello world'",
			},
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
			expected: []string{
				"RUN apt-get update",
				"apt-get install -y curl",
				"apt-get clean",
			},
		},
		{
			name: "empty inline",
			provisioner: builder.Provisioner{
				Type:   "shell",
				Inline: []string{},
			},
			notExpected: []string{
				"RUN",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{buildxAvailable: true}
			var dockerfile strings.Builder
			b.addShellProvisioner(&dockerfile, tt.provisioner)
			result := dockerfile.String()

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nGenerated:\n%s", expected, result)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nGenerated:\n%s", notExpected, result)
				}
			}
		})
	}
}

// TestAddAnsibleProvisioner tests Ansible provisioner generation
func TestAddAnsibleProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		provisioner builder.Provisioner
		expected    []string
		notExpected []string
	}{
		{
			name: "basic playbook",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "/path/to/playbook.yml",
			},
			expected: []string{
				"# Ansible provisioner",
				"COPY playbook.yml /tmp/playbook.yml",
				"RUN ansible-playbook /tmp/playbook.yml",
				"-i localhost,",
				"-c local",
			},
		},
		{
			name: "with galaxy requirements",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "/path/to/playbook.yml",
				GalaxyFile:   "/path/to/requirements.yml",
			},
			expected: []string{
				"COPY requirements.yml /tmp/requirements.yml",
				"RUN ansible-galaxy install -r /tmp/requirements.yml",
			},
		},
		{
			name: "with extra vars",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "/path/to/playbook.yml",
				ExtraVars: map[string]string{
					"var1": "value1",
					"var2": "value2",
				},
			},
			expected: []string{
				"ansible-playbook",
				"-e var1=value1",
				"-e var2=value2",
			},
		},
		{
			name: "empty playbook path",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "",
			},
			notExpected: []string{
				"COPY",
				"RUN ansible-playbook",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{buildxAvailable: true}
			var dockerfile strings.Builder
			b.addAnsibleProvisioner(&dockerfile, tt.provisioner)
			result := dockerfile.String()

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nGenerated:\n%s", expected, result)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nGenerated:\n%s", notExpected, result)
				}
			}
		})
	}
}

// TestAddFileProvisioner tests file provisioner generation
func TestAddFileProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		provisioner builder.Provisioner
		expected    []string
		notExpected []string
	}{
		{
			name: "basic file copy",
			provisioner: builder.Provisioner{
				Type:        "file",
				Source:      "/path/to/source.txt",
				Destination: "/opt/destination.txt",
			},
			expected: []string{
				"# File provisioner",
				"COPY source.txt /opt/destination.txt",
			},
			notExpected: []string{
				"chmod",
			},
		},
		{
			name: "with permissions",
			provisioner: builder.Provisioner{
				Type:        "file",
				Source:      "/path/to/script.sh",
				Destination: "/usr/local/bin/script.sh",
				Mode:        "0755",
			},
			expected: []string{
				"# File provisioner",
				"COPY script.sh /usr/local/bin/script.sh",
				"RUN chmod 0755 /usr/local/bin/script.sh",
			},
		},
		{
			name: "missing source",
			provisioner: builder.Provisioner{
				Type:        "file",
				Source:      "",
				Destination: "/opt/file.txt",
			},
			notExpected: []string{
				"COPY",
			},
		},
		{
			name: "missing destination",
			provisioner: builder.Provisioner{
				Type:   "file",
				Source: "/path/to/file.txt",
			},
			notExpected: []string{
				"COPY",
			},
		},
		{
			name: "with environment variable in source",
			provisioner: builder.Provisioner{
				Type:        "file",
				Source:      "$HOME/config.json",
				Destination: "/etc/config.json",
			},
			expected: []string{
				"# File provisioner",
				"COPY config.json /etc/config.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{buildxAvailable: true}
			var dockerfile strings.Builder
			b.addFileProvisioner(&dockerfile, tt.provisioner)
			result := dockerfile.String()

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nGenerated:\n%s", expected, result)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nGenerated:\n%s", notExpected, result)
				}
			}
		})
	}
}

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

// TestBuildAnsibleCommand tests Ansible command building
func TestBuildAnsibleCommand(t *testing.T) {
	tests := []struct {
		name        string
		provisioner builder.Provisioner
		expected    []string
		notExpected []string
	}{
		{
			name: "basic command",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "/path/to/playbook.yml",
			},
			expected: []string{
				"ansible-playbook /tmp/playbook.yml",
				"-i localhost,",
				"-c local",
			},
		},
		{
			name: "with extra vars",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "/path/to/playbook.yml",
				ExtraVars: map[string]string{
					"foo": "bar",
					"baz": "qux",
				},
			},
			expected: []string{
				"ansible-playbook /tmp/playbook.yml",
				"-e foo=bar",
				"-e baz=qux",
			},
		},
		{
			name: "without extra vars",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "/path/to/playbook.yml",
				ExtraVars:    nil,
			},
			expected: []string{
				"ansible-playbook /tmp/playbook.yml",
			},
			notExpected: []string{
				"-e",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{buildxAvailable: true}
			result := b.buildAnsibleCommand(tt.provisioner)

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected command to contain %q, but it didn't.\nGenerated command:\n%s", expected, result)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected command NOT to contain %q, but it did.\nGenerated command:\n%s", notExpected, result)
				}
			}
		})
	}
}

// TestAddPostChanges tests post-build changes generation
func TestAddPostChanges(t *testing.T) {
	tests := []struct {
		name        string
		postChanges []string
		expected    []string
		notExpected []string
	}{
		{
			name: "with multiple changes",
			postChanges: []string{
				"USER appuser",
				"WORKDIR /app",
				"EXPOSE 8080",
			},
			expected: []string{
				"# Post-build changes",
				"USER appuser",
				"WORKDIR /app",
				"EXPOSE 8080",
			},
		},
		{
			name:        "empty changes",
			postChanges: []string{},
			notExpected: []string{
				"# Post-build changes",
			},
		},
		{
			name: "with ENTRYPOINT and CMD",
			postChanges: []string{
				`ENTRYPOINT ["/usr/bin/myapp"]`,
				`CMD ["--help"]`,
			},
			expected: []string{
				"# Post-build changes",
				`ENTRYPOINT ["/usr/bin/myapp"]`,
				`CMD ["--help"]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{buildxAvailable: true}
			var dockerfile strings.Builder
			b.addPostChanges(&dockerfile, tt.postChanges)
			result := dockerfile.String()

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nGenerated:\n%s", expected, result)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nGenerated:\n%s", notExpected, result)
				}
			}
		})
	}
}

// TestCopyFile tests file copying functionality
func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	content := "test content\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test successful copy
	t.Run("successful copy", func(t *testing.T) {
		dstPath := filepath.Join(tmpDir, "dest.txt")
		if err := copyFile(srcPath, dstPath); err != nil {
			t.Fatalf("copyFile failed: %v", err)
		}

		// Verify destination file exists and has correct content
		dstContent, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("Failed to read destination file: %v", err)
		}

		if string(dstContent) != content {
			t.Errorf("Expected content %q, got %q", content, string(dstContent))
		}
	})

	// Test copy with non-existent source
	t.Run("non-existent source", func(t *testing.T) {
		nonExistentSrc := filepath.Join(tmpDir, "nonexistent.txt")
		dstPath := filepath.Join(tmpDir, "dest2.txt")
		if err := copyFile(nonExistentSrc, dstPath); err == nil {
			t.Error("Expected error for non-existent source, got nil")
		}
	})

	// Test copy to invalid destination
	t.Run("invalid destination", func(t *testing.T) {
		invalidDst := filepath.Join(tmpDir, "nonexistent", "dest.txt")
		if err := copyFile(srcPath, invalidDst); err == nil {
			t.Error("Expected error for invalid destination, got nil")
		}
	})
}

// TestCopyDirectory tests directory copying functionality
func TestCopyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create files in source directory
	files := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
	}
	for path, content := range files {
		fullPath := filepath.Join(srcDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Test successful directory copy
	t.Run("successful copy", func(t *testing.T) {
		dstDir := filepath.Join(tmpDir, "dest")
		if err := copyDirectory(srcDir, dstDir); err != nil {
			t.Fatalf("copyDirectory failed: %v", err)
		}

		// Verify destination files exist and have correct content
		for path, expectedContent := range files {
			fullPath := filepath.Join(dstDir, path)
			content, err := os.ReadFile(fullPath)
			if err != nil {
				t.Errorf("Failed to read file %s: %v", path, err)
				continue
			}
			if string(content) != expectedContent {
				t.Errorf("File %s: expected content %q, got %q", path, expectedContent, string(content))
			}
		}
	})

	// Test copy with non-existent source
	t.Run("non-existent source", func(t *testing.T) {
		nonExistentSrc := filepath.Join(tmpDir, "nonexistent")
		dstDir := filepath.Join(tmpDir, "dest2")
		if err := copyDirectory(nonExistentSrc, dstDir); err == nil {
			t.Error("Expected error for non-existent source, got nil")
		}
	})

	// Test that .git directories are skipped
	t.Run("skip .git directories", func(t *testing.T) {
		srcWithGit := filepath.Join(tmpDir, "source-git")
		gitDir := filepath.Join(srcWithGit, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("Failed to create .git directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644); err != nil {
			t.Fatalf("Failed to create git config: %v", err)
		}

		dstDir := filepath.Join(tmpDir, "dest-no-git")
		if err := copyDirectory(srcWithGit, dstDir); err != nil {
			t.Fatalf("copyDirectory failed: %v", err)
		}

		// Verify .git was not copied
		dstGitDir := filepath.Join(dstDir, ".git")
		if _, err := os.Stat(dstGitDir); !os.IsNotExist(err) {
			t.Error(".git directory should not be copied")
		}
	})
}

// TestCopyProvisionerFiles tests the copyProvisionerFiles method
func TestCopyProvisionerFiles(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("Failed to create build directory: %v", err)
	}

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	playbookFile := filepath.Join(tmpDir, "playbook.yml")
	if err := os.WriteFile(playbookFile, []byte("---\n- hosts: localhost\n"), 0644); err != nil {
		t.Fatalf("Failed to create playbook: %v", err)
	}

	tests := []struct {
		name          string
		config        builder.Config
		expectError   bool
		expectedFiles []string
	}{
		{
			name: "copy file provisioner",
			config: builder.Config{
				Provisioners: []builder.Provisioner{
					{
						Type:        "file",
						Source:      testFile,
						Destination: "/opt/test.txt",
					},
				},
			},
			expectError:   false,
			expectedFiles: []string{"test.txt"},
		},
		{
			name: "copy ansible provisioner",
			config: builder.Config{
				Provisioners: []builder.Provisioner{
					{
						Type:         "ansible",
						PlaybookPath: playbookFile,
					},
				},
			},
			expectError:   false,
			expectedFiles: []string{"playbook.yml"},
		},
		{
			name: "copy multiple provisioners",
			config: builder.Config{
				Provisioners: []builder.Provisioner{
					{
						Type:        "file",
						Source:      testFile,
						Destination: "/opt/test.txt",
					},
					{
						Type:         "ansible",
						PlaybookPath: playbookFile,
					},
				},
			},
			expectError:   false,
			expectedFiles: []string{"test.txt", "playbook.yml"},
		},
		{
			name: "non-existent file",
			config: builder.Config{
				Provisioners: []builder.Provisioner{
					{
						Type:        "file",
						Source:      filepath.Join(tmpDir, "nonexistent.txt"),
						Destination: "/opt/file.txt",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean build directory
			if err := os.RemoveAll(buildDir); err != nil {
				t.Fatalf("Failed to clean build directory: %v", err)
			}
			if err := os.MkdirAll(buildDir, 0755); err != nil {
				t.Fatalf("Failed to recreate build directory: %v", err)
			}

			b := &BuildKitBuilder{buildxAvailable: true}
			err := b.copyProvisionerFiles(tt.config, buildDir)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify expected files exist
				for _, filename := range tt.expectedFiles {
					filePath := filepath.Join(buildDir, filename)
					if _, err := os.Stat(filePath); os.IsNotExist(err) {
						t.Errorf("Expected file %s does not exist", filename)
					}
				}
			}
		})
	}
}

// TestAddBaseEnvVars tests environment variable generation
func TestAddBaseEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected []string
	}{
		{
			name: "single env var",
			env: map[string]string{
				"FOO": "bar",
			},
			expected: []string{
				"ENV FOO=bar",
			},
		},
		{
			name: "multiple env vars",
			env: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			expected: []string{
				"ENV VAR1=value1",
				"ENV VAR2=value2",
			},
		},
		{
			name:     "nil env",
			env:      nil,
			expected: []string{},
		},
		{
			name:     "empty env",
			env:      map[string]string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{buildxAvailable: true}
			var dockerfile strings.Builder
			b.addBaseEnvVars(&dockerfile, tt.env)
			result := dockerfile.String()

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nGenerated:\n%s", expected, result)
				}
			}
		})
	}
}

// TestCopyProvisionerFilesWithCollection tests Ansible collection copying
func TestCopyProvisionerFilesWithCollection(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("Failed to create build directory: %v", err)
	}

	// Create a mock collection structure
	collectionRoot := filepath.Join(tmpDir, "my-collection")
	playbooksDir := filepath.Join(collectionRoot, "playbooks")
	rolesDir := filepath.Join(collectionRoot, "roles")
	if err := os.MkdirAll(playbooksDir, 0755); err != nil {
		t.Fatalf("Failed to create playbooks dir: %v", err)
	}
	if err := os.MkdirAll(rolesDir, 0755); err != nil {
		t.Fatalf("Failed to create roles dir: %v", err)
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

	// Create a role
	roleFile := filepath.Join(rolesDir, "main.yml")
	if err := os.WriteFile(roleFile, []byte("---\n- name: test role\n"), 0644); err != nil {
		t.Fatalf("Failed to create role: %v", err)
	}

	config := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: playbookPath,
			},
		},
	}

	b := &BuildKitBuilder{buildxAvailable: true}
	if err := b.copyProvisionerFiles(config, buildDir); err != nil {
		t.Fatalf("copyProvisionerFiles failed: %v", err)
	}

	// Verify playbook was copied
	copiedPlaybook := filepath.Join(buildDir, "test.yml")
	if _, err := os.Stat(copiedPlaybook); os.IsNotExist(err) {
		t.Error("Playbook should have been copied")
	}

	// Verify collection directory was copied
	copiedCollection := filepath.Join(buildDir, "collection")
	if _, err := os.Stat(copiedCollection); os.IsNotExist(err) {
		t.Error("Collection directory should have been copied")
	}

	// Verify galaxy.yml was copied in the collection
	copiedGalaxy := filepath.Join(copiedCollection, "galaxy.yml")
	if _, err := os.Stat(copiedGalaxy); os.IsNotExist(err) {
		t.Error("Collection galaxy.yml should have been copied")
	}

	// Verify roles directory was copied
	copiedRoles := filepath.Join(copiedCollection, "roles", "main.yml")
	if _, err := os.Stat(copiedRoles); os.IsNotExist(err) {
		t.Error("Collection roles should have been copied")
	}
}

// TestAddAnsibleProvisionerWithCollection tests Ansible provisioner with collection
func TestAddAnsibleProvisionerWithCollection(t *testing.T) {
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

	provisioner := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookPath,
	}

	b := &BuildKitBuilder{buildxAvailable: true}
	var dockerfile strings.Builder
	b.addAnsibleProvisioner(&dockerfile, provisioner)
	result := dockerfile.String()

	// Should include collection copy and install commands
	expectedParts := []string{
		"# Ansible provisioner",
		"COPY test.yml /tmp/playbook.yml",
		"COPY collection/ /tmp/ansible-collection/",
		"RUN ansible-galaxy collection install /tmp/ansible-collection/",
		"RUN ansible-playbook /tmp/playbook.yml",
	}

	for _, expected := range expectedParts {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGenerated:\n%s", expected, result)
		}
	}
}

// TestGenerateDockerfileIntegration tests complete Dockerfile generation with all provisioner types
func TestGenerateDockerfileIntegration(t *testing.T) {
	config := builder.Config{
		Name:    "test-integration",
		Version: "1.0.0",
		Base: builder.BaseImage{
			Image: "ubuntu:22.04",
			Env: map[string]string{
				"DEBIAN_FRONTEND": "noninteractive",
			},
		},
		Provisioners: []builder.Provisioner{
			{
				Type: "shell",
				Inline: []string{
					"apt-get update",
					"apt-get install -y curl wget",
				},
			},
			{
				Type:        "file",
				Source:      "/local/config.json",
				Destination: "/etc/app/config.json",
				Mode:        "0644",
			},
			{
				Type: "shell",
				Inline: []string{
					"apt-get clean",
				},
			},
		},
		PostChanges: []string{
			"USER appuser",
			"WORKDIR /app",
			"EXPOSE 8080",
			`ENTRYPOINT ["/usr/bin/app"]`,
		},
	}

	b := &BuildKitBuilder{buildxAvailable: true}
	dockerfile, err := b.generateDockerfile(config)
	if err != nil {
		t.Fatalf("generateDockerfile failed: %v", err)
	}

	// Verify the Dockerfile has all expected components in order
	expectedParts := []string{
		"FROM ubuntu:22.04",
		"ENV DEBIAN_FRONTEND=noninteractive",
		"RUN apt-get update",
		"apt-get install -y curl wget",
		"# File provisioner",
		"COPY config.json /etc/app/config.json",
		"RUN chmod 0644 /etc/app/config.json",
		"RUN apt-get clean",
		"# Post-build changes",
		"USER appuser",
		"WORKDIR /app",
		"EXPOSE 8080",
		`ENTRYPOINT ["/usr/bin/app"]`,
	}

	for _, expected := range expectedParts {
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("Expected Dockerfile to contain %q, but it didn't.\nGenerated Dockerfile:\n%s", expected, dockerfile)
		}
	}

	// Verify proper ordering: FROM should come first
	fromIndex := strings.Index(dockerfile, "FROM")
	envIndex := strings.Index(dockerfile, "ENV")
	if fromIndex >= envIndex {
		t.Error("FROM should come before ENV")
	}

	// Verify post-changes come at the end
	postChangesIndex := strings.Index(dockerfile, "# Post-build changes")
	if postChangesIndex == -1 {
		t.Error("Post-build changes comment not found")
	}

	t.Logf("Generated Dockerfile:\n%s", dockerfile)
}
