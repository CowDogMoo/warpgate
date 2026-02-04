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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
)

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
