/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCacheDir(t *testing.T) {
	t.Parallel()

	t.Run("creates directory with subdirectory", func(t *testing.T) {
		t.Parallel()

		result, err := GetCacheDir("test-subdir")
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "test-subdir")

		// Verify the directory was created
		info, err := os.Stat(result)
		require.NoError(t, err)
		assert.True(t, info.IsDir())

		// Clean up
		_ = os.RemoveAll(result)
	})

	t.Run("returned path ends with subdirectory", func(t *testing.T) {
		t.Parallel()

		result, err := GetCacheDir("my-cache")
		require.NoError(t, err)
		assert.Equal(t, "my-cache", filepath.Base(result))

		_ = os.RemoveAll(result)
	})
}
