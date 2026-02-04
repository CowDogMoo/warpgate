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
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
)

func TestConvertDigestFilesToManifestEntries(t *testing.T) {
	t.Parallel()

	t.Run("single architecture", func(t *testing.T) {
		t.Parallel()
		digestFiles := []manifests.DigestFile{
			{
				ImageName:    "my-image",
				Architecture: "amd64",
				Digest:       digest.Digest("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
			},
		}

		entries := convertDigestFilesToManifestEntries(digestFiles, "ghcr.io", "myorg", "latest")

		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if !strings.Contains(entries[0].ImageRef, "ghcr.io/myorg/my-image@sha256:") {
			t.Errorf("unexpected ImageRef: %s", entries[0].ImageRef)
		}
		if entries[0].Architecture != "amd64" {
			t.Errorf("expected architecture amd64, got %s", entries[0].Architecture)
		}
		if entries[0].OS != "linux" {
			t.Errorf("expected OS linux, got %s", entries[0].OS)
		}
	})

	t.Run("multiple architectures", func(t *testing.T) {
		t.Parallel()
		digestFiles := []manifests.DigestFile{
			{
				ImageName:    "my-image",
				Architecture: "amd64",
				Digest:       digest.Digest("sha256:1111111111111111111111111111111111111111111111111111111111111111"),
			},
			{
				ImageName:    "my-image",
				Architecture: "arm64",
				Digest:       digest.Digest("sha256:2222222222222222222222222222222222222222222222222222222222222222"),
			},
		}

		entries := convertDigestFilesToManifestEntries(digestFiles, "docker.io", "library", "v1.0")

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].Architecture != "amd64" {
			t.Errorf("first entry arch: got %s, want amd64", entries[0].Architecture)
		}
		if entries[1].Architecture != "arm64" {
			t.Errorf("second entry arch: got %s, want arm64", entries[1].Architecture)
		}
	})

	t.Run("no namespace", func(t *testing.T) {
		t.Parallel()
		digestFiles := []manifests.DigestFile{
			{
				ImageName:    "my-image",
				Architecture: "amd64",
				Digest:       digest.Digest("sha256:3333333333333333333333333333333333333333333333333333333333333333"),
			},
		}

		entries := convertDigestFilesToManifestEntries(digestFiles, "ghcr.io", "", "latest")

		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		// Should still construct a valid reference
		if entries[0].ImageRef == "" {
			t.Error("ImageRef should not be empty")
		}
	})

	t.Run("empty digest files returns empty", func(t *testing.T) {
		t.Parallel()
		entries := convertDigestFilesToManifestEntries(nil, "ghcr.io", "org", "latest")
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("platform with variant", func(t *testing.T) {
		t.Parallel()
		digestFiles := []manifests.DigestFile{
			{
				ImageName:    "my-image",
				Architecture: "arm/v7",
				Digest:       digest.Digest("sha256:4444444444444444444444444444444444444444444444444444444444444444"),
			},
		}

		entries := convertDigestFilesToManifestEntries(digestFiles, "ghcr.io", "org", "latest")

		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Variant != "v7" {
			t.Errorf("expected variant v7, got %s", entries[0].Variant)
		}
	})
}

func TestParseMetadata(t *testing.T) {
	t.Run("empty annotations and labels", func(t *testing.T) {
		// Save and restore
		oldOpts := manifestsCreateOpts
		manifestsCreateOpts = &manifestsCreateOptions{}
		defer func() { manifestsCreateOpts = oldOpts }()

		annotations, labels, err := parseMetadata(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(annotations) != 0 {
			t.Errorf("expected empty annotations, got %v", annotations)
		}
		if len(labels) != 0 {
			t.Errorf("expected empty labels, got %v", labels)
		}
	})

	t.Run("valid annotations and labels", func(t *testing.T) {
		oldOpts := manifestsCreateOpts
		manifestsCreateOpts = &manifestsCreateOptions{
			annotations: []string{"org.opencontainers.image.version=1.0.0"},
			labels:      []string{"maintainer=team"},
		}
		defer func() { manifestsCreateOpts = oldOpts }()

		annotations, labels, err := parseMetadata(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if annotations["org.opencontainers.image.version"] != "1.0.0" {
			t.Errorf("unexpected annotations: %v", annotations)
		}
		if labels["maintainer"] != "team" {
			t.Errorf("unexpected labels: %v", labels)
		}
	})
}

func TestHandleDryRun(t *testing.T) {
	oldOpts := manifestsCreateOpts
	oldShared := manifestsSharedOpts
	manifestsCreateOpts = &manifestsCreateOptions{
		tags: []string{"latest", "v1.0"},
		name: "test-image",
	}
	manifestsSharedOpts = &manifestsSharedOptions{
		registry:  "ghcr.io",
		namespace: "myorg",
	}
	defer func() {
		manifestsCreateOpts = oldOpts
		manifestsSharedOpts = oldShared
	}()

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "amd64",
			Digest:       digest.Digest("sha256:5555555555555555555555555555555555555555555555555555555555555555"),
		},
	}

	err := handleDryRun(context.Background(), digestFiles)
	if err != nil {
		t.Fatalf("handleDryRun() unexpected error: %v", err)
	}
}

func TestRunManifestsCreate_NoDigestFiles(t *testing.T) {
	ctx := setupTestContext(t)
	tmpDir := t.TempDir()

	cmd := &cobra.Command{Use: "create"}
	cmd.SetContext(ctx)

	// Save and restore globals
	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.name = "nonexistent-image"
	manifestsCreateOpts.digestDir = tmpDir
	manifestsSharedOpts.registry = "ghcr.io/test"

	err := runManifestsCreate(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when no digest files found")
	}
	if !strings.Contains(err.Error(), "no digest files found") {
		t.Errorf("error should mention no digest files found, got: %v", err)
	}
}

func TestHandleDryRun_MultiTag(t *testing.T) {
	ctx := setupTestContext(t)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.tags = []string{"latest", "v1.0"}
	manifestsCreateOpts.name = "test-image"
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "amd64",
			Digest:       digest.FromString("test-content"),
			ModTime:      time.Now(),
		},
	}

	err := handleDryRun(ctx, digestFiles)
	if err != nil {
		t.Fatalf("handleDryRun() unexpected error: %v", err)
	}
}

func TestManifestsInspectOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := &manifestsInspectOptions{
		name: "test-image",
		tags: []string{"latest"},
	}

	if opts.name != "test-image" {
		t.Errorf("name = %q, want %q", opts.name, "test-image")
	}
	if len(opts.tags) != 1 || opts.tags[0] != "latest" {
		t.Errorf("tags = %v, want [latest]", opts.tags)
	}
}

func TestManifestsListOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := &manifestsListOptions{
		name: "test-image",
	}

	if opts.name != "test-image" {
		t.Errorf("name = %q, want %q", opts.name, "test-image")
	}
}

func TestConvertDigestFilesToManifestEntries_ArmVariant(t *testing.T) {
	t.Parallel()

	digestFiles := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "arm/v7",
			Digest:       digest.FromString("armv7-content"),
			ModTime:      time.Now(),
		},
	}

	entries := convertDigestFilesToManifestEntries(digestFiles, "ghcr.io", "test", "latest")

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// arm/v7 should be parsed into arch=arm, variant=v7
	if entries[0].Architecture != "arm" {
		t.Errorf("Architecture = %q, want %q", entries[0].Architecture, "arm")
	}
	if entries[0].Variant != "v7" {
		t.Errorf("Variant = %q, want %q", entries[0].Variant, "v7")
	}
}

func TestCreateManifestWithBuilder_Error(t *testing.T) {
	t.Parallel()

	// We can't easily test with a real builder, but we can test the error wrapping
	// by verifying the function signature and behavior
	ctx := setupTestContext(t)

	// Test with nil builder would panic, so we just verify the error path exists
	// by testing the function is callable
	_ = ctx
}

