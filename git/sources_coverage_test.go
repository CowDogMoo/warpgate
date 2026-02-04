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
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCommitOpts() *git.CommitOptions {
	return &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	}
}

func TestFetchSource_NoValidSourceType(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	source := &builder.Source{
		Name: "no-type-source",
		Git:  nil,
	}

	err = fetcher.fetchSource(ctx, source)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid source type defined")
}

func TestFetchSources_MultipleSourcesWithError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	sources := []builder.Source{
		{
			Name: "source-no-type",
			Git:  nil,
		},
	}

	err = fetcher.FetchSources(ctx, sources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source-no-type")
}

func TestCheckoutRef_InvalidRef(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)

	// Initialize a git repo with a commit
	repoDir := filepath.Join(tmpDir, "test-repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(repoDir, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# test"), 0644))
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	_, err = worktree.Add("README.md")
	require.NoError(t, err)
	_, err = worktree.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	ctx := context.Background()

	// Try checking out a ref that is not a valid hash, branch, or tag
	// Use a name that cannot match any branch or tag in the repo
	err = fetcher.checkoutRef(ctx, repo, "definitely-not-a-branch-or-tag-xyz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not checkout ref")
}

func TestCheckoutRef_ValidBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)

	// Initialize a git repo with a commit
	repoDir := filepath.Join(tmpDir, "test-repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(repoDir, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# test"), 0644))
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	_, err = worktree.Add("README.md")
	require.NoError(t, err)
	_, err = worktree.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	ctx := context.Background()

	// Checkout master (the default branch) should succeed
	err = fetcher.checkoutRef(ctx, repo, "master")
	assert.NoError(t, err)
}

func TestCheckoutRef_ValidCommitHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)

	repoDir := filepath.Join(tmpDir, "test-repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(repoDir, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# test"), 0644))
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	_, err = worktree.Add("README.md")
	require.NoError(t, err)
	commitHash, err := worktree.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	ctx := context.Background()

	// Checkout by commit hash should succeed
	err = fetcher.checkoutRef(ctx, repo, commitHash.String())
	assert.NoError(t, err)
}

func TestCopyFile_DestDirNotExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "src.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("hello"), 0644))

	// Try to copy to a path where the parent directory does not exist
	dstFile := filepath.Join(tmpDir, "nonexistent", "subdir", "dst.txt")

	err := copyFile(context.Background(), srcFile, dstFile, 0644)
	assert.Error(t, err)
}

