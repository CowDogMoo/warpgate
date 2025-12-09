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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/builder"
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