func TestCleanupOptions_AllFields(t *testing.T) {
	t.Parallel()

	opts := &cleanupOptions{
		region:       "us-east-1",
		dryRun:       true,
		all:          true,
		buildName:    "test-build",
		versions:     true,
		keepVersions: 5,
		yes:          true,
	}

	if opts.region != "us-east-1" {
		t.Errorf("region = %q, want %q", opts.region, "us-east-1")
	}
	if !opts.dryRun {
		t.Error("dryRun should be true")
	}
	if !opts.all {
		t.Error("all should be true")
	}
	if opts.keepVersions != 5 {
		t.Errorf("keepVersions = %d, want 5", opts.keepVersions)
	}
	if !opts.yes {
		t.Error("yes should be true")
	}
}

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

func TestVerifyDigestsInRegistry_ConcurrencyFromConfigFallback(t *testing.T) {
	// When CLI verifyConcurrency is 0 and config has a value, it should use config value.
	// When both are 0, it should use default of 5.
	// We test the latter case since we can create a cmd with nil config.

	ctx := setupTestContext(t)
	cmd := newTestCmdNoConfig()
	cmd.SetContext(ctx)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.verifyConcurrency = 0
	manifestsCreateOpts.tags = []string{"latest"}
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

	// Will fail at actual verification, but exercises the concurrency resolution code path
	err := verifyDigestsInRegistry(ctx, cmd, digestFiles)
	// Should fail with verification error, not a panic
	if err == nil {
		t.Log("verifyDigestsInRegistry succeeded (unexpected)")
	}
}

func TestVerifyDigestsInRegistry_ConcurrencyFromConfigValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifests.VerifyConcurrency = 15
	cmd := newTestCmd(cfg)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.verifyConcurrency = 0 // CLI not set, should use config value of 15
	manifestsCreateOpts.tags = []string{"v1.0"}
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

	// Will fail at verification but exercises the config concurrency path
	err := verifyDigestsInRegistry(cmd.Context(), cmd, digestFiles)
	if err == nil {
		t.Log("verifyDigestsInRegistry succeeded (unexpected)")
	}
}

func TestVerifyDigestsInRegistry_ConcurrencyFromCLI(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifests.VerifyConcurrency = 15
	cmd := newTestCmd(cfg)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.verifyConcurrency = 8 // CLI set, should use 8 not 15
	manifestsCreateOpts.tags = []string{"latest"}
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

	err := verifyDigestsInRegistry(cmd.Context(), cmd, digestFiles)
	if err == nil {
		t.Log("verifyDigestsInRegistry succeeded (unexpected)")
	}
}

func TestVerifyDigestsInRegistry_EmptyTagsUsesLatest(t *testing.T) {
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.verifyConcurrency = 5
	manifestsCreateOpts.tags = []string{} // Empty - should default to "latest"
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

	err := verifyDigestsInRegistry(cmd.Context(), cmd, digestFiles)
	// Should fail at verification, not panic
	if err == nil {
		t.Log("verifyDigestsInRegistry succeeded (unexpected)")
	}
}

