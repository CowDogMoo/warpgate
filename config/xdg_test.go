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
	"testing"
)

// TestGetConfigDirs_WithXDGConfigHome tests GetConfigDirs with XDG_CONFIG_HOME set
func TestGetConfigDirs_WithXDGConfigHome(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Setenv("XDG_CONFIG_HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set XDG_CONFIG_HOME: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
			t.Logf("Failed to unset XDG_CONFIG_HOME: %v", err)
		}
	}()

	dirs := GetConfigDirs()

	// First directory should be XDG_CONFIG_HOME/warpgate
	expectedFirst := filepath.Join(tmpDir, "warpgate")
	if len(dirs) == 0 || dirs[0] != expectedFirst {
		t.Errorf("Expected first dir to be %s, got %v", expectedFirst, dirs)
	}

	// Should include legacy ~/.warpgate path
	home, _ := os.UserHomeDir()
	legacyPath := filepath.Join(home, ".warpgate")
	found := false
	for _, dir := range dirs {
		if dir == legacyPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find legacy path %s in dirs %v", legacyPath, dirs)
	}
}

// TestGetConfigDirs_WithoutXDGConfigHome tests GetConfigDirs defaults to ~/.config
func TestGetConfigDirs_WithoutXDGConfigHome(t *testing.T) {
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Logf("Failed to unset XDG_CONFIG_HOME: %v", err)
	}

	dirs := GetConfigDirs()

	// First directory should be ~/.config/warpgate
	home, _ := os.UserHomeDir()
	expectedFirst := filepath.Join(home, ".config", "warpgate")
	if len(dirs) == 0 || dirs[0] != expectedFirst {
		t.Errorf("Expected first dir to be %s, got %v", expectedFirst, dirs)
	}

	// Second should be legacy ~/.warpgate
	expectedLegacy := filepath.Join(home, ".warpgate")
	if len(dirs) < 2 || dirs[1] != expectedLegacy {
		t.Errorf("Expected second dir to be %s, got %v", expectedLegacy, dirs)
	}
}

// TestGetConfigDirs_SystemPaths tests system-wide config directories on Linux
func TestGetConfigDirs_SystemPaths(t *testing.T) {
	// Only test on Linux
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "openbsd" {
		t.Skip("System paths only apply on Linux/BSD")
	}

	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Logf("Failed to unset XDG_CONFIG_HOME: %v", err)
	}
	if err := os.Unsetenv("XDG_CONFIG_DIRS"); err != nil {
		t.Logf("Failed to unset XDG_CONFIG_DIRS: %v", err)
	}

	dirs := GetConfigDirs()

	// Should include /etc/xdg/warpgate on Linux
	expectedSystem := filepath.Join("/etc", "xdg", "warpgate")
	found := false
	for _, dir := range dirs {
		if dir == expectedSystem {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find system path %s in dirs %v", expectedSystem, dirs)
	}
}

// TestGetConfigDirs_CustomXDGConfigDirs tests custom XDG_CONFIG_DIRS on Linux
func TestGetConfigDirs_CustomXDGConfigDirs(t *testing.T) {
	// Only test on Linux
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "openbsd" {
		t.Skip("XDG_CONFIG_DIRS only applies on Linux/BSD")
	}

	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom")
	if err := os.Setenv("XDG_CONFIG_DIRS", customPath); err != nil {
		t.Fatalf("Failed to set XDG_CONFIG_DIRS: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CONFIG_DIRS"); err != nil {
			t.Logf("Failed to unset XDG_CONFIG_DIRS: %v", err)
		}
	}()

	dirs := GetConfigDirs()

	// Should include custom path
	expectedCustom := filepath.Join(customPath, "warpgate")
	found := false
	for _, dir := range dirs {
		if dir == expectedCustom {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find custom path %s in dirs %v", expectedCustom, dirs)
	}
}

// TestGetConfigDirs_macOSNoSystemPaths tests that macOS doesn't include /etc/xdg
func TestGetConfigDirs_macOSNoSystemPaths(t *testing.T) {
	// Only test on macOS
	if runtime.GOOS != "darwin" {
		t.Skip("This test only applies to macOS")
	}

	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Logf("Failed to unset XDG_CONFIG_HOME: %v", err)
	}
	if err := os.Unsetenv("XDG_CONFIG_DIRS"); err != nil {
		t.Logf("Failed to unset XDG_CONFIG_DIRS: %v", err)
	}

	dirs := GetConfigDirs()

	// Should NOT include /etc/xdg/warpgate on macOS
	systemPath := filepath.Join("/etc", "xdg", "warpgate")
	for _, dir := range dirs {
		if dir == systemPath {
			t.Errorf("macOS should not include system path %s, but found it in %v", systemPath, dirs)
		}
	}
}

