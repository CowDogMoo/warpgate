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

	"github.com/spf13/viper"
)

// Config represents the global Warpgate configuration
// This is for user preferences and environment-specific settings,
// NOT for template definitions (which use plain YAML)
type Config struct {
	Registry  RegistryConfig  `mapstructure:"registry"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Templates TemplatesConfig `mapstructure:"templates"`
	Build     BuildConfig     `mapstructure:"build"`
	Log       LogConfig       `mapstructure:"log"`
	AWS       AWSConfig       `mapstructure:"aws"`
	Container ContainerConfig `mapstructure:"container"`
	Convert   ConvertConfig   `mapstructure:"convert"`
}

// RegistryConfig holds registry-related configuration
type RegistryConfig struct {
	Default  string `mapstructure:"default"`
	Username string `mapstructure:"username"`
	Token    string `mapstructure:"token"`
}

// StorageConfig holds storage backend configuration
type StorageConfig struct {
	Driver string `mapstructure:"driver"`
	Root   string `mapstructure:"root"`
}

// TemplatesConfig holds template-related configuration
type TemplatesConfig struct {
	CacheDir    string `mapstructure:"cache_dir"`
	DefaultRepo string `mapstructure:"default_repo"`
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

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// AWSConfig holds AWS-related configuration
type AWSConfig struct {
	Region          string    `mapstructure:"region"`
	Profile         string    `mapstructure:"profile"`
	AccessKeyID     string    `mapstructure:"access_key_id"`
	SecretAccessKey string    `mapstructure:"secret_access_key"`
	AMI             AMIConfig `mapstructure:"ami"`
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
	DefaultPlatform    string   `mapstructure:"default_platform"`
	DefaultPlatforms   []string `mapstructure:"default_platforms"`
	DefaultRegistry    string   `mapstructure:"default_registry"`
	DefaultBaseImage   string   `mapstructure:"default_base_image"`
	DefaultBaseVersion string   `mapstructure:"default_base_version"`
	Runtime            string   `mapstructure:"runtime"` // OCI runtime path (e.g., /usr/bin/crun, /usr/bin/runc)
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

// Load reads and parses the global configuration file
// Returns a Config with defaults if no config file exists
func Load() (*Config, error) {
	v := viper.New()

	// Set config name and type
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Look in these locations (in order)
	home, err := os.UserHomeDir()
	if err == nil {
		v.AddConfigPath(filepath.Join(home, ".warpgate"))
		v.AddConfigPath(filepath.Join(home, ".config", "warpgate")) // XDG standard
	}
	v.AddConfigPath(".")

	// Set sensible defaults
	setDefaults(v)

	// Environment variable support
	// WARPGATE_REGISTRY_DEFAULT, WARPGATE_LOG_LEVEL, etc.
	v.SetEnvPrefix("WARPGATE")
	v.AutomaticEnv()
	bindEnvVars(v)

	// Read config file (optional - doesn't error if missing)
	_ = v.ReadInConfig()

	// Unmarshal into Config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadFromPath loads configuration from a specific file path
func LoadFromPath(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Set sensible defaults
	setDefaults(v)

	// Environment variable support
	v.SetEnvPrefix("WARPGATE")
	v.AutomaticEnv()
	bindEnvVars(v)

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	// Unmarshal into Config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// setDefaults sets default values for all configuration options
func setDefaults(v *viper.Viper) {
	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "color")

	// Storage defaults
	v.SetDefault("storage.driver", "overlay")
	home, err := os.UserHomeDir()
	if err == nil {
		v.SetDefault("storage.root", filepath.Join(home, ".local", "share", "containers", "storage"))
		v.SetDefault("templates.cache_dir", filepath.Join(home, ".warpgate", "cache", "templates"))
	}

	// Build defaults
	v.SetDefault("build.parallel_builds", true)
	v.SetDefault("build.default_arch", []string{"amd64"})
	v.SetDefault("build.timeout", "2h")
	v.SetDefault("build.concurrency", 2)
	v.SetDefault("build.qemu_slowdown_factor", 3)
	v.SetDefault("build.parallelism_limit", 2)
	v.SetDefault("build.cpu_fraction", 0.5)
	v.SetDefault("build.baseline_build_time_min", 10)

	// Registry defaults
	v.SetDefault("registry.default", "ghcr.io")

	// Templates defaults
	v.SetDefault("templates.default_repo", "github.com/cowdogmoo/warpgate-templates")

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
	v.SetDefault("container.default_registry", "localhost")
	v.SetDefault("container.default_base_image", "ubuntu")
	v.SetDefault("container.default_base_version", "latest")
	v.SetDefault("container.runtime", "/usr/bin/crun") // Default to crun, fallback to runc if not available

	// Convert/Packer defaults
	v.SetDefault("convert.default_version", "1.0.0")
	v.SetDefault("convert.default_license", "MIT")
	v.SetDefault("convert.warpgate_version", ">=1.0.0")
	v.SetDefault("convert.ami_instance_type", "t3.micro")
	v.SetDefault("convert.ami_volume_size", 50)
}

// bindEnvVars explicitly binds environment variables to config keys
func bindEnvVars(v *viper.Viper) {
	// Registry
	_ = v.BindEnv("registry.default", "WARPGATE_REGISTRY_DEFAULT")
	_ = v.BindEnv("registry.username", "WARPGATE_REGISTRY_USERNAME")
	_ = v.BindEnv("registry.token", "WARPGATE_REGISTRY_TOKEN")

	// Storage
	_ = v.BindEnv("storage.driver", "WARPGATE_STORAGE_DRIVER")
	_ = v.BindEnv("storage.root", "WARPGATE_STORAGE_ROOT")

	// Templates
	_ = v.BindEnv("templates.cache_dir", "WARPGATE_TEMPLATES_CACHE_DIR")
	_ = v.BindEnv("templates.default_repo", "WARPGATE_TEMPLATES_DEFAULT_REPO")

	// Build
	_ = v.BindEnv("build.default_arch", "WARPGATE_BUILD_DEFAULT_ARCH")
	_ = v.BindEnv("build.parallel_builds", "WARPGATE_BUILD_PARALLEL_BUILDS")
	_ = v.BindEnv("build.timeout", "WARPGATE_BUILD_TIMEOUT")
	_ = v.BindEnv("build.concurrency", "WARPGATE_BUILD_CONCURRENCY")
	_ = v.BindEnv("build.qemu_slowdown_factor", "WARPGATE_BUILD_QEMU_SLOWDOWN_FACTOR")
	_ = v.BindEnv("build.parallelism_limit", "WARPGATE_BUILD_PARALLELISM_LIMIT")
	_ = v.BindEnv("build.cpu_fraction", "WARPGATE_BUILD_CPU_FRACTION")
	_ = v.BindEnv("build.baseline_build_time_min", "WARPGATE_BUILD_BASELINE_BUILD_TIME_MIN")

	// Log
	_ = v.BindEnv("log.level", "WARPGATE_LOG_LEVEL")
	_ = v.BindEnv("log.format", "WARPGATE_LOG_FORMAT")

	// AWS (also supports standard AWS_ env vars through AWS SDK)
	_ = v.BindEnv("aws.region", "AWS_REGION", "AWS_DEFAULT_REGION")
	_ = v.BindEnv("aws.profile", "AWS_PROFILE")
	_ = v.BindEnv("aws.access_key_id", "AWS_ACCESS_KEY_ID")
	_ = v.BindEnv("aws.secret_access_key", "AWS_SECRET_ACCESS_KEY")

	// AWS AMI
	_ = v.BindEnv("aws.ami.instance_type", "WARPGATE_AWS_AMI_INSTANCE_TYPE")
	_ = v.BindEnv("aws.ami.volume_size", "WARPGATE_AWS_AMI_VOLUME_SIZE")
	_ = v.BindEnv("aws.ami.device_name", "WARPGATE_AWS_AMI_DEVICE_NAME")
	_ = v.BindEnv("aws.ami.volume_type", "WARPGATE_AWS_AMI_VOLUME_TYPE")
	_ = v.BindEnv("aws.ami.polling_interval_sec", "WARPGATE_AWS_AMI_POLLING_INTERVAL_SEC")
	_ = v.BindEnv("aws.ami.build_timeout_min", "WARPGATE_AWS_AMI_BUILD_TIMEOUT_MIN")
	_ = v.BindEnv("aws.ami.default_parent_image", "WARPGATE_AWS_AMI_DEFAULT_PARENT_IMAGE")

	// Container
	_ = v.BindEnv("container.default_platform", "WARPGATE_CONTAINER_DEFAULT_PLATFORM")
	_ = v.BindEnv("container.default_platforms", "WARPGATE_CONTAINER_DEFAULT_PLATFORMS")
	_ = v.BindEnv("container.default_registry", "WARPGATE_CONTAINER_DEFAULT_REGISTRY")
	_ = v.BindEnv("container.default_base_image", "WARPGATE_CONTAINER_DEFAULT_BASE_IMAGE")
	_ = v.BindEnv("container.default_base_version", "WARPGATE_CONTAINER_DEFAULT_BASE_VERSION")
	_ = v.BindEnv("container.runtime", "WARPGATE_CONTAINER_RUNTIME")

	// Convert
	_ = v.BindEnv("convert.default_version", "WARPGATE_CONVERT_DEFAULT_VERSION")
	_ = v.BindEnv("convert.default_license", "WARPGATE_CONVERT_DEFAULT_LICENSE")
	_ = v.BindEnv("convert.warpgate_version", "WARPGATE_CONVERT_WARPGATE_VERSION")
	_ = v.BindEnv("convert.ami_instance_type", "WARPGATE_CONVERT_AMI_INSTANCE_TYPE")
	_ = v.BindEnv("convert.ami_volume_size", "WARPGATE_CONVERT_AMI_VOLUME_SIZE")
}

// Get returns the global config instance
// This is a convenience function that wraps Load()
func Get() (*Config, error) {
	return Load()
}
