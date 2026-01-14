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
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
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

func TestGetGitAuth(t *testing.T) {
	fetcher, err := NewSourceFetcher("")
	require.NoError(t, err)
	defer func() { _ = fetcher.Cleanup() }()

	ctx := context.Background()

	t.Run("returns nil for nil auth", func(t *testing.T) {
		gitSource := &builder.GitSource{
			Repository: "https://github.com/org/repo.git",
			Auth:       nil,
		}

		auth, err := fetcher.getGitAuth(ctx, gitSource)
		require.NoError(t, err)
		assert.Nil(t, auth)
	})

	t.Run("returns token auth with default username", func(t *testing.T) {
		gitSource := &builder.GitSource{
			Repository: "https://github.com/org/repo.git",
			Auth: &builder.GitAuth{
				Token: "ghp_testtoken123",
			},
		}

		auth, err := fetcher.getGitAuth(ctx, gitSource)
		require.NoError(t, err)
		require.NotNil(t, auth)

		switch basicAuth := auth.(type) {
		case *githttp.BasicAuth:
			assert.Equal(t, "x-access-token", basicAuth.Username)
			assert.Equal(t, "ghp_testtoken123", basicAuth.Password)
		default:
			t.Fatalf("expected BasicAuth type, got %T", auth)
		}
	})

	t.Run("returns token auth with custom username", func(t *testing.T) {
		gitSource := &builder.GitSource{
			Repository: "https://gitlab.com/org/repo.git",
			Auth: &builder.GitAuth{
				Username: "oauth2",
				Token:    "glpat_testtoken123",
			},
		}

		auth, err := fetcher.getGitAuth(ctx, gitSource)
		require.NoError(t, err)
		require.NotNil(t, auth)

		switch basicAuth := auth.(type) {
		case *githttp.BasicAuth:
			assert.Equal(t, "oauth2", basicAuth.Username)
			assert.Equal(t, "glpat_testtoken123", basicAuth.Password)
		default:
			t.Fatalf("expected BasicAuth type, got %T", auth)
		}
	})

	t.Run("returns basic auth for username/password", func(t *testing.T) {
		gitSource := &builder.GitSource{
			Repository: "https://github.com/org/repo.git",
			Auth: &builder.GitAuth{
				Username: "testuser",
				Password: "testpass",
			},
		}

		auth, err := fetcher.getGitAuth(ctx, gitSource)
		require.NoError(t, err)
		require.NotNil(t, auth)

		switch basicAuth := auth.(type) {
		case *githttp.BasicAuth:
			assert.Equal(t, "testuser", basicAuth.Username)
			assert.Equal(t, "testpass", basicAuth.Password)
		default:
			t.Fatalf("expected BasicAuth type, got %T", auth)
		}
	})

	t.Run("returns nil for empty auth struct", func(t *testing.T) {
		gitSource := &builder.GitSource{
			Repository: "https://github.com/org/repo.git",
			Auth:       &builder.GitAuth{},
		}

		auth, err := fetcher.getGitAuth(ctx, gitSource)
		require.NoError(t, err)
		assert.Nil(t, auth)
	})
}

func TestGetSSHAuth(t *testing.T) {
	fetcher, err := NewSourceFetcher("")
	require.NoError(t, err)
	defer func() { _ = fetcher.Cleanup() }()

	ctx := context.Background()

	t.Run("returns nil for empty auth", func(t *testing.T) {
		auth, err := fetcher.getSSHAuth(ctx, &builder.GitAuth{})
		require.NoError(t, err)
		assert.Nil(t, auth)
	})

	t.Run("returns error for invalid SSH key file", func(t *testing.T) {
		auth, err := fetcher.getSSHAuth(ctx, &builder.GitAuth{
			SSHKeyFile: "/nonexistent/path/to/key",
		})
		assert.Error(t, err)
		assert.Nil(t, auth)
		assert.Contains(t, err.Error(), "failed to load SSH key")
	})

	t.Run("returns error for invalid inline SSH key", func(t *testing.T) {
		auth, err := fetcher.getSSHAuth(ctx, &builder.GitAuth{
			SSHKey: "not-a-valid-ssh-key",
		})
		assert.Error(t, err)
		assert.Nil(t, auth)
		assert.Contains(t, err.Error(), "failed to parse SSH key")
	})

	t.Run("loads SSH key from file when available", func(t *testing.T) {
		// Try to use the user's default SSH key if available
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("Cannot get home directory")
		}

		defaultKey := filepath.Join(home, ".ssh", "id_ed25519")
		if _, err := os.Stat(defaultKey); os.IsNotExist(err) {
			defaultKey = filepath.Join(home, ".ssh", "id_rsa")
			if _, err := os.Stat(defaultKey); os.IsNotExist(err) {
				t.Skip("No SSH key available for testing")
			}
		}

		auth, err := fetcher.getSSHAuth(ctx, &builder.GitAuth{
			SSHKeyFile: defaultKey,
		})
		// May fail if key has passphrase, which is expected
		if err != nil {
			t.Skipf("SSH key loading failed (may have passphrase): %v", err)
		}
		assert.NotNil(t, auth)
	})
}

func TestInjectTokenIntoURL_Extended(t *testing.T) {
	t.Run("does not modify ssh:// URL", func(t *testing.T) {
		result, err := InjectTokenIntoURL("ssh://git@github.com/org/repo.git", "token123")
		require.NoError(t, err)
		assert.Equal(t, "ssh://git@github.com/org/repo.git", result)
	})

	t.Run("returns error for invalid URL", func(t *testing.T) {
		_, err := InjectTokenIntoURL("://invalid-url", "token123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid repository URL")
	})

	t.Run("does not modify HTTP URL", func(t *testing.T) {
		result, err := InjectTokenIntoURL("http://github.com/org/repo.git", "token123")
		require.NoError(t, err)
		// HTTP URLs are not modified (only HTTPS)
		assert.Equal(t, "http://github.com/org/repo.git", result)
	})

	t.Run("handles URL with port", func(t *testing.T) {
		result, err := InjectTokenIntoURL("https://github.com:443/org/repo.git", "token123")
		require.NoError(t, err)
		assert.Contains(t, result, "x-access-token:token123@")
	})
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
