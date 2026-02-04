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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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
	// ErrConfigNotFound is expected when no config file exists - this is not an error
	if err != nil && !IsNotFoundError(err) {
		t.Fatalf("Failed to load defaults: %v", err)
	}

	// Check defaults
	if config.Log.Level != "info" {
		t.Errorf("Expected log level 'info', got '%s'", config.Log.Level)
	}

	if config.Log.Format != "color" {
		t.Errorf("Expected log format 'color', got '%s'", config.Log.Format)
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

templates:
  cache_dir: /custom/cache
  repositories:
    custom: github.com/custom/templates
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from path: %v", err)
	}

	// Verify all config sections
	verifyRegistryConfig(t, config)
	verifyLogConfig(t, config)
	verifyBuildConfig(t, config)
	verifyTemplatesConfig(t, config)
}

// verifyRegistryConfig verifies registry configuration values
func verifyRegistryConfig(t *testing.T, config *Config) {
	t.Helper()
	if config.Registry.Default != "docker.io/myorg" {
		t.Errorf("Expected registry 'docker.io/myorg', got '%s'", config.Registry.Default)
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
	// ErrConfigNotFound is expected when no config file exists - this is not an error
	if err != nil && !IsNotFoundError(err) {
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
	originalDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")

	// Set AWS credentials via environment variables
	setEnvOrFatal(t, "AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	setEnvOrFatal(t, "AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	setEnvOrFatal(t, "AWS_SESSION_TOKEN", "FwoGZXIvYXdzEBYaDEXAMPLE")

	// Unset AWS_PROFILE, AWS_REGION, and AWS_DEFAULT_REGION to let config file values take precedence
	unsetEnvSafe(t, "AWS_PROFILE")
	unsetEnvSafe(t, "AWS_REGION")
	unsetEnvSafe(t, "AWS_DEFAULT_REGION")

	return func() {
		unsetEnvSafe(t, "AWS_ACCESS_KEY_ID")
		unsetEnvSafe(t, "AWS_SECRET_ACCESS_KEY")
		unsetEnvSafe(t, "AWS_SESSION_TOKEN")

		// Restore original env vars
		restoreEnvVar(t, "AWS_PROFILE", originalProfile)
		restoreEnvVar(t, "AWS_REGION", originalRegion)
		restoreEnvVar(t, "AWS_DEFAULT_REGION", originalDefaultRegion)
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
// and region variables that could override config file values
func clearAllCredentialEnvVars(t *testing.T) {
	t.Helper()
	awsEnvVars := []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_PROFILE",
	}

	for _, envVar := range awsEnvVars {
		unsetEnvSafe(t, envVar)
	}
}

// verifyCredentialsNotFromFile ensures credentials are empty and not loaded from config file
func verifyCredentialsNotFromFile(t *testing.T, config *Config) {
	t.Helper()
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

// TestIsNotFoundError_Various tests IsNotFoundError with different error types
func TestIsNotFoundError_Various(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "ErrConfigNotFound",
			err:  ErrConfigNotFound,
			want: true,
		},
		{
			name: "wrapped ErrConfigNotFound",
			err:  fmt.Errorf("something: %w", ErrConfigNotFound),
			want: true,
		},
		{
			name: "viper ConfigFileNotFoundError",
			err:  viper.ConfigFileNotFoundError{},
			want: true,
		},
		{
			name: "generic error",
			err:  fmt.Errorf("some other error"),
			want: false,
		},
		{
			name: "os.ErrNotExist",
			err:  os.ErrNotExist,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsNotFoundError(tt.err)
			if got != tt.want {
				t.Errorf("IsNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPopulateAWSCredentials_EnvVarsSet tests that AWS credentials are populated from env vars
func TestPopulateAWSCredentials_EnvVarsSet(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIATEST123")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secrettest456")
	t.Setenv("AWS_SESSION_TOKEN", "sessiontest789")

	cfg := &Config{}
	populateAWSCredentials(cfg)

	if cfg.AWS.AccessKeyID != "AKIATEST123" {
		t.Errorf("Expected AccessKeyID 'AKIATEST123', got '%s'", cfg.AWS.AccessKeyID)
	}
	if cfg.AWS.SecretAccessKey != "secrettest456" {
		t.Errorf("Expected SecretAccessKey 'secrettest456', got '%s'", cfg.AWS.SecretAccessKey)
	}
	if cfg.AWS.SessionToken != "sessiontest789" {
		t.Errorf("Expected SessionToken 'sessiontest789', got '%s'", cfg.AWS.SessionToken)
	}
}

// TestPopulateAWSCredentials_EnvVarsEmpty tests that empty env vars result in empty fields
func TestPopulateAWSCredentials_EnvVarsEmpty(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")

	cfg := &Config{}
	populateAWSCredentials(cfg)

	if cfg.AWS.AccessKeyID != "" {
		t.Errorf("Expected empty AccessKeyID, got '%s'", cfg.AWS.AccessKeyID)
	}
	if cfg.AWS.SecretAccessKey != "" {
		t.Errorf("Expected empty SecretAccessKey, got '%s'", cfg.AWS.SecretAccessKey)
	}
	if cfg.AWS.SessionToken != "" {
		t.Errorf("Expected empty SessionToken, got '%s'", cfg.AWS.SessionToken)
	}
}

// TestBindEnvVars_RegistryOverride tests that env vars override config via bindEnvVars
func TestBindEnvVars_RegistryOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `registry:
  default: ghcr.io
log:
  level: info
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	t.Setenv("WARPGATE_REGISTRY_DEFAULT", "quay.io")
	t.Setenv("WARPGATE_LOG_LEVEL", "warn")
	t.Setenv("WARPGATE_BUILD_TIMEOUT", "6h")

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Registry.Default != "quay.io" {
		t.Errorf("Expected registry 'quay.io' from env var, got '%s'", config.Registry.Default)
	}
	if config.Log.Level != "warn" {
		t.Errorf("Expected log level 'warn' from env var, got '%s'", config.Log.Level)
	}
	if config.Build.Timeout != "6h" {
		t.Errorf("Expected build timeout '6h' from env var, got '%s'", config.Build.Timeout)
	}
}

// TestLoad_FindsConfigInCurrentDir tests Load finds config.yaml in current directory
func TestLoad_FindsConfigInCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `log:
  level: warn
  format: json
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Log.Level != "warn" {
		t.Errorf("Expected log level 'warn', got '%s'", config.Log.Level)
	}
	if config.Log.Format != "json" {
		t.Errorf("Expected log format 'json', got '%s'", config.Log.Format)
	}
}

// TestLoad_NoConfigFile tests Load with no config file returns defaults + ErrConfigNotFound
func TestLoad_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := Load()
	if !IsNotFoundError(err) {
		t.Fatalf("Expected ErrConfigNotFound, got %v", err)
	}
	// Should still return valid config with defaults
	if config == nil {
		t.Fatal("Expected non-nil config even when no file found")
	}
	if config.Log.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", config.Log.Level)
	}
}

// TestLoadFromPath_ConvertConfig tests loading convert-specific configuration
func TestLoadFromPath_ConvertConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `convert:
  default_version: "2.0.0"
  default_license: Apache-2.0
  warpgate_version: ">=2.0.0"
  ami_instance_type: c5.xlarge
  ami_volume_size: 100
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Convert.DefaultVersion != "2.0.0" {
		t.Errorf("Expected convert default version '2.0.0', got '%s'", config.Convert.DefaultVersion)
	}
	if config.Convert.DefaultLicense != "Apache-2.0" {
		t.Errorf("Expected convert default license 'Apache-2.0', got '%s'", config.Convert.DefaultLicense)
	}
	if config.Convert.WarpgateVersion != ">=2.0.0" {
		t.Errorf("Expected warpgate version '>=2.0.0', got '%s'", config.Convert.WarpgateVersion)
	}
	if config.Convert.AMIInstanceType != "c5.xlarge" {
		t.Errorf("Expected AMI instance type 'c5.xlarge', got '%s'", config.Convert.AMIInstanceType)
	}
	if config.Convert.AMIVolumeSize != 100 {
		t.Errorf("Expected AMI volume size 100, got %d", config.Convert.AMIVolumeSize)
	}
}

// TestLoadFromPath_ManifestsConfig tests loading manifests configuration
func TestLoadFromPath_ManifestsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `manifests:
  verify_concurrency: 10
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Manifests.VerifyConcurrency != 10 {
		t.Errorf("Expected verify concurrency 10, got %d", config.Manifests.VerifyConcurrency)
	}
}

// TestLoadFromPath_TemplatesLocalPaths tests loading templates local paths
func TestLoadFromPath_TemplatesLocalPaths(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `templates:
  local_paths:
    - /opt/templates
    - ~/my-templates
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(config.Templates.LocalPaths) != 2 {
		t.Errorf("Expected 2 local paths, got %d", len(config.Templates.LocalPaths))
	}
}

// TestLoadFromPath_InvalidYAML tests LoadFromPath with malformed YAML
func TestLoadFromPath_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML
	configContent := `registry:
  default: [invalid yaml syntax
    broken: nesting
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

// TestLoadFromPath_NonexistentFile tests LoadFromPath with a file that does not exist
func TestLoadFromPath_NonexistentFile(t *testing.T) {
	_, err := LoadFromPath("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

// TestLoad_WithConfigInXDGDir tests Load finds config in XDG config directory
func TestLoad_WithConfigInXDGDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `log:
  level: error
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Change to a different dir so Load doesn't find config in current dir
	emptyDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Log.Level != "error" {
		t.Errorf("Expected log level 'error', got '%s'", config.Log.Level)
	}
}

// loadDefaultConfig loads a config with all defaults for testing.
func loadDefaultConfig(t *testing.T) *Config {
	t.Helper()
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := Load()
	if err != nil && !IsNotFoundError(err) {
		t.Fatalf("Failed to load defaults: %v", err)
	}
	return config
}

// TestLoad_UnmarshalWithAllDefaults tests that defaults cover all config sections
func TestLoad_UnmarshalWithAllDefaults(t *testing.T) {
	config := loadDefaultConfig(t)

	t.Run("AWS AMI defaults", func(t *testing.T) {
		assert.Equal(t, "t3.medium", config.AWS.AMI.InstanceType)
		assert.Equal(t, 8, config.AWS.AMI.VolumeSize)
		assert.Equal(t, "/dev/sda1", config.AWS.AMI.DeviceName)
		assert.Equal(t, "gp3", config.AWS.AMI.VolumeType)
		assert.Equal(t, 30, config.AWS.AMI.PollingIntervalSec)
		assert.Equal(t, 30, config.AWS.AMI.BuildTimeoutMin)
	})

	t.Run("container defaults", func(t *testing.T) {
		assert.Equal(t, "linux/amd64", config.Container.DefaultPlatform)
		assert.Equal(t, "ubuntu", config.Container.DefaultBaseImage)
		assert.Equal(t, "latest", config.Container.DefaultBaseVersion)
		assert.Len(t, config.Container.DefaultPlatforms, 2)
	})

	t.Run("convert defaults", func(t *testing.T) {
		assert.Equal(t, "1.0.0", config.Convert.DefaultVersion)
		assert.Equal(t, "MIT", config.Convert.DefaultLicense)
	})

	t.Run("manifests defaults", func(t *testing.T) {
		assert.Equal(t, 5, config.Manifests.VerifyConcurrency)
	})

	t.Run("build defaults", func(t *testing.T) {
		assert.Equal(t, 2, config.Build.Concurrency)
		assert.Equal(t, 3, config.Build.QEMUSlowdownFactor)
		assert.Equal(t, 2, config.Build.ParallelismLimit)
		assert.InDelta(t, 0.5, config.Build.CPUFraction, 0.001)
		assert.Equal(t, 10, config.Build.BaselineBuildTimeMin)
	})
}

// TestBindEnvVars_ContainerRuntime tests that container runtime env var override works
func TestBindEnvVars_ContainerRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `container:
  runtime: /usr/bin/runc
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	t.Setenv("WARPGATE_CONTAINER_RUNTIME", "/custom/bin/crun")

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Container.Runtime != "/custom/bin/crun" {
		t.Errorf("Expected container runtime '/custom/bin/crun' from env var, got '%s'", config.Container.Runtime)
	}
}

// TestBindEnvVars_AWSRegionOverride tests that AWS_REGION env var overrides config
func TestBindEnvVars_AWSRegionOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `aws:
  region: us-west-2
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	t.Setenv("AWS_REGION", "eu-west-1")

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.AWS.Region != "eu-west-1" {
		t.Errorf("Expected AWS region 'eu-west-1' from env var, got '%s'", config.AWS.Region)
	}
}

// TestPopulateAWSCredentials_Unset tests that unset env vars result in empty fields
func TestPopulateAWSCredentials_Unset(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")

	// Also unset them to test the "not set at all" case
	// t.Setenv already handles cleanup, but let's be explicit
	cfg := &Config{
		AWS: AWSConfig{
			AccessKeyID:     "should-be-overwritten",
			SecretAccessKey: "should-be-overwritten",
			SessionToken:    "should-be-overwritten",
		},
	}
	populateAWSCredentials(cfg)

	if cfg.AWS.AccessKeyID != "" {
		t.Errorf("Expected empty AccessKeyID, got '%s'", cfg.AWS.AccessKeyID)
	}
	if cfg.AWS.SecretAccessKey != "" {
		t.Errorf("Expected empty SecretAccessKey, got '%s'", cfg.AWS.SecretAccessKey)
	}
	if cfg.AWS.SessionToken != "" {
		t.Errorf("Expected empty SessionToken, got '%s'", cfg.AWS.SessionToken)
	}
}

// TestLoadFromPath_AWSAMIConfig tests loading AMI-specific configuration from file
func TestLoadFromPath_AWSAMIConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `aws:
  region: us-east-1
  ami:
    instance_type: m5.large
    volume_size: 50
    device_name: /dev/xvda
    volume_type: gp2
    polling_interval_sec: 60
    build_timeout_min: 120
    instance_profile_name: my-profile
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.AWS.AMI.InstanceType != "m5.large" {
		t.Errorf("Expected instance type 'm5.large', got '%s'", config.AWS.AMI.InstanceType)
	}
	if config.AWS.AMI.VolumeSize != 50 {
		t.Errorf("Expected volume size 50, got %d", config.AWS.AMI.VolumeSize)
	}
	if config.AWS.AMI.InstanceProfileName != "my-profile" {
		t.Errorf("Expected instance profile 'my-profile', got '%s'", config.AWS.AMI.InstanceProfileName)
	}
}

// TestLoadFromPath_BuildKitConfig tests loading BuildKit configuration from file
func TestLoadFromPath_BuildKitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `buildkit:
  endpoint: tcp://buildkitd:1234
  tls_enabled: true
  tls_ca_cert: /path/to/ca.pem
  tls_cert: /path/to/cert.pem
  tls_key: /path/to/key.pem
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.BuildKit.Endpoint != "tcp://buildkitd:1234" {
		t.Errorf("Expected endpoint 'tcp://buildkitd:1234', got '%s'", config.BuildKit.Endpoint)
	}
	if !config.BuildKit.TLSEnabled {
		t.Error("Expected TLS to be enabled")
	}
	if config.BuildKit.TLSCACert != "/path/to/ca.pem" {
		t.Errorf("Expected TLS CA cert path, got '%s'", config.BuildKit.TLSCACert)
	}
}

// TestDetectOCIRuntime_ReturnsValidPathOrEmpty tests DetectOCIRuntime behavior
func TestDetectOCIRuntime_ReturnsValidPathOrEmpty(t *testing.T) {
	t.Parallel()

	runtime := DetectOCIRuntime()

	if runtime == "" {
		// On systems without crun/runc (e.g., macOS), empty is valid
		t.Log("No OCI runtime detected (expected on macOS)")
		return
	}

	// If a runtime is found, it should be an absolute path
	if !filepath.IsAbs(runtime) {
		t.Errorf("Expected absolute path, got %q", runtime)
	}

	// Verify the file exists
	info, err := os.Stat(runtime)
	if err != nil {
		t.Errorf("Detected runtime path does not exist: %s", runtime)
		return
	}

	// Should be a regular file
	if !info.Mode().IsRegular() {
		t.Errorf("Detected runtime is not a regular file: %s", runtime)
	}

	// Should be executable
	if info.Mode()&0111 == 0 {
		t.Errorf("Detected runtime is not executable: %s", runtime)
	}
}

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
	if err := os.Setenv("WARPGATE_REGISTRY_DEFAULT", "docker.io/fromenv"); err != nil {
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

	// Environment variable should win over config file
	if config.Registry.Default != "docker.io/fromenv" {
		t.Errorf("Expected registry 'docker.io/fromenv', got '%s'", config.Registry.Default)
	}

	if config.Log.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Log.Level)
	}

	// Values not set anywhere should have defaults
	// Template cache dir should have default value
	if config.Templates.CacheDir == "" {
		t.Error("Expected templates cache dir to have a default value")
	}
}

// TestIntegration_RealWorldScenario tests a realistic user scenario
func TestIntegration_RealWorldScenario(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Simulate a user's config (credentials NOT in config file)
	configContent := `registry:
  default: ghcr.io/myorg

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

	config, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify all values loaded correctly
	if config.Registry.Default != "ghcr.io/myorg" {
		t.Errorf("Expected registry 'ghcr.io/myorg', got '%s'", config.Registry.Default)
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
