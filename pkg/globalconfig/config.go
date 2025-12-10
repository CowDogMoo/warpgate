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

// Package globalconfig manages the global Warpgate configuration from files and environment variables.
package globalconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/viper"
)

// Standard file and directory permissions
const (
	// FilePermReadWrite is the standard permission for readable/writable files (rw-r--r--)
	FilePermReadWrite = 0o644
	// DirPermReadWriteExec is the standard permission for directories (rwxr-xr-x)
	DirPermReadWriteExec = 0o755
)

// Config represents the global Warpgate configuration
// This is for user preferences and environment-specific settings,
// NOT for template definitions (which use plain YAML)
type Config struct {
	Registry  RegistryConfig  `mapstructure:"registry"`
	Templates TemplatesConfig `mapstructure:"templates"`
	Build     BuildConfig     `mapstructure:"build"`
	Manifests ManifestsConfig `mapstructure:"manifests"`
	Log       LogConfig       `mapstructure:"log"`
	AWS       AWSConfig       `mapstructure:"aws"`
	Container ContainerConfig `mapstructure:"container"`
	Convert   ConvertConfig   `mapstructure:"convert"`
	BuildKit  BuildKitConfig  `mapstructure:"buildkit"`
}

// RegistryConfig holds registry-related configuration
type RegistryConfig struct {
	// Default registry URL (safe to store in config)
	Default string `mapstructure:"default" yaml:"default"`

	// Credentials are ONLY read from environment variables (not from config file)
	// Hidden from YAML output to prevent accidental storage in config files
	Username string `mapstructure:"-" yaml:"-"` // From WARPGATE_REGISTRY_USERNAME or REGISTRY_USERNAME
	Token    string `mapstructure:"-" yaml:"-"` // From WARPGATE_REGISTRY_TOKEN or REGISTRY_TOKEN
}

// TemplatesConfig holds template-related configuration
type TemplatesConfig struct {
	CacheDir string `mapstructure:"cache_dir" yaml:"cache_dir"`
	// Repositories maps repository names to their git URLs or local paths
	// Example:
	//   repositories:
	//     official: https://github.com/cowdogmoo/warpgate-templates.git
	//     local: /path/to/local/templates
	//     private: https://github.com/myorg/private-templates.git
	Repositories map[string]string `mapstructure:"repositories" yaml:"repositories"`
	// LocalPaths lists additional local directories to search for templates
	// These are scanned in order when listing/discovering templates
	LocalPaths []string `mapstructure:"local_paths" yaml:"local_paths"`
}

// BuildConfig holds build-related configuration
type BuildConfig struct {
	DefaultArch          []string `mapstructure:"default_arch"`
	ParallelBuilds       bool     `mapstructure:"parallel_builds"`
	Timeout              string   `mapstructure:"timeout"`
	Concurrency          int      `mapstructure:"concurrency"`
	QEMUSlowdownFactor   int      `mapstructure:"qemu_slowdown_factor"`
	ParallelismLimit     int      `mapstructure:"parallelism_limit"`
	CPUFraction          float64  `mapstructure:"cpu_fraction"`
	BaselineBuildTimeMin int      `mapstructure:"baseline_build_time_min"`
}

