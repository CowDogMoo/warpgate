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

// SourceFetcher handles fetching external sources before builds
type SourceFetcher struct {
	// BaseDir is the base directory where sources will be cloned
	BaseDir string
}

// NewSourceFetcher creates a new source fetcher
// If baseDir is empty, a temporary directory will be created
func NewSourceFetcher(baseDir string) (*SourceFetcher, error) {
	if baseDir == "" {
		tmpDir, err := os.MkdirTemp("", "warpgate-sources-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory for sources: %w", err)
		}
		baseDir = tmpDir
	}

	return &SourceFetcher{
		BaseDir: baseDir,
	}, nil
}

// FetchSources fetches all sources defined in the config
// It populates the Path field of each source with the local path
func (f *SourceFetcher) FetchSources(ctx context.Context, sources []builder.Source) error {
	for i := range sources {
		source := &sources[i]
		if err := f.fetchSource(ctx, source); err != nil {
			return fmt.Errorf("failed to fetch source %q: %w", source.Name, err)
		}
	}
	return nil
}

// fetchSource fetches a single source
func (f *SourceFetcher) fetchSource(ctx context.Context, source *builder.Source) error {
	if source.Git != nil {
		return f.fetchGitSource(ctx, source)
	}

	// Future: support other source types (http, s3, etc.)
	return fmt.Errorf("source %q has no valid source type defined (git, etc.)", source.Name)
}

// fetchGitSource clones a git repository
func (f *SourceFetcher) fetchGitSource(ctx context.Context, source *builder.Source) error {
	gitSource := source.Git

	// Create destination directory
	destDir := filepath.Join(f.BaseDir, source.Name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	logging.InfoContext(ctx, "Cloning %s to %s", gitSource.Repository, destDir)

	// Build clone options
	cloneOpts := &git.CloneOptions{
		URL:      gitSource.Repository,
		Progress: os.Stdout,
	}

	// Set depth for shallow clone
	if gitSource.Depth > 0 {
		cloneOpts.Depth = gitSource.Depth
	}

	// Set up authentication
	auth, err := f.getGitAuth(ctx, gitSource)
	if err != nil {
		return fmt.Errorf("failed to configure git auth: %w", err)
	}
	if auth != nil {
		cloneOpts.Auth = auth
	}

	// Set reference if specified
	if gitSource.Ref != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(gitSource.Ref)
		cloneOpts.SingleBranch = true
	}

	// Clone the repository
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
				// Try to checkout the ref as a commit hash
				err = f.checkoutRef(ctx, repo, gitSource.Ref)
			}
		}

		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	// Update source with the local path
	source.Path = destDir

	head, err := repo.Head()
	if err == nil {
		logging.InfoContext(ctx, "Cloned %s at %s", source.Name, head.Hash().String()[:8])
	}

	return nil
}

// checkoutRef checks out a specific ref (commit, tag, or branch)
func (f *SourceFetcher) checkoutRef(ctx context.Context, repo *git.Repository, ref string) error {
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Try as commit hash first
	hash := plumbing.NewHash(ref)
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: hash,
	})
	if err == nil {
		return nil
	}

	// Try as branch
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(ref),
	})
	if err == nil {
		return nil
	}

	// Try as tag
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewTagReferenceName(ref),
	})
	if err == nil {
		return nil
	}

	return fmt.Errorf("could not checkout ref %s: not a valid branch, tag, or commit", ref)
}

// getGitAuth returns the appropriate authentication method for the git source
func (f *SourceFetcher) getGitAuth(ctx context.Context, gitSource *builder.GitSource) (transport.AuthMethod, error) {
	if gitSource.Auth == nil {
		return nil, nil
	}

	auth := gitSource.Auth

	// SSH key authentication
	if auth.SSHKey != "" || auth.SSHKeyFile != "" {
		return f.getSSHAuth(ctx, auth)
	}

	// Token authentication (GitHub PAT, GitLab token, etc.)
	if auth.Token != "" {
		// For GitHub, GitLab, etc., use token as password with any username
		username := auth.Username
		if username == "" {
			username = "x-access-token" // Works for GitHub, GitLab, etc.
		}
		return &http.BasicAuth{
			Username: username,
			Password: auth.Token,
		}, nil
	}

	// Basic auth with username/password
	if auth.Username != "" && auth.Password != "" {
		return &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Password,
		}, nil
	}

	return nil, nil
}

// getSSHAuth returns SSH authentication
func (f *SourceFetcher) getSSHAuth(ctx context.Context, auth *builder.GitAuth) (transport.AuthMethod, error) {
	// SSH key from file
	if auth.SSHKeyFile != "" {
		keyPath := expandPath(auth.SSHKeyFile)
		publicKeys, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", keyPath, err)
		}
		return publicKeys, nil
	}

	// SSH key from content (e.g., from environment variable)
	if auth.SSHKey != "" {
		publicKeys, err := ssh.NewPublicKeys("git", []byte(auth.SSHKey), "")
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}
		return publicKeys, nil
	}

	return nil, nil
}

// Cleanup removes the fetched sources directory
func (f *SourceFetcher) Cleanup() error {
	if f.BaseDir != "" && strings.Contains(f.BaseDir, "warpgate-sources-") {
		return os.RemoveAll(f.BaseDir)
	}
	return nil
}

// GetSourcePath returns the local path for a named source
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

// expandPath expands ~ and environment variables in a path
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

	sourcesDir := filepath.Join(configDir, ".warpgate-sources")
	if err := os.MkdirAll(sourcesDir, 0755); err != nil {
		_ = fetcher.Cleanup()
		return nil, fmt.Errorf("failed to create sources directory: %w", err)
	}

	// Copy each fetched source into the sources directory
	for i := range sources {
		source := &sources[i]
		if source.Path == "" {
			continue
		}

		destPath := filepath.Join(sourcesDir, source.Name)
		logging.InfoContext(ctx, "Copying source from %s to %s", source.Path, destPath)

		if err := copyDir(source.Path, destPath); err != nil {
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
			logging.WarnContext(ctx, "Failed to cleanup .warpgate-sources: %v", cleanupErr)
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

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if err := srcFile.Close(); err != nil {
				logging.Warn("Failed to close source file %s: %v", path, err)
			}
		}()

		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer func() {
			if err := destFile.Close(); err != nil {
				logging.Warn("Failed to close destination file %s: %v", destPath, err)
			}
		}()

		if _, err := io.Copy(destFile, srcFile); err != nil {
			return err
		}

		return os.Chmod(destPath, info.Mode())
	})
}
