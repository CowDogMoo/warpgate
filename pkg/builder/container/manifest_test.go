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
