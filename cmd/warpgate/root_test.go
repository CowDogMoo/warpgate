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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
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
