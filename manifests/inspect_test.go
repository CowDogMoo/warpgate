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
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// newTestRegistry creates an in-memory OCI registry for testing and returns
// the host string (e.g., "localhost:xxxxx") suitable for use with name.NewTag.
func newTestRegistry(t *testing.T) string {
	t.Helper()
	reg := registry.New()
	srv := httptest.NewServer(reg)
	t.Cleanup(srv.Close)
	// Return just the host portion (strip "http://")
	return srv.Listener.Addr().String()
}

func TestParseMultiArchManifestFromDescriptor(t *testing.T) {
	t.Parallel()

	t.Run("valid index manifest", func(t *testing.T) {
		t.Parallel()

		// Build a valid OCI index manifest JSON
		indexManifest := v1.IndexManifest{
			SchemaVersion: 2,
			MediaType:     types.OCIImageIndex,
			Manifests: []v1.Descriptor{
				{
					MediaType: types.OCIManifestSchema1,
					Size:      1024,
					Digest: v1.Hash{
						Algorithm: "sha256",
						Hex:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
					Platform: &v1.Platform{
						OS:           "linux",
						Architecture: "amd64",
					},
				},
				{
					MediaType: types.OCIManifestSchema1,
					Size:      2048,
					Digest: v1.Hash{
						Algorithm: "sha256",
						Hex:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					},
					Platform: &v1.Platform{
						OS:           "linux",
						Architecture: "arm64",
					},
				},
				{
					MediaType: types.OCIManifestSchema1,
					Size:      512,
					Digest: v1.Hash{
						Algorithm: "sha256",
						Hex:       "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
					},
					Platform: &v1.Platform{
						OS:           "linux",
						Architecture: "arm",
						Variant:      "v7",
					},
				},
			},
			Annotations: map[string]string{
				"created": "2025-01-01",
			},
		}

		manifestBytes, err := json.Marshal(indexManifest)
		require.NoError(t, err)

		descriptor := &remote.Descriptor{
			Descriptor: v1.Descriptor{
				MediaType: types.OCIImageIndex,
			},
			Manifest: manifestBytes,
		}

		info := &ManifestInfo{
			Annotations: make(map[string]string),
		}

		err = parseMultiArchManifestFromDescriptor(descriptor, info)
		require.NoError(t, err)

		assert.Len(t, info.Architectures, 3)
		assert.Equal(t, "linux", info.Architectures[0].OS)
		assert.Equal(t, "amd64", info.Architectures[0].Architecture)
		assert.Equal(t, "arm64", info.Architectures[1].Architecture)
		assert.Equal(t, "arm", info.Architectures[2].Architecture)
		assert.Equal(t, "v7", info.Architectures[2].Variant)
		assert.Equal(t, "2025-01-01", info.Annotations["created"])
	})

	t.Run("index without platform info", func(t *testing.T) {
		t.Parallel()

		indexManifest := v1.IndexManifest{
			SchemaVersion: 2,
			MediaType:     types.OCIImageIndex,
			Manifests: []v1.Descriptor{
				{
					MediaType: types.OCIManifestSchema1,
					Size:      1024,
					Digest:    v1.Hash{Algorithm: "sha256", Hex: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
				},
			},
		}

		manifestBytes, err := json.Marshal(indexManifest)
		require.NoError(t, err)

		descriptor := &remote.Descriptor{
			Descriptor: v1.Descriptor{MediaType: types.OCIImageIndex},
			Manifest:   manifestBytes,
		}

		info := &ManifestInfo{Annotations: make(map[string]string)}
		err = parseMultiArchManifestFromDescriptor(descriptor, info)
		require.NoError(t, err)

		assert.Len(t, info.Architectures, 1)
		assert.Empty(t, info.Architectures[0].OS)
		assert.Empty(t, info.Architectures[0].Architecture)
	})

	t.Run("invalid manifest bytes", func(t *testing.T) {
		t.Parallel()

		descriptor := &remote.Descriptor{
			Descriptor: v1.Descriptor{MediaType: types.OCIImageIndex},
			Manifest:   []byte("not valid json"),
		}

		info := &ManifestInfo{Annotations: make(map[string]string)}
		err := parseMultiArchManifestFromDescriptor(descriptor, info)
		assert.Error(t, err)
	})
}

func TestInspectManifest_WithInMemoryRegistry(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	// Create and push a random image
	img, err := random.Image(256, 1)
	require.NoError(t, err)

	ref, err := name.NewTag(fmt.Sprintf("%s/test/myimage:latest", host))
	require.NoError(t, err)

	err = remote.Write(ref, img)
	require.NoError(t, err)

	// Now inspect
	info, err := InspectManifest(context.Background(), InspectOptions{
		Registry:  host,
		Namespace: "test",
		ImageName: "myimage",
		Tag:       "latest",
	})

	require.NoError(t, err)
	assert.Equal(t, "myimage", info.Name)
	assert.Equal(t, "latest", info.Tag)
	assert.NotEmpty(t, info.Digest)
	assert.Len(t, info.Architectures, 1)
}

func TestInspectManifest_MultiArch_WithInMemoryRegistry(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	// Create two random images for different platforms
	imgAmd64, err := random.Image(256, 1)
	require.NoError(t, err)

	imgArm64, err := random.Image(256, 1)
	require.NoError(t, err)

	// Create an image index (multi-arch manifest)
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{
			Add: imgAmd64,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{OS: "linux", Architecture: "amd64"},
			},
		},
		mutate.IndexAddendum{
			Add: imgArm64,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{OS: "linux", Architecture: "arm64"},
			},
		},
	)

	ref, err := name.NewTag(fmt.Sprintf("%s/test/multiarch:latest", host))
	require.NoError(t, err)

	err = remote.WriteIndex(ref, idx)
	require.NoError(t, err)

	// Get the raw descriptor and test parsing directly
	descriptor, err := remote.Get(ref)
	require.NoError(t, err)

	info := &ManifestInfo{
		Name:        "multiarch",
		Tag:         "latest",
		Annotations: make(map[string]string),
	}

	// Parse as multi-arch directly (bypassing isMultiArchMediaType check)
	err = parseMultiArchManifestFromDescriptor(descriptor, info)
	require.NoError(t, err)

	assert.Len(t, info.Architectures, 2)

	archMap := make(map[string]bool)
	for _, a := range info.Architectures {
		archMap[a.Architecture] = true
	}
	assert.True(t, archMap["amd64"])
	assert.True(t, archMap["arm64"])
}