// ManifestsConfig holds manifest-related configuration
type ManifestsConfig struct {
	// VerifyConcurrency is the number of concurrent digest verifications
	// Default: 5, Max: 20
	VerifyConcurrency int `mapstructure:"verify_concurrency" yaml:"verify_concurrency"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// AWSConfig holds AWS-related configuration
type AWSConfig struct {
	// Region is the AWS region to use (can also be set via AWS_REGION)
	Region string `mapstructure:"region" yaml:"region,omitempty"`

	// Profile is the AWS profile to use from ~/.aws/config (recommended for SSO)
	// Can also be set via AWS_PROFILE environment variable
	Profile string `mapstructure:"profile" yaml:"profile,omitempty"`

	// Credentials are ONLY read from environment variables (not from config file)
	// Hidden from YAML output to prevent accidental storage in config files
	AccessKeyID     string `mapstructure:"-" yaml:"-"` // From AWS_ACCESS_KEY_ID env var only
	SecretAccessKey string `mapstructure:"-" yaml:"-"` // From AWS_SECRET_ACCESS_KEY env var only
	SessionToken    string `mapstructure:"-" yaml:"-"` // From AWS_SESSION_TOKEN env var only

	AMI AMIConfig `mapstructure:"ami" yaml:"ami,omitempty"`
}

// AMIConfig holds AMI-specific configuration
type AMIConfig struct {
	InstanceType       string `mapstructure:"instance_type"`
	VolumeSize         int    `mapstructure:"volume_size"`
	DeviceName         string `mapstructure:"device_name"`
	VolumeType         string `mapstructure:"volume_type"`
	PollingIntervalSec int    `mapstructure:"polling_interval_sec"`
	BuildTimeoutMin    int    `mapstructure:"build_timeout_min"`
	DefaultParentImage string `mapstructure:"default_parent_image"`
}

// ContainerConfig holds container build configuration
type ContainerConfig struct {
	DefaultPlatform  string   `mapstructure:"default_platform"`
	DefaultPlatforms []string `mapstructure:"default_platforms"`
	// DefaultRegistry sets the default registry for image commits
	// Leave empty (recommended) to use the --registry flag value
	// Set explicitly only if you want all builds to default to a specific registry
	DefaultRegistry    string `mapstructure:"default_registry"`
	DefaultBaseImage   string `mapstructure:"default_base_image"`
	DefaultBaseVersion string `mapstructure:"default_base_version"`
	// Runtime is the OCI runtime path (auto-detected: crun preferred, runc fallback)
	// Can be overridden via config or WARPGATE_CONTAINER_RUNTIME env var
	Runtime string `mapstructure:"runtime"`
}

// ConvertConfig holds Packer conversion defaults
// These settings are ONLY used when converting Packer templates to warpgate format.
// For actual builds, use the settings in AWS.AMI and Container configs.
type ConvertConfig struct {
	DefaultVersion  string `mapstructure:"default_version"`  // Default version for converted templates
	DefaultLicense  string `mapstructure:"default_license"`  // Default license for converted templates
	WarpgateVersion string `mapstructure:"warpgate_version"` // Required warpgate version

	// AMI conversion overrides - these override AWS.AMI settings ONLY during Packer->Warpgate conversion
	// If empty/zero, falls back to AWS.AMI settings
	AMIInstanceType string `mapstructure:"ami_instance_type"` // Override for AWS.AMI.InstanceType during conversion
	AMIVolumeSize   int    `mapstructure:"ami_volume_size"`   // Override for AWS.AMI.VolumeSize during conversion
}

// BuildKitConfig holds BuildKit daemon connection configuration
// This allows Warpgate to connect to remote BuildKit daemons (e.g., in Kubernetes)
type BuildKitConfig struct {
	// Endpoint is the BuildKit daemon address
	// Examples:
	//   - "" (empty): Auto-detect local docker buildx builder (default)
	//   - "docker-container://buildx_buildkit_default0": Connect to specific buildx container
	//   - "tcp://buildkitd:1234": Connect to remote BuildKit daemon (K8s service)
	//   - "unix:///run/buildkit/buildkitd.sock": Connect via Unix socket
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint,omitempty"`

	// TLSEnabled enables TLS for remote connections (tcp:// endpoints)
	TLSEnabled bool `mapstructure:"tls_enabled" yaml:"tls_enabled,omitempty"`

	// TLSCACert is the path to the CA certificate for TLS verification
	TLSCACert string `mapstructure:"tls_ca_cert" yaml:"tls_ca_cert,omitempty"`

	// TLSCert is the path to the client certificate for TLS authentication
	TLSCert string `mapstructure:"tls_cert" yaml:"tls_cert,omitempty"`

	// TLSKey is the path to the client key for TLS authentication
	TLSKey string `mapstructure:"tls_key" yaml:"tls_key,omitempty"`
}