// TestConfigFile_WithXDGConfigHome tests ConfigFile with XDG_CONFIG_HOME set
func TestConfigFile_WithXDGConfigHome(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Setenv("XDG_CONFIG_HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set XDG_CONFIG_HOME: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
			t.Logf("Failed to unset XDG_CONFIG_HOME: %v", err)
		}
	}()

	path, err := ConfigFile("config.yaml")
	if err != nil {
		t.Fatalf("ConfigFile failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "warpgate", "config.yaml")
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Errorf("Directory not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dir)
	}
}

// TestConfigFile_WithoutXDGConfigHome tests ConfigFile defaults to ~/.config
func TestConfigFile_WithoutXDGConfigHome(t *testing.T) {
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Logf("Failed to unset XDG_CONFIG_HOME: %v", err)
	}

	path, err := ConfigFile("config.yaml")
	if err != nil {
		t.Fatalf("ConfigFile failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "warpgate", "config.yaml")
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}
}

// TestConfigFile_CreatesParentDirs tests that ConfigFile creates parent directories
func TestConfigFile_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Setenv("XDG_CONFIG_HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set XDG_CONFIG_HOME: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
			t.Logf("Failed to unset XDG_CONFIG_HOME: %v", err)
		}
	}()

	// Request a file with nested subdirectories
	path, err := ConfigFile("subdir/config.yaml")
	if err != nil {
		t.Fatalf("ConfigFile failed: %v", err)
	}

	// Verify all parent directories were created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dir)
	}

	// Verify permissions are 0755
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected directory permissions 0755, got %v", info.Mode().Perm())
	}
}

// TestCacheFile_WithXDGCacheHome tests CacheFile with XDG_CACHE_HOME set
func TestCacheFile_WithXDGCacheHome(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Setenv("XDG_CACHE_HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set XDG_CACHE_HOME: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CACHE_HOME"); err != nil {
			t.Logf("Failed to unset XDG_CACHE_HOME: %v", err)
		}
	}()

	path, err := CacheFile("templates.db")
	if err != nil {
		t.Fatalf("CacheFile failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "warpgate", "templates.db")
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Errorf("Directory not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dir)
	}
}

// TestCacheFile_WithoutXDGCacheHome tests CacheFile defaults to ~/.cache
func TestCacheFile_WithoutXDGCacheHome(t *testing.T) {
	if err := os.Unsetenv("XDG_CACHE_HOME"); err != nil {
		t.Logf("Failed to unset XDG_CACHE_HOME: %v", err)
	}

	path, err := CacheFile("templates.db")
	if err != nil {
		t.Fatalf("CacheFile failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".cache", "warpgate", "templates.db")
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}
}

// TestGetConfigHome_WithXDGConfigHome tests getConfigHome with XDG_CONFIG_HOME set
func TestGetConfigHome_WithXDGConfigHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	result := getConfigHome()
	if result != tmpDir {
		t.Errorf("Expected %s, got %s", tmpDir, result)
	}
}

// TestGetConfigHome_WithoutXDGConfigHome tests getConfigHome falls back to ~/.config
func TestGetConfigHome_WithoutXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	result := getConfigHome()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	expected := filepath.Join(home, ".config")
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

// TestGetCacheHome_WithXDGCacheHome tests getCacheHome with XDG_CACHE_HOME set
func TestGetCacheHome_WithXDGCacheHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	result := getCacheHome()
	if result != tmpDir {
		t.Errorf("Expected %s, got %s", tmpDir, result)
	}
}

