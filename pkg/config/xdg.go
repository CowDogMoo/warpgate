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

package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// getConfigHome returns the base config directory following CLI tool conventions.
// Uses $XDG_CONFIG_HOME or ~/.config on Unix-like systems.
func getConfigHome() string {
	// Check XDG_CONFIG_HOME first (works on both Linux and macOS)
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return configHome
	}

	// Default to ~/.config on all Unix-like systems for CLI tool consistency
	// This provides better cross-platform dotfile management
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config")
	}

	// Fallback (should rarely happen)
	return ""
}

// getCacheHome returns the base cache directory following CLI tool conventions
// On Unix-like systems (Linux, macOS, BSD), uses $XDG_CACHE_HOME or ~/.cache
func getCacheHome() string {
	// Check XDG_CACHE_HOME first (works on both Linux and macOS)
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return cacheHome
	}

	// Default to ~/.cache on all Unix-like systems for CLI tool consistency
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache")
	}

	// Fallback (should rarely happen)
	return ""
}

// GetConfigDirs returns all config directories to search (in priority order).
func GetConfigDirs() []string {
	return getConfigDirs()
}

// getConfigDirs returns all config directories to search (in priority order).
func getConfigDirs() []string {
	dirs := []string{}

	// Primary: XDG config home (e.g., ~/.config)
	if configHome := getConfigHome(); configHome != "" {
		dirs = append(dirs, filepath.Join(configHome, "warpgate"))
	}

	// Secondary: Legacy warpgate location (backward compatibility)
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".warpgate"))
	}

	// Tertiary: System-wide config (Linux/BSD)
	// On macOS, we skip /etc/xdg as it's not commonly used for CLI tools
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" {
		if xdgConfigDirs := os.Getenv("XDG_CONFIG_DIRS"); xdgConfigDirs != "" {
			// Parse colon-separated paths
			for _, dir := range filepath.SplitList(xdgConfigDirs) {
				if dir != "" {
					dirs = append(dirs, filepath.Join(dir, "warpgate"))
				}
			}
		} else {
			// Default XDG system directory
			dirs = append(dirs, filepath.Join("/etc", "xdg", "warpgate"))
		}
	}

	return dirs
}

// ConfigFile returns the path for creating a new config file.
func ConfigFile(filename string) (string, error) {
	configHome := getConfigHome()
	if configHome == "" {
		return "", os.ErrNotExist
	}

	configPath := filepath.Join(configHome, "warpgate", filename)
	configDir := filepath.Dir(configPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, DirPermReadWriteExec); err != nil {
		return "", err
	}

	return configPath, nil
}

// CacheFile returns the path for a cache file
func CacheFile(filename string) (string, error) {
	cacheHome := getCacheHome()
	if cacheHome == "" {
		return "", os.ErrNotExist
	}

	cachePath := filepath.Join(cacheHome, "warpgate", filename)
	cacheDir := filepath.Dir(cachePath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, DirPermReadWriteExec); err != nil {
		return "", err
	}

	return cachePath, nil
}
