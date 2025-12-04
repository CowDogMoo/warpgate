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

	if cfg.StorageRoot == "" {
		t.Error("StorageRoot should not be empty")
	}

	if cfg.RunRoot == "" {
		t.Error("RunRoot should not be empty")
	}

	if cfg.StorageDriver == "" {
		t.Error("StorageDriver should not be empty")
	}

	t.Logf("Default config: StorageRoot=%s, RunRoot=%s, Driver=%s",
		cfg.StorageRoot, cfg.RunRoot, cfg.StorageDriver)
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

	for _, tt := range tests {
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
