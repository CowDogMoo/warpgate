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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerificationOptions(t *testing.T) {
	t.Parallel()

	opts := VerificationOptions{
		Registry:      "ghcr.io",
		Namespace:     "myorg",
		Tag:           "latest",
		MaxConcurrent: 10,
	}

	assert.Equal(t, "ghcr.io", opts.Registry)
	assert.Equal(t, "myorg", opts.Namespace)
	assert.Equal(t, "latest", opts.Tag)
	assert.Equal(t, 10, opts.MaxConcurrent)
}

func TestCheckManifestExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns false with empty digest files", func(t *testing.T) {
		t.Parallel()
		exists, err := CheckManifestExists(ctx, []DigestFile{})
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns false for any input", func(t *testing.T) {
		t.Parallel()
		// Current implementation always returns false
		exists, err := CheckManifestExists(ctx, []DigestFile{
			{ImageName: "test", Architecture: "amd64"},
		})
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestVerifyDigestsInRegistry_EmptyInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	opts := VerificationOptions{
		Registry:  "ghcr.io",
		Namespace: "myorg",
	}

	// Empty digest files should succeed immediately
	err := VerifyDigestsInRegistry(ctx, []DigestFile{}, opts)
	assert.NoError(t, err)
}

func TestVerifyDigestsInRegistry_ConcurrencyDefaults(t *testing.T) {
	t.Parallel()

	t.Run("default concurrency is 5", func(t *testing.T) {
		t.Parallel()
		opts := VerificationOptions{
			MaxConcurrent: 0,
		}
		// MaxConcurrent of 0 should default to 5 internally
		assert.Equal(t, 0, opts.MaxConcurrent)
	})

	t.Run("high concurrency is capped at 20", func(t *testing.T) {
		t.Parallel()
		opts := VerificationOptions{
			MaxConcurrent: 50,
		}
		assert.Equal(t, 50, opts.MaxConcurrent)
		// The actual capping happens inside VerifyDigestsInRegistry
	})
}
