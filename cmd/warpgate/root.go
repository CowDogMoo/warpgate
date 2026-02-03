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

// Package main implements the warpgate CLI tool for building container images and AWS AMIs.
// It provides commands for building, validating, converting templates, and managing manifests.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Context key type for storing config
type configKeyType struct{}

var (
	// configKey is the context key for storing the config
	configKey = configKeyType{}

	// Root command options
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "warpgate",
	Short: "Warpgate - Container and AMI image builder",
	Long: `Warpgate is a pure Go tool for building container images and AWS AMIs.
It replaces Packer with a simpler, more integrated workflow.

Configuration Precedence (highest to lowest):
  1. CLI flags (--log-level, --registry, etc.)
  2. Environment variables (WARPGATE_LOG_LEVEL, WARPGATE_REGISTRY_DEFAULT, etc.)
  3. Configuration file (~/.config/warpgate/config.yaml or ~/.warpgate/config.yaml)
  4. Built-in defaults

Configuration files are searched in the following locations:
  1. $XDG_CONFIG_HOME/warpgate/config.yaml (typically ~/.config/warpgate/)
  2. ~/.warpgate/config.yaml (legacy, for backward compatibility)
  3. ./config.yaml (current directory)

To initialize a config file with defaults: warpgate config init
To show current configuration: warpgate config show`,
	Version:           version,
	Args:              cobra.NoArgs,
	PersistentPreRunE: initConfig,
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Config file (default is $HOME/.warpgate/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "", "Log format (text, json, color)")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Quiet mode - only show errors")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose mode - show debug output")

	// Register shell completions for root persistent flags
	registerRootCompletions(rootCmd)

	// Add subcommands
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(convertCmd)
	rootCmd.AddCommand(templatesCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(manifestsCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(cleanupCmd)
}

// registerRootCompletions registers dynamic shell completion functions for root command flags.
func registerRootCompletions(cmd *cobra.Command) {
	// Log level completion
	_ = cmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"debug\tShow all messages including debug",
			"info\tShow info, warn, and error messages (default)",
			"warn\tShow only warnings and errors",
			"error\tShow only errors",
		}, cobra.ShellCompDirectiveNoFileComp
	})

	// Log format completion
	_ = cmd.RegisterFlagCompletionFunc("log-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"color\tColorized text output (default)",
			"text\tPlain text output",
			"json\tJSON structured output",
		}, cobra.ShellCompDirectiveNoFileComp
	})
}

// configFromContext retrieves the config from the command context.
// Returns nil if no config is stored in context.
func configFromContext(cmd *cobra.Command) *config.Config {
	if cfg, ok := cmd.Context().Value(configKey).(*config.Config); ok {
		return cfg
	}
	return nil
}