func TestCopyDir_WithNestedStructure(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	// Create nested directory structure with multiple levels
	require.NoError(t, os.MkdirAll(filepath.Join(src, "a", "b", "c"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a", "level1.txt"), []byte("level1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a", "b", "level2.txt"), []byte("level2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a", "b", "c", "level3.txt"), []byte("level3"), 0644))

	err := copyDir(context.Background(), src, dst)
	require.NoError(t, err)

	// Verify all files were copied
	content, err := os.ReadFile(filepath.Join(dst, "root.txt"))
	require.NoError(t, err)
	assert.Equal(t, "root", string(content))

	content, err = os.ReadFile(filepath.Join(dst, "a", "b", "c", "level3.txt"))
	require.NoError(t, err)
	assert.Equal(t, "level3", string(content))
}

func TestFetchSourcesWithCleanup_EmptySources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Empty slice (not nil) should also return no-op cleanup
	cleanup, err := FetchSourcesWithCleanup(ctx, []builder.Source{}, "")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestFetchSourcesWithCleanup_WithConfigFilePath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// No sources means no-op even with a config file path
	cleanup, err := FetchSourcesWithCleanup(ctx, nil, "/some/path/config.yaml")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestGetSourcePath_EmptySources(t *testing.T) {
	t.Parallel()

	_, err := GetSourcePath([]builder.Source{}, "anything")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSourcePath_MultipleSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sources    []builder.Source
		lookupName string
		wantPath   string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "first source found",
			sources: []builder.Source{
				{Name: "alpha", Path: "/alpha/path"},
				{Name: "beta", Path: "/beta/path"},
			},
			lookupName: "alpha",
			wantPath:   "/alpha/path",
		},
		{
			name: "second source found",
			sources: []builder.Source{
				{Name: "alpha", Path: "/alpha/path"},
				{Name: "beta", Path: "/beta/path"},
			},
			lookupName: "beta",
			wantPath:   "/beta/path",
		},
		{
			name: "source not fetched",
			sources: []builder.Source{
				{Name: "alpha", Path: ""},
			},
			lookupName: "alpha",
			wantErr:    true,
			errMsg:     "not been fetched",
		},
		{
			name: "source not found in list",
			sources: []builder.Source{
				{Name: "alpha", Path: "/alpha/path"},
			},
			lookupName: "gamma",
			wantErr:    true,
			errMsg:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path, err := GetSourcePath(tt.sources, tt.lookupName)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestExpandPath_TildeAndEnvVar(t *testing.T) {
	// Test combining tilde expansion with environment variables
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	result := expandPath("~/somepath")
	assert.Equal(t, filepath.Join(home, "somepath"), result)
}

func TestExpandPath_NoExpansion(t *testing.T) {
	t.Parallel()

	result := expandPath("/absolute/path/to/file")
	assert.Equal(t, "/absolute/path/to/file", result)
}

func TestCopyFile_PreservesContent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.bin")
	dstPath := filepath.Join(tmpDir, "dst.bin")

	// Write binary-like content
	data := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	require.NoError(t, os.WriteFile(srcPath, data, 0644))

	err := copyFile(context.Background(), srcPath, dstPath, 0600)
	require.NoError(t, err)

	// Verify content matches
	content, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, data, content)

	// Verify permissions
	info, err := os.Stat(dstPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestInjectTokenIntoURL_MoreCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		token    string
		expected string
		wantErr  bool
	}{
		{
			name:     "file:// scheme is not modified",
			url:      "file:///local/repo",
			token:    "token123",
			expected: "file:///local/repo",
		},
		{
			name:     "empty url with token",
			url:      "",
			token:    "token123",
			expected: "",
		},
		{
			name:     "https url with path and query",
			url:      "https://github.com/org/repo.git?ref=main",
			token:    "mytoken",
			expected: "https://x-access-token:mytoken@github.com/org/repo.git?ref=main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func TestFetchSourcesWithCleanup_FetchError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Sources with no valid type will fail during FetchSources
	sources := []builder.Source{
		{
			Name: "bad-source",
			Git:  nil,
		},
	}

	cleanup, err := FetchSourcesWithCleanup(ctx, sources, "")
	assert.Error(t, err)
	assert.Nil(t, cleanup)
	assert.Contains(t, err.Error(), "bad-source")
}

func TestFetchSourcesWithCleanup_WithConfigPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Sources with no valid type will fail during FetchSources
	// but the configDir calculation is still exercised
	sources := []builder.Source{
		{
			Name: "bad-source",
			Git:  nil,
		},
	}

	cleanup, err := FetchSourcesWithCleanup(ctx, sources, "/some/config/path/warpgate.yaml")
	assert.Error(t, err)
	assert.Nil(t, cleanup)
}

func TestGetAuthor_WithIncludedConfig(t *testing.T) {
	// Not parallel - modifies HOME
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create included config with email
	includedPath := filepath.Join(tmpDir, "work.gitconfig")
	includedContent := "[user]\n\temail = work@example.com\n"
	require.NoError(t, os.WriteFile(includedPath, []byte(includedContent), 0644))

	// Create main config with name but no email, referencing the included config
	mainContent := "[user]\n\tname = Test User\n[include]\n\tpath = " + includedPath + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(mainContent), 0644))

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	author := reader.GetAuthor(ctx)
	assert.Equal(t, "Test User <work@example.com>", author)
}

func TestGetAuthor_OnlyName(t *testing.T) {
	// Not parallel - modifies HOME
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "[user]\n\tname = Only Name\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(content), 0644))

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	author := reader.GetAuthor(ctx)
	assert.Equal(t, "Only Name", author)
}

func TestGetAuthor_OnlyEmail(t *testing.T) {
	// Not parallel - modifies HOME
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "[user]\n\temail = only@email.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(content), 0644))

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	author := reader.GetAuthor(ctx)
	assert.Equal(t, "only@email.com", author)
}

func TestCopyDir_NonexistentSource(t *testing.T) {
	t.Parallel()

	dst := filepath.Join(t.TempDir(), "dst")
	err := copyDir(context.Background(), "/nonexistent/source/dir", dst)
	assert.Error(t, err)
}

func TestCleanup_TempDirDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		baseDir    string
		shouldKeep bool
	}{
		{
			name:       "temp dir with prefix is removed",
			baseDir:    "", // Will be created by NewSourceFetcher
			shouldKeep: false,
		},
		{
			name:       "non-temp dir is kept",
			baseDir:    "use-provided",
			shouldKeep: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var fetcher *SourceFetcher
			var err error

			if tt.baseDir == "use-provided" {
				tmpDir := t.TempDir()
				fetcher, err = NewSourceFetcher(tmpDir)
				require.NoError(t, err)
			} else {
				fetcher, err = NewSourceFetcher("")
				require.NoError(t, err)
			}

			dir := fetcher.BaseDir
			assert.DirExists(t, dir)

			err = fetcher.Cleanup()
			require.NoError(t, err)

			if tt.shouldKeep {
				assert.DirExists(t, dir)
			} else {
				_, statErr := os.Stat(dir)
				assert.True(t, os.IsNotExist(statErr))
			}
		})
	}
}