// loadConfigWithViper is a helper function that encapsulates the common Viper configuration loading pattern.
// It takes a setup function that allows callers to inject config-source-specific logic (e.g., file paths vs. config name).
// This consolidates the duplicated logic between Load() and LoadFromPath().
func loadConfigWithViper(setupFunc func(*viper.Viper) error) (*Config, error) {
	v := viper.New()

	// Set config type
	v.SetConfigType("yaml")

	// Set sensible defaults
	setDefaults(v)

	// Environment variable support
	// WARPGATE_REGISTRY_DEFAULT, WARPGATE_LOG_LEVEL, etc.
	v.SetEnvPrefix("WARPGATE")
	v.AutomaticEnv()
	bindEnvVars(v)

	// Call the setup function to configure the Viper instance
	if err := setupFunc(v); err != nil {
		return nil, err
	}

	// Unmarshal into Config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Populate credentials directly from environment variables
	// This ensures they can NEVER come from config files
	populateAWSCredentials(&config)
	populateRegistryCredentials(&config)

	return &config, nil
}

// Load reads and parses the global configuration file
// Returns a Config with defaults if no config file exists
func Load() (*Config, error) {
	return loadConfigWithViper(func(v *viper.Viper) error {
		// Set config name (without extension)
		v.SetConfigName("config")

		// Look in these locations (in order of precedence):
		// 1. XDG config directories (modern, preferred: ~/.config on all Unix-like systems)
		// 2. Legacy ~/.warpgate directory (backward compatibility)
		// 3. Current directory

		// Add config paths using our CLI-specific XDG implementation
		for _, dir := range getConfigDirs() {
			v.AddConfigPath(dir)
		}

		// Current directory (lowest priority)
		v.AddConfigPath(".")

		// Read config file (optional - doesn't error if missing)
		_ = v.ReadInConfig()

		return nil
	})
}

// LoadFromPath loads configuration from a specific file path
func LoadFromPath(path string) (*Config, error) {
	return loadConfigWithViper(func(v *viper.Viper) error {
		// Set specific config file path
		v.SetConfigFile(path)

		// Read the config file (required when loading from a specific path)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		return nil
	})
}

// getEnvWithFallback returns the value of the primary environment variable if set,
// otherwise returns the value of the fallback environment variable.
// Returns an empty string if neither variable is set.
func getEnvWithFallback(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(fallback)
}

// populateAWSCredentials reads AWS credentials directly from environment variables
// This ensures they can never come from config files, preventing accidental credential leaks
func populateAWSCredentials(cfg *Config) {
	cfg.AWS.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	cfg.AWS.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	cfg.AWS.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
}

// populateRegistryCredentials reads registry credentials directly from environment variables
// This ensures they can never come from config files, preventing accidental credential leaks
// Supports both WARPGATE_* and standard REGISTRY_* environment variables
func populateRegistryCredentials(cfg *Config) {
	// Try WARPGATE_* prefix first, fall back to REGISTRY_*
	cfg.Registry.Username = getEnvWithFallback("WARPGATE_REGISTRY_USERNAME", "REGISTRY_USERNAME")
	cfg.Registry.Token = getEnvWithFallback("WARPGATE_REGISTRY_TOKEN", "REGISTRY_TOKEN")
}

// DetectOCIRuntime finds an available OCI runtime on the system
// Returns the first available runtime in order of preference: crun, runc
// First searches PATH using exec.LookPath (most portable), then falls back to common paths
// Paths are based on containers/common default runtime search locations
func DetectOCIRuntime() string {
	// First try to find in PATH (most portable and idiomatic)
	preferredRuntimes := []string{"crun", "runc"}
	for _, runtime := range preferredRuntimes {
		if path, err := exec.LookPath(runtime); err == nil {
			return path
		}
	}

	// Fallback to common installation paths if not in PATH
	// These paths match the search order used by containers/common
	fallbackPaths := []string{
		// crun locations (preferred: faster, written in C)
		"/usr/bin/crun",
		"/usr/sbin/crun",
		"/usr/local/bin/crun",
		"/usr/local/sbin/crun",
		"/sbin/crun",
		"/bin/crun",
		"/run/current-system/sw/bin/crun", // NixOS
		// runc locations (fallback: standard OCI runtime)
		"/usr/bin/runc",
		"/usr/sbin/runc",
		"/usr/local/bin/runc",
		"/usr/local/sbin/runc",
		"/sbin/runc",
		"/bin/runc",
		"/usr/lib/cri-o-runc/sbin/runc",   // CRI-O specific
		"/run/current-system/sw/bin/runc", // NixOS
	}

	for _, path := range fallbackPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If nothing found, return empty string
	return ""
}

