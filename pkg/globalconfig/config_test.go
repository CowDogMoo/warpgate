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

	// Storage driver defaults to empty (delegates to system defaults)
	// NOTE: If user has ~/.warpgate/config.yaml with storage.driver set,
	// it will be used instead. Both empty and user-configured values are valid.
	t.Logf("Storage driver: '%s' (empty means system default, non-empty means user override)", config.Storage.Driver)

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

	// Create a test config file (credentials NOT in config file - security!)
	configContent := `registry:
  default: docker.io/myorg

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
  repositories:
    custom: github.com/custom/templates
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set credentials via environment variables (secure!)
	cleanup := setupRegistryEnvVars(t)
	defer cleanup()

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from path: %v", err)
	}

	// Verify all config sections
	verifyRegistryConfig(t, config)
	verifyLogConfig(t, config)
	verifyBuildConfig(t, config)
	verifyStorageConfig(t, config)
	verifyTemplatesConfig(t, config)
}

// setupRegistryEnvVars sets up test registry credentials and returns a cleanup function
func setupRegistryEnvVars(t *testing.T) func() {
	t.Helper()
	setEnvOrFatal(t, "WARPGATE_REGISTRY_USERNAME", "testuser")
	setEnvOrFatal(t, "WARPGATE_REGISTRY_TOKEN", "testtoken")

	return func() {
		unsetEnvSafe(t, "WARPGATE_REGISTRY_USERNAME")
		unsetEnvSafe(t, "WARPGATE_REGISTRY_TOKEN")
	}
}

// verifyRegistryConfig verifies registry configuration values
func verifyRegistryConfig(t *testing.T, config *Config) {
	t.Helper()
	if config.Registry.Default != "docker.io/myorg" {
		t.Errorf("Expected registry 'docker.io/myorg', got '%s'", config.Registry.Default)
	}
	if config.Registry.Username != "testuser" {
		t.Errorf("Expected username 'testuser' from env var, got '%s'", config.Registry.Username)
	}
	if config.Registry.Token != "testtoken" {
		t.Errorf("Expected token 'testtoken' from env var, got '%s'", config.Registry.Token)
	}
}

// verifyLogConfig verifies log configuration values
func verifyLogConfig(t *testing.T, config *Config) {
	t.Helper()
	if config.Log.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Log.Level)
	}
	if config.Log.Format != "json" {
		t.Errorf("Expected log format 'json', got '%s'", config.Log.Format)
	}
}

// verifyBuildConfig verifies build configuration values
func verifyBuildConfig(t *testing.T, config *Config) {
	t.Helper()
	if config.Build.ParallelBuilds {
		t.Error("Expected parallel builds to be disabled")
	}
	if len(config.Build.DefaultArch) != 2 {
		t.Errorf("Expected 2 architectures, got %d", len(config.Build.DefaultArch))
	}
	if config.Build.Timeout != "4h" {
		t.Errorf("Expected timeout '4h', got '%s'", config.Build.Timeout)
	}
}

// verifyStorageConfig verifies storage configuration values
func verifyStorageConfig(t *testing.T, config *Config) {
	t.Helper()
	if config.Storage.Driver != "overlay" {
		t.Errorf("Expected storage driver 'overlay', got '%s'", config.Storage.Driver)
	}
}

// verifyTemplatesConfig verifies templates configuration values
func verifyTemplatesConfig(t *testing.T, config *Config) {
	t.Helper()
	if config.Templates.CacheDir != "/custom/cache" {
		t.Errorf("Expected cache dir '/custom/cache', got '%s'", config.Templates.CacheDir)
	}
	if config.Templates.Repositories["custom"] != "github.com/custom/templates" {
		t.Errorf("Expected custom repo 'github.com/custom/templates', got '%s'", config.Templates.Repositories["custom"])
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
	if err := os.Setenv("WARPGATE_REGISTRY_DEFAULT", "docker.io"); err != nil {
		t.Fatalf("Failed to set WARPGATE_REGISTRY_DEFAULT: %v", err)
	}
	if err := os.Setenv("WARPGATE_LOG_LEVEL", "debug"); err != nil {
		t.Fatalf("Failed to set WARPGATE_LOG_LEVEL: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("WARPGATE_REGISTRY_DEFAULT"); err != nil {
			t.Logf("Failed to unset WARPGATE_REGISTRY_DEFAULT: %v", err)
		}
		if err := os.Unsetenv("WARPGATE_LOG_LEVEL"); err != nil {
			t.Logf("Failed to unset WARPGATE_LOG_LEVEL: %v", err)
		}
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

	// Storage driver defaults to empty (delegates to system defaults)
	if config.Storage.Driver != "" {
		t.Errorf("Expected empty storage driver (system default), got '%s'", config.Storage.Driver)
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

// TestDetectOCIRuntime tests the runtime detection logic
func TestDetectOCIRuntime(t *testing.T) {
	runtime := DetectOCIRuntime()

	// On systems without crun/runc, should return empty string
	// On systems with runtimes, should return a valid path
	if runtime != "" {
		// If a runtime was detected, verify it exists
		info, err := os.Stat(runtime)
		if err != nil {
			t.Errorf("Detected runtime doesn't exist: %s", runtime)
			return
		}

		// Verify it's actually executable (check any execute bit)
		mode := info.Mode()
		if mode&0111 == 0 { // Check if any execute bit (owner, group, or other) is set
			t.Errorf("Detected runtime is not executable: %s (permissions: %v)", runtime, mode)
			return
		}

		// Verify it's a regular file, not a directory
		if !mode.IsRegular() {
			t.Errorf("Detected runtime is not a regular file: %s (mode: %v)", runtime, mode)
			return
		}

		t.Logf("Detected OCI runtime: %s (permissions: %v)", runtime, mode)
	} else {
		t.Log("No OCI runtime detected (expected on macOS or systems without crun/runc)")
	}
}

// TestLoad_RegistryCredentialsFallback tests that REGISTRY_* env vars work without WARPGATE_ prefix
func TestLoad_RegistryCredentialsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a minimal config file
	configContent := `registry:
  default: docker.io
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set credentials using generic REGISTRY_* env vars (without WARPGATE_ prefix)
	if err := os.Setenv("REGISTRY_USERNAME", "genericuser"); err != nil {
		t.Fatalf("Failed to set REGISTRY_USERNAME: %v", err)
	}
	if err := os.Setenv("REGISTRY_TOKEN", "generictoken"); err != nil {
		t.Fatalf("Failed to set REGISTRY_TOKEN: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("REGISTRY_USERNAME"); err != nil {
			t.Logf("Failed to unset REGISTRY_USERNAME: %v", err)
		}
		if err := os.Unsetenv("REGISTRY_TOKEN"); err != nil {
			t.Logf("Failed to unset REGISTRY_TOKEN: %v", err)
		}
	}()

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Should use generic REGISTRY_* env vars
	if config.Registry.Username != "genericuser" {
		t.Errorf("Expected username 'genericuser' from REGISTRY_USERNAME, got '%s'", config.Registry.Username)
	}

	if config.Registry.Token != "generictoken" {
		t.Errorf("Expected token 'generictoken' from REGISTRY_TOKEN, got '%s'", config.Registry.Token)
	}
}

