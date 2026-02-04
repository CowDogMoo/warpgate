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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
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
			gitURL:   "https://git.example.com/jdoe/repo.git",
			version:  "",
			contains: []string{cacheDir, "git.example.com", "jdoe", "repo"},
		},
		{
			name:     "https url with version",
			gitURL:   "https://git.example.com/jdoe/repo.git",
			version:  "v1.2.0",
			contains: []string{cacheDir, "git.example.com", "jdoe", "repo"},
		},
		{
			name:     "ssh url",
			gitURL:   "git@github.com:user/repo.git",
			version:  "",
			contains: []string{cacheDir, "github.com", "user", "repo"},
		},
		{
			name:     "url with main branch",
			gitURL:   "https://git.example.com/jdoe/repo.git",
			version:  "main",
			contains: []string{cacheDir, "git.example.com", "jdoe", "repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := gitOps.getCachePath(tt.gitURL, tt.version)

			for _, part := range tt.contains {
				assert.Contains(t, path, part)
			}

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
			gitURL:  "https://git.example.com/jdoe/repo.git",
			version: "",
		},
		{
			name:    "https without .git",
			gitURL:  "https://git.example.com/jdoe/repo",
			version: "",
		},
		{
			name:    "http url",
			gitURL:  "http://git.example.com/jdoe/repo.git",
			version: "",
		},
		{
			name:    "ssh url",
			gitURL:  "git@github.com:user/repo.git",
			version: "",
		},
	}

	for _, tt := range tests {
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

	gitURL := "https://git.example.com/jdoe/repo.git"

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

	for _, tt := range tests {
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
	path1, err := gitOps.CloneOrUpdate(context.Background(), gitURL, "")
	require.NoError(t, err)
	assert.NotEmpty(t, path1)
	assert.True(t, dirExists(path1))

	// Second call should use cached version
	path2, err := gitOps.CloneOrUpdate(context.Background(), gitURL, "")
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

	gitURL := "https://git.example.com/jdoe/repo.git"

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

func TestCloneOrUpdate_FreshClone(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Skipping test: git not found in PATH")
	}

	// Create a local bare repo to clone from
	bareDir := t.TempDir()
	workDir := t.TempDir()
	cacheDir := t.TempDir()

	// Initialize a bare repo
	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to init bare repo: %v\n%s", err, out)
	}

	// Create a temporary working copy, commit something, push to bare
	cmd = exec.Command("git", "clone", bareDir, workDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone bare repo: %v\n%s", err, out)
	}

	// Configure git user in workDir
	cmd = exec.Command("git", "-C", workDir, "config", "user.email", "test@test.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to configure git email: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", workDir, "config", "user.name", "Test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to configure git name: %v\n%s", err, out)
	}

	// Create a file and commit
	testFile := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	cmd = exec.Command("git", "-C", workDir, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", workDir, "commit", "-m", "initial")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", workDir, "push", "origin", "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git push: %v\n%s", err, out)
	}

	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	path, err := gitOps.CloneOrUpdate(ctx, bareDir, "")
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.True(t, dirExists(path))

	// Verify .git directory exists in cloned repo
	assert.True(t, dirExists(filepath.Join(path, ".git")))

	// Second call should use cached version (pull updates)
	path2, err := gitOps.CloneOrUpdate(ctx, bareDir, "")
	require.NoError(t, err)
	assert.Equal(t, path, path2)
}

func TestCloneWithRetry_TagFallback(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Skipping test: git not found in PATH")
	}

	// Create a local bare repo with a branch
	bareDir := t.TempDir()
	workDir := t.TempDir()
	cacheDir := t.TempDir()

	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to init bare repo: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "clone", bareDir, workDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone bare repo: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "-C", workDir, "config", "user.email", "test@test.com")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "config", "user.name", "Test")
	_ = cmd.Run()

	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	cmd = exec.Command("git", "-C", workDir, "add", ".")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "commit", "-m", "initial")
	_ = cmd.Run()

	// Create a branch (not a tag)
	cmd = exec.Command("git", "-C", workDir, "checkout", "-b", "my-branch")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create branch: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(workDir, "branch-file.txt"), []byte("branch content"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	cmd = exec.Command("git", "-C", workDir, "add", ".")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "commit", "-m", "branch commit")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "push", "origin", "my-branch")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to push branch: %v\n%s", err, out)
	}

	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	// Clone with version "my-branch" -- tag won't exist, should fall back to branch
	path, err := gitOps.CloneOrUpdate(ctx, bareDir, "my-branch")
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.True(t, dirExists(path))
}

