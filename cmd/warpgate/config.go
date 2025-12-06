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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage warpgate configuration",
	Long: `Manage warpgate's global configuration file.

The configuration file stores user preferences and environment-specific settings
like default registry, AWS region, build options, etc.

Configuration precedence (highest to lowest):
1. CLI flags
2. Environment variables (WARPGATE_*)
3. Configuration file (~/.warpgate/config.yaml)
4. Built-in defaults`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize default configuration file",
	Long: `Create a new configuration file with default values.

This will create ~/.warpgate/config.yaml with sensible defaults.
If the file already exists, it will be overwritten only with --force.`,
	RunE: runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long: `Display the current configuration with values from all sources.

This shows the effective configuration after merging:
- Built-in defaults
- Configuration file values
- Environment variables
- CLI flag overrides`,
	RunE: runConfigShow,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Long:  `Display the path to the configuration file.`,
	RunE:  runConfigPath,
}

var configSetCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set a configuration value",
	Long: `Set a configuration value in the config file.

Examples:
  warpgate config set log.level debug
  warpgate config set aws.region us-west-2
  warpgate config set registry.default ghcr.io

Use dot notation to set nested values.`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configGetCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Get a configuration value",
	Long: `Get a specific configuration value.

Examples:
  warpgate config get log.level
  warpgate config get aws.region
  warpgate config get registry.default`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

var (
	configForce bool
)

func init() {
	// Add subcommands
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)

	// Flags
	configInitCmd.Flags().BoolVarP(&configForce, "force", "f", false, "Overwrite existing config file")
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".warpgate")
	configPath := filepath.Join(configDir, "config.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !configForce {
		return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load default config
	config, err := globalconfig.Load()
	if err != nil {
		return fmt.Errorf("failed to load default config: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logging.Info("Configuration file created at: %s", configPath)
	logging.Info("Edit this file to customize your warpgate settings")

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	config, err := globalconfig.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Marshal to YAML for display
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println("# Current Warpgate Configuration")
	fmt.Println("# Sources: defaults -> config file -> environment variables -> CLI flags")
	fmt.Println()
	fmt.Print(string(data))

	// Show config file path if it exists
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	home, err := os.UserHomeDir()
	if err == nil {
		v.AddConfigPath(filepath.Join(home, ".warpgate"))
		v.AddConfigPath(filepath.Join(home, ".config", "warpgate"))
	}
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err == nil {
		fmt.Printf("\n# Config file: %s\n", v.ConfigFileUsed())
	} else {
		fmt.Println("\n# No config file found (using defaults)")
	}

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Try to find existing config
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(filepath.Join(home, ".warpgate"))
	v.AddConfigPath(filepath.Join(home, ".config", "warpgate"))
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err == nil {
		fmt.Println(v.ConfigFileUsed())
	} else {
		// Show default path
		defaultPath := filepath.Join(home, ".warpgate", "config.yaml")
		fmt.Printf("%s (not created yet)\n", defaultPath)
		logging.Info("Run 'warpgate config init' to create the config file")
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".warpgate")
	configPath := filepath.Join(configDir, "config.yaml")

	// Create viper instance
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Try to read existing config
	if err := v.ReadInConfig(); err != nil {
		// Config doesn't exist, prompt to create it
		if os.IsNotExist(err) {
			logging.Warn("Config file doesn't exist. Creating it now...")
			if err := runConfigInit(cmd, []string{}); err != nil {
				return err
			}
			// Re-read the newly created config
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read newly created config: %w", err)
			}
		} else {
			return fmt.Errorf("failed to read config: %w", err)
		}
	}

	// Set the value
	v.Set(key, value)

	// Write back to file
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	logging.Info("Set %s = %s", key, value)
	logging.Info("Config file updated: %s", configPath)

	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	config, err := globalconfig.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use viper to navigate the nested structure
	v := viper.New()
	v.SetConfigType("yaml")

	// Marshal config to YAML and reload into viper for easy key access
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := v.ReadConfig(strings.NewReader(string(data))); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	value := v.Get(key)
	if value == nil {
		return fmt.Errorf("key not found: %s", key)
	}

	fmt.Println(value)

	return nil
}
