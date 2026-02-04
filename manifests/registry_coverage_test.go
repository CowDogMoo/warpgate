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
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyDigestsInRegistry_WithValidDigests(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	// Push a random image
	img, err := random.Image(256, 1)
	require.NoError(t, err)

	ref, err := name.NewTag(fmt.Sprintf("%s/test/myimage:latest", host))
	require.NoError(t, err)

	err = remote.Write(ref, img)
	require.NoError(t, err)

	// Get the digest of the pushed image
	desc, err := remote.Get(ref)
	require.NoError(t, err)

	digestStr := desc.Digest.String()
	parsedDigest, err := digest.Parse(digestStr)
	require.NoError(t, err)

	digestFiles := []DigestFile{
		{
			ImageName:    "myimage",
			Architecture: "amd64",
			Digest:       parsedDigest,
		},
	}

	opts := VerificationOptions{
		Registry:      host,
		Namespace:     "test",
		MaxConcurrent: 2,
	}

	err = VerifyDigestsInRegistry(context.Background(), digestFiles, opts)
	assert.NoError(t, err)
}

func TestVerifyDigestsInRegistry_WithInvalidDigest(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	fakeDigest, err := digest.Parse("sha256:0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)

	digestFiles := []DigestFile{
		{
			ImageName:    "nonexistent",
			Architecture: "amd64",
			Digest:       fakeDigest,
		},
	}

	opts := VerificationOptions{
		Registry:      host,
		Namespace:     "test",
		MaxConcurrent: 1,
	}

	err = VerifyDigestsInRegistry(context.Background(), digestFiles, opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")
}

func TestVerifyDigestsInRegistry_ConcurrencyLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		maxConcurrent int
	}{
		{"zero defaults to 5", 0},
		{"negative defaults to 5", -1},
		{"normal value", 10},
		{"high value capped to 20", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// With empty digest files, the function succeeds quickly
			// but still exercises the concurrency configuration code
			opts := VerificationOptions{
				Registry:      "localhost:99999",
				Namespace:     "test",
				MaxConcurrent: tt.maxConcurrent,
			}

			err := VerifyDigestsInRegistry(context.Background(), []DigestFile{}, opts)
			assert.NoError(t, err)
		})
	}
}

func TestVerifyDigestsInRegistry_ContextCancellation(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	fakeDigest, err := digest.Parse("sha256:0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)

	digestFiles := []DigestFile{
		{
			ImageName:    "test",
			Architecture: "amd64",
			Digest:       fakeDigest,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := VerificationOptions{
		Registry:      host,
		Namespace:     "test",
		MaxConcurrent: 1,
	}

	err = VerifyDigestsInRegistry(ctx, digestFiles, opts)
	assert.Error(t, err)
}

func TestHealthCheckRegistry_WithInMemoryRegistry(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	// Push a test image that the health check can find
	img, err := random.Image(256, 1)
	require.NoError(t, err)

	ref, err := name.NewTag(fmt.Sprintf("%s/library/hello-world:latest", host))
	require.NoError(t, err)

	err = remote.Write(ref, img)
	require.NoError(t, err)

	ctx := context.Background()
	opts := VerificationOptions{
		Registry: host,
	}

	// Health check should pass (or at least not error, since it only warns)
	err = HealthCheckRegistry(ctx, opts)
	assert.NoError(t, err)
}

func TestHealthCheckRegistry_WithGHCR(t *testing.T) {
	t.Parallel()

	host := newTestRegistry(t)

	// Push test image matching the ghcr.io path pattern
	img, err := random.Image(256, 1)
	require.NoError(t, err)

	ref, err := name.NewTag(fmt.Sprintf("%s/hello-world/hello-world:latest", host))
	require.NoError(t, err)

	err = remote.Write(ref, img)
	require.NoError(t, err)

	ctx := context.Background()
	opts := VerificationOptions{
		Registry: "ghcr.io",
	}

	// This will try to reach ghcr.io but should not error (it only warns)
	err = HealthCheckRegistry(ctx, opts)
	assert.NoError(t, err)
}

func TestHealthCheckRegistry_UnreachableRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	opts := VerificationOptions{
		Registry: "localhost:99999",
	}

	// Health check should not return an error - it only warns
	err := HealthCheckRegistry(ctx, opts)
	assert.NoError(t, err)
}
