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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/manifests"
)

func TestDisplayManifestInfo_MultiArch(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:      "my-image",
		Tag:       "latest",
		Digest:    "sha256:abc123",
		MediaType: "application/vnd.oci.image.index.v1+json",
		Size:      2048,
		Annotations: map[string]string{
			"org.opencontainers.image.created": "2025-01-01",
		},
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:amd64digest",
				Size:         1024,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
			{
				OS:           "linux",
				Architecture: "arm64",
				Digest:       "sha256:arm64digest",
				Size:         1024,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
		},
	}

	output := captureStdout(t, func() {
		displayManifestInfo(info)
	})

	expectations := []string{
		"Name:         my-image",
		"Tag:          latest",
		"sha256:abc123",
		"multi-architecture manifest",
		"2048 bytes",
		"Annotations:",
		"org.opencontainers.image.created",
		"Architectures (2)",
		"linux/amd64",
		"linux/arm64",
		"Manifest Digest:",
	}

	for _, expected := range expectations {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing %q", expected)
		}
	}
}

func TestDisplayManifestInfo_SingleArch(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:      "my-image",
		Tag:       "v1.0",
		Digest:    "sha256:single123",
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Size:      512,
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:configdigest",
				Size:         256,
				MediaType:    "application/vnd.docker.container.image.v1+json",
			},
		},
	}

	output := captureStdout(t, func() {
		displayManifestInfo(info)
	})

	expectations := []string{
		"Name:         my-image",
		"Tag:          v1.0",
		"single-architecture manifest",
		"Platform",
		"linux/amd64",
		"Config Digest:",
	}

	for _, expected := range expectations {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing %q", expected)
		}
	}

	// Should NOT contain multi-arch indicators
	if strings.Contains(output, "Architectures (") {
		t.Error("single-arch should not show Architectures section header")
	}
}

func TestDisplayManifestInfo_WithVariant(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:      "my-image",
		Tag:       "latest",
		Digest:    "sha256:variant123",
		MediaType: "application/vnd.oci.image.index.v1+json",
		Size:      1024,
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "arm",
				Variant:      "v7",
				Digest:       "sha256:armv7digest",
				Size:         512,
			},
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:amd64digest",
				Size:         512,
			},
		},
	}

	output := captureStdout(t, func() {
		displayManifestInfo(info)
	})

	if !strings.Contains(output, "linux/arm/v7") {
		t.Error("output missing linux/arm/v7 variant")
	}
}

func TestDisplayManifestInfo_NoAnnotations(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:      "my-image",
		Tag:       "latest",
		Digest:    "sha256:noannotations",
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Size:      256,
		Architectures: []manifests.ArchitectureInfo{
			{OS: "linux", Architecture: "amd64", Digest: "sha256:d", Size: 128},
		},
	}

	output := captureStdout(t, func() {
		displayManifestInfo(info)
	})

	if strings.Contains(output, "Annotations:") {
		t.Error("should not show annotations section when empty")
	}
}
