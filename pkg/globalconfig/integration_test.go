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

// TestIntegration_ConfigPrecedence tests the full precedence chain:
// Defaults < Config File < Environment Variables
func TestIntegration_ConfigPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config file with some values
	configContent := `registry:
  default: ghcr.io/fromfile
log:
  level: info
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Set environment variable to override registry
	os.Setenv("WARPGATE_REGISTRY_DEFAULT", "docker.io/fromenv")
	os.Setenv("WARPGATE_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("WARPGATE_REGISTRY_DEFAULT")
		os.Unsetenv("WARPGATE_LOG_LEVEL")
	}()

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Environment variable should win over config file
	if config.Registry.Default != "docker.io/fromenv" {
		t.Errorf("Expected registry 'docker.io/fromenv', got '%s'", config.Registry.Default)
	}

	if config.Log.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Log.Level)
	}

	// Values not set anywhere should have defaults
	if config.Storage.Driver != "vfs" {
		t.Errorf("Expected default storage driver 'vfs', got '%s'", config.Storage.Driver)
	}
}

// TestIntegration_RealWorldScenario tests a realistic user scenario
func TestIntegration_RealWorldScenario(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Simulate a user's config with GitHub credentials
	configContent := `registry:
  default: ghcr.io/myorg
  username: testuser

build:
  default_arch:
    - amd64
    - arm64
  parallel_builds: true

log:
  level: info
  format: color
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Simulate CI environment with token
	os.Setenv("WARPGATE_REGISTRY_TOKEN", "ghp_secrettoken")
	defer os.Unsetenv("WARPGATE_REGISTRY_TOKEN")

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify all values loaded correctly
	if config.Registry.Default != "ghcr.io/myorg" {
		t.Errorf("Expected registry 'ghcr.io/myorg', got '%s'", config.Registry.Default)
	}

	if config.Registry.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", config.Registry.Username)
	}

	if config.Registry.Token != "ghp_secrettoken" {
		t.Errorf("Expected token from env, got '%s'", config.Registry.Token)
	}

	if len(config.Build.DefaultArch) != 2 {
		t.Errorf("Expected 2 architectures, got %d", len(config.Build.DefaultArch))
	}

	if !config.Build.ParallelBuilds {
		t.Error("Expected parallel builds to be enabled")
	}

	if config.Log.Level != "info" {
		t.Errorf("Expected log level 'info', got '%s'", config.Log.Level)
	}

	if config.Log.Format != "color" {
		t.Errorf("Expected log format 'color', got '%s'", config.Log.Format)
	}
}
