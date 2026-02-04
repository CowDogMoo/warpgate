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
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitConfig_WithNonexistentConfigFile(t *testing.T) {
	// Save and restore cfgFile global
	oldCfgFile := cfgFile
	defer func() { cfgFile = oldCfgFile }()
	cfgFile = "/nonexistent/config/file.yaml"

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("log-level", "", "log level")
	cmd.PersistentFlags().String("log-format", "", "log format")
	cmd.PersistentFlags().Bool("quiet", false, "quiet")
	cmd.PersistentFlags().Bool("verbose", false, "verbose")
	cmd.SetContext(context.Background())

	err := initConfig(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Errorf("error should mention failed to load config, got: %v", err)
	}
}

func TestInitConfig_DefaultAutoDiscovery(t *testing.T) {
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
	cmd.SetContext(context.Background())

	err := initConfig(cmd, []string{})
	if err != nil {
		t.Fatalf("initConfig() with auto-discovery unexpected error: %v", err)
	}

	// Verify config was set in context
	cfg := configFromContext(cmd)
	if cfg == nil {
		t.Error("config should be set in context after initConfig")
	}
}

func TestInitConfig_WithVerboseFlag(t *testing.T) {
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
	_ = cmd.PersistentFlags().Set("verbose", "true")
	cmd.SetContext(context.Background())

	err := initConfig(cmd, []string{})
	if err != nil {
		t.Fatalf("initConfig() with verbose flag unexpected error: %v", err)
	}
}

func TestInitConfig_WithQuietFlag(t *testing.T) {
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
	_ = cmd.PersistentFlags().Set("quiet", "true")
	cmd.SetContext(context.Background())

	err := initConfig(cmd, []string{})
	if err != nil {
		t.Fatalf("initConfig() with quiet flag unexpected error: %v", err)
	}
}

func TestCliOverridesStruct(t *testing.T) {
	t.Parallel()

	overrides := cliOverrides{}
	overrides.Log.Level = "debug"
	overrides.Log.Format = "json"
	overrides.Registry.Default = "ghcr.io"
	overrides.Build.DefaultArch = []string{"amd64", "arm64"}

	if overrides.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", overrides.Log.Level, "debug")
	}
	if overrides.Log.Format != "json" {
		t.Errorf("Log.Format = %q, want %q", overrides.Log.Format, "json")
	}
	if overrides.Registry.Default != "ghcr.io" {
		t.Errorf("Registry.Default = %q, want %q", overrides.Registry.Default, "ghcr.io")
	}
	if len(overrides.Build.DefaultArch) != 2 {
		t.Errorf("Build.DefaultArch length = %d, want 2", len(overrides.Build.DefaultArch))
	}
}

func TestConfigKeyType(t *testing.T) {
	t.Parallel()

	// configKey should be a unique type to avoid collisions
	key1 := configKeyType{}
	key2 := configKeyType{}
	if key1 != key2 {
		t.Error("configKeyType instances should be equal")
	}
}

func TestExecuteFunction(t *testing.T) {
	// We can't easily test Execute() since it calls rootCmd.Execute()
	// which would actually run the CLI. But we can verify it exists
	// and the function signature is correct.
	_ = Execute // verify function exists
}

func TestRegisterRootCompletions(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("log-level", "", "log level")
	cmd.PersistentFlags().String("log-format", "", "log format")

	// Should not panic
	registerRootCompletions(cmd)
}
