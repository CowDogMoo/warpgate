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

package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyDir(t *testing.T) {
	t.Parallel()

	t.Run("copies directory structure and files", func(t *testing.T) {
		t.Parallel()
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dst")

		// Create source directory structure
		require.NoError(t, os.MkdirAll(filepath.Join(src, "subdir"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(src, "file1.txt"), []byte("content1"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(src, "subdir", "file2.txt"), []byte("content2"), 0644))

		err := copyDir(context.Background(), src, dst)
		require.NoError(t, err)

		// Verify copied files
		content1, err := os.ReadFile(filepath.Join(dst, "file1.txt"))
		require.NoError(t, err)
		assert.Equal(t, "content1", string(content1))

		content2, err := os.ReadFile(filepath.Join(dst, "subdir", "file2.txt"))
		require.NoError(t, err)
		assert.Equal(t, "content2", string(content2))
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dst")

		require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0644))

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := copyDir(ctx, src, dst)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "canceled")
	})

	t.Run("copies empty directory", func(t *testing.T) {
		t.Parallel()
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dst")

		err := copyDir(context.Background(), src, dst)
		require.NoError(t, err)

		assert.DirExists(t, dst)
	})
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	t.Run("copies file content and mode", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "src.txt")
		dst := filepath.Join(tmpDir, "dst.txt")

		require.NoError(t, os.WriteFile(src, []byte("hello world"), 0644))

		err := copyFile(context.Background(), src, dst, 0755)
		require.NoError(t, err)

		content, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))

		info, err := os.Stat(dst)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "src.txt")
		dst := filepath.Join(tmpDir, "dst.txt")

		require.NoError(t, os.WriteFile(src, []byte("content"), 0644))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := copyFile(ctx, src, dst, 0644)
		assert.Error(t, err)
	})

	t.Run("returns error for nonexistent source", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		dst := filepath.Join(tmpDir, "dst.txt")

		err := copyFile(context.Background(), "/nonexistent/file", dst, 0644)
		assert.Error(t, err)
	})
}

func TestFetchSourcesWithCleanup_NoSources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cleanup, err := FetchSourcesWithCleanup(ctx, nil, "")
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	// Should be a no-op
	cleanup()
}
