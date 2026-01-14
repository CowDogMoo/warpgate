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
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

const (
	// sourcesTempDirPrefix is the prefix used for temporary source directories.
	sourcesTempDirPrefix = "warpgate-sources-"
	// sourcesLocalDirName is the name of the directory where sources are copied within the build context.
	sourcesLocalDirName = ".warpgate-sources"
)

type SourceFetcher struct {
	BaseDir string
}

// If baseDir is empty, a temporary directory will be created
func NewSourceFetcher(baseDir string) (*SourceFetcher, error) {
	if baseDir == "" {
		tmpDir, err := os.MkdirTemp("", sourcesTempDirPrefix+"*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory for sources: %w", err)
		}
		baseDir = tmpDir
	}

	return &SourceFetcher{
		BaseDir: baseDir,
	}, nil
}

// Populates the Path field of each source with the local path
func (f *SourceFetcher) FetchSources(ctx context.Context, sources []builder.Source) error {
	for i := range sources {
		source := &sources[i]
		if err := f.fetchSource(ctx, source); err != nil {
			return fmt.Errorf("failed to fetch source %q: %w", source.Name, err)
		}
	}
	return nil
}

func (f *SourceFetcher) fetchSource(ctx context.Context, source *builder.Source) error {
	if source.Git != nil {
		return f.fetchGitSource(ctx, source)
	}

	// Future: support other source types (http, s3, etc.)
	return fmt.Errorf("source %q has no valid source type defined (git, etc.)", source.Name)
}

func (f *SourceFetcher) fetchGitSource(ctx context.Context, source *builder.Source) error {
	gitSource := source.Git

	destDir := filepath.Join(f.BaseDir, source.Name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	logging.DebugContext(ctx, "Cloning %s to %s", gitSource.Repository, destDir)

	cloneOpts := &git.CloneOptions{
		URL:      gitSource.Repository,
		Progress: os.Stdout,
	}

	if gitSource.Depth > 0 {
		cloneOpts.Depth = gitSource.Depth
	}

	auth, err := f.getGitAuth(ctx, gitSource)
	if err != nil {
		return fmt.Errorf("failed to configure git auth: %w", err)
	}
	if auth != nil {
		cloneOpts.Auth = auth
	}

	if gitSource.Ref != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(gitSource.Ref)
		cloneOpts.SingleBranch = true
	}

	repo, err := git.PlainCloneContext(ctx, destDir, false, cloneOpts)
	if err != nil {
		// If branch reference failed, try as a tag
		if gitSource.Ref != "" && strings.Contains(err.Error(), "reference not found") {
			logging.DebugContext(ctx, "Branch %s not found, trying as tag", gitSource.Ref)
			cloneOpts.ReferenceName = plumbing.NewTagReferenceName(gitSource.Ref)
			repo, err = git.PlainCloneContext(ctx, destDir, false, cloneOpts)
		}

		// If still failing and we have a ref, try cloning without ref and checkout
		if err != nil && gitSource.Ref != "" {
			logging.DebugContext(ctx, "Reference clone failed, trying full clone with checkout")
			cloneOpts.ReferenceName = ""
			cloneOpts.SingleBranch = false
			repo, err = git.PlainCloneContext(ctx, destDir, false, cloneOpts)
			if err == nil {
				err = f.checkoutRef(ctx, repo, gitSource.Ref)
			}
		}

		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	source.Path = destDir

	head, err := repo.Head()
	if err == nil {
		logging.InfoContext(ctx, "Cloned %s at %s", source.Name, head.Hash().String()[:8])
	}

	return nil
}

func (f *SourceFetcher) checkoutRef(ctx context.Context, repo *git.Repository, ref string) error {
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	hash := plumbing.NewHash(ref)
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: hash,
	})
	if err == nil {
		return nil
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(ref),
	})
	if err == nil {
		return nil
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewTagReferenceName(ref),
	})
	if err == nil {
		return nil
	}

	return fmt.Errorf("could not checkout ref %s: not a valid branch, tag, or commit", ref)
}

func (f *SourceFetcher) getGitAuth(ctx context.Context, gitSource *builder.GitSource) (transport.AuthMethod, error) {
	if gitSource.Auth == nil {
		return nil, nil
	}

	auth := gitSource.Auth

	if auth.SSHKey != "" || auth.SSHKeyFile != "" {
		return f.getSSHAuth(ctx, auth)
	}

	if auth.Token != "" {
		username := auth.Username
		if username == "" {
			username = "x-access-token"
		}
		return &http.BasicAuth{
			Username: username,
			Password: auth.Token,
		}, nil
	}

	if auth.Username != "" && auth.Password != "" {
		return &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Password,
		}, nil
	}

	return nil, nil
}

func (f *SourceFetcher) getSSHAuth(ctx context.Context, auth *builder.GitAuth) (transport.AuthMethod, error) {
	if auth.SSHKeyFile != "" {
		keyPath := expandPath(auth.SSHKeyFile)
		publicKeys, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", keyPath, err)
		}
		return publicKeys, nil
	}

	if auth.SSHKey != "" {
		publicKeys, err := ssh.NewPublicKeys("git", []byte(auth.SSHKey), "")
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}
		return publicKeys, nil
	}

	return nil, nil
}

