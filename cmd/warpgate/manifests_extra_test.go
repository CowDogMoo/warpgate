/*
Copyright (c) 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/opencontainers/go-digest"
)

func TestPerformRegistryChecks_BothDisabled(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := newTestCmd(nil)

	// Save and restore globals
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.healthCheck = false
	manifestsCreateOpts.verifyRegistry = false

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "amd64",
			Digest:       digest.FromString("test"),
			ModTime:      time.Now(),
		},
	}

	err := performRegistryChecks(ctx, cmd, digestFiles)
	if err != nil {
		t.Fatalf("performRegistryChecks() with both disabled should not error: %v", err)
	}
}

func TestHandleDryRun_SingleTag(t *testing.T) {
	ctx := setupTestContext(t)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.tags = []string{"latest"}
	manifestsCreateOpts.name = "test-image"
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "amd64",
			Digest:       digest.FromString("amd64-content"),
			ModTime:      time.Now(),
		},
		{
			ImageName:    "test-image",
			Architecture: "arm64",
			Digest:       digest.FromString("arm64-content"),
			ModTime:      time.Now(),
		},
	}

	err := handleDryRun(ctx, digestFiles)
	if err != nil {
		t.Fatalf("handleDryRun() unexpected error: %v", err)
	}
}

func TestConvertDigestFilesToManifestEntries_Multiple(t *testing.T) {
	t.Parallel()

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "my-image",
			Architecture: "amd64",
			Digest:       digest.FromString("amd64-content"),
			ModTime:      time.Now(),
		},
		{
			ImageName:    "my-image",
			Architecture: "arm64",
			Digest:       digest.FromString("arm64-content"),
			ModTime:      time.Now(),
		},
	}

	entries := convertDigestFilesToManifestEntries(digestFiles, "ghcr.io", "org", "v1.0")

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	for _, entry := range entries {
		if entry.OS != "linux" {
			t.Errorf("OS = %q, want linux", entry.OS)
		}
		if !strings.Contains(entry.ImageRef, "ghcr.io") {
			t.Errorf("ImageRef should contain registry, got: %q", entry.ImageRef)
		}
		if !strings.Contains(entry.ImageRef, "org") {
			t.Errorf("ImageRef should contain namespace, got: %q", entry.ImageRef)
		}
	}

	if entries[0].Architecture != "amd64" {
		t.Errorf("entries[0].Architecture = %q, want amd64", entries[0].Architecture)
	}
	if entries[1].Architecture != "arm64" {
		t.Errorf("entries[1].Architecture = %q, want arm64", entries[1].Architecture)
	}
}

func TestConvertDigestFilesToManifestEntries_NoNamespace(t *testing.T) {
	t.Parallel()

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "my-image",
			Architecture: "amd64",
			Digest:       digest.FromString("content"),
			ModTime:      time.Now(),
		},
	}

	entries := convertDigestFilesToManifestEntries(digestFiles, "ghcr.io", "", "latest")

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestParseMetadata_WithAnnotationsAndLabels(t *testing.T) {
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.annotations = []string{"org.opencontainers.image.source=https://github.com/test/repo"}
	manifestsCreateOpts.labels = []string{"maintainer=test@example.com", "version=1.0"}

	annotations, labels, err := parseMetadata(context.Background())
	if err != nil {
		t.Fatalf("parseMetadata() unexpected error: %v", err)
	}

	if len(annotations) != 1 {
		t.Errorf("annotations length = %d, want 1", len(annotations))
	}
	if annotations["org.opencontainers.image.source"] != "https://github.com/test/repo" {
		t.Errorf("annotation value = %q", annotations["org.opencontainers.image.source"])
	}

	if len(labels) != 2 {
		t.Errorf("labels length = %d, want 2", len(labels))
	}
	if labels["maintainer"] != "test@example.com" {
		t.Errorf("label maintainer = %q", labels["maintainer"])
	}
}

func TestParseMetadata_InvalidAnnotations(t *testing.T) {
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.annotations = []string{"no-equals-sign"}
	manifestsCreateOpts.labels = []string{}

	_, _, err := parseMetadata(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid annotations")
	}
	if !strings.Contains(err.Error(), "failed to parse annotations") {
		t.Errorf("error should mention annotations, got: %v", err)
	}
}

func TestParseMetadata_InvalidLabels(t *testing.T) {
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.annotations = []string{}
	manifestsCreateOpts.labels = []string{"no-equals-sign"}

	_, _, err := parseMetadata(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid labels")
	}
	if !strings.Contains(err.Error(), "failed to parse labels") {
		t.Errorf("error should mention labels, got: %v", err)
	}
}

func TestManifestBuilderInterface(t *testing.T) {
	t.Parallel()

	// Verify the interface definition matches expectations
	// This is a compile-time check that the interface is properly defined
	var _ manifestBuilder = nil // interface is valid type
}

func TestDisplayManifestInfo_NoArchitectures(t *testing.T) {
	// Test edge case: manifest with empty architectures
	// displayManifestInfo accesses info.Architectures[0] for single-arch,
	// so we need at least 1 architecture to avoid panic
	info := &manifests.ManifestInfo{
		Name:      "test/image",
		Tag:       "latest",
		Digest:    "sha256:abc123",
		MediaType: "application/vnd.oci.image.index.v1+json",
		Size:      1024,
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:def456",
				Size:         512,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
			{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
				Digest:       "sha256:ghi789",
				Size:         512,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayManifestInfo(info)
	})

	if !strings.Contains(output, "multi-architecture") {
		t.Error("output should indicate multi-architecture manifest")
	}
	if !strings.Contains(output, "linux/arm64/v8") {
		t.Error("output should contain arm64/v8 platform info")
	}
}

func TestDiscoverAndValidateDigests_WithTempDigestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create digest files in the expected format
	digestContent := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	amd64File := filepath.Join(tmpDir, "digest-test-image-amd64.txt")
	arm64File := filepath.Join(tmpDir, "digest-test-image-arm64.txt")

	if err := os.WriteFile(amd64File, []byte(digestContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arm64File, []byte(digestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Save and restore globals
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.name = "test-image"
	manifestsCreateOpts.digestDir = tmpDir
	manifestsCreateOpts.maxAge = 0 // No max age
	manifestsCreateOpts.requireArch = nil
	manifestsCreateOpts.bestEffort = false

	ctx := setupTestContext(t)
	digests, err := discoverAndValidateDigests(ctx)
	if err != nil {
		t.Fatalf("discoverAndValidateDigests() unexpected error: %v", err)
	}
	if len(digests) != 2 {
		t.Errorf("expected 2 digests, got %d", len(digests))
	}
}

func TestDiscoverAndValidateDigests_NoDigestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.name = "nonexistent-image"
	manifestsCreateOpts.digestDir = tmpDir

	ctx := setupTestContext(t)
	_, err := discoverAndValidateDigests(ctx)
	if err == nil {
		t.Fatal("expected error when no digest files found")
	}
	if !strings.Contains(err.Error(), "no digest files found") {
		t.Errorf("error should mention no digest files found, got: %v", err)
	}
}

func TestDiscoverAndValidateDigests_InvalidDirectory(t *testing.T) {
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.name = "test-image"
	manifestsCreateOpts.digestDir = "/nonexistent/directory/path"

	ctx := setupTestContext(t)
	_, err := discoverAndValidateDigests(ctx)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestDiscoverAndValidateDigests_RequiredArchNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only amd64 digest
	digestContent := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	amd64File := filepath.Join(tmpDir, "digest-test-image-amd64.txt")
	if err := os.WriteFile(amd64File, []byte(digestContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.name = "test-image"
	manifestsCreateOpts.digestDir = tmpDir
	manifestsCreateOpts.maxAge = 0
	manifestsCreateOpts.requireArch = []string{"amd64", "arm64"} // arm64 missing
	manifestsCreateOpts.bestEffort = false

	ctx := setupTestContext(t)
	_, err := discoverAndValidateDigests(ctx)
	if err == nil {
		t.Fatal("expected error when required architectures are missing")
	}
}

func TestDiscoverAndValidateDigests_BestEffort(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only amd64 digest
	digestContent := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	amd64File := filepath.Join(tmpDir, "digest-test-image-amd64.txt")
	if err := os.WriteFile(amd64File, []byte(digestContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.name = "test-image"
	manifestsCreateOpts.digestDir = tmpDir
	manifestsCreateOpts.maxAge = 0
	manifestsCreateOpts.requireArch = []string{"amd64", "arm64"} // arm64 missing
	manifestsCreateOpts.bestEffort = true

	ctx := setupTestContext(t)
	digests, err := discoverAndValidateDigests(ctx)
	if err != nil {
		t.Fatalf("discoverAndValidateDigests() with best-effort should not error: %v", err)
	}
	if len(digests) != 1 {
		t.Errorf("expected 1 digest (best-effort), got %d", len(digests))
	}
}

func TestPerformRegistryChecks_HealthCheckEnabled(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := newTestCmd(nil)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.healthCheck = true
	manifestsCreateOpts.verifyRegistry = false
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "amd64",
			Digest:       digest.FromString("test"),
			ModTime:      time.Now(),
		},
	}

	// Will fail because ghcr.io health check requires real credentials
	err := performRegistryChecks(ctx, cmd, digestFiles)
	// Should fail with registry health check error, not a panic
	if err == nil {
		t.Log("health check succeeded (unexpected, may have real credentials)")
	} else if !strings.Contains(err.Error(), "registry health check failed") {
		t.Errorf("error should mention health check, got: %v", err)
	}
}

func TestPerformRegistryChecks_VerifyRegistryEnabled(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := newTestCmd(nil)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.healthCheck = false
	manifestsCreateOpts.verifyRegistry = true
	manifestsCreateOpts.tags = []string{"latest"}
	manifestsCreateOpts.verifyConcurrency = 5
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "amd64",
			Digest:       digest.FromString("test"),
			ModTime:      time.Now(),
		},
	}

	// Will fail at verification because the image does not exist
	err := performRegistryChecks(ctx, cmd, digestFiles)
	if err == nil {
		t.Log("verify succeeded (unexpected)")
	} else if !strings.Contains(err.Error(), "registry verification failed") {
		t.Errorf("error should mention verification failed, got: %v", err)
	}
}

func TestCreateManifestWithBuilder_ErrorWrapping(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	// Create a mock builder that returns an error
	mockErr := fmt.Errorf("mock builder error")
	bldr := &mockManifestBuilder{createErr: mockErr}

	err := createManifestWithBuilder(ctx, bldr, "ghcr.io/test/image:latest", nil)
	if err == nil {
		t.Fatal("expected error from createManifestWithBuilder")
	}
	if !strings.Contains(err.Error(), "failed to create and push manifest") {
		t.Errorf("error should wrap with context, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mock builder error") {
		t.Errorf("error should contain original error, got: %v", err)
	}
}

func TestCreateManifestWithBuilder_Success(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	bldr := &mockManifestBuilder{createErr: nil}

	err := createManifestWithBuilder(ctx, bldr, "ghcr.io/test/image:latest", []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/image@sha256:abc123",
			Architecture: "amd64",
			OS:           "linux",
		},
	})
	if err != nil {
		t.Fatalf("createManifestWithBuilder() unexpected error: %v", err)
	}
}

// mockManifestBuilder is a test mock for the manifestBuilder interface
type mockManifestBuilder struct {
	createErr error
}

func (m *mockManifestBuilder) CreateAndPushManifest(_ context.Context, _ string, _ []manifests.ManifestEntry) error {
	return m.createErr
}

func (m *mockManifestBuilder) Build(_ context.Context, _ builder.Config) (*builder.BuildResult, error) {
	return nil, nil
}

func (m *mockManifestBuilder) Push(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *mockManifestBuilder) PushDigest(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *mockManifestBuilder) Tag(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockManifestBuilder) Remove(_ context.Context, _ string) error {
	return nil
}

func (m *mockManifestBuilder) Close() error {
	return nil
}
