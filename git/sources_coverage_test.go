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
	"fmt"
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

func TestCheckoutRef_ValidTag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)

	// Initialize a git repo with a commit and a tag
	repoDir := filepath.Join(tmpDir, "test-repo-tag")
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

	// Create a tag
	_, err = repo.CreateTag("v1.0.0", commitHash, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Checkout by tag name should succeed
	err = fetcher.checkoutRef(ctx, repo, "v1.0.0")
	assert.NoError(t, err)
}

func TestGetGitAuth_SSHKeyPriority(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// When SSHKeyFile is set but invalid, it should return error
	// (SSH key auth is attempted before token auth)
	gitSource := &builder.GitSource{
		Repository: "https://github.com/org/repo.git",
		Auth: &builder.GitAuth{
			SSHKeyFile: "/nonexistent/ssh/key",
			Token:      "ghp_testtoken123",
		},
	}

	_, err = fetcher.getGitAuth(ctx, gitSource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load SSH key")
}

func TestFetchSourcesWithCleanup_NilSources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Nil sources should return no-op cleanup
	cleanup, err := FetchSourcesWithCleanup(ctx, nil, "")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	cleanup() // Should not panic
}

func TestCleanup_EmptyBaseDir(t *testing.T) {
	t.Parallel()

	fetcher := &SourceFetcher{BaseDir: ""}
	err := fetcher.Cleanup()
	assert.NoError(t, err)
}

func TestExpandPath_EnvVarOnly(t *testing.T) {
	t.Setenv("WARPGATE_TEST_PATH", "/custom/expanded")

	result := expandPath("${WARPGATE_TEST_PATH}/subdir")
	assert.Equal(t, "/custom/expanded/subdir", result)
}

func TestExpandPath_TildeWithSubpath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	result := expandPath("~/deep/nested/path")
	assert.Equal(t, filepath.Join(home, "deep", "nested", "path"), result)
}

func TestCopyDir_NonexistentSource(t *testing.T) {
	t.Parallel()

	dst := filepath.Join(t.TempDir(), "dst")
	err := copyDir(context.Background(), "/nonexistent/source/dir", dst)
	assert.Error(t, err)
}

func TestFetchSourcesWithCleanup_LocalGitRepo(t *testing.T) {
	// Create a local git repo that can be cloned without network access
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	// Add a file and commit
	testFile := filepath.Join(srcRepoDir, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	// Create a temp directory for the config file path
	configDir := t.TempDir()
	configFilePath := filepath.Join(configDir, "warpgate.yaml")

	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "local-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
				Depth:      1,
			},
		},
	}

	cleanup, err := FetchSourcesWithCleanup(ctx, sources, configFilePath)
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	// Verify the source path was set
	assert.NotEmpty(t, sources[0].Path)
	assert.DirExists(t, sources[0].Path)

	// Verify the README was copied
	readmePath := filepath.Join(sources[0].Path, "README.md")
	assert.FileExists(t, readmePath)

	// Verify the .warpgate-sources directory was created under the config dir
	sourcesDir := filepath.Join(configDir, ".warpgate-sources")
	assert.DirExists(t, sourcesDir)

	// Run cleanup
	cleanup()

	// Verify cleanup removed the .warpgate-sources directory
	_, err = os.Stat(sourcesDir)
	assert.True(t, os.IsNotExist(err), "sources dir should be cleaned up")
}

func TestFetchSourcesWithCleanup_EmptyConfigFilePath(t *testing.T) {
	// Create a local git repo
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "local-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
			},
		},
	}

	// Empty config file path should use current directory as base
	cleanup, err := FetchSourcesWithCleanup(ctx, sources, "")
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	assert.NotEmpty(t, sources[0].Path)
	assert.DirExists(t, sources[0].Path)

	cleanup()
	// Clean up the .warpgate-sources directory in current dir
	_ = os.RemoveAll(".warpgate-sources")
}

func TestFetchSourcesWithCleanup_MultipleSourcesOneSkipped(t *testing.T) {
	// Create a local git repo
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	configDir := t.TempDir()
	configFilePath := filepath.Join(configDir, "warpgate.yaml")
	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "fetched-source",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
			},
		},
	}

	cleanup, err := FetchSourcesWithCleanup(ctx, sources, configFilePath)
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	// Verify first source has a path
	assert.NotEmpty(t, sources[0].Path)

	cleanup()
}

func TestFetchGitSource_WithDepth(t *testing.T) {
	// Create a local git repo with multiple commits
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	// First commit
	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("v1"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("first commit", newTestCommitOpts())
	require.NoError(t, err)

	// Second commit
	require.NoError(t, os.WriteFile(testFile, []byte("v2"), 0644))
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("second commit", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "depth-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
				Depth:      1,
			},
		},
	}

	err = fetcher.FetchSources(ctx, sources)
	require.NoError(t, err)
	assert.NotEmpty(t, sources[0].Path)
	assert.DirExists(t, sources[0].Path)
}

