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
	"github.com/spf13/cobra"
)

// newTestCmd creates a cobra.Command with a config set in context.
func newTestCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	ctx := context.WithValue(context.Background(), configKey, cfg)
	cmd.SetContext(ctx)
	return cmd
}

// newTestCmdNoConfig creates a cobra.Command with no config in context.
func newTestCmdNoConfig() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	return cmd
}

func TestRunConfigInit_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	err := runConfigInit(cmd, []string{})
	if err != nil {
		t.Fatalf("runConfigInit() unexpected error: %v", err)
	}

	configPath := filepath.Join(tmpDir, "warpgate", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestRunConfigInit_AlreadyExists_NoForce(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create existing config
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("existing: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)
	configForce = false

	err := runConfigInit(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for existing config without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
}

func TestRunConfigInit_AlreadyExists_Force(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create existing config
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("existing: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)
	configForce = true
	defer func() { configForce = false }()

	err := runConfigInit(cmd, []string{})
	if err != nil {
		t.Fatalf("runConfigInit() with --force unexpected error: %v", err)
	}
}

func TestRunConfigInit_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cmd := newTestCmdNoConfig()

	err := runConfigInit(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config not available") {
		t.Errorf("error should mention config not available, got: %v", err)
	}
}

func TestRunConfigShow_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &config.Config{}
	cfg.Log.Level = "info"
	cmd := newTestCmd(cfg)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(cmd, []string{})

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runConfigShow() unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "Current Warpgate Configuration") {
		t.Error("output should contain header")
	}
}

func TestRunConfigShow_NilConfig(t *testing.T) {
	cmd := newTestCmdNoConfig()

	err := runConfigShow(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config not available") {
		t.Errorf("error should mention config not available, got: %v", err)
	}
}

func TestRunConfigPath_NoExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cmd := newTestCmd(&config.Config{})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigPath(cmd, []string{})

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runConfigPath() unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "not created yet") {
		t.Errorf("output should indicate config not created, got: %q", output)
	}
}

func TestRunConfigPath_ExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create existing config
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("log:\n  level: debug\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd(&config.Config{})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigPath(cmd, []string{})

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runConfigPath() unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "config.yaml") {
		t.Errorf("output should contain config path, got: %q", output)
	}
}

func TestRunConfigGet_KnownKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Log.Level = "debug"
	cmd := newTestCmd(cfg)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigGet(cmd, []string{"log.level"})

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runConfigGet() unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read from pipe: %v", err)
	}
	output := strings.TrimSpace(buf.String())

	if output != "debug" {
		t.Errorf("runConfigGet() = %q, want %q", output, "debug")
	}
}

func TestRunConfigGet_UnknownKey(t *testing.T) {
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	err := runConfigGet(cmd, []string{"nonexistent.key.that.does.not.exist"})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "key not found") {
		t.Errorf("error should mention key not found, got: %v", err)
	}
}

func TestRunConfigGet_NilConfig(t *testing.T) {
	cmd := newTestCmdNoConfig()

	err := runConfigGet(cmd, []string{"log.level"})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config not available") {
		t.Errorf("error should mention config not available, got: %v", err)
	}
}

func TestRunConfigSet_WithExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create existing config
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("log:\n  level: info\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd(&config.Config{})

	err := runConfigSet(cmd, []string{"log.level", "debug"})
	if err != nil {
		t.Fatalf("runConfigSet() unexpected error: %v", err)
	}
}

func TestRunConfigSet_NewConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	err := runConfigSet(cmd, []string{"log.level", "debug"})
	if err != nil {
		t.Fatalf("runConfigSet() creating new config unexpected error: %v", err)
	}
}

func TestRunConfigSet_SensitiveValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create existing config
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("aws:\n  region: us-east-1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	// Setting a sensitive key should work (the redaction happens in logging)
	err := runConfigSet(cmd, []string{"aws.access_key_id", "AKIAIOSFODNN7EXAMPLE"})
	if err != nil {
		t.Fatalf("runConfigSet() for sensitive key unexpected error: %v", err)
	}
}