func TestCheckoutVersion_Tag(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Skipping test: git not found in PATH")
	}

	// Create a local bare repo with a tag
	bareDir := t.TempDir()
	workDir := t.TempDir()
	cloneDir := t.TempDir()

	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to init bare repo: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "clone", bareDir, workDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone bare repo: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "-C", workDir, "config", "user.email", "test@test.com")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "config", "user.name", "Test")
	_ = cmd.Run()

	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	cmd = exec.Command("git", "-C", workDir, "add", ".")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "commit", "-m", "initial")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "tag", "v1.0.0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create tag: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", workDir, "push", "origin", "--all")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "push", "origin", "--tags")
	_ = cmd.Run()

	// Clone the repo, then checkout tag
	cmd = exec.Command("git", "clone", bareDir, cloneDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone for checkout test: %v\n%s", err, out)
	}

	repo, err := git.PlainOpen(cloneDir)
	require.NoError(t, err)

	err = checkoutVersion(repo, "v1.0.0")
	assert.NoError(t, err)
}

func TestCheckoutVersion_Branch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Skipping test: git not found in PATH")
	}

	bareDir := t.TempDir()
	workDir := t.TempDir()
	cacheDir := t.TempDir()

	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to init bare repo: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "clone", bareDir, workDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "-C", workDir, "config", "user.email", "test@test.com")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "config", "user.name", "Test")
	_ = cmd.Run()

	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	cmd = exec.Command("git", "-C", workDir, "add", ".")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "commit", "-m", "initial")
	_ = cmd.Run()

	// Create a branch
	cmd = exec.Command("git", "-C", workDir, "checkout", "-b", "dev-branch")
	_ = cmd.Run()
	if err := os.WriteFile(filepath.Join(workDir, "dev.txt"), []byte("dev"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	cmd = exec.Command("git", "-C", workDir, "add", ".")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "commit", "-m", "dev commit")
	_ = cmd.Run()
	cmd = exec.Command("git", "-C", workDir, "push", "origin", "--all")
	_ = cmd.Run()

	// Use CloneOrUpdate with version="dev-branch" to test the full flow
	// This exercises clone() -> cloneWithRetry() -> checkoutVersion() for branches
	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	path, err := gitOps.CloneOrUpdate(ctx, bareDir, "dev-branch")
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.True(t, dirExists(path))
}

func TestGetCachePath_WithVersion(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	path := gitOps.getCachePath("https://github.com/user/repo.git", "v1.0.0")

	assert.Contains(t, path, cacheDir)
	assert.Contains(t, path, "github.com")
	assert.Contains(t, path, "user")
	assert.Contains(t, path, "repo")

	// Version path should differ from no-version path
	pathNoVersion := gitOps.getCachePath("https://github.com/user/repo.git", "")
	assert.NotEqual(t, path, pathNoVersion)
}

func TestGetCachePath_WithoutVersion(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	path := gitOps.getCachePath("https://github.com/user/repo.git", "")
	pathMain := gitOps.getCachePath("https://github.com/user/repo.git", "main")
	pathMaster := gitOps.getCachePath("https://github.com/user/repo.git", "master")

	// Empty version, "main", and "master" should all produce the same path
	assert.Equal(t, path, pathMain)
	assert.Equal(t, path, pathMaster)
}

func TestGitOperations_URLVariations(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)

	tests := []struct {
		name     string
		url      string
		contains []string
	}{
		{
			name:     "https with .git",
			url:      "https://git.example.com/jdoe/repo.git",
			contains: []string{"git.example.com", "jdoe", "repo"},
		},
		{
			name:     "https without .git",
			url:      "https://git.example.com/jdoe/repo",
			contains: []string{"git.example.com", "jdoe", "repo"},
		},
		{
			name:     "http with .git",
			url:      "http://git.example.com/jdoe/repo.git",
			contains: []string{"git.example.com", "jdoe", "repo"},
		},
		{
			name:     "ssh github format",
			url:      "git@github.com:user/repo.git",
			contains: []string{"github.com", "user", "repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := gitOps.getCachePath(tt.url, "")

			for _, part := range tt.contains {
				assert.Contains(t, path, part, "Path should contain %s", part)
			}

			assert.NotContains(t, path, ".git")
		})
	}
}
