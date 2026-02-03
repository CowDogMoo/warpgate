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

package manifests

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
)

func TestIsMultiArchMediaType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mediaType types.MediaType
		want      bool
	}{
		{"OCI index is multi-arch", types.OCIManifestSchema1, true},
		{"Docker manifest list is multi-arch", types.DockerManifestList, true},
		{"Docker manifest schema2 is not multi-arch", types.DockerManifestSchema2, false},
		{"OCI manifest is not multi-arch", types.OCIManifestSchema1, true},
		{"empty media type is not multi-arch", types.MediaType(""), false},
		{"random string is not multi-arch", types.MediaType("application/json"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isMultiArchMediaType(tt.mediaType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInspectOptions(t *testing.T) {
	t.Parallel()

	opts := InspectOptions{
		Registry:  "ghcr.io",
		Namespace: "myorg",
		ImageName: "myimage",
		Tag:       "latest",
	}

	assert.Equal(t, "ghcr.io", opts.Registry)
	assert.Equal(t, "myorg", opts.Namespace)
	assert.Equal(t, "myimage", opts.ImageName)
	assert.Equal(t, "latest", opts.Tag)
}

func TestListOptions(t *testing.T) {
	t.Parallel()

	opts := ListOptions{
		Registry:  "docker.io",
		Namespace: "library",
		ImageName: "nginx",
	}

	assert.Equal(t, "docker.io", opts.Registry)
	assert.Equal(t, "library", opts.Namespace)
	assert.Equal(t, "nginx", opts.ImageName)
}

func TestManifestInfo(t *testing.T) {
	t.Parallel()

	info := ManifestInfo{
		Name:      "myimage",
		Tag:       "latest",
		Digest:    "sha256:abc123",
		MediaType: "application/vnd.docker.distribution.manifest.list.v2+json",
		Size:      1234,
		Annotations: map[string]string{
			"org.opencontainers.image.created": "2025-01-01",
		},
		Architectures: []ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:def456",
				Size:         5678,
				MediaType:    "application/vnd.docker.distribution.manifest.v2+json",
			},
		},
	}

	assert.Equal(t, "myimage", info.Name)
	assert.Len(t, info.Architectures, 1)
	assert.Equal(t, "linux", info.Architectures[0].OS)
	assert.Equal(t, "amd64", info.Architectures[0].Architecture)
}

func TestArchitectureInfo(t *testing.T) {
	t.Parallel()

	t.Run("with variant", func(t *testing.T) {
		t.Parallel()
		info := ArchitectureInfo{
			OS:           "linux",
			Architecture: "arm",
			Variant:      "v7",
			Digest:       "sha256:abc",
			Size:         100,
		}
		assert.Equal(t, "v7", info.Variant)
		assert.Equal(t, "arm", info.Architecture)
	})

	t.Run("without variant", func(t *testing.T) {
		t.Parallel()
		info := ArchitectureInfo{
			OS:           "linux",
			Architecture: "amd64",
			Digest:       "sha256:abc",
			Size:         100,
		}
		assert.Empty(t, info.Variant)
	})
}
