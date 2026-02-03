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
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestConfigSubcommands(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
