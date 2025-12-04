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

package globalconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_Defaults tests that defaults work without a config file
func TestLoad_Defaults(t *testing.T) {
	// Change to a temp directory where no config file exists
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load defaults: %v", err)
	}

	// Check defaults
	if config.Log.Level != "info" {
		t.Errorf("Expected log level 'info', got '%s'", config.Log.Level)
	}

	if config.Log.Format != "color" {
		t.Errorf("Expected log format 'color', got '%s'", config.Log.Format)
	}

	if config.Storage.Driver != "vfs" {
		t.Errorf("Expected storage driver 'vfs', got '%s'", config.Storage.Driver)
	}

	if config.Registry.Default != "ghcr.io" {
		t.Errorf("Expected registry 'ghcr.io', got '%s'", config.Registry.Default)
	}

	if !config.Build.ParallelBuilds {
		t.Error("Expected parallel builds to be enabled by default")
	}

	if len(config.Build.DefaultArch) != 1 || config.Build.DefaultArch[0] != "amd64" {
		t.Errorf("Expected default arch ['amd64'], got %v", config.Build.DefaultArch)
	}

	if config.Build.Timeout != "2h" {
		t.Errorf("Expected build timeout '2h', got '%s'", config.Build.Timeout)
	}
}

// TestLoadFromPath tests loading from a specific file
func TestLoadFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a test config file
	configContent := `registry:
  default: docker.io/myorg
  username: testuser
  token: testtoken

log:
  level: debug
  format: json

build:
  default_arch:
    - amd64
    - arm64
  parallel_builds: false
  timeout: 4h

storage:
  driver: overlay
  root: /custom/storage

templates:
  cache_dir: /custom/cache
  default_repo: github.com/custom/templates
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from path: %v", err)
	}

	// Verify loaded values
	if config.Registry.Default != "docker.io/myorg" {
		t.Errorf("Expected registry 'docker.io/myorg', got '%s'", config.Registry.Default)
	}

	if config.Registry.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", config.Registry.Username)
	}

	if config.Registry.Token != "testtoken" {
		t.Errorf("Expected token 'testtoken', got '%s'", config.Registry.Token)
	}

	if config.Log.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Log.Level)
	}

	if config.Log.Format != "json" {
		t.Errorf("Expected log format 'json', got '%s'", config.Log.Format)
	}

	if config.Build.ParallelBuilds {
		t.Error("Expected parallel builds to be disabled")
	}

	if len(config.Build.DefaultArch) != 2 {
		t.Errorf("Expected 2 architectures, got %d", len(config.Build.DefaultArch))
	}

	if config.Build.Timeout != "4h" {
		t.Errorf("Expected timeout '4h', got '%s'", config.Build.Timeout)
	}

	if config.Storage.Driver != "overlay" {
		t.Errorf("Expected storage driver 'overlay', got '%s'", config.Storage.Driver)
	}

	if config.Templates.CacheDir != "/custom/cache" {
		t.Errorf("Expected cache dir '/custom/cache', got '%s'", config.Templates.CacheDir)
	}

	if config.Templates.DefaultRepo != "github.com/custom/templates" {
		t.Errorf("Expected repo 'github.com/custom/templates', got '%s'", config.Templates.DefaultRepo)
	}
}

// TestLoad_EnvVarOverride tests that environment variables override config file
func TestLoad_EnvVarOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a test config file
	configContent := `registry:
  default: ghcr.io
log:
  level: info
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set environment variables
	os.Setenv("WARPGATE_REGISTRY_DEFAULT", "docker.io")
	os.Setenv("WARPGATE_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("WARPGATE_REGISTRY_DEFAULT")
		os.Unsetenv("WARPGATE_LOG_LEVEL")
	}()

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Environment variables should override file values
	if config.Registry.Default != "docker.io" {
		t.Errorf("Expected registry 'docker.io' from env var, got '%s'", config.Registry.Default)
	}

	if config.Log.Level != "debug" {
		t.Errorf("Expected log level 'debug' from env var, got '%s'", config.Log.Level)
	}
}

// TestLoad_PartialConfig tests loading a partial config file with defaults
func TestLoad_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a minimal config file (most fields missing)
	configContent := `registry:
  default: custom.registry.io
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Should have custom value
	if config.Registry.Default != "custom.registry.io" {
		t.Errorf("Expected registry 'custom.registry.io', got '%s'", config.Registry.Default)
	}

	// Should have defaults for everything else
	if config.Log.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", config.Log.Level)
	}

	if config.Storage.Driver != "vfs" {
		t.Errorf("Expected default storage driver 'vfs', got '%s'", config.Storage.Driver)
	}
}

// TestGet tests the convenience Get function
func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// Should return defaults when no config file exists
	if config.Log.Level != "info" {
		t.Errorf("Expected log level 'info', got '%s'", config.Log.Level)
	}
}
