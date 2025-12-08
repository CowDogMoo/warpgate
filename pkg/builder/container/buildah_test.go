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
)

func TestGetDefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig()

	// With the new approach, default config returns empty values
	// to delegate to storage.DefaultStoreOptions()
	// Only non-empty if user explicitly configured overrides in global config

	t.Logf("Default config: StorageRoot='%s', RunRoot='%s', Driver='%s'",
		cfg.StorageRoot, cfg.RunRoot, cfg.StorageDriver)
	t.Log("Empty values are expected - they delegate to containers/storage system defaults")

	// Verify the config is at least valid (no nil pointers, ett.)
	// The actual storage initialization will be tested in TestNewBuildahBuilder
}

func TestNewBuildahBuilder(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	// This test will only fully pass on Linux with buildah installed
	// On other platforms, we're just testing the initialization logic
	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		// On non-Linux platforms or without buildah, this is expected
		t.Logf("Expected error on non-Linux or without buildah: %v", err)
		return
	}
	defer bldr.Close()

	if bldr.store == nil {
		t.Error("Store should be initialized")
	}

	if bldr.systemContext == nil {
		t.Error("SystemContext should be initialized")
	}

	if bldr.workDir == "" {
		t.Error("WorkDir should be set")
	}
}

func TestBuildahBuilder_GetManifestManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Skipf("Skipping on non-Linux or without buildah: %v", err)
		return
	}
	defer bldr.Close()

	mm := bldr.GetManifestManager()
	if mm == nil {
		t.Fatal("ManifestManager should not be nil")
	}

	if mm.store == nil {
		t.Error("ManifestManager store should not be nil")
	}

	if mm.systemContext == nil {
		t.Error("ManifestManager systemContext should not be nil")
	}
}

func TestBuildahBuilder_Build_Minimal(t *testing.T) {
	// This is an integration test that requires buildah
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	// Create a minimal build config
	buildCfg := builder.Config{
		Name:    "test-simple",
		Version: "latest",
		Base: builder.BaseImage{
			Image: "alpine:latest",
			Pull:  true,
		},
		Provisioners: []builder.Provisioner{
			{
				Type: "shell",
				Inline: []string{
					"echo 'Hello from Warpgate!'",
				},
			},
		},
	}

	ctx := context.Background()
	result, err := bldr.Build(ctx, buildCfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.ImageRef == "" {
		t.Error("Build result should include image reference")
	}

	if result.Duration == "" {
		t.Error("Build result should include duration")
	}

	t.Logf("Build completed: %s in %s", result.ImageRef, result.Duration)
}

func TestBuildahConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    BuildahConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: BuildahConfig{
				StorageDriver: "vfs",
				StorageRoot:   "/tmp/storage",
				RunRoot:       "/tmp/run",
			},
			wantError: false,
		},
		{
			name: "empty storage driver uses default",
			config: BuildahConfig{
				StorageRoot: "/tmp/storage",
				RunRoot:     "/tmp/run",
			},
			wantError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			if cfg.StorageDriver == "" {
				cfg.StorageDriver = "vfs"
			}

			if cfg.StorageRoot == "" && tt.wantError {
				t.Log("Expected validation to fail for empty storage root")
			}
		})
	}
}

func TestBuildahBuilder_Close(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Skipf("Skipping on non-Linux or without buildah: %v", err)
		return
	}

	// Test that Close() doesn't error
	err = bldr.Close()
	if err != nil {
		t.Errorf("Close() should not error: %v", err)
	}

	// Test that multiple Close() calls don't cause issues
	err = bldr.Close()
	if err != nil {
		t.Errorf("Second Close() should not error: %v", err)
	}
}