func (f *SourceFetcher) Cleanup() error {
	if f.BaseDir != "" && strings.Contains(f.BaseDir, sourcesTempDirPrefix) {
		return os.RemoveAll(f.BaseDir)
	}
	return nil
}

func GetSourcePath(sources []builder.Source, name string) (string, error) {
	for _, s := range sources {
		if s.Name == name {
			if s.Path == "" {
				return "", fmt.Errorf("source %q has not been fetched yet", name)
			}
			return s.Path, nil
		}
	}
	return "", fmt.Errorf("source %q not found", name)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return os.ExpandEnv(path)
}

// FetchSourcesWithCleanup fetches external sources and returns a cleanup function.
// If no sources are defined, it returns a no-op cleanup function.
// The cleanup function removes both the fetched sources and the copied sources directory.
func FetchSourcesWithCleanup(ctx context.Context, sources []builder.Source, configFilePath string) (func(), error) {
	if len(sources) == 0 {
		return func() {}, nil
	}

	fetcher, err := NewSourceFetcher("")
	if err != nil {
		return nil, fmt.Errorf("failed to create source fetcher: %w", err)
	}

	if err := fetcher.FetchSources(ctx, sources); err != nil {
		_ = fetcher.Cleanup()
		return nil, err
	}

	configDir := "."
	if configFilePath != "" {
		configDir = filepath.Dir(configFilePath)
	}

	sourcesDir := filepath.Join(configDir, sourcesLocalDirName)
	if err := os.MkdirAll(sourcesDir, 0755); err != nil {
		_ = fetcher.Cleanup()
		return nil, fmt.Errorf("failed to create sources directory: %w", err)
	}

	for i := range sources {
		source := &sources[i]
		if source.Path == "" {
			continue
		}

		destPath := filepath.Join(sourcesDir, source.Name)
		logging.DebugContext(ctx, "Copying source from %s to %s", source.Path, destPath)

		if err := copyDir(ctx, source.Path, destPath); err != nil {
			_ = fetcher.Cleanup()
			_ = os.RemoveAll(sourcesDir)
			return nil, fmt.Errorf("failed to copy source %s: %w", source.Name, err)
		}

		absDestPath, err := filepath.Abs(destPath)
		if err != nil {
			_ = fetcher.Cleanup()
			_ = os.RemoveAll(sourcesDir)
			return nil, fmt.Errorf("failed to get absolute path for source %s: %w", source.Name, err)
		}
		source.Path = absDestPath
	}

	cleanup := func() {
		if cleanupErr := fetcher.Cleanup(); cleanupErr != nil {
			logging.WarnContext(ctx, "Failed to cleanup fetched sources: %v", cleanupErr)
		}
		if cleanupErr := os.RemoveAll(sourcesDir); cleanupErr != nil {
			logging.WarnContext(ctx, "Failed to cleanup %s: %v", sourcesLocalDirName, cleanupErr)
		}
	}

	return cleanup, nil
}

// InjectTokenIntoURL injects an auth token into an HTTPS git URL
// This is useful for CI/CD systems that pass tokens via URL
func InjectTokenIntoURL(repoURL, token string) (string, error) {
	if token == "" {
		return repoURL, nil
	}

	// Don't modify SSH URLs (git@... or ssh://...)
	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {
		return repoURL, nil
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid repository URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return repoURL, nil // Only inject for HTTPS URLs
	}

	parsed.User = url.UserPassword("x-access-token", token)
	return parsed.String(), nil
}

// copyDir recursively copies a directory from src to dst.
// It preserves file modes and directory structure.
//
// The context is checked before each file copy, allowing the operation to be
// cancelled during long-running copies of large directories.
//
// Race Condition Considerations:
//   - This function is NOT safe for concurrent modification of the source directory.
//   - If files are added/removed/modified in src during the copy, behavior is undefined:
//   - Added files may or may not be copied
//   - Removed files will cause errors
//   - Modified files may be partially copied
//   - The caller MUST ensure src is stable (not being modified) during the copy.
//   - For git repositories, this is safe because sources are freshly cloned and
//     not modified until the copy completes.
//
// Thread Safety: This function is not goroutine-safe for the same src/dst pair.
// Multiple goroutines may call copyDir concurrently with different directories.
func copyDir(ctx context.Context, src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		// Check for cancellation before processing each entry
		if err := ctx.Err(); err != nil {
			return err
		}

		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(ctx, path, destPath, info.Mode())
	})
}

// copyFile copies a single file from src to dst with proper error handling for file closes.
// The context is checked before the copy begins to support cancellation.
func copyFile(ctx context.Context, src, dst string, mode os.FileMode) (retErr error) {
	// Check for cancellation before starting the copy
	if err := ctx.Err(); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close source file: %w", closeErr)
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close destination file: %w", closeErr)
		}
	}()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	return os.Chmod(dst, mode)
}
