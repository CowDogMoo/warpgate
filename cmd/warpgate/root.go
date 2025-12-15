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

	"github.com/cowdogmoo/warpgate/v3/pkg/config"
	"github.com/cowdogmoo/warpgate/v3/pkg/logging"
	"github.com/spf13/cobra"
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
It replaces Packer with a simpler, more integrated workflow.`,
	Version:           version,
	PersistentPreRunE: initConfig,
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Config file (default is $HOME/.warpgate/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "", "Log format (text, json, color)")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Quiet mode - only show errors")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose mode - show debug output")

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
}

// configFromContext retrieves the config from the command context.
// Returns nil if no config is stored in context.
func configFromContext(cmd *cobra.Command) *config.Config {
	if cfg, ok := cmd.Context().Value(configKey).(*config.Config); ok {
		return cfg
	}
	return nil
}

// initConfig initializes configuration with proper precedence:
// CLI Flags > Environment Variables > Config File > Defaults
func initConfig(cmd *cobra.Command, args []string) error {
	// 1. Load global config (handles defaults, env vars, and config file)
	var cfg *config.Config
	var err error
	if cfgFile != "" {
		cfg, err = config.LoadFromPath(cfgFile)
	} else {
		cfg, err = config.Load()
	}

	if err != nil {
		// Use default config as fallback
		logging.Warn("failed to load config, using defaults: %v", err)
		cfg = &config.Config{}
	}

	// 2. Create a new Viper instance for flag binding
	v := viper.New()

	// Set defaults from loaded config
	v.SetDefault("log.level", cfg.Log.Level)
	v.SetDefault("log.format", cfg.Log.Format)
	v.SetDefault("registry.default", cfg.Registry.Default)
	v.SetDefault("build.default_arch", cfg.Build.DefaultArch)

	// 3. Bind environment variables
	v.SetEnvPrefix("WARPGATE")
	v.AutomaticEnv()

	// 4. Bind Cobra flags to Viper (this enables: flags > env > config > defaults)
	if err := v.BindPFlag("log.level", cmd.Root().PersistentFlags().Lookup("log-level")); err != nil {
		return fmt.Errorf("failed to bind log-level flag: %w", err)
	}
	if err := v.BindPFlag("log.format", cmd.Root().PersistentFlags().Lookup("log-format")); err != nil {
		return fmt.Errorf("failed to bind log-format flag: %w", err)
	}

	// 5. Get final values from Viper (single source of truth)
	logLevel := v.GetString("log.level")
	logFormat := v.GetString("log.format")
	quiet, _ := cmd.Flags().GetBool("quiet")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// 6. Initialize logging with final values
	if err := logging.Initialize(logLevel, logFormat, quiet, verbose); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}

	// 7. Update config with final Viper values (for use in subcommands)
	cfg.Log.Level = logLevel
	cfg.Log.Format = logFormat
	cfg.Registry.Default = v.GetString("registry.default")
	cfg.Build.DefaultArch = v.GetStringSlice("build.default_arch")

	// 8. Create a context-aware logger and store it in context
	logger := logging.FromContext(cmd.Context()) // Get the initialized logger
	ctx := context.WithValue(cmd.Context(), configKey, cfg)
	ctx = logging.WithLogger(ctx, logger)
	cmd.SetContext(ctx)

	return nil
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