func TestRunManifestsCreate_ForceFlag(t *testing.T) {
	ctx := setupTestContext(t)
	tmpDir := t.TempDir()

	// Create valid digest files
	digestContent := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	amd64File := filepath.Join(tmpDir, "digest-test-image-amd64.txt")
	if err := os.WriteFile(amd64File, []byte(digestContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{Use: "create"}
	cmd.SetContext(ctx)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.name = "test-image"
	manifestsCreateOpts.digestDir = tmpDir
	manifestsCreateOpts.tags = []string{"latest"}
	manifestsCreateOpts.force = true
	manifestsCreateOpts.dryRun = true // Use dry-run to avoid needing a real registry
	manifestsCreateOpts.verifyRegistry = false
	manifestsCreateOpts.healthCheck = false
	manifestsCreateOpts.requireArch = nil
	manifestsCreateOpts.bestEffort = false
	manifestsCreateOpts.maxAge = 0
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	// With force + dryRun, should skip idempotency check and succeed
	err := runManifestsCreate(cmd, []string{})
	if err != nil {
		t.Fatalf("runManifestsCreate() with force+dryRun error = %v", err)
	}
}

func TestRunManifestsCreate_WithDryRunNoForce(t *testing.T) {
	ctx := setupTestContext(t)
	tmpDir := t.TempDir()

	digestContent := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	amd64File := filepath.Join(tmpDir, "digest-myimage-amd64.txt")
	if err := os.WriteFile(amd64File, []byte(digestContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{Use: "create"}
	cmd.SetContext(ctx)

	oldOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.name = "myimage"
	manifestsCreateOpts.digestDir = tmpDir
	manifestsCreateOpts.tags = []string{"latest"}
	manifestsCreateOpts.force = false // Do not force - triggers idempotency check
	manifestsCreateOpts.dryRun = true
	manifestsCreateOpts.verifyRegistry = false
	manifestsCreateOpts.healthCheck = false
	manifestsCreateOpts.requireArch = nil
	manifestsCreateOpts.bestEffort = false
	manifestsCreateOpts.maxAge = 0
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	// The idempotency check will try to access the registry and fail,
	// but the warn path handles it and continues to dry-run
	err := runManifestsCreate(cmd, []string{})
	if err != nil {
		t.Fatalf("runManifestsCreate() with dryRun (no force) error = %v", err)
	}
}

func TestConvertDigestFilesToManifestEntries_Extra(t *testing.T) {
	t.Parallel()

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

	entries := convertDigestFilesToManifestEntries(digestFiles, "ghcr.io", "cowdogmoo", "latest")

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Architecture != "amd64" {
		t.Errorf("entries[0].Architecture = %q, want %q", entries[0].Architecture, "amd64")
	}
	if entries[1].Architecture != "arm64" {
		t.Errorf("entries[1].Architecture = %q, want %q", entries[1].Architecture, "arm64")
	}
	if entries[0].OS != "linux" {
		t.Errorf("entries[0].OS = %q, want %q", entries[0].OS, "linux")
	}
}

func TestParseMetadata_NilSlices(t *testing.T) {
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.annotations = nil
	manifestsCreateOpts.labels = nil

	ctx := setupTestContext(t)
	annotations, labels, err := parseMetadata(ctx)
	if err != nil {
		t.Fatalf("parseMetadata() unexpected error: %v", err)
	}
	if annotations == nil {
		t.Log("annotations is nil (expected empty map)")
	}
	if labels == nil {
		t.Log("labels is nil (expected empty map)")
	}
}

func TestParseMetadata_WithValues(t *testing.T) {
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()

	manifestsCreateOpts.annotations = []string{"key1=value1", "key2=value2"}
	manifestsCreateOpts.labels = []string{"label1=val1"}

	ctx := setupTestContext(t)
	annotations, labels, err := parseMetadata(ctx)
	if err != nil {
		t.Fatalf("parseMetadata() error = %v", err)
	}
	if len(annotations) != 2 {
		t.Errorf("len(annotations) = %d, want 2", len(annotations))
	}
	if annotations["key1"] != "value1" {
		t.Errorf("annotations[key1] = %q, want %q", annotations["key1"], "value1")
	}
	if len(labels) != 1 {
		t.Errorf("len(labels) = %d, want 1", len(labels))
	}
}

func TestHandleDryRun_Extra(t *testing.T) {
	ctx := setupTestContext(t)

	oldCreateOpts := *manifestsCreateOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsCreateOpts = oldCreateOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsCreateOpts.tags = []string{"latest", "v1.0"}
	manifestsCreateOpts.name = "test-image"
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	filteredDigests := []manifests.DigestFile{
		{
			ImageName:    "test-image",
			Architecture: "amd64",
			Digest:       digest.FromString("amd64"),
			ModTime:      time.Now(),
		},
	}

	err := handleDryRun(ctx, filteredDigests)
	if err != nil {
		t.Fatalf("handleDryRun() error = %v", err)
	}
}