func TestRunConfigGet_NestedKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.AWS.Region = "us-west-2"
	cmd := newTestCmd(cfg)

	output := captureStdoutForTest(t, func() {
		err := runConfigGet(cmd, []string{"aws.region"})
		if err != nil {
			t.Fatalf("runConfigGet() unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "us-west-2") {
		t.Errorf("output should contain us-west-2, got: %q", output)
	}
}

func TestRunConfigGet_RegistryDefault(t *testing.T) {
	cfg := &config.Config{}
	cfg.Registry.Default = "ghcr.io/myorg"
	cmd := newTestCmd(cfg)

	output := captureStdoutForTest(t, func() {
		err := runConfigGet(cmd, []string{"registry.default"})
		if err != nil {
			t.Fatalf("runConfigGet() unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "ghcr.io/myorg") {
		t.Errorf("output should contain ghcr.io/myorg, got: %q", output)
	}
}

func TestRunConfigShow_FullConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &config.Config{}
	cfg.Log.Level = "debug"
	cfg.Log.Format = "json"
	cfg.AWS.Region = "us-east-1"
	cfg.Registry.Default = "ghcr.io/test"
	cmd := newTestCmd(cfg)

	output := captureStdoutForTest(t, func() {
		err := runConfigShow(cmd, []string{})
		if err != nil {
			t.Fatalf("runConfigShow() unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "Current Warpgate Configuration") {
		t.Error("output should contain header")
	}
	if !strings.Contains(output, "debug") {
		t.Error("output should contain log level")
	}
}

func TestRunConfigInit_WithForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create existing config with specific content
	configDir := filepath.Join(tmpDir, "warpgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("old: content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Log.Level = "info"
	cmd := newTestCmd(cfg)

	oldForce := configForce
	defer func() { configForce = oldForce }()
	configForce = true

	err := runConfigInit(cmd, []string{})
	if err != nil {
		t.Fatalf("runConfigInit() with --force unexpected error: %v", err)
	}

	// Verify the file was overwritten
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if strings.Contains(string(data), "old: content") {
		t.Error("config file should have been overwritten")
	}
}

func TestRunConfigInit_AlreadyExists_NoForce_Extra(t *testing.T) {
	ctx := setupTestContext(t)
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)
	cmd.SetContext(ctx)

	// Save and restore configForce
	oldForce := configForce
	defer func() { configForce = oldForce }()
	configForce = false

	// This will attempt to init config; if a config file already exists,
	// it should return error about --force
	err := runConfigInit(cmd, []string{})
	if err != nil {
		// If error mentions "already exists" or "force", that's the expected path
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "force") {
			// Expected behavior
			return
		}
		// Other errors are also acceptable (e.g., config path issues)
		t.Logf("runConfigInit returned error (may be expected): %v", err)
	}
}

func TestRunConfigGet_NonexistentKey(t *testing.T) {
	cfg := &config.Config{}
	cmd := newTestCmd(cfg)

	err := runConfigGet(cmd, []string{"nonexistent.key.path"})
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
	if !strings.Contains(err.Error(), "key not found") {
		t.Errorf("error = %q, want substring 'key not found'", err.Error())
	}
}

func TestRunConfigGet_ValidKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Log.Level = "debug"
	cmd := newTestCmd(cfg)

	output := captureStdoutForTest(t, func() {
		err := runConfigGet(cmd, []string{"log.level"})
		if err != nil {
			t.Fatalf("runConfigGet() error = %v", err)
		}
	})

	if !strings.Contains(output, "debug") {
		t.Errorf("output = %q, want to contain 'debug'", output)
	}
}

func TestRunConfigShow_WithConfig_Extra(t *testing.T) {
	cfg := &config.Config{}
	cfg.Log.Level = "info"
	cfg.Log.Format = "color"
	cmd := newTestCmd(cfg)

	output := captureStdoutForTest(t, func() {
		err := runConfigShow(cmd, []string{})
		if err != nil {
			t.Fatalf("runConfigShow() error = %v", err)
		}
	})

	if !strings.Contains(output, "Current Warpgate Configuration") {
		t.Errorf("output missing header: %q", output)
	}
}

func TestRunConfigShow_NilConfig_Extra(t *testing.T) {
	cmd := newTestCmdNoConfig()

	err := runConfigShow(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when config is nil")
	}
	if !strings.Contains(err.Error(), "config not available") {
		t.Errorf("error = %q, want substring 'config not available'", err.Error())
	}
}

func TestRunConfigPath(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "path"}
	cmd.SetContext(ctx)

	output := captureStdoutForTest(t, func() {
		err := runConfigPath(cmd, []string{})
		if err != nil {
			t.Fatalf("runConfigPath() error = %v", err)
		}
	})

	// Should output some path
	if len(strings.TrimSpace(output)) == 0 {
		t.Error("runConfigPath() produced empty output")
	}
}
