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
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage warpgate configuration",
	Long: `Manage warpgate's global configuration file.

The configuration file stores user preferences and environment-specific settings
like default registry, AWS region, build options, etc.

Configuration locations (searched in order):
1. $XDG_CONFIG_HOME/warpgate/config.yaml (typically ~/.config/warpgate/config.yaml)
2. ~/.warpgate/config.yaml (legacy, for backward compatibility)
3. ./config.yaml (current directory)

Configuration precedence (highest to lowest):
1. CLI flags
2. Environment variables (WARPGATE_*)
3. Configuration file
4. Built-in defaults`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize default configuration file",
	Long: `Create a new configuration file with default values.

This will create an XDG-compliant config file at:
  $XDG_CONFIG_HOME/warpgate/config.yaml (typically ~/.config/warpgate/config.yaml)

If a legacy config exists at ~/.warpgate/config.yaml, you'll be notified.
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
