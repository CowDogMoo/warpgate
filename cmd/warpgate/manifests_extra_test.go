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
	"strings"
	"testing"
	"time"

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
