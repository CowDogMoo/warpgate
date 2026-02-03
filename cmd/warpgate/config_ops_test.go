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

	w.Close()
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
