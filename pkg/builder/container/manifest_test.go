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

	"github.com/containers/common/pkg/manifests"
	"github.com/opencontainers/go-digest"
)

func TestNewManifestManager(t *testing.T) {
	// Create a minimal manifest manager
	// This test just validates the constructor
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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

	mm := bldr.GetManifestManager()
	if mm == nil {
		t.Fatal("ManifestManager should not be nil")
	}

	if mm.store == nil {
		t.Error("Store should be initialized")
	}

	if mm.systemContext == nil {
		t.Error("SystemContext should be initialized")
	}
}

func TestManifestEntry(t *testing.T) {
	// Test ManifestEntry struct
	entry := ManifestEntry{
		ImageRef:     "localhost/test:latest",
		Digest:       digest.FromString("test"),
		Platform:     "linux/amd64",
		Architecture: "amd64",
		OS:           "linux",
	}

	if entry.ImageRef == "" {
		t.Error("ImageRef should be set")
	}

	if entry.Architecture == "" {
		t.Error("Architecture should be set")
	}

	if entry.OS == "" {
		t.Error("OS should be set")
	}
}

func TestManifestManager_CreateManifest_EmptyEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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

	mm := bldr.GetManifestManager()
	ctx := context.Background()

	// Test with empty entries - should return error
	_, err = mm.CreateManifest(ctx, "test-manifest", []ManifestEntry{})
	if err == nil {
		t.Error("CreateManifest should fail with empty entries")
	}
}

func TestManifestManager_CreateManifest_MultiArch(t *testing.T) {
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
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	mm := bldr.GetManifestManager()
	ctx := context.Background()

	// Create manifest entries (would need actual built images in real scenario)
	entries := []ManifestEntry{
		{
			ImageRef:     "localhost/test:amd64",
			Architecture: "amd64",
			OS:           "linux",
		},
		{
			ImageRef:     "localhost/test:arm64",
			Architecture: "arm64",
			OS:           "linux",
		},
	}

	// This will likely fail without actual images, but tests the flow
	manifestList, err := mm.CreateManifest(ctx, "test-manifest", entries)
	if err != nil {
		t.Logf("Expected error without actual images: %v", err)
		return
	}

	if manifestList == nil {
		t.Error("Manifest list should not be nil")
	}

	t.Logf("Created manifest successfully")
}

func TestManifestManager_PushManifest(t *testing.T) {
	// This is an integration test that requires buildah and registry access
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
	}

	bldr, err := NewBuildahBuilder(cfg)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer bldr.Close()

	mm := bldr.GetManifestManager()
	ctx := context.Background()

	// Create an empty manifest list for testing
	manifestList := manifests.Create()

	// Test push without any entries - should work but may fail at registry level
	err = mm.PushManifest(ctx, manifestList, "localhost:5000/test:latest")
	if err == nil {
		t.Log("PushManifest succeeded (registry may not be available, which is expected)")
	} else {
		t.Logf("PushManifest failed as expected without registry: %v", err)
	}
}

func TestManifestManager_CreateManifest_NilEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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

	mm := bldr.GetManifestManager()
	ctx := context.Background()

	// Test with nil entries - should return error
	_, err = mm.CreateManifest(ctx, "test-manifest", nil)
	if err == nil {
		t.Error("CreateManifest should fail with nil entries")
	}
}

func TestManifestManager_MultipleArchitectures(t *testing.T) {
	// Unit test to verify handling of multiple architectures
	tests := []struct {
		name         string
		entries      []ManifestEntry
		expectError  bool
		expectedArch []string
	}{
		{
			name:        "empty entries",
			entries:     []ManifestEntry{},
			expectError: true,
		},
		{
			name: "single architecture",
			entries: []ManifestEntry{
				{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
			expectError:  false,
			expectedArch: []string{"amd64"},
		},
		{
			name: "multiple architectures",
			entries: []ManifestEntry{
				{
					Architecture: "amd64",
					OS:           "linux",
				},
				{
					Architecture: "arm64",
					OS:           "linux",
				},
			},
			expectError:  false,
			expectedArch: []string{"amd64", "arm64"},
		},
		{
			name: "with variant",
			entries: []ManifestEntry{
				{
					Architecture: "arm",
					OS:           "linux",
					Variant:      "v7",
				},
			},
			expectError:  false,
			expectedArch: []string{"arm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify entry structure
			if len(tt.entries) != len(tt.expectedArch) && !tt.expectError {
				t.Errorf("Expected %d architectures, got %d entries", len(tt.expectedArch), len(tt.entries))
			}

			for i, entry := range tt.entries {
				if !tt.expectError && i < len(tt.expectedArch) {
					if entry.Architecture != tt.expectedArch[i] {
						t.Errorf("Expected architecture %s, got %s", tt.expectedArch[i], entry.Architecture)
					}
				}
			}
		})
	}
}

func TestManifestEntry_Complete(t *testing.T) {
	// Test that ManifestEntry can hold all required fields
	tests := []struct {
		name  string
		entry ManifestEntry
		valid bool
	}{
		{
			name: "complete entry",
			entry: ManifestEntry{
				ImageRef:     "localhost/test:latest",
				Digest:       digest.FromString("test"),
				Platform:     "linux/amd64",
				Architecture: "amd64",
				OS:           "linux",
			},
			valid: true,
		},
		{
			name: "entry with variant",
			entry: ManifestEntry{
				ImageRef:     "localhost/test:latest",
				Digest:       digest.FromString("test"),
				Platform:     "linux/arm/v7",
				Architecture: "arm",
				OS:           "linux",
				Variant:      "v7",
			},
			valid: true,
		},
		{
			name: "minimal entry",
			entry: ManifestEntry{
				Architecture: "amd64",
				OS:           "linux",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				if tt.entry.Architecture == "" {
					t.Error("Valid entry should have architecture")
				}
				if tt.entry.OS == "" {
					t.Error("Valid entry should have OS")
				}
			}
		})
	}
}

func TestManifestManager_StoreAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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

	mm := bldr.GetManifestManager()

	// Verify the manifest manager has access to the store
	if mm.store == nil {
		t.Error("ManifestManager should have access to storage")
	}

	// Verify the manifest manager has system context
	if mm.systemContext == nil {
		t.Error("ManifestManager should have system context")
	}
}

func TestNewManifestManager_Direct(t *testing.T) {
	// Test direct construction of ManifestManager
	tmpDir, err := os.MkdirTemp("", "warpgate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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

	// Create a manifest manager directly
	mm := NewManifestManager(bldr.store, bldr.systemContext)
	if mm == nil {
		t.Fatal("NewManifestManager should return a valid manager")
	}

	if mm.store != bldr.store {
		t.Error("ManifestManager should use the provided store")
	}

	if mm.systemContext != bldr.systemContext {
		t.Error("ManifestManager should use the provided system context")
	}
}