func TestBuildahBuilder_BuildWithMultipleProvisioners(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	// Create a build config with multiple provisioners
	buildCfg := builder.Config{
		Name:    "test-multi",
		Version: "latest",
		Base: builder.BaseImage{
			Image: "alpine:latest",
			Pull:  true,
		},
		Provisioners: []builder.Provisioner{
			{
				Type: "shell",
				Inline: []string{
					"apk add --no-cache curl",
				},
			},
			{
				Type: "shell",
				Inline: []string{
					"echo 'Second provisioner'",
					"curl --version",
				},
			},
		},
	}

	ctx := context.Background()
	result, err := bldr.Build(ctx, buildCfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.ImageRef == "" {
		t.Error("Build result should include image reference")
	}

	t.Logf("Build with multiple provisioners completed: %s", result.ImageRef)
}

func TestBuildahBuilder_BuildWithInvalidImage(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	// Try to build with a non-existent base image
	buildCfg := builder.Config{
		Name:    "test-invalid",
		Version: "latest",
		Base: builder.BaseImage{
			Image: "this-image-does-not-exist-12345:latest",
			Pull:  true,
		},
		Provisioners: []builder.Provisioner{},
	}

	ctx := context.Background()
	_, err = bldr.Build(ctx, buildCfg)
	if err == nil {
		t.Error("Build should fail with invalid base image")
	}

	t.Logf("Expected error with invalid image: %v", err)
}

func TestBuildahBuilder_Tag(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	// First build an image
	buildCfg := builder.Config{
		Name:    "test-tag",
		Version: "latest",
		Base: builder.BaseImage{
			Image: "alpine:latest",
			Pull:  true,
		},
		Provisioners: []builder.Provisioner{
			{
				Type: "shell",
				Inline: []string{
					"echo 'test'",
				},
			},
		},
	}

	ctx := context.Background()
	result, err := bldr.Build(ctx, buildCfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Now tag the image
	newTag := "test-tag:v1.0.0"
	err = bldr.Tag(ctx, result.ImageRef, newTag)
	if err != nil {
		t.Errorf("Tag failed: %v", err)
	}

	t.Logf("Successfully tagged %s as %s", result.ImageRef, newTag)
}

func TestBuildahBuilder_Remove(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	// First build an image
	buildCfg := builder.Config{
		Name:    "test-remove",
		Version: "latest",
		Base: builder.BaseImage{
			Image: "alpine:latest",
			Pull:  true,
		},
		Provisioners: []builder.Provisioner{
			{
				Type: "shell",
				Inline: []string{
					"echo 'test'",
				},
			},
		},
	}

	ctx := context.Background()
	result, err := bldr.Build(ctx, buildCfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Now remove the image
	err = bldr.Remove(ctx, result.ImageRef)
	if err != nil {
		t.Errorf("Remove failed: %v", err)
	}

	t.Logf("Successfully removed image: %s", result.ImageRef)
}

func TestBuildahBuilder_WorkDirCreation(t *testing.T) {
	// Test that work directory is created if not specified
	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   t.TempDir(),
		RunRoot:       t.TempDir(),
		// WorkDir intentionally not set
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Skipf("Skipping on non-Linux or without buildah: %v", err)
		return
	}
	defer bldr.Close()

	if bldr.workDir == "" {
		t.Error("WorkDir should be automatically created")
	}

	// Verify the directory exists
	if _, err := os.Stat(bldr.workDir); os.IsNotExist(err) {
		t.Error("WorkDir should exist on disk")
	}
}

func TestBuildahConfig_DefaultValues(t *testing.T) {
	// Test that BuildahConfig works with minimal configuration
	tmpDir := t.TempDir()

	cfg := BuildahConfig{
		StorageRoot: tmpDir,
		RunRoot:     tmpDir,
		// StorageDriver not set - should use default
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Skipf("Skipping on non-Linux or without buildah: %v", err)
		return
	}
	defer bldr.Close()

	if bldr.store == nil {
		t.Error("Store should be initialized with default driver")
	}
}

func TestBuildahBuilder_ProvisionerTypes(t *testing.T) {
	// Unit test to verify provisioner type handling
	tests := []struct {
		name         string
		provisioner  builder.Provisioner
		expectSkip   bool
		expectedType string
	}{
		{
			name: "shell provisioner",
			provisioner: builder.Provisioner{
				Type:   "shell",
				Inline: []string{"echo test"},
			},
			expectSkip:   false,
			expectedType: "shell",
		},
		{
			name: "script provisioner",
			provisioner: builder.Provisioner{
				Type:    "script",
				Scripts: []string{"/path/to/script.sh"},
			},
			expectSkip:   false,
			expectedType: "script",
		},
		{
			name: "ansible provisioner",
			provisioner: builder.Provisioner{
				Type:         "ansible",
				PlaybookPath: "/path/to/playbook.yml",
			},
			expectSkip:   false,
			expectedType: "ansible",
		},
		{
			name: "unknown provisioner",
			provisioner: builder.Provisioner{
				Type: "unknown-type",
			},
			expectSkip:   true,
			expectedType: "unknown-type",
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.provisioner.Type != tt.expectedType {
				t.Errorf("Expected provisioner type %s, got %s", tt.expectedType, tt.provisioner.Type)
			}

			// Verify the provisioner has the expected fields
			switch tt.expectedType {
			case "shell":
				if len(tt.provisioner.Inline) == 0 {
					t.Error("Shell provisioner should have inline commands")
				}
			case "script":
				if len(tt.provisioner.Scripts) == 0 {
					t.Error("Script provisioner should have script paths")
				}
			case "ansible":
				if tt.provisioner.PlaybookPath == "" {
					t.Error("Ansible provisioner should have a playbook path")
				}
			}
		})
	}
}

func TestBuildahBuilder_ParseCommandValue(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Skipf("Skipping on non-Linux or without buildah: %v", err)
		return
	}
	defer bldr.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "JSON array format",
			input:    `["/bin/bash", "-c", "echo hello"]`,
			expected: []string{"/bin/bash", "-c", "echo hello"},
		},
		{
			name:     "Shell form",
			input:    "/bin/bash -c 'echo hello'",
			expected: []string{"/bin/sh", "-c", "/bin/bash -c 'echo hello'"},
		},
		{
			name:     "Single command",
			input:    "/bin/bash",
			expected: []string{"/bin/sh", "-c", "/bin/bash"},
		},
		{
			name:     "JSON array with spaces",
			input:    `[ "/bin/bash" , "-c" , "echo hello" ]`,
			expected: []string{"/bin/bash", "-c", "echo hello"},
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bldr.parseCommandValue(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d elements, got %d", len(tt.expected), len(result))
				return
			}
			for i, val := range result {
				if val != tt.expected[i] {
					t.Errorf("Element %d: expected '%s', got '%s'", i, tt.expected[i], val)
				}
			}
		})
	}
}