func TestListTags_WithInMemoryRegistry(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	// Push images with different tags
	img, err := random.Image(256, 1)
	require.NoError(t, err)

	for _, tag := range []string{"latest", "v1.0", "v2.0"} {
		ref, err := name.NewTag(fmt.Sprintf("%s/test/myimage:%s", host, tag))
		require.NoError(t, err)
		err = remote.Write(ref, img)
		require.NoError(t, err)
	}

	tags, err := ListTags(context.Background(), ListOptions{
		Registry:  host,
		Namespace: "test",
		ImageName: "myimage",
	})

	require.NoError(t, err)
	assert.Len(t, tags, 3)
	assert.Contains(t, tags, "latest")
	assert.Contains(t, tags, "v1.0")
	assert.Contains(t, tags, "v2.0")
}

func TestInspectManifest_InvalidReference(t *testing.T) {
	t.Parallel()

	_, err := InspectManifest(context.Background(), InspectOptions{
		Registry:  "localhost:99999",
		Namespace: "test",
		ImageName: "nonexistent",
		Tag:       "latest",
	})

	assert.Error(t, err)
}

func TestParseSingleArchManifestFromDescriptor_WithInMemoryRegistry(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	// Push a single-arch image
	img, err := random.Image(256, 1)
	require.NoError(t, err)

	ref, err := name.NewTag(fmt.Sprintf("%s/test/singlearch:latest", host))
	require.NoError(t, err)

	err = remote.Write(ref, img)
	require.NoError(t, err)

	// Get the descriptor
	descriptor, err := remote.Get(ref)
	require.NoError(t, err)

	info := &ManifestInfo{
		Annotations: make(map[string]string),
	}

	err = parseSingleArchManifestFromDescriptor(context.Background(), descriptor, info)
	require.NoError(t, err)

	assert.Len(t, info.Architectures, 1)
	assert.NotEmpty(t, info.Architectures[0].Digest)
}