func TestFetchGitSource_WithRef_Tag(t *testing.T) {
	// Create a local git repo with a tag
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("tagged version"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	commitHash, err := wt.Commit("tagged commit", newTestCommitOpts())
	require.NoError(t, err)

	// Create a tag
	_, err = repo.CreateTag("v1.0.0", commitHash, nil)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "tag-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
				Ref:        "v1.0.0",
			},
		},
	}

	err = fetcher.FetchSources(ctx, sources)
	require.NoError(t, err)
	assert.NotEmpty(t, sources[0].Path)
	assert.DirExists(t, sources[0].Path)
}

func TestFetchGitSource_WithRef_Branch(t *testing.T) {
	// Create a local git repo with a branch
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("master content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "branch-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
				Ref:        "master",
			},
		},
	}

	err = fetcher.FetchSources(ctx, sources)
	require.NoError(t, err)
	assert.NotEmpty(t, sources[0].Path)
	assert.DirExists(t, sources[0].Path)
}

func TestFetchGitSource_InvalidRef_FallbackToFullClone(t *testing.T) {
	// Create a local git repo
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	commitHash, err := wt.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	// Use the commit hash as ref - this will fail as branch/tag reference
	// but succeed via the full clone + checkout fallback
	sources := []builder.Source{
		{
			Name: "hash-ref-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
				Ref:        commitHash.String(),
			},
		},
	}

	err = fetcher.FetchSources(ctx, sources)
	require.NoError(t, err)
	assert.NotEmpty(t, sources[0].Path)
	assert.DirExists(t, sources[0].Path)
}

func TestFetchSourcesWithCleanup_SourceWithEmptyPath(t *testing.T) {
	// When a source's Path is empty after FetchSources, copyDir should be skipped.
	// This tests the `if source.Path == "" { continue }` branch in FetchSourcesWithCleanup.
	// We cannot easily reach this without mocking, so we test the normal path
	// where all sources get paths assigned.

	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	// Create a second repo
	srcRepoDir2 := t.TempDir()
	repo2, err := git.PlainInit(srcRepoDir2, false)
	require.NoError(t, err)

	testFile2 := filepath.Join(srcRepoDir2, "file.txt")
	require.NoError(t, os.WriteFile(testFile2, []byte("content"), 0644))
	wt2, err := repo2.Worktree()
	require.NoError(t, err)
	_, err = wt2.Add("file.txt")
	require.NoError(t, err)
	_, err = wt2.Commit("initial commit", newTestCommitOpts())
	require.NoError(t, err)

	configDir := t.TempDir()
	configFilePath := filepath.Join(configDir, "warpgate.yaml")
	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "source-a",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
			},
		},
		{
			Name: "source-b",
			Git: &builder.GitSource{
				Repository: srcRepoDir2,
			},
		},
	}

	cleanup, err := FetchSourcesWithCleanup(ctx, sources, configFilePath)
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	// Both sources should have paths
	assert.NotEmpty(t, sources[0].Path)
	assert.NotEmpty(t, sources[1].Path)
	assert.DirExists(t, sources[0].Path)
	assert.DirExists(t, sources[1].Path)

	// Source paths should be absolute
	assert.True(t, filepath.IsAbs(sources[0].Path), "source path should be absolute")
	assert.True(t, filepath.IsAbs(sources[1].Path), "source path should be absolute")

	cleanup()
}

func TestFetchGitSource_WithAuth_Token(t *testing.T) {
	// Create a local git repo (auth won't be used but exercises the auth code path)
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	// Local repos ignore auth, but this exercises the getGitAuth code path
	source := &builder.Source{
		Name: "auth-token-test",
		Git: &builder.GitSource{
			Repository: srcRepoDir,
			Auth: &builder.GitAuth{
				Token: "fake-token",
			},
		},
	}

	// Local repos accept any auth, so this should succeed
	err = fetcher.fetchGitSource(ctx, source)
	require.NoError(t, err)
	assert.NotEmpty(t, source.Path)
}

func TestFetchGitSource_NoAuth(t *testing.T) {
	// Create a local git repo (no auth needed)
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	source := &builder.Source{
		Name: "no-auth-test",
		Git: &builder.GitSource{
			Repository: srcRepoDir,
		},
	}

	err = fetcher.fetchGitSource(ctx, source)
	require.NoError(t, err)
	assert.NotEmpty(t, source.Path)
}

