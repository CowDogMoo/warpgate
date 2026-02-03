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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/opencontainers/go-digest"
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
	t.Parallel()

	t.Run("empty annotations and labels", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
	t.Parallel()

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
