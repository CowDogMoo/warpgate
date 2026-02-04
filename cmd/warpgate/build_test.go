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
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
)

func TestBuildCommandFlags(t *testing.T) {
	t.Parallel()

	flags := []struct {
		name      string
		shorthand string
	}{
		{"template", ""},
		{"from-git", ""},
		{"target", ""},
		{"push", ""},
		{"push-digest", ""},
		{"registry", ""},
		{"arch", ""},
		{"tag", "t"},
		{"save-digests", ""},
		{"digest-dir", ""},
		{"region", ""},
		{"instance-type", ""},
		{"var", ""},
		{"var-file", ""},
		{"cache-from", ""},
		{"cache-to", ""},
		{"label", ""},
		{"build-arg", ""},
		{"no-cache", ""},
		{"force", ""},
		{"dry-run", ""},
		{"regions", ""},
		{"parallel-regions", ""},
		{"copy-to-regions", ""},
		{"stream-logs", ""},
		{"show-ec2-status", ""},
		{"output-manifest", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := buildCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("missing flag --%s on build command", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("flag --%s shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestValidBuildTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target string
		valid  bool
	}{
		{"container", true},
		{"ami", true},
		{"", true},
		{"invalid", false},
		{"docker", false},
	}

	for _, tt := range tests {
		t.Run("target_"+tt.target, func(t *testing.T) {
			t.Parallel()
			if got := validBuildTargets[tt.target]; got != tt.valid {
				t.Errorf("validBuildTargets[%q] = %v, want %v", tt.target, got, tt.valid)
			}
		})
	}
}

func TestShouldPerformPush(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     buildOptions
		expected bool
	}{
		{
			name:     "push with registry",
			opts:     buildOptions{push: true, registry: "ghcr.io/test"},
			expected: true,
		},
		{
			name:     "push-digest with registry",
			opts:     buildOptions{pushDigest: true, registry: "ghcr.io/test"},
			expected: true,
		},
		{
			name:     "push without registry",
			opts:     buildOptions{push: true, registry: ""},
			expected: false,
		},
		{
			name:     "no push",
			opts:     buildOptions{push: false, pushDigest: false, registry: "ghcr.io/test"},
			expected: false,
		},
		{
			name:     "neither push nor registry",
			opts:     buildOptions{push: false, pushDigest: false, registry: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldPerformPush(&tt.opts)
			if got != tt.expected {
				t.Errorf("shouldPerformPush() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetermineTargetRegions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.Config
		opts     *buildOptions
		expected []string
	}{
		{
			name:     "regions flag takes priority",
			cfg:      &config.Config{},
			opts:     &buildOptions{regions: []string{"us-east-1", "us-west-2"}, region: "eu-west-1"},
			expected: []string{"us-east-1", "us-west-2"},
		},
		{
			name:     "region flag takes priority over config",
			cfg:      &config.Config{},
			opts:     &buildOptions{region: "us-west-2"},
			expected: []string{"us-west-2"},
		},
		{
			name: "config region used as fallback",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.AWS.Region = "eu-central-1"
				return c
			}(),
			opts:     &buildOptions{},
			expected: []string{"eu-central-1"},
		},
		{
			name:     "no regions specified",
			cfg:      &config.Config{},
			opts:     &buildOptions{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := determineTargetRegions(tt.cfg, tt.opts)
			if len(got) != len(tt.expected) {
				t.Fatalf("determineTargetRegions() returned %d regions, want %d", len(got), len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("region[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetCommonAWSRegions(t *testing.T) {
	t.Parallel()

	regions := getCommonAWSRegions()
	if len(regions) == 0 {
		t.Fatal("getCommonAWSRegions() returned empty list")
	}

	// Verify some expected regions are present
	found := false
	for _, r := range regions {
		if r == "us-east-1\tUS East (N. Virginia)" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected us-east-1 in common AWS regions")
	}
}

func TestGetCommonRegistries(t *testing.T) {
	t.Parallel()

	registries := getCommonRegistries()
	if len(registries) == 0 {
		t.Fatal("getCommonRegistries() returned empty list")
	}

	found := false
	for _, r := range registries {
		if r == "ghcr.io\tGitHub Container Registry" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ghcr.io in common registries")
	}
}

func TestResolveSourceReference(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceMap := map[string]string{
		"arsenal": "/tmp/arsenal",
		"tools":   "/opt/tools",
	}

	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name:     "simple source reference",
			source:   "${sources.arsenal}",
			expected: "/tmp/arsenal",
		},
		{
			name:     "source reference with subpath",
			source:   "${sources.arsenal/scripts/setup.sh}",
			expected: "/tmp/arsenal/scripts/setup.sh",
		},
		{
			name:     "non-reference passthrough",
			source:   "/usr/local/bin/script.sh",
			expected: "/usr/local/bin/script.sh",
		},
		{
			name:     "unknown source reference",
			source:   "${sources.nonexistent}",
			expected: "${sources.nonexistent}",
		},
		{
			name:     "empty string",
			source:   "",
			expected: "",
		},
		{
			name:     "partial match not a reference",
			source:   "sources.arsenal",
			expected: "sources.arsenal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveSourceReference(ctx, tt.source, sourceMap)
			if got != tt.expected {
				t.Errorf("resolveSourceReference(%q) = %q, want %q", tt.source, got, tt.expected)
			}
		})
	}
}

func TestUpdateProvisionerSourcePaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cfg := &builder.Config{
		Sources: []builder.Source{
			{Name: "arsenal", Path: "/tmp/arsenal-clone"},
		},
		Provisioners: []builder.Provisioner{
			{
				Type:   "file",
				Source: "${sources.arsenal}",
			},
			{
				Type:   "file",
				Source: "${sources.arsenal/scripts/run.sh}",
			},
			{
				Type:   "shell",
				Source: "",
				Inline: []string{"echo hello"},
			},
			{
				Type:   "file",
				Source: "/absolute/path/file.txt",
			},
		},
	}

	updateProvisionerSourcePaths(ctx, cfg)

	expected := []string{
		"/tmp/arsenal-clone",
		"/tmp/arsenal-clone/scripts/run.sh",
		"",
		"/absolute/path/file.txt",
	}

	for i, want := range expected {
		got := cfg.Provisioners[i].Source
		if got != want {
			t.Errorf("provisioner[%d].Source = %q, want %q", i, got, want)
		}
	}
}

func TestSourceRefPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		matches bool
	}{
		{"valid source ref", "${sources.arsenal}", true},
		{"valid with subpath", "${sources.arsenal/scripts/run.sh}", true},
		{"valid with hyphens", "${sources.my-template}", true},
		{"valid with underscores", "${sources.my_template}", true},
		{"invalid no dollar", "sources.arsenal", false},
		{"invalid no braces", "$sources.arsenal", false},
		{"invalid empty name", "${sources.}", false},
		{"invalid spaces", "${sources. arsenal}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sourceRefPattern.MatchString(tt.input)
			if got != tt.matches {
				t.Errorf("sourceRefPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.matches)
			}
		})
	}
}