// TestLoad_RegistryCredentialsPrecedence tests that WARPGATE_* env vars take precedence over REGISTRY_*
func TestLoad_RegistryCredentialsPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a minimal config file
	configContent := `registry:
  default: docker.io
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set both WARPGATE_* and REGISTRY_* env vars
	if err := os.Setenv("WARPGATE_REGISTRY_USERNAME", "warpgateuser"); err != nil {
		t.Fatalf("Failed to set WARPGATE_REGISTRY_USERNAME: %v", err)
	}
	if err := os.Setenv("REGISTRY_USERNAME", "genericuser"); err != nil {
		t.Fatalf("Failed to set REGISTRY_USERNAME: %v", err)
	}
	if err := os.Setenv("WARPGATE_REGISTRY_TOKEN", "warpgatetoken"); err != nil {
		t.Fatalf("Failed to set WARPGATE_REGISTRY_TOKEN: %v", err)
	}
	if err := os.Setenv("REGISTRY_TOKEN", "generictoken"); err != nil {
		t.Fatalf("Failed to set REGISTRY_TOKEN: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("WARPGATE_REGISTRY_USERNAME"); err != nil {
			t.Logf("Failed to unset WARPGATE_REGISTRY_USERNAME: %v", err)
		}
		if err := os.Unsetenv("REGISTRY_USERNAME"); err != nil {
			t.Logf("Failed to unset REGISTRY_USERNAME: %v", err)
		}
		if err := os.Unsetenv("WARPGATE_REGISTRY_TOKEN"); err != nil {
			t.Logf("Failed to unset WARPGATE_REGISTRY_TOKEN: %v", err)
		}
		if err := os.Unsetenv("REGISTRY_TOKEN"); err != nil {
			t.Logf("Failed to unset REGISTRY_TOKEN: %v", err)
		}
	}()

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// WARPGATE_* should take precedence
	if config.Registry.Username != "warpgateuser" {
		t.Errorf("Expected username 'warpgateuser' from WARPGATE_REGISTRY_USERNAME, got '%s'", config.Registry.Username)
	}

	if config.Registry.Token != "warpgatetoken" {
		t.Errorf("Expected token 'warpgatetoken' from WARPGATE_REGISTRY_TOKEN, got '%s'", config.Registry.Token)
	}
}

// TestLoad_AWSCredentialsFromEnv tests that AWS credentials are loaded from environment variables
func TestLoad_AWSCredentialsFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a config file with AWS region but NO credentials
	configContent := `aws:
  region: us-west-2
  profile: dev
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Setup test environment with AWS credentials
	cleanup := setupAWSEnvVars(t)
	defer cleanup()

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config values
	verifyAWSConfigValues(t, config)
	verifyAWSCredentialsFromEnv(t, config)
}