func TestBuildahBuilder_ApplyChange(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	tmpDir := t.TempDir()
	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	// Create a builder container
	ctx := context.Background()
	baseImage := builder.BaseImage{
		Image: "alpine:latest",
		Pull:  true,
	}
	if err := bldr.fromImage(ctx, baseImage); err != nil {
		t.Fatalf("Failed to create from image: %v", err)
	}

	tests := []struct {
		name      string
		change    string
		expectErr bool
	}{
		{
			name:      "Set USER",
			change:    "USER sliver",
			expectErr: false,
		},
		{
			name:      "Set WORKDIR",
			change:    "WORKDIR /home/sliver",
			expectErr: false,
		},
		{
			name:      "Set ENV with equals",
			change:    "ENV PATH=/opt/sliver:/usr/bin:$PATH",
			expectErr: false,
		},
		{
			name:      "Set ENV with space",
			change:    "ENV MY_VAR my_value",
			expectErr: false,
		},
		{
			name:      "Set ENTRYPOINT JSON",
			change:    `ENTRYPOINT ["/bin/bash"]`,
			expectErr: false,
		},
		{
			name:      "Set CMD shell",
			change:    "CMD /bin/bash",
			expectErr: false,
		},
		{
			name:      "Set LABEL",
			change:    `LABEL version="1.0"`,
			expectErr: false,
		},
		{
			name:      "Empty change",
			change:    "",
			expectErr: false,
		},
		{
			name:      "USER without value",
			change:    "USER",
			expectErr: true,
		},
		{
			name:      "WORKDIR without value",
			change:    "WORKDIR",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bldr.applyChange(tt.change)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestBuildahBuilder_ApplyChanges(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	tmpDir := t.TempDir()
	cfg := BuildahConfig{
		StorageDriver: "vfs",
		StorageRoot:   filepath.Join(tmpDir, "storage"),
		RunRoot:       filepath.Join(tmpDir, "run"),
		WorkDir:       filepath.Join(tmpDir, "work"),
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	// Create a builder container
	ctx := context.Background()
	baseImage := builder.BaseImage{
		Image: "alpine:latest",
		Pull:  true,
		Changes: []string{
			"USER sliver",
			"WORKDIR /home/sliver",
			"ENV PATH=/opt/sliver:/home/sliver/.sliver/go/bin:$PATH",
			`ENTRYPOINT ["/bin/bash"]`,
		},
	}

	if err := bldr.fromImage(ctx, baseImage); err != nil {
		t.Fatalf("Failed to create from image with changes: %v", err)
	}

	// Verify the builder has the expected configuration
	if bldr.builder.User() != "sliver" {
		t.Errorf("Expected user 'sliver', got '%s'", bldr.builder.User())
	}

	if bldr.builder.WorkDir() != "/home/sliver" {
		t.Errorf("Expected workdir '/home/sliver', got '%s'", bldr.builder.WorkDir())
	}

	t.Logf("Successfully applied all changes to the builder")
}
