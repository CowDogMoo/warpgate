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

// Package pathexpand provides functionality for expanding paths with tilde and environment variables.
package pathexpand

import (
	"os"
	"path/filepath"
	"strings"
)

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