// TestGetCacheHome_WithoutXDGCacheHome tests getCacheHome falls back to ~/.cache
func TestGetCacheHome_WithoutXDGCacheHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")

	result := getCacheHome()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	expected := filepath.Join(home, ".cache")
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

// TestGetConfigDirs_LegacyPath tests that legacy ~/.warpgate is always included
func TestGetConfigDirs_LegacyPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dirs := GetConfigDirs()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	legacyPath := filepath.Join(home, ".warpgate")
	found := false
	for _, dir := range dirs {
		if dir == legacyPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected legacy path %s in dirs %v", legacyPath, dirs)
	}
}

// TestGetConfigDirs_EmptyXDGConfigDirs tests that empty entries in XDG_CONFIG_DIRS are skipped
func TestGetConfigDirs_EmptyXDGConfigDirs(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" && runtime.GOOS != "openbsd" {
		t.Skip("XDG_CONFIG_DIRS only applies on Linux/BSD")
	}

	// Set XDG_CONFIG_DIRS with empty entries (simulates trailing colons)
	t.Setenv("XDG_CONFIG_DIRS", ":/tmp/custom:")

	dirs := GetConfigDirs()

	// Should include /tmp/custom/warpgate but not empty entries
	foundCustom := false
	for _, dir := range dirs {
		if dir == "/tmp/custom/warpgate" {
			foundCustom = true
		}
		// Verify no empty-based paths
		if dir == "/warpgate" || dir == "warpgate" {
			t.Errorf("Found entry from empty XDG_CONFIG_DIRS component: %s", dir)
		}
	}
	if !foundCustom {
		t.Errorf("Expected to find /tmp/custom/warpgate in dirs %v", dirs)
	}
}

// TestConfigFile_NestedSubdirectory tests ConfigFile with deeply nested path
func TestConfigFile_NestedSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path, err := ConfigFile("deep/nested/sub/config.yaml")
	if err != nil {
		t.Fatalf("ConfigFile failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "warpgate", "deep", "nested", "sub", "config.yaml")
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}

	// Verify all parent directories were created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dir)
	}
}

// TestCacheFile_NestedSubdirectory tests CacheFile with deeply nested path
func TestCacheFile_NestedSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	path, err := CacheFile("deep/nested/cache.db")
	if err != nil {
		t.Fatalf("CacheFile failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "warpgate", "deep", "nested", "cache.db")
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}

	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dir)
	}
}

// TestGetConfigDirs_CountAndOrder verifies the number and order of config directories
func TestGetConfigDirs_CountAndOrder(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	dirs := GetConfigDirs()

	// Should have at least 2 entries: XDG config home + legacy path
	if len(dirs) < 2 {
		t.Errorf("Expected at least 2 config dirs, got %d: %v", len(dirs), dirs)
	}

	// First should be XDG based
	expectedFirst := filepath.Join(tmpDir, "warpgate")
	if dirs[0] != expectedFirst {
		t.Errorf("Expected first dir to be %s, got %s", expectedFirst, dirs[0])
	}

	// Second should be legacy
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}
	expectedSecond := filepath.Join(home, ".warpgate")
	if dirs[1] != expectedSecond {
		t.Errorf("Expected second dir to be %s, got %s", expectedSecond, dirs[1])
	}
}

// TestCacheFile_CreatesParentDirs tests that CacheFile creates parent directories
func TestCacheFile_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Setenv("XDG_CACHE_HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set XDG_CACHE_HOME: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CACHE_HOME"); err != nil {
			t.Logf("Failed to unset XDG_CACHE_HOME: %v", err)
		}
	}()

	// Request a file with nested subdirectories
	path, err := CacheFile("templates/metadata.json")
	if err != nil {
		t.Fatalf("CacheFile failed: %v", err)
	}

	// Verify all parent directories were created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dir)
	}

	// Verify permissions are 0755
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected directory permissions 0755, got %v", info.Mode().Perm())
	}
}