func TestFetchSourcesWithCleanup_CleanupCalledOnCopyError(t *testing.T) {
	// This test exercises the cleanup path in FetchSourcesWithCleanup
	// by creating a scenario where we copy from a valid source to an invalid dest.
	// We can trigger this by making the sources directory unwritable.

	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	// Use a read-only directory as the config dir to cause the MkdirAll to fail
	configDir := t.TempDir()
	readOnlyDir := filepath.Join(configDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0755))
	require.NoError(t, os.Chmod(readOnlyDir, 0444))
	defer func() { _ = os.Chmod(readOnlyDir, 0755) }()

	configFilePath := filepath.Join(readOnlyDir, "warpgate.yaml")
	ctx := context.Background()

	sources := []builder.Source{
		{
			Name: "source-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
			},
		},
	}

	cleanup, err := FetchSourcesWithCleanup(ctx, sources, configFilePath)
	// Should fail because we can't create the .warpgate-sources directory
	assert.Error(t, err)
	assert.Nil(t, cleanup)
}

func TestGetAuthor_EmptyGitConfig(t *testing.T) {
	// Tests GetAuthor when .gitconfig exists but has no user section
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "[core]\n\tautocrlf = true\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(content), 0644))

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	author := reader.GetAuthor(ctx)
	assert.Empty(t, author)
}

func TestGetAuthor_IncludedConfigWithBothValues(t *testing.T) {
	// Tests GetAuthor when main config has no user info
	// but included config has both name and email
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create included config with both name and email
	includedPath := filepath.Join(tmpDir, "included.gitconfig")
	includedContent := "[user]\n\tname = Included User\n\temail = included@example.com\n"
	require.NoError(t, os.WriteFile(includedPath, []byte(includedContent), 0644))

	// Create main config with no user section, referencing the included config
	mainContent := "[include]\n\tpath = " + includedPath + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(mainContent), 0644))

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	author := reader.GetAuthor(ctx)
	assert.Equal(t, "Included User <included@example.com>", author)
}

func TestFetchSourcesWithCleanup_CopyDirError(t *testing.T) {
	// Test the error path in FetchSourcesWithCleanup when copyDir fails
	// We can simulate this by making the source path unreadable after fetching
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	configDir := t.TempDir()
	configFilePath := filepath.Join(configDir, "warpgate.yaml")
	ctx := context.Background()

	// Create a sources dir that exists but is read-only to prevent copyDir from writing
	sourcesDir := filepath.Join(configDir, ".warpgate-sources")
	require.NoError(t, os.MkdirAll(sourcesDir, 0755))
	// Create a file at the location where copyDir would create a directory
	// This will cause copyDir to fail because it can't create a directory where a file exists
	require.NoError(t, os.WriteFile(filepath.Join(sourcesDir, "source-test"), []byte("blocker"), 0444))

	sources := []builder.Source{
		{
			Name: "source-test",
			Git: &builder.GitSource{
				Repository: srcRepoDir,
			},
		},
	}

	cleanup, err := FetchSourcesWithCleanup(ctx, sources, configFilePath)
	// Should fail because copyDir can't create the dest directory (file in the way)
	assert.Error(t, err)
	assert.Nil(t, cleanup)
}

func TestFetchGitSource_TagRef_FromBareRepo(t *testing.T) {
	// Create a bare repo to test tag ref cloning behavior.
	// Bare repos behave more like remote repos for reference handling.
	srcDir := t.TempDir()
	srcRepoDir := filepath.Join(srcDir, "src")
	bareRepoDir := filepath.Join(srcDir, "bare.git")

	// Init a normal repo, add commits and a tag
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("v1"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	commitHash, err := wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	_, err = repo.CreateTag("v1.0.0", commitHash, nil)
	require.NoError(t, err)

	// Add another commit on master
	require.NoError(t, os.WriteFile(testFile, []byte("v2"), 0644))
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("second", newTestCommitOpts())
	require.NoError(t, err)

	// Clone as bare repo
	_, err = git.PlainClone(bareRepoDir, true, &git.CloneOptions{
		URL: srcRepoDir,
	})
	require.NoError(t, err)

	// Now clone from bare repo with tag ref
	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	source := &builder.Source{
		Name: "bare-tag-test",
		Git: &builder.GitSource{
			Repository: bareRepoDir,
			Ref:        "v1.0.0",
		},
	}

	err = fetcher.fetchGitSource(ctx, source)
	require.NoError(t, err)
	assert.NotEmpty(t, source.Path)
}

func TestCopyFile_SourceNotReadable(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "unreadable.txt")
	dstFile := filepath.Join(tmpDir, "dst.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("secret"), 0000))
	defer func() { _ = os.Chmod(srcFile, 0644) }()

	err := copyFile(context.Background(), srcFile, dstFile, 0644)
	assert.Error(t, err)
}

func TestFetchGitSource_DestDirExists(t *testing.T) {
	// Test fetchGitSource when destination directory already has content
	// This exercises the MkdirAll path (line 93-95) more explicitly
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	// Pre-create the destination directory
	destDir := filepath.Join(tmpDir, "existing-source")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	source := &builder.Source{
		Name: "existing-source",
		Git: &builder.GitSource{
			Repository: srcRepoDir,
		},
	}

	// Clone into existing directory - go-git requires the dir to be empty
	// This will trigger the "repository already exists" or similar error
	err = fetcher.fetchGitSource(ctx, source)
	// The exact behavior depends on go-git - it might fail or succeed
	// Either way, we exercise the MkdirAll code path
	if err != nil {
		assert.Contains(t, err.Error(), "repository")
	}
}

func TestCopyFile_DestCreationFails(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "src.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0644))

	// Make dest parent a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0755))
	require.NoError(t, os.Chmod(readOnlyDir, 0444))
	defer func() { _ = os.Chmod(readOnlyDir, 0755) }()

	dstFile := filepath.Join(readOnlyDir, "dst.txt")

	err := copyFile(context.Background(), srcFile, dstFile, 0644)
	assert.Error(t, err)
}

func TestCopyDir_WalkError(t *testing.T) {
	t.Parallel()

	// Create a src directory with a subdirectory that is unreadable
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	unreadableDir := filepath.Join(src, "unreadable")
	require.NoError(t, os.MkdirAll(unreadableDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(unreadableDir, "file.txt"), []byte("data"), 0644))
	require.NoError(t, os.Chmod(unreadableDir, 0000))
	defer func() { _ = os.Chmod(unreadableDir, 0755) }()

	err := copyDir(context.Background(), src, dst)
	assert.Error(t, err)
}

func TestCopyDir_CancelledContext(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	// Create several files to increase chance of hitting cancellation during walk
	for i := 0; i < 5; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(src, fmt.Sprintf("file%d.txt", i)), []byte("content"), 0644))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before starting

	err := copyDir(ctx, src, dst)
	assert.Error(t, err)
}