// setDefaults sets default values for all configuration options
func setDefaults(v *viper.Viper) {
	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "color")

	// Templates defaults - use CLI-specific cache directory (~/.cache on Unix-like systems)
	v.SetDefault("templates.cache_dir", filepath.Join(getCacheHome(), "warpgate", "templates"))

	// Build defaults
	v.SetDefault("build.parallel_builds", true)
	v.SetDefault("build.default_arch", []string{"amd64"})
	v.SetDefault("build.timeout", "2h")
	v.SetDefault("build.concurrency", 2)
	v.SetDefault("build.qemu_slowdown_factor", 3)
	v.SetDefault("build.parallelism_limit", 2)
	v.SetDefault("build.cpu_fraction", 0.5)
	v.SetDefault("build.baseline_build_time_min", 10)

	// Manifests defaults
	v.SetDefault("manifests.verify_concurrency", 5)

	// Registry defaults
	v.SetDefault("registry.default", "ghcr.io")

	// Templates defaults
	// Don't set default repositories - let the registry handle defaults
	// This prevents Viper from merging defaults with user config
	v.SetDefault("templates.repositories", map[string]string{})
	v.SetDefault("templates.local_paths", []string{})

	// AWS defaults (will use AWS SDK defaults if not set)
	v.SetDefault("aws.region", "")
	v.SetDefault("aws.profile", "")

	// AWS AMI defaults
	v.SetDefault("aws.ami.instance_type", "t3.medium")
	v.SetDefault("aws.ami.volume_size", 8)
	v.SetDefault("aws.ami.device_name", "/dev/sda1")
	v.SetDefault("aws.ami.volume_type", "gp3")
	v.SetDefault("aws.ami.polling_interval_sec", 30)
	v.SetDefault("aws.ami.build_timeout_min", 30)
	v.SetDefault("aws.ami.default_parent_image", "") // Empty = user must specify

	// Container defaults
	v.SetDefault("container.default_platform", "linux/amd64")
	v.SetDefault("container.default_platforms", []string{"linux/amd64", "linux/arm64"})
	// Empty default_registry allows --registry flag to take precedence during builds
	// Set explicitly in config if you want a default registry for all builds
	v.SetDefault("container.default_registry", "")
	v.SetDefault("container.default_base_image", "ubuntu")
	v.SetDefault("container.default_base_version", "latest")
	// Auto-detect available OCI runtime (crun preferred, runc fallback)
	v.SetDefault("container.runtime", DetectOCIRuntime())

	// Convert/Packer defaults
	v.SetDefault("convert.default_version", "1.0.0")
	v.SetDefault("convert.default_license", "MIT")
	v.SetDefault("convert.warpgate_version", ">=1.0.0")
	v.SetDefault("convert.ami_instance_type", "t3.micro")
	v.SetDefault("convert.ami_volume_size", 50)

	// BuildKit defaults
	v.SetDefault("buildkit.endpoint", "")       // Empty = auto-detect local buildx
	v.SetDefault("buildkit.tls_enabled", false) // TLS disabled by default
	v.SetDefault("buildkit.tls_ca_cert", "")    // No CA cert by default
	v.SetDefault("buildkit.tls_cert", "")       // No client cert by default
	v.SetDefault("buildkit.tls_key", "")        // No client key by default
}

