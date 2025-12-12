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
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GitOperations handles git operations for template repositories
type GitOperations struct {
	cacheDir string
}

// NewGitOperations creates a new git operations handler
func NewGitOperations(cacheDir string) *GitOperations {
	return &GitOperations{
		cacheDir: cacheDir,
	}
}

// CloneOrUpdate clones a repository if it doesn't exist, or updates it if it does
func (g *GitOperations) CloneOrUpdate(gitURL, version string) (string, error) {
	repoPath := g.getCachePath(gitURL, version)

	// Check if already cached
	if dirExists(repoPath) {
		logging.Debug("Repository already cached at %s, pulling updates", repoPath)
		if err := g.pullUpdates(repoPath); err != nil {
			logging.Warn("Failed to pull updates, using cached version: %v", err)
		}
		return repoPath, nil
	}

	// Clone fresh
	logging.Info("Cloning repository from %s", gitURL)
	return g.clone(gitURL, version, repoPath)
}

// clone clones a repository to the specified path
func (g *GitOperations) clone(gitURL, version, repoPath string) (string, error) {
	cloneOpts := &git.CloneOptions{
		URL: gitURL,
	}

	// Only show progress if not in quiet mode
	if !logging.IsQuiet() {
		cloneOpts.Progress = os.Stdout
	}

	// If a specific version is requested, try to checkout that tag/branch
	if version != "" && version != "main" && version != "master" {
		// Try as a tag first
		cloneOpts.ReferenceName = plumbing.NewTagReferenceName(version)
		cloneOpts.SingleBranch = true
	}

	repo, err := git.PlainClone(repoPath, false, cloneOpts)
	if err != nil {
		// If tag clone failed, try as a branch
		if version != "" && version != "main" && version != "master" {
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(version)
			repo, err = git.PlainClone(repoPath, false, cloneOpts)
			if err != nil {
				return "", fmt.Errorf("failed to clone repository: %w", err)
			}
		} else {
			return "", fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	// If version is specified, try to checkout
	if version != "" && version != "main" && version != "master" {
		w, err := repo.Worktree()
		if err != nil {
			return repoPath, nil // Return path even if checkout fails
		}

		// Try to checkout the version
		checkoutOpts := &git.CheckoutOptions{
			Branch: plumbing.NewTagReferenceName(version),
		}
		if err := w.Checkout(checkoutOpts); err != nil {
			// Try as a branch
			checkoutOpts.Branch = plumbing.NewBranchReferenceName(version)
			if err := w.Checkout(checkoutOpts); err != nil {
				logging.Warn("Could not checkout version %s, using default branch", version)
			}
		}
	}

	return repoPath, nil
}

// pullUpdates pulls the latest changes from the remote
func (g *GitOperations) pullUpdates(repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	err = w.Pull(&git.PullOptions{
		RemoteName: "origin",
		Progress:   os.Stdout,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull updates: %w", err)
	}

	return nil
}

// getCachePath generates a cache path for a repository
func (g *GitOperations) getCachePath(gitURL, version string) string {
	// Clean the URL for use in path
	cleanURL := strings.TrimPrefix(gitURL, "https://")
	cleanURL = strings.TrimPrefix(cleanURL, "http://")
	cleanURL = strings.TrimPrefix(cleanURL, "git@")
	cleanURL = strings.ReplaceAll(cleanURL, ":", "/")
	cleanURL = strings.TrimSuffix(cleanURL, ".git")

	// Add version to path if specified
	if version != "" && version != "main" && version != "master" {
		hash := sha256.Sum256([]byte(version))
		versionHash := fmt.Sprintf("%x", hash)[:8]
		cleanURL = filepath.Join(cleanURL, versionHash)
	}

	return filepath.Join(g.cacheDir, cleanURL)
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