// setupAWSEnvVars sets up test AWS environment variables and returns a cleanup function
func setupAWSEnvVars(t *testing.T) func() {
	t.Helper()

	// Save existing env vars
	originalProfile := os.Getenv("AWS_PROFILE")
	originalRegion := os.Getenv("AWS_REGION")

	// Set AWS credentials via environment variables
	setEnvOrFatal(t, "AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	setEnvOrFatal(t, "AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	setEnvOrFatal(t, "AWS_SESSION_TOKEN", "FwoGZXIvYXdzEBYaDEXAMPLE")

	// Unset AWS_PROFILE and AWS_REGION to let config file values take precedence
	unsetEnvSafe(t, "AWS_PROFILE")
	unsetEnvSafe(t, "AWS_REGION")

	return func() {
		unsetEnvSafe(t, "AWS_ACCESS_KEY_ID")
		unsetEnvSafe(t, "AWS_SECRET_ACCESS_KEY")
		unsetEnvSafe(t, "AWS_SESSION_TOKEN")

		// Restore original env vars
		restoreEnvVar(t, "AWS_PROFILE", originalProfile)
		restoreEnvVar(t, "AWS_REGION", originalRegion)
	}
}

// setEnvOrFatal sets an environment variable or fails the test
func setEnvOrFatal(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed to set %s: %v", key, err)
	}
}

// unsetEnvSafe unsets an environment variable, logging any errors
func unsetEnvSafe(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Logf("Failed to unset %s: %v", key, err)
	}
}

// restoreEnvVar restores an environment variable if it was previously set
func restoreEnvVar(t *testing.T, key, value string) {
	t.Helper()
	if value != "" {
		if err := os.Setenv(key, value); err != nil {
			t.Logf("Failed to restore %s: %v", key, err)
		}
	}
}

