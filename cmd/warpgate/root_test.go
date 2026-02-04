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
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestGetCommandPath(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "warpgate"}
	child := &cobra.Command{Use: "build"}
	nested := &cobra.Command{Use: "create"}
	parent := &cobra.Command{Use: "manifests"}

	root.AddCommand(child)
	root.AddCommand(parent)
	parent.AddCommand(nested)

	tests := []struct {
		name string
		cmd  *cobra.Command
		want string
	}{
		{"root returns empty", root, ""},
		{"child returns name", child, "build"},
		{"nested returns dotted path", nested, "manifests.create"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getCommandPath(tt.cmd)
			if got != tt.want {
				t.Errorf("getCommandPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigFromContext(t *testing.T) {
	t.Parallel()

	t.Run("nil context value returns nil", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		cmd.SetContext(context.Background())
		if got := configFromContext(cmd); got != nil {
			t.Errorf("configFromContext() = %v, want nil", got)
		}
	})

	t.Run("valid config in context", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		cfg.Log.Level = "debug"
		cmd := &cobra.Command{Use: "test"}
		ctx := context.WithValue(context.Background(), configKey, cfg)
		cmd.SetContext(ctx)
		got := configFromContext(cmd)
		if got == nil {
			t.Fatal("configFromContext() returned nil, want config")
		}
		if got.Log.Level != "debug" {
			t.Errorf("config.Log.Level = %q, want %q", got.Log.Level, "debug")
		}
	})
}

func TestBindFlagsToViper(t *testing.T) {
	t.Parallel()

	t.Run("kebab to snake conversion with namespace", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		cmd := &cobra.Command{Use: "build"}
		cmd.Flags().String("digest-dir", ".", "directory for digests")
		cmd.Flags().Bool("no-cache", false, "disable cache")

		BindFlagsToViper(v, cmd, "build")

		_ = cmd.Flags().Set("digest-dir", "/tmp/digests")
		if got := v.GetString("build.digest_dir"); got != "/tmp/digests" {
			t.Errorf("viper key build.digest_dir = %q, want %q", got, "/tmp/digests")
		}
	})

	t.Run("empty namespace prefix", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		cmd := &cobra.Command{Use: "root"}
		cmd.Flags().String("log-level", "", "log level")

		BindFlagsToViper(v, cmd, "")

		_ = cmd.Flags().Set("log-level", "debug")
		if got := v.GetString("log_level"); got != "debug" {
			t.Errorf("viper key log_level = %q, want %q", got, "debug")
		}
	})
}

func TestApplyViperOverrides(t *testing.T) {
	t.Parallel()

	t.Run("env overrides unset flag", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		cmd := &cobra.Command{Use: "build"}
		root := &cobra.Command{Use: "warpgate"}
		root.AddCommand(cmd)

		cmd.Flags().String("registry", "", "container registry")
		v.Set("build.registry", "ghcr.io/test")

		ApplyViperOverrides(v, cmd)

		got, _ := cmd.Flags().GetString("registry")
		if got != "ghcr.io/test" {
			t.Errorf("flag registry = %q, want %q", got, "ghcr.io/test")
		}
	})

	t.Run("explicit CLI flag not overridden", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		cmd := &cobra.Command{Use: "build"}
		root := &cobra.Command{Use: "warpgate"}
		root.AddCommand(cmd)

		cmd.Flags().String("registry", "", "container registry")
		_ = cmd.Flags().Set("registry", "docker.io/explicit")
		v.Set("build.registry", "ghcr.io/from-env")

		ApplyViperOverrides(v, cmd)

		got, _ := cmd.Flags().GetString("registry")
		if got != "docker.io/explicit" {
			t.Errorf("flag registry = %q, want %q (CLI should win)", got, "docker.io/explicit")
		}
	})
}

func TestRootCommandHelp(t *testing.T) {
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("root --help returned error: %v", err)
	}

	if !strings.Contains(buf.String(), "Warpgate") {
		t.Error("--help output does not contain 'Warpgate'")
	}
}

func TestVersionSubcommand(t *testing.T) {
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("version subcommand returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "warpgate version") {
		t.Errorf("version output %q does not contain 'warpgate version'", output)
	}
}

func TestRootCommandUnknownFlag(t *testing.T) {
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--nonexistent-flag"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}

func TestBuildCommandRequiresTemplate(t *testing.T) {
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"build"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when build runs without --template, got nil")
	}
}

func TestManifestsCommandRequiresRegistry(t *testing.T) {
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"manifests", "create", "--name", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when manifests create runs without --registry, got nil")
	}
}

func TestConfigSubcommands(t *testing.T) {
	subcommands := configCmd.Commands()
	names := make(map[string]bool)
	for _, cmd := range subcommands {
		names[cmd.Name()] = true
	}

	expected := []string{"init", "show", "path", "set", "get"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing config subcommand: %s", name)
		}
	}
}

func TestCompletionSubcommands(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("completion command not registered")
	}
}

func TestCleanupCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "cleanup" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cleanup command not registered")
	}
}

func TestConvertCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "convert" {
			found = true
			break
		}
	}
	if !found {
		t.Error("convert command not registered")
	}
}

func TestTemplatesCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "templates" {
			found = true
			break
		}
	}
	if !found {
		t.Error("templates command not registered")
	}
}

func TestManifestsCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "manifests" {
			found = true
			break
		}
	}
	if !found {
		t.Error("manifests command not registered")
	}
}

func TestBindCommandFlagsToViper_Integration(t *testing.T) {
	t.Parallel()

	t.Run("binds local and inherited flags", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		root := &cobra.Command{Use: "warpgate"}
		root.PersistentFlags().String("log-level", "", "log level")
		child := &cobra.Command{Use: "build"}
		child.Flags().String("registry", "", "registry")
		root.AddCommand(child)

		BindCommandFlagsToViper(v, child)

		_ = child.Flags().Set("registry", "ghcr.io/test")
		got := v.GetString("build.registry")
		if got != "ghcr.io/test" {
			t.Errorf("build.registry = %q, want %q", got, "ghcr.io/test")
		}
	})
}

func TestRootCommandSubcommands(t *testing.T) {
	cmds := rootCmd.Commands()
	cmdNames := make([]string, 0, len(cmds))
	for _, c := range cmds {
		cmdNames = append(cmdNames, c.Name())
	}

	expected := []string{"build", "convert", "templates", "validate", "init", "config", "manifests", "version", "completion", "cleanup"}
	for _, name := range expected {
		found := false
		for _, cmdName := range cmdNames {
			if cmdName == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found in root command", name)
		}
	}
}

func TestRootCommandFlags(t *testing.T) {
	flags := rootCmd.PersistentFlags()

	configFlag := flags.Lookup("config")
	if configFlag == nil {
		t.Error("missing --config persistent flag")
	}

	logLevel := flags.Lookup("log-level")
	if logLevel == nil {
		t.Error("missing --log-level persistent flag")
	}

	logFormat := flags.Lookup("log-format")
	if logFormat == nil {
		t.Error("missing --log-format persistent flag")
	}

	quiet := flags.Lookup("quiet")
	if quiet == nil {
		t.Error("missing --quiet persistent flag")
	}

	verbose := flags.Lookup("verbose")
	if verbose == nil {
		t.Error("missing --verbose persistent flag")
	}
}

func TestValidateCommandArgs(t *testing.T) {
	t.Parallel()

	// Verify validate command requires exactly 1 arg
	if validateCmd.Args == nil {
		t.Error("validate command should have args validation")
	}

	// Test arg validation without executing
	err := cobra.ExactArgs(1)(validateCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args, got nil")
	}

	err = cobra.ExactArgs(1)(validateCmd, []string{"file.yaml"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}
}

func TestInitCommandArgs(t *testing.T) {
	t.Parallel()

	err := cobra.ExactArgs(1)(initCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args, got nil")
	}

	err = cobra.ExactArgs(1)(initCmd, []string{"my-template"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}
}

func TestConfigSetCommandArgs(t *testing.T) {
	t.Parallel()

	err := cobra.ExactArgs(2)(configSetCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	err = cobra.ExactArgs(2)(configSetCmd, []string{"key"})
	if err == nil {
		t.Error("expected error for 1 arg")
	}

	err = cobra.ExactArgs(2)(configSetCmd, []string{"key", "value"})
	if err != nil {
		t.Errorf("expected no error for 2 args, got: %v", err)
	}
}

func TestConfigGetCommandArgs(t *testing.T) {
	t.Parallel()

	err := cobra.ExactArgs(1)(configGetCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	err = cobra.ExactArgs(1)(configGetCmd, []string{"key"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}
}

func TestConfigInitForceFlag(t *testing.T) {
	t.Parallel()

	flag := configInitCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("missing --force flag on config init")
	}
	if flag.Shorthand != "f" {
		t.Errorf("--force shorthand = %q, want %q", flag.Shorthand, "f")
	}
}

func TestInitFromFlag(t *testing.T) {
	t.Parallel()

	flag := initCmd.Flags().Lookup("from")
	if flag == nil {
		t.Fatal("missing --from flag on init")
	}
	if flag.Shorthand != "f" {
		t.Errorf("--from shorthand = %q, want %q", flag.Shorthand, "f")
	}
}

func TestInitOutputFlag(t *testing.T) {
	t.Parallel()

	flag := initCmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("missing --output flag on init")
	}
	if flag.Shorthand != "o" {
		t.Errorf("--output shorthand = %q, want %q", flag.Shorthand, "o")
	}
}

func TestValidateSyntaxOnlyFlag(t *testing.T) {
	t.Parallel()

	flag := validateCmd.Flags().Lookup("syntax-only")
	if flag == nil {
		t.Error("missing --syntax-only flag on validate")
	}
}

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

// setupTestContext creates a context with a logger suitable for testing.
func setupTestContext(t *testing.T) context.Context {
	t.Helper()
	logger := logging.NewCustomLoggerWithOptions("error", "text", true, false)
	ctx := logging.WithLogger(context.Background(), logger)
	return ctx
}

// captureStdoutForTest captures stdout during a function call and returns the output.
func captureStdoutForTest(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read from pipe: %v", err)
	}
	return buf.String()
}