// cliOverrides holds configuration values that can be overridden via CLI flags.
// This struct is used for type-safe unmarshalling from Viper.
type cliOverrides struct {
	Log struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"log"`
	Registry struct {
		Default string `mapstructure:"default"`
	} `mapstructure:"registry"`
	Build struct {
		DefaultArch []string `mapstructure:"default_arch"`
	} `mapstructure:"build"`
}

// initConfig initializes configuration with proper precedence:
// CLI Flags > Environment Variables > Config File > Defaults
func initConfig(cmd *cobra.Command, args []string) error {
	var cfg *config.Config
	var err error
	if cfgFile != "" {
		// Explicitly requested config file - errors are fatal
		cfg, err = config.LoadFromPath(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config from %s: %w", cfgFile, err)
		}
	} else {
		// Auto-discovery - fall back to defaults if no config found
		cfg, err = config.Load()
		if err != nil {
			// Only warn if config file exists but failed to load
			// Don't warn if simply no config file was found
			// Note: We can't use context-based logging here yet since logger isn't initialized
			// Use fmt.Fprintf to stderr instead
			if !config.IsNotFoundError(err) {
				fmt.Fprintf(os.Stderr, "Warning: failed to load config, using defaults: %v\n", err)
			}
			cfg = &config.Config{}
		}
	}

	v := viper.New()

	// Set defaults from loaded config (these can be overridden by flags/env)
	v.SetDefault("log.level", cfg.Log.Level)
	v.SetDefault("log.format", cfg.Log.Format)
	v.SetDefault("registry.default", cfg.Registry.Default)
	v.SetDefault("build.default_arch", cfg.Build.DefaultArch)

	v.SetEnvPrefix("WARPGATE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.BindPFlag("log.level", cmd.Root().PersistentFlags().Lookup("log-level")); err != nil {
		return fmt.Errorf("failed to bind log-level flag: %w", err)
	}
	if err := v.BindPFlag("log.format", cmd.Root().PersistentFlags().Lookup("log-format")); err != nil {
		return fmt.Errorf("failed to bind log-format flag: %w", err)
	}

	if err := v.BindPFlag("quiet", cmd.Root().PersistentFlags().Lookup("quiet")); err != nil {
		return fmt.Errorf("failed to bind quiet flag: %w", err)
	}
	if err := v.BindPFlag("verbose", cmd.Root().PersistentFlags().Lookup("verbose")); err != nil {
		return fmt.Errorf("failed to bind verbose flag: %w", err)
	}

	BindCommandFlagsToViper(v, cmd)
	ApplyViperOverrides(v, cmd)

	var overrides cliOverrides
	if err := v.Unmarshal(&overrides); err != nil {
		return fmt.Errorf("failed to unmarshal config overrides: %w", err)
	}

	quiet := v.GetBool("quiet")
	verbose := v.GetBool("verbose")

	logger := logging.NewCustomLoggerWithOptions(overrides.Log.Level, overrides.Log.Format, quiet, verbose)

	cfg.Log.Level = overrides.Log.Level
	cfg.Log.Format = overrides.Log.Format
	cfg.Registry.Default = overrides.Registry.Default
	cfg.Build.DefaultArch = overrides.Build.DefaultArch

	ctx := context.WithValue(cmd.Context(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)
	cmd.SetContext(ctx)

	return nil
}

// Execute invokes the top-level cobra command and returns any error.
func Execute() error {
	return rootCmd.Execute()
}

// BindFlagsToViper binds all flags from a command to a Viper instance.
// This enables the configuration precedence: CLI Flags > Environment Variables > Config File > Defaults.
// The viperKey parameter allows specifying a prefix for the Viper keys (e.g., "build" for build command flags).
func BindFlagsToViper(v *viper.Viper, cmd *cobra.Command, viperKey string) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Convert flag name to viper key format (e.g., "digest-dir" -> "digest_dir")
		key := strings.ReplaceAll(f.Name, "-", "_")
		if viperKey != "" {
			key = viperKey + "." + key
		}

		// Only bind if not already set (avoids overwriting persistent flags)
		if err := v.BindPFlag(key, f); err != nil {
			// Note: We can't use context-based logging here since this is called during init
			// Use fmt.Fprintf to stderr instead
			fmt.Fprintf(os.Stderr, "Warning: failed to bind flag %s to viper: %v\n", f.Name, err)
		}
	})
}

// BindCommandFlagsToViper binds flags from the current command and its parent persistent flags to Viper.
// This is called during command execution to ensure all flags follow the configuration precedence chain.
func BindCommandFlagsToViper(v *viper.Viper, cmd *cobra.Command) {
	// Get the command path for namespacing (e.g., "build", "manifests.create")
	cmdPath := getCommandPath(cmd)

	// Bind the command's local flags
	BindFlagsToViper(v, cmd, cmdPath)

	// Map inherited flag names to their nested Viper config keys
	// when the flag name collides with a nested config struct.
	nestedKeyMap := map[string]string{
		"registry": "registry.default",
	}

	// Also bind persistent flags from parent commands
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		key := strings.ReplaceAll(f.Name, "-", "_")
		if mappedKey, ok := nestedKeyMap[key]; ok {
			key = mappedKey
		}
		if err := v.BindPFlag(key, f); err != nil {
			// Note: We can't use context-based logging here since this is called during init
			// Use fmt.Fprintf to stderr instead
			fmt.Fprintf(os.Stderr, "Warning: failed to bind inherited flag %s to viper: %v\n", f.Name, err)
		}
	})
}

// getCommandPath returns the command path for Viper key namespacing.
// For example, "warpgate manifests create" returns "manifests.create".
func getCommandPath(cmd *cobra.Command) string {
	var parts []string
	current := cmd

	for current != nil && current.Parent() != nil {
		parts = append([]string{current.Name()}, parts...)
		current = current.Parent()
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ".")
}

// ApplyViperOverrides pushes Viper values (from env vars or config file)
// back into Cobra flags that were not explicitly set on the command line.
// This ensures that opts structs populated via pflag see env/config values.
func ApplyViperOverrides(v *viper.Viper, cmd *cobra.Command) {
	cmdPath := getCommandPath(cmd)
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			return // explicitly set on CLI — highest precedence
		}
		key := strings.ReplaceAll(f.Name, "-", "_")
		if cmdPath != "" {
			key = cmdPath + "." + key
		}
		if v.IsSet(key) {
			val := v.GetString(key)
			_ = cmd.Flags().Set(f.Name, val)
		}
	})
}
