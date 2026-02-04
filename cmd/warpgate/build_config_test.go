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
	"errors"
	"strings"
	"testing"
)

func TestBuildOptsToCliOpts(t *testing.T) {
	t.Parallel()

	t.Run("with args", func(t *testing.T) {
		t.Parallel()
		opts := &buildOptions{
			template:   "my-template",
			fromGit:    "https://github.com/user/repo.git",
			targetType: "container",
			push:       true,
			pushDigest: false,
			registry:   "ghcr.io/test",
			arch:       []string{"amd64", "arm64"},
			tags:       []string{"latest", "v1.0"},
			noCache:    true,
			digestDir:  "/tmp/digests",
		}
		args := []string{"warpgate.yaml"}

		cliOpts := buildOptsToCliOpts(args, opts)

		if cliOpts.ConfigFile != "warpgate.yaml" {
			t.Errorf("ConfigFile = %q, want %q", cliOpts.ConfigFile, "warpgate.yaml")
		}
		if cliOpts.Template != "my-template" {
			t.Errorf("Template = %q, want %q", cliOpts.Template, "my-template")
		}
		if cliOpts.FromGit != "https://github.com/user/repo.git" {
			t.Errorf("FromGit = %q, want %q", cliOpts.FromGit, "https://github.com/user/repo.git")
		}
		if cliOpts.TargetType != "container" {
			t.Errorf("TargetType = %q, want %q", cliOpts.TargetType, "container")
		}
		if !cliOpts.Push {
			t.Error("Push = false, want true")
		}
		if cliOpts.PushDigest {
			t.Error("PushDigest = true, want false")
		}
		if cliOpts.Registry != "ghcr.io/test" {
			t.Errorf("Registry = %q, want %q", cliOpts.Registry, "ghcr.io/test")
		}
		if len(cliOpts.Architectures) != 2 {
			t.Errorf("Architectures length = %d, want 2", len(cliOpts.Architectures))
		}
		if !cliOpts.NoCache {
			t.Error("NoCache = false, want true")
		}
		if cliOpts.DigestDir != "/tmp/digests" {
			t.Errorf("DigestDir = %q, want %q", cliOpts.DigestDir, "/tmp/digests")
		}
	})

	t.Run("without args", func(t *testing.T) {
		t.Parallel()
		opts := &buildOptions{
			template: "attack-box",
		}
		args := []string{}

		cliOpts := buildOptsToCliOpts(args, opts)

		if cliOpts.ConfigFile != "" {
			t.Errorf("ConfigFile = %q, want empty", cliOpts.ConfigFile)
		}
		if cliOpts.Template != "attack-box" {
			t.Errorf("Template = %q, want %q", cliOpts.Template, "attack-box")
		}
	})
}

func TestEnhanceBuildKitError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputErr    error
		wantContain string
	}{
		{
			name:        "no active buildx builder",
			inputErr:    errors.New("no active buildx builder found"),
			wantContain: "docker buildx create",
		},
		{
			name:        "docker daemon not running",
			inputErr:    errors.New("Cannot connect to the Docker daemon"),
			wantContain: "docker is not running",
		},
		{
			name:        "connection refused",
			inputErr:    errors.New("connection refused to socket"),
			wantContain: "docker is not running",
		},
		{
			name:        "docker buildx not available",
			inputErr:    errors.New("docker buildx command not found"),
			wantContain: "docker buildx not available",
		},
		{
			name:        "generic error",
			inputErr:    errors.New("some unknown error"),
			wantContain: "BuildKit error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := enhanceBuildKitError(tt.inputErr)
			if !strings.Contains(got.Error(), tt.wantContain) {
				t.Errorf("enhanceBuildKitError() = %q, want to contain %q", got.Error(), tt.wantContain)
			}
		})
	}
}

func TestLoadBuildConfig_NoInputs(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)
	opts := &buildOptions{}
	args := []string{}

	_, err := loadBuildConfig(ctx, args, opts)
	if err == nil {
		t.Fatal("expected error when no inputs are provided")
	}
	if !strings.Contains(err.Error(), "specify config file") {
		t.Errorf("error should mention specifying config file, got: %v", err)
	}
}

func TestLoadFromFile_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := loadFromFile("/nonexistent/path/warpgate.yaml", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