// verifyAWSConfigValues verifies AWS configuration values from config file
func verifyAWSConfigValues(t *testing.T, config *Config) {
	t.Helper()
	if config.AWS.Region != "us-west-2" {
		t.Errorf("Expected region 'us-west-2', got '%s'", config.AWS.Region)
	}
	if config.AWS.Profile != "dev" {
		t.Errorf("Expected profile 'dev', got '%s'", config.AWS.Profile)
	}
}

// verifyAWSCredentialsFromEnv verifies AWS credentials from environment variables
func verifyAWSCredentialsFromEnv(t *testing.T, config *Config) {
	t.Helper()
	if config.AWS.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("Expected AWS access key from env var, got '%s'", config.AWS.AccessKeyID)
	}
	if config.AWS.SecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("Expected AWS secret key from env var, got '%s'", config.AWS.SecretAccessKey)
	}
	if config.AWS.SessionToken != "FwoGZXIvYXdzEBYaDEXAMPLE" {
		t.Errorf("Expected AWS session token from env var, got '%s'", config.AWS.SessionToken)
	}
}

// TestLoad_CredentialsNeverFromConfigFile tests that credentials are NEVER read from config files
// This is a critical security test to ensure credentials can't be accidentally stored in config
func TestLoad_CredentialsNeverFromConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Try to put credentials in the config file (should be ignored!)
	configContent := `registry:
  default: docker.io
  username: malicious_user_from_file
  token: malicious_token_from_file

aws:
  region: us-west-2
  access_key_id: MALICIOUS_KEY_FROM_FILE
  secret_access_key: MALICIOUS_SECRET_FROM_FILE
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Don't set any environment variables - credentials should be empty
	clearAllCredentialEnvVars(t)

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// CRITICAL: Credentials should be empty, NOT from the config file
	verifyCredentialsNotFromFile(t, config)

	// Non-credential fields should still be loaded normally
	verifyNonCredentialFieldsLoaded(t, config)
}

// clearAllCredentialEnvVars unsets all credential-related environment variables
func clearAllCredentialEnvVars(t *testing.T) {
	t.Helper()
	credentialEnvVars := []string{
		"WARPGATE_REGISTRY_USERNAME",
		"WARPGATE_REGISTRY_TOKEN",
		"REGISTRY_USERNAME",
		"REGISTRY_TOKEN",
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
	}

	for _, envVar := range credentialEnvVars {
		unsetEnvSafe(t, envVar)
	}
}

// verifyCredentialsNotFromFile ensures credentials are empty and not loaded from config file
func verifyCredentialsNotFromFile(t *testing.T, config *Config) {
	t.Helper()
	if config.Registry.Username != "" {
		t.Errorf("SECURITY ISSUE: Registry username loaded from config file! Got '%s'", config.Registry.Username)
	}
	if config.Registry.Token != "" {
		t.Errorf("SECURITY ISSUE: Registry token loaded from config file! Got '%s'", config.Registry.Token)
	}
	if config.AWS.AccessKeyID != "" {
		t.Errorf("SECURITY ISSUE: AWS access key loaded from config file! Got '%s'", config.AWS.AccessKeyID)
	}
	if config.AWS.SecretAccessKey != "" {
		t.Errorf("SECURITY ISSUE: AWS secret key loaded from config file! Got '%s'", config.AWS.SecretAccessKey)
	}
}

// verifyNonCredentialFieldsLoaded ensures non-credential fields are loaded correctly
func verifyNonCredentialFieldsLoaded(t *testing.T, config *Config) {
	t.Helper()
	if config.Registry.Default != "docker.io" {
		t.Errorf("Expected registry 'docker.io', got '%s'", config.Registry.Default)
	}
	if config.AWS.Region != "us-west-2" {
		t.Errorf("Expected region 'us-west-2', got '%s'", config.AWS.Region)
	}
}
