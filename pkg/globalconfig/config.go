/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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
	DefaultArch    []string `mapstructure:"default_arch"`
	ParallelBuilds bool     `mapstructure:"parallel_builds"`
	Timeout        string   `mapstructure:"timeout"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
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
	v.SetDefault("storage.driver", "vfs")
	home, err := os.UserHomeDir()
	if err == nil {
		v.SetDefault("storage.root", filepath.Join(home, ".local", "share", "containers", "storage"))
		v.SetDefault("templates.cache_dir", filepath.Join(home, ".warpgate", "cache", "templates"))
	}

	// Build defaults
	v.SetDefault("build.parallel_builds", true)
	v.SetDefault("build.default_arch", []string{"amd64"})
	v.SetDefault("build.timeout", "2h")

	// Registry defaults
	v.SetDefault("registry.default", "ghcr.io")

	// Templates defaults
	v.SetDefault("templates.default_repo", "github.com/cowdogmoo/warpgate-templates")
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

	// Log
	_ = v.BindEnv("log.level", "WARPGATE_LOG_LEVEL")
	_ = v.BindEnv("log.format", "WARPGATE_LOG_FORMAT")
}

// Get returns the global config instance
// This is a convenience function that wraps Load()
func Get() (*Config, error) {
	return Load()
}