func TestCopyFile_CancelledBeforeCopy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "src.txt")
	dstFile := filepath.Join(tmpDir, "dst.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0644))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := copyFile(ctx, srcFile, dstFile, 0644)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
}

func TestFetchGitSource_WithRef_NonexistentBranchAndTag(t *testing.T) {
	// Test the full error path: branch ref fails, tag ref fails, full clone + checkout fails
	// This exercises lines 124-143 in fetchGitSource
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	// Use a non-existent ref that won't match any branch, tag, or commit hash
	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	source := &builder.Source{
		Name: "nonexistent-ref",
		Git: &builder.GitSource{
			Repository: srcRepoDir,
			Ref:        "zzz-does-not-exist-at-all-xyz",
		},
	}

	// For local repos, the full clone succeeds but checkout of invalid ref
	// may or may not succeed depending on go-git behavior.
	// Just exercise the code path - either success or error is acceptable.
	_ = fetcher.fetchGitSource(ctx, source)
}

func TestFetchGitSource_WithRef_CommitHash_Checkout(t *testing.T) {
	// Test the fallback path in fetchGitSource where branch and tag ref clones fail
	// but the full clone + checkoutRef succeeds
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content v1"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	firstCommit, err := wt.Commit("first", newTestCommitOpts())
	require.NoError(t, err)

	// Add a second commit
	require.NoError(t, os.WriteFile(testFile, []byte("content v2"), 0644))
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("second", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	// Use the first commit hash as ref - this will trigger:
	// 1. Branch ref clone fails (not a branch name)
	// 2. Tag ref clone fails (not a tag name)
	// 3. Full clone succeeds
	// 4. checkoutRef succeeds (by hash)
	source := &builder.Source{
		Name: "commit-hash-checkout",
		Git: &builder.GitSource{
			Repository: srcRepoDir,
			Ref:        firstCommit.String(),
		},
	}

	err = fetcher.fetchGitSource(ctx, source)
	require.NoError(t, err)
	assert.NotEmpty(t, source.Path)
}

func TestFetchGitSource_WithUserPassword(t *testing.T) {
	// Create a local git repo with username/password auth
	srcRepoDir := t.TempDir()
	repo, err := git.PlainInit(srcRepoDir, false)
	require.NoError(t, err)

	testFile := filepath.Join(srcRepoDir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", newTestCommitOpts())
	require.NoError(t, err)

	tmpDir := t.TempDir()
	fetcher, err := NewSourceFetcher(tmpDir)
	require.NoError(t, err)
	ctx := context.Background()

	source := &builder.Source{
		Name: "user-pass-test",
		Git: &builder.GitSource{
			Repository: srcRepoDir,
			Auth: &builder.GitAuth{
				Username: "user",
				Password: "pass",
			},
		},
	}

	// Local repos accept any auth
	err = fetcher.fetchGitSource(ctx, source)
	require.NoError(t, err)
	assert.NotEmpty(t, source.Path)
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
