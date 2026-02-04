/*
Copyright (c) 2025 Jayson Grace <jayson.e.grace@gmail.com>

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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestInitConfig_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("log:\n  level: debug\n  format: json\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldCfgFile := cfgFile
	defer func() { cfgFile = oldCfgFile }()
	cfgFile = configPath

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("log-level", "", "log level")
	cmd.PersistentFlags().String("log-format", "", "log format")
	cmd.PersistentFlags().Bool("quiet", false, "quiet")
	cmd.PersistentFlags().Bool("verbose", false, "verbose")
	cmd.SetContext(context.Background())

	err := initConfig(cmd, []string{})
	if err != nil {
		t.Fatalf("initConfig() with valid config file unexpected error: %v", err)
	}

	cfg := configFromContext(cmd)
	if cfg == nil {
		t.Fatal("config should be set in context")
	}
}

func TestInitConfig_WithLogLevelFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	oldCfgFile := cfgFile
	defer func() { cfgFile = oldCfgFile }()
	cfgFile = ""

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("log-level", "", "log level")
	cmd.PersistentFlags().String("log-format", "", "log format")
	cmd.PersistentFlags().Bool("quiet", false, "quiet")
	cmd.PersistentFlags().Bool("verbose", false, "verbose")
	_ = cmd.PersistentFlags().Set("log-level", "debug")
	cmd.SetContext(context.Background())

	err := initConfig(cmd, []string{})
	if err != nil {
		t.Fatalf("initConfig() with log-level flag unexpected error: %v", err)
	}
}

func TestInitConfig_WithLogFormatFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	oldCfgFile := cfgFile
	defer func() { cfgFile = oldCfgFile }()
	cfgFile = ""

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("log-level", "", "log level")
	cmd.PersistentFlags().String("log-format", "", "log format")
	cmd.PersistentFlags().Bool("quiet", false, "quiet")
	cmd.PersistentFlags().Bool("verbose", false, "verbose")
	_ = cmd.PersistentFlags().Set("log-format", "json")
	cmd.SetContext(context.Background())

	err := initConfig(cmd, []string{})
	if err != nil {
		t.Fatalf("initConfig() with log-format flag unexpected error: %v", err)
	}
}

func TestBindFlagsToViper_WithPrefix(t *testing.T) {
	t.Parallel()

	v := viper.New()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("my-flag", "default-value", "a test flag")
	cmd.Flags().Bool("enable-feature", false, "a boolean flag")

	BindFlagsToViper(v, cmd, "build")

	// The flag should be bound with the prefix
	// We can check that the key exists in viper
	// Note: the flag value is the default since we didn't set it
}

func TestBindFlagsToViper_NoPrefix(t *testing.T) {
	t.Parallel()

	v := viper.New()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("test-flag", "value", "test")

	BindFlagsToViper(v, cmd, "")
}

func TestBindCommandFlagsToViper_WithInheritedFlags(t *testing.T) {
	t.Parallel()

	v := viper.New()
	parent := &cobra.Command{Use: "parent"}
	parent.PersistentFlags().String("registry", "", "registry")

	child := &cobra.Command{Use: "child"}
	parent.AddCommand(child)

	BindCommandFlagsToViper(v, child)
}

func TestApplyViperOverrides_WithSetValues(t *testing.T) {
	t.Parallel()

	v := viper.New()
	root := &cobra.Command{Use: "warpgate"}
	cmd := &cobra.Command{Use: "test"}
	root.AddCommand(cmd)
	cmd.Flags().String("my-option", "", "test option")

	// Bind and set a value in viper - command path is "test"
	BindFlagsToViper(v, cmd, "test")
	v.Set("test.my_option", "viper-value")

	ApplyViperOverrides(v, cmd)

	// The flag should now have the viper value
	val, err := cmd.Flags().GetString("my-option")
	if err != nil {
		t.Fatalf("GetString error: %v", err)
	}
	if val != "viper-value" {
		t.Errorf("flag value = %q, want %q", val, "viper-value")
	}
}

func TestApplyViperOverrides_ExplicitFlagNotOverridden(t *testing.T) {
	t.Parallel()

	v := viper.New()
	root := &cobra.Command{Use: "warpgate"}
	cmd := &cobra.Command{Use: "test"}
	root.AddCommand(cmd)
	cmd.Flags().String("my-option", "", "test option")

	// Simulate explicitly setting the flag
	_ = cmd.Flags().Set("my-option", "cli-value")

	// Set a different value in viper
	BindFlagsToViper(v, cmd, "test")
	v.Set("test.my_option", "viper-value")

	ApplyViperOverrides(v, cmd)

	// The flag should keep the CLI value (higher precedence)
	val, _ := cmd.Flags().GetString("my-option")
	if val != "cli-value" {
		t.Errorf("flag value = %q, want %q (CLI should take precedence)", val, "cli-value")
	}
}

func TestConfigFromContext_WithConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Log.Level = "debug"
	ctx := context.WithValue(context.Background(), configKey, cfg)

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(ctx)

	result := configFromContext(cmd)
	if result == nil {
		t.Fatal("configFromContext should return config")
	}
	if result.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", result.Log.Level, "debug")
	}
}

func TestConfigFromContext_WithWrongType(t *testing.T) {
	t.Parallel()

	// Store something that's not a *config.Config
	ctx := context.WithValue(context.Background(), configKey, "not-a-config")

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(ctx)

	result := configFromContext(cmd)
	if result != nil {
		t.Error("configFromContext should return nil for wrong type")
	}
}

func TestGetCommandPath_RootCommand(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "root"}
	path := getCommandPath(cmd)
	if path != "" {
		t.Errorf("getCommandPath() for root = %q, want empty", path)
	}
}

func TestGetCommandPath_NestedCommands(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "warpgate"}
	parent := &cobra.Command{Use: "manifests"}
	child := &cobra.Command{Use: "create"}

	root.AddCommand(parent)
	parent.AddCommand(child)

	path := getCommandPath(child)
	if path != "manifests.create" {
		t.Errorf("getCommandPath() = %q, want %q", path, "manifests.create")
	}
}

func TestGetCommandPath_SingleLevel(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "warpgate"}
	child := &cobra.Command{Use: "build"}
	root.AddCommand(child)

	path := getCommandPath(child)
	if path != "build" {
		t.Errorf("getCommandPath() = %q, want %q", path, "build")
	}
}
