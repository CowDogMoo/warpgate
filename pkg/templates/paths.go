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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/errors"
)

// PathValidator handles template path validation and normalization.
type PathValidator struct{}

// NewPathValidator creates a new path validator.
func NewPathValidator() *PathValidator {
	return &PathValidator{}
}

// IsGitURL checks if a string is a git URL.
func (pv *PathValidator) IsGitURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@")
}

// IsLocalPath checks if a path is a local directory.
func (pv *PathValidator) IsLocalPath(path string) bool {
	if filepath.IsAbs(path) {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}

	if strings.HasPrefix(path, ".") || strings.HasPrefix(path, "~") {
		expandedPath, err := ExpandPath(path)
		if err != nil {
			return false
		}
		info, err := os.Stat(expandedPath)
		return err == nil && info.IsDir()
	}

	return !pv.IsGitURL(path)
}

// NormalizePath normalizes a path for comparison by expanding ~ and converting to absolute path.
func (pv *PathValidator) NormalizePath(path string) (string, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return "", errors.Wrap("normalize path", path, err)
	}

	// Make path absolute if relative (and it looks like a path)
	if !filepath.IsAbs(expandedPath) && (strings.Contains(expandedPath, "/") || strings.HasPrefix(expandedPath, ".")) {
		absPath, err := filepath.Abs(expandedPath)
		if err != nil {
			return "", errors.Wrap("get absolute path", expandedPath, err)
		}
		return absPath, nil
	}

	return expandedPath, nil
}

// ValidateLocalPath validates that a path exists and is a directory.
func (pv *PathValidator) ValidateLocalPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}
		return fmt.Errorf("failed to access path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	return nil
}

// ExpandPath expands ~ in paths and converts to absolute paths.
func (pv *PathValidator) ExpandPath(path string) (string, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return "", errors.Wrap("expand path", path, err)
	}

	// Make path absolute if relative
	if !filepath.IsAbs(expandedPath) {
		absPath, err := filepath.Abs(expandedPath)
		if err != nil {
			return "", errors.Wrap("get absolute path", expandedPath, err)
		}
		return absPath, nil
	}

	return expandedPath, nil
}

// FileExists checks if a file or directory exists.
func (pv *PathValidator) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a path exists and is a directory.
func (pv *PathValidator) DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ExtractRepoName extracts a repository name from a git URL.
// For example: https://git.example.com/jdoe/my-templates.git => my-templates
func ExtractRepoName(gitURL string) string {
	// Remove .git suffix if present
	name := strings.TrimSuffix(gitURL, ".git")

	// Extract the last part of the path
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}

	// For git@github.com:user/repo format
	if strings.Contains(name, ":") {
		parts := strings.Split(name, ":")
		if len(parts) > 1 {
			subparts := strings.Split(parts[1], "/")
			if len(subparts) > 0 {
				name = subparts[len(subparts)-1]
			}
		}
	}

	// Clean up the name
	name = strings.TrimSpace(name)
	if name == "" {
		name = "templates"
	}

	return name
}

// ExpandPath expands environment variables and home directory in a path.
// It handles both ${VAR} syntax and ~ (tilde) for the home directory.
//
// Examples:
//   - "~/projects" -> "/home/user/projects"
//   - "${HOME}/work" -> "/home/user/work"
//   - "~" -> "/home/user"
//   - "~/path/to/dir" -> "/home/user/path/to/dir"
func ExpandPath(path string) (string, error) {
	path = os.ExpandEnv(path)

	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

// MustExpandPath is like ExpandPath but panics on error.
// Use this only when you're certain the home directory can be determined.
func MustExpandPath(path string) string {
	expanded, err := ExpandPath(path)
	if err != nil {
		return os.ExpandEnv(path)
	}
	return expanded
}
