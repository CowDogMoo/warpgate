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

package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitOperations(t *testing.T) {
	cacheDir := "/tmp/test-cache"
	gitOps := NewGitOperations(cacheDir)

	assert.NotNil(t, gitOps)
	assert.Equal(t, cacheDir, gitOps.cacheDir)
}

func TestGitOperations_GetCachePath(t *testing.T) {
	cacheDir := "/tmp/test-cache"
	gitOps := NewGitOperations(cacheDir)

	tests := []struct {
		name     string
		gitURL   string
		version  string
		contains []string // parts that should be in the path
	}{
		{
			name:     "https url without version",
			gitURL:   "https://github.com/user/repo.git",
			version:  "",
			contains: []string{cacheDir, "github.com", "user", "repo"},
		},
		{
			name:     "https url with version",
			gitURL:   "https://github.com/user/repo.git",
			version:  "v1.2.0",
			contains: []string{cacheDir, "github.com", "user", "repo"},
		},
		{
			name:     "ssh url",
			gitURL:   "git@github.com:user/repo.git",
			version:  "",
			contains: []string{cacheDir, "github.com", "user", "repo"},
		},
		{
			name:     "url with main branch",
			gitURL:   "https://github.com/user/repo.git",
			version:  "main",
			contains: []string{cacheDir, "github.com", "user", "repo"},
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := gitOps.getCachePath(tt.gitURL, tt.version)

			for _, part := range tt.contains {
				assert.Contains(t, path, part)
			}

			// Verify no .git suffix in path
			assert.NotContains(t, path, ".git")
		})
	}
}

func TestGitOperations_CachePathCleaning(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	tests := []struct {
		name    string
		gitURL  string
		version string
	}{
		{
			name:    "https with .git",
			gitURL:  "https://github.com/user/repo.git",
			version: "",
		},
		{
			name:    "https without .git",
			gitURL:  "https://github.com/user/repo",
			version: "",
		},
		{
			name:    "http url",
			gitURL:  "http://github.com/user/repo.git",
			version: "",
		},
		{
			name:    "ssh url",
			gitURL:  "git@github.com:user/repo.git",
			version: "",
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := gitOps.getCachePath(tt.gitURL, tt.version)

			// Verify path doesn't contain protocol prefixes
			assert.NotContains(t, path, "https://")
			assert.NotContains(t, path, "http://")
			assert.NotContains(t, path, "git@")

			// Verify path doesn't end with .git
			assert.False(t, filepath.Ext(path) == ".git")

			// Verify path starts with cache dir
			assert.True(t, filepath.IsAbs(path) || strings.HasPrefix(path, cacheDir))
		})
	}
}

func TestGitOperations_VersionHashing(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	gitURL := "https://github.com/user/repo.git"

	// Get paths for different versions
	pathWithoutVersion := gitOps.getCachePath(gitURL, "")
	pathWithVersion1 := gitOps.getCachePath(gitURL, "v1.0.0")
	pathWithVersion2 := gitOps.getCachePath(gitURL, "v2.0.0")
	pathWithMain := gitOps.getCachePath(gitURL, "main")

	// Paths with specific versions should be different
	assert.NotEqual(t, pathWithVersion1, pathWithVersion2)

	// Path with 'main' should be same as without version
	assert.Equal(t, pathWithoutVersion, pathWithMain)
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		expected bool
		setup    func() string
	}{
		{
			name:     "existing directory",
			expected: true,
			setup: func() string {
				dir := filepath.Join(tmpDir, "exists")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("failed to create test directory: %v", err)
				}
				return dir
			},
		},
		{
			name:     "non-existent directory",
			expected: false,
			setup: func() string {
				return filepath.Join(tmpDir, "does-not-exist")
			},
		},
		{
			name:     "file instead of directory",
			expected: false,
			setup: func() string {
				file := filepath.Join(tmpDir, "file.txt")
				if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return file
			},
		},
	}

	for _, tc := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			result := dirExists(path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitOperations_CloneOrUpdate_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
	}

	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	// Use a small public repo for testing
	gitURL := "https://github.com/cowdogmoo/warpgate-templates.git"

	// First clone
	path1, err := gitOps.CloneOrUpdate(gitURL, "")
	require.NoError(t, err)
	assert.NotEmpty(t, path1)
	assert.True(t, dirExists(path1))

	// Second call should use cached version
	path2, err := gitOps.CloneOrUpdate(gitURL, "")
	require.NoError(t, err)
	assert.Equal(t, path1, path2)

	// Verify .git directory exists
	gitDir := filepath.Join(path1, ".git")
	assert.True(t, dirExists(gitDir))
}

func TestGitOperations_CacheStructure(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	gitURL := "https://github.com/cowdogmoo/warpgate-templates.git"
	version := "v1.0.0"

	path := gitOps.getCachePath(gitURL, version)

	// Verify path structure
	assert.Contains(t, path, cacheDir)
	assert.Contains(t, path, "github.com")
	assert.Contains(t, path, "cowdogmoo")
	assert.Contains(t, path, "warpgate-templates")

	// With version, should have hash directory
	if version != "" && version != "main" && version != "master" {
		// The path should be longer due to version hash
		pathNoVersion := gitOps.getCachePath(gitURL, "")
		assert.NotEqual(t, len(path), len(pathNoVersion))
	}
}

func TestGitOperations_MultipleVersions(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	gitURL := "https://github.com/user/repo.git"

	versions := []string{"v1.0.0", "v1.1.0", "v2.0.0"}
	paths := make(map[string]string)

	for _, version := range versions {
		path := gitOps.getCachePath(gitURL, version)
		paths[version] = path
	}

	// All paths should be unique
	for i, v1 := range versions {
		for j, v2 := range versions {
			if i != j {
				assert.NotEqual(t, paths[v1], paths[v2], "Paths for %s and %s should be different", v1, v2)
			}
		}
	}
}

func TestGitOperations_URLVariations(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	// Different URL formats for the same repo
	urls := []string{
		"https://github.com/user/repo.git",
		"https://github.com/user/repo",
		"http://github.com/user/repo.git",
		"git@github.com:user/repo.git",
	}

	paths := make([]string, len(urls))
	for i, url := range urls {
		paths[i] = gitOps.getCachePath(url, "")
	}

	// All paths should resolve to similar structure (github.com/user/repo)
	for _, path := range paths {
		assert.Contains(t, path, "github.com")
		assert.Contains(t, path, "user")
		assert.Contains(t, path, "repo")
	}
}