// bindEnvVars explicitly binds environment variables to config keys
func bindEnvVars(v *viper.Viper) {
	// Helper to bind with error logging
	bind := func(input ...string) {
		if len(input) == 0 {
			return
		}
		if err := v.BindEnv(input...); err != nil {
			// Log as debug since BindEnv errors are rare and non-critical
			// They typically only occur with invalid keys
			fmt.Fprintf(os.Stderr, "debug: failed to bind env var for %s: %v\n", input[0], err)
		}
	}

	// Registry
	bind("registry.default", "WARPGATE_REGISTRY_DEFAULT")
	// Note: Registry credentials are NOT bound here - they're read directly from env vars
	// after unmarshaling to prevent them from being stored in config files

	// Templates
	bind("templates.cache_dir", "WARPGATE_TEMPLATES_CACHE_DIR")
	bind("templates.repositories", "WARPGATE_TEMPLATES_REPOSITORIES")
	bind("templates.local_paths", "WARPGATE_TEMPLATES_LOCAL_PATHS")

	// Build
	bind("build.default_arch", "WARPGATE_BUILD_DEFAULT_ARCH")

	// Manifests
	bind("manifests.verify_concurrency", "WARPGATE_MANIFESTS_VERIFY_CONCURRENCY")
	bind("build.parallel_builds", "WARPGATE_BUILD_PARALLEL_BUILDS")
	bind("build.timeout", "WARPGATE_BUILD_TIMEOUT")
	bind("build.concurrency", "WARPGATE_BUILD_CONCURRENCY")
	bind("build.qemu_slowdown_factor", "WARPGATE_BUILD_QEMU_SLOWDOWN_FACTOR")
	bind("build.parallelism_limit", "WARPGATE_BUILD_PARALLELISM_LIMIT")
	bind("build.cpu_fraction", "WARPGATE_BUILD_CPU_FRACTION")
	bind("build.baseline_build_time_min", "WARPGATE_BUILD_BASELINE_BUILD_TIME_MIN")

	// Log
	bind("log.level", "WARPGATE_LOG_LEVEL")
	bind("log.format", "WARPGATE_LOG_FORMAT")

	// AWS (also supports standard AWS_ env vars through AWS SDK)
	bind("aws.region", "AWS_REGION", "AWS_DEFAULT_REGION")
	bind("aws.profile", "AWS_PROFILE")
	// Note: AWS credentials are NOT bound here - they're read directly from env vars
	// after unmarshaling to prevent them from being stored in config files

	// AWS AMI
	bind("aws.ami.instance_type", "WARPGATE_AWS_AMI_INSTANCE_TYPE")
	bind("aws.ami.volume_size", "WARPGATE_AWS_AMI_VOLUME_SIZE")
	bind("aws.ami.device_name", "WARPGATE_AWS_AMI_DEVICE_NAME")
	bind("aws.ami.volume_type", "WARPGATE_AWS_AMI_VOLUME_TYPE")
	bind("aws.ami.polling_interval_sec", "WARPGATE_AWS_AMI_POLLING_INTERVAL_SEC")
	bind("aws.ami.build_timeout_min", "WARPGATE_AWS_AMI_BUILD_TIMEOUT_MIN")
	bind("aws.ami.default_parent_image", "WARPGATE_AWS_AMI_DEFAULT_PARENT_IMAGE")

	// Container
	bind("container.default_platform", "WARPGATE_CONTAINER_DEFAULT_PLATFORM")
	bind("container.default_platforms", "WARPGATE_CONTAINER_DEFAULT_PLATFORMS")
	bind("container.default_registry", "WARPGATE_CONTAINER_DEFAULT_REGISTRY")
	bind("container.default_base_image", "WARPGATE_CONTAINER_DEFAULT_BASE_IMAGE")
	bind("container.default_base_version", "WARPGATE_CONTAINER_DEFAULT_BASE_VERSION")
	bind("container.runtime", "WARPGATE_CONTAINER_RUNTIME")

	// Convert
	bind("convert.default_version", "WARPGATE_CONVERT_DEFAULT_VERSION")
	bind("convert.default_license", "WARPGATE_CONVERT_DEFAULT_LICENSE")
	bind("convert.warpgate_version", "WARPGATE_CONVERT_WARPGATE_VERSION")
	bind("convert.ami_instance_type", "WARPGATE_CONVERT_AMI_INSTANCE_TYPE")
	bind("convert.ami_volume_size", "WARPGATE_CONVERT_AMI_VOLUME_SIZE")

	// BuildKit
	bind("buildkit.endpoint", "WARPGATE_BUILDKIT_ENDPOINT")
	bind("buildkit.tls_enabled", "WARPGATE_BUILDKIT_TLS_ENABLED")
	bind("buildkit.tls_ca_cert", "WARPGATE_BUILDKIT_TLS_CA_CERT")
	bind("buildkit.tls_cert", "WARPGATE_BUILDKIT_TLS_CERT")
	bind("buildkit.tls_key", "WARPGATE_BUILDKIT_TLS_KEY")
}

// Get returns the global config instance
// This is a convenience function that wraps Load()
func Get() (*Config, error) {
	return Load()
}
