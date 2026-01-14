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

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSourceFetcher(t *testing.T) {
	t.Run("creates temp directory when baseDir is empty", func(t *testing.T) {
		fetcher, err := NewSourceFetcher("")
		require.NoError(t, err)
		defer func() { _ = fetcher.Cleanup() }()

		assert.NotEmpty(t, fetcher.BaseDir)
		assert.Contains(t, fetcher.BaseDir, "warpgate-sources-")

		// Verify directory exists
		_, err = os.Stat(fetcher.BaseDir)
		assert.NoError(t, err)
	})

	t.Run("uses provided baseDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		fetcher, err := NewSourceFetcher(tmpDir)
		require.NoError(t, err)

		assert.Equal(t, tmpDir, fetcher.BaseDir)
	})
}

func TestSourceFetcher_FetchGitSource(t *testing.T) {
	// Skip in CI if network is not available
	if os.Getenv("CI") != "" && os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test in CI")
	}

	t.Run("clones public repository", func(t *testing.T) {
		ctx := context.Background()
		fetcher, err := NewSourceFetcher("")
		require.NoError(t, err)
		defer func() { _ = fetcher.Cleanup() }()

		sources := []builder.Source{
			{
				Name: "test-repo",
				Git: &builder.GitSource{
					Repository: "https://github.com/octocat/Hello-World.git",
					Depth:      1,
				},
			},
		}

		err = fetcher.FetchSources(ctx, sources)
		require.NoError(t, err)

		// Verify source was fetched
		assert.NotEmpty(t, sources[0].Path)
		assert.DirExists(t, sources[0].Path)

		// Verify git directory exists
		gitDir := filepath.Join(sources[0].Path, ".git")
		assert.DirExists(t, gitDir)
	})

	t.Run("clones with specific ref", func(t *testing.T) {
		ctx := context.Background()
		fetcher, err := NewSourceFetcher("")
		require.NoError(t, err)
		defer func() { _ = fetcher.Cleanup() }()

		sources := []builder.Source{
			{
				Name: "test-repo-ref",
				Git: &builder.GitSource{
					Repository: "https://github.com/octocat/Hello-World.git",
					Ref:        "master",
					Depth:      1,
				},
			},
		}

		err = fetcher.FetchSources(ctx, sources)
		require.NoError(t, err)

		assert.NotEmpty(t, sources[0].Path)
		assert.DirExists(t, sources[0].Path)
	})

	t.Run("fails with invalid repository", func(t *testing.T) {
		ctx := context.Background()
		fetcher, err := NewSourceFetcher("")
		require.NoError(t, err)
		defer func() { _ = fetcher.Cleanup() }()

		sources := []builder.Source{
			{
				Name: "invalid-repo",
				Git: &builder.GitSource{
					Repository: "https://github.com/nonexistent/nonexistent-repo-12345.git",
				},
			},
		}

		err = fetcher.FetchSources(ctx, sources)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid-repo")
	})
}

func TestSourceFetcher_Cleanup(t *testing.T) {
	t.Run("removes temp directory", func(t *testing.T) {
		fetcher, err := NewSourceFetcher("")
		require.NoError(t, err)

		baseDir := fetcher.BaseDir
		assert.DirExists(t, baseDir)

		err = fetcher.Cleanup()
		require.NoError(t, err)

		// Directory should be removed
		_, err = os.Stat(baseDir)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("does not remove non-temp directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		fetcher, err := NewSourceFetcher(tmpDir)
		require.NoError(t, err)

		err = fetcher.Cleanup()
		require.NoError(t, err)

		// Directory should still exist (not a warpgate-sources- temp dir)
		assert.DirExists(t, tmpDir)
	})
}

func TestGetSourcePath(t *testing.T) {
	sources := []builder.Source{
		{Name: "source1", Path: "/path/to/source1"},
		{Name: "source2", Path: "/path/to/source2"},
		{Name: "unfetched", Path: ""},
	}

	t.Run("returns path for existing source", func(t *testing.T) {
		path, err := GetSourcePath(sources, "source1")
		require.NoError(t, err)
		assert.Equal(t, "/path/to/source1", path)
	})

	t.Run("returns error for non-existent source", func(t *testing.T) {
		_, err := GetSourcePath(sources, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for unfetched source", func(t *testing.T) {
		_, err := GetSourcePath(sources, "unfetched")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not been fetched")
	})
}

func TestInjectTokenIntoURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		token    string
		expected string
		wantErr  bool
	}{
		{
			name:     "injects token into HTTPS URL",
			url:      "https://github.com/org/repo.git",
			token:    "ghp_test123",
			expected: "https://x-access-token:ghp_test123@github.com/org/repo.git",
		},
		{
			name:     "does not modify SSH URL",
			url:      "git@github.com:org/repo.git",
			token:    "ghp_test123",
			expected: "git@github.com:org/repo.git",
		},
		{
			name:     "returns original URL when token is empty",
			url:      "https://github.com/org/repo.git",
			token:    "",
			expected: "https://github.com/org/repo.git",
		},
		{
			name:     "handles URL with existing auth",
			url:      "https://user:pass@github.com/org/repo.git",
			token:    "ghp_test123",
			expected: "https://x-access-token:ghp_test123@github.com/org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := InjectTokenIntoURL(tt.url, tt.token)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		setup    func()
		teardown func()
		check    func(t *testing.T, result string)
	}{
		{
			name: "expands tilde",
			path: "~/test",
			check: func(t *testing.T, result string) {
				home, _ := os.UserHomeDir()
				assert.Equal(t, filepath.Join(home, "test"), result)
			},
		},
		{
			name: "expands environment variable",
			path: "${TEST_VAR}/test",
			setup: func() {
				_ = os.Setenv("TEST_VAR", "/custom/path")
			},
			teardown: func() {
				_ = os.Unsetenv("TEST_VAR")
			},
			check: func(t *testing.T, result string) {
				assert.Equal(t, "/custom/path/test", result)
			},
		},
		{
			name: "returns path unchanged if no expansion needed",
			path: "/absolute/path",
			check: func(t *testing.T, result string) {
				assert.Equal(t, "/absolute/path", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.teardown != nil {
				defer tt.teardown()
			}

			result := expandPath(tt.path)
			tt.check(t, result)
		})
	}
}
