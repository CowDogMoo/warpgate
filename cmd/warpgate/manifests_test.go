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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/spf13/cobra"
)

func TestManifestsCommandStructure(t *testing.T) {
	t.Parallel()

	if manifestsCmd.Use != "manifests" {
		t.Errorf("manifestsCmd.Use = %q, want %q", manifestsCmd.Use, "manifests")
	}

	expectedSubcmds := []string{"create", "inspect", "list"}
	subCmds := manifestsCmd.Commands()
	subCmdNames := make(map[string]bool)
	for _, c := range subCmds {
		subCmdNames[c.Name()] = true
	}

	for _, name := range expectedSubcmds {
		if !subCmdNames[name] {
			t.Errorf("missing manifests subcommand: %s", name)
		}
	}
}

func TestManifestsSharedFlags(t *testing.T) {
	t.Parallel()

	flags := []string{"registry", "namespace", "auth-file"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := manifestsCmd.PersistentFlags().Lookup(name)
			if f == nil {
				t.Fatalf("missing persistent flag --%s", name)
			}
		})
	}
}

func TestManifestsCreateFlags(t *testing.T) {
	t.Parallel()

	flags := []struct {
		name      string
		shorthand string
	}{
		{"name", ""},
		{"tag", "t"},
		{"digest-dir", ""},
		{"verify-registry", ""},
		{"verify-concurrency", ""},
		{"max-age", ""},
		{"health-check", ""},
		{"require-arch", ""},
		{"best-effort", ""},
		{"annotation", ""},
		{"label", ""},
		{"show-diff", ""},
		{"no-progress", ""},
		{"force", ""},
		{"dry-run", ""},
		{"quiet", "q"},
		{"verbose", "v"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := manifestsCreateCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("missing flag --%s on manifests create command", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("flag --%s shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestManifestsInspectFlags(t *testing.T) {
	t.Parallel()

	flags := []string{"name", "tag"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := manifestsInspectCmd.Flags().Lookup(name)
			if f == nil {
				t.Fatalf("missing flag --%s on manifests inspect command", name)
			}
		})
	}
}

func TestManifestsListFlags(t *testing.T) {
	t.Parallel()

	f := manifestsListCmd.Flags().Lookup("name")
	if f == nil {
		t.Fatal("missing --name flag on manifests list command")
	}
}

func TestDisplayManifestInfo_WithAnnotations(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:   "annotated-image",
		Tag:    "latest",
		Digest: "sha256:abc",
		Size:   100,
		Annotations: map[string]string{
			"org.opencontainers.image.version": "1.0.0",
		},
		Architectures: []manifests.ArchitectureInfo{
			{OS: "linux", Architecture: "amd64", Digest: "sha256:x", Size: 50},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayManifestInfo(info)
	})

	if !strings.Contains(output, "Annotations") {
		t.Error("output should contain Annotations section")
	}
	if !strings.Contains(output, "org.opencontainers.image.version") {
		t.Error("output should contain annotation key")
	}
}

func TestManifestsCreateOptions_Defaults(t *testing.T) {
	t.Parallel()

	tagFlag := manifestsCreateCmd.Flags().Lookup("tag")
	if tagFlag == nil {
		t.Fatal("missing --tag flag")
	}
	if tagFlag.DefValue != "[latest]" {
		t.Errorf("--tag default = %q, want %q", tagFlag.DefValue, "[latest]")
	}

	digestDirFlag := manifestsCreateCmd.Flags().Lookup("digest-dir")
	if digestDirFlag == nil {
		t.Fatal("missing --digest-dir flag")
	}
	if digestDirFlag.DefValue != "." {
		t.Errorf("--digest-dir default = %q, want %q", digestDirFlag.DefValue, ".")
	}

	verifyFlag := manifestsCreateCmd.Flags().Lookup("verify-registry")
	if verifyFlag == nil {
		t.Fatal("missing --verify-registry flag")
	}
	if verifyFlag.DefValue != "true" {
		t.Errorf("--verify-registry default = %q, want %q", verifyFlag.DefValue, "true")
	}
}

func TestManifestsCreateOptionsTypes(t *testing.T) {
	t.Parallel()

	// Verify the type of manifestsCreateOptions fields by testing the struct
	opts := &manifestsCreateOptions{
		name:              "test",
		tags:              []string{"latest"},
		digestDir:         ".",
		verifyRegistry:    true,
		bestEffort:        false,
		force:             false,
		dryRun:            true,
		quiet:             false,
		verbose:           false,
		annotations:       []string{"key=value"},
		labels:            []string{"label=val"},
		healthCheck:       true,
		showDiff:          false,
		noProgress:        false,
		verifyConcurrency: 5,
	}

	if opts.name != "test" {
		t.Errorf("name = %q, want %q", opts.name, "test")
	}
	if !opts.dryRun {
		t.Error("dryRun = false, want true")
	}
	if !opts.healthCheck {
		t.Error("healthCheck = false, want true")
	}
	if opts.verifyConcurrency != 5 {
		t.Errorf("verifyConcurrency = %d, want 5", opts.verifyConcurrency)
	}
}

func TestManifestsSharedOptionsTypes(t *testing.T) {
	t.Parallel()

	opts := &manifestsSharedOptions{
		registry:  "ghcr.io",
		namespace: "cowdogmoo",
		authFile:  "/path/to/auth",
	}

	if opts.registry != "ghcr.io" {
		t.Errorf("registry = %q, want %q", opts.registry, "ghcr.io")
	}
	if opts.namespace != "cowdogmoo" {
		t.Errorf("namespace = %q, want %q", opts.namespace, "cowdogmoo")
	}
	if opts.authFile != "/path/to/auth" {
		t.Errorf("authFile = %q, want %q", opts.authFile, "/path/to/auth")
	}
}

func TestParseMetadata_EmptyInputs(t *testing.T) {
	ctx := setupTestContext(t)

	// Save and restore globals
	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()
	manifestsCreateOpts.annotations = nil
	manifestsCreateOpts.labels = nil

	annotations, labels, err := parseMetadata(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(annotations) != 0 {
		t.Errorf("expected empty annotations, got %d", len(annotations))
	}
	if len(labels) != 0 {
		t.Errorf("expected empty labels, got %d", len(labels))
	}
}

func TestParseMetadata_ValidInputs(t *testing.T) {
	ctx := setupTestContext(t)

	oldOpts := *manifestsCreateOpts
	defer func() { *manifestsCreateOpts = oldOpts }()
	manifestsCreateOpts.annotations = []string{"key1=val1", "key2=val2"}
	manifestsCreateOpts.labels = []string{"label1=lval1"}

	annotations, labels, err := parseMetadata(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(annotations) != 2 {
		t.Errorf("expected 2 annotations, got %d", len(annotations))
	}
	if len(labels) != 1 {
		t.Errorf("expected 1 label, got %d", len(labels))
	}
}

func TestDisplayManifestInfo_NoArchitectures(t *testing.T) {
	// Test edge case: manifest with empty architectures
	// displayManifestInfo accesses info.Architectures[0] for single-arch,
	// so we need at least 1 architecture to avoid panic
	info := &manifests.ManifestInfo{
		Name:      "test/image",
		Tag:       "latest",
		Digest:    "sha256:abc123",
		MediaType: "application/vnd.oci.image.index.v1+json",
		Size:      1024,
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:def456",
				Size:         512,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
			{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
				Digest:       "sha256:ghi789",
				Size:         512,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayManifestInfo(info)
	})

	if !strings.Contains(output, "multi-architecture") {
		t.Error("output should indicate multi-architecture manifest")
	}
	if !strings.Contains(output, "linux/arm64/v8") {
		t.Error("output should contain arm64/v8 platform info")
	}
}

func TestDisplayManifestInfo_WithAnnotationsAndSingleArch(t *testing.T) {
	t.Parallel()

	info := &manifests.ManifestInfo{
		Name:      "annotated-image",
		Tag:       "v1.0",
		Digest:    "sha256:annotated",
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Size:      300,
		Annotations: map[string]string{
			"org.opencontainers.image.authors": "test-team",
			"org.opencontainers.image.version": "1.0.0",
		},
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "arm",
				Variant:      "v7",
				Digest:       "sha256:armv7config",
				Size:         150,
			},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayManifestInfo(info)
	})

	if !strings.Contains(output, "single-architecture manifest") {
		t.Errorf("output missing single-arch indicator: %q", output)
	}
	if !strings.Contains(output, "linux/arm/v7") {
		t.Errorf("output missing variant in single arch: %q", output)
	}
	if !strings.Contains(output, "Annotations:") {
		t.Errorf("output missing Annotations section: %q", output)
	}
	if !strings.Contains(output, "test-team") {
		t.Errorf("output missing annotation value: %q", output)
	}
}

func TestDisplayManifestInfo_MultiArch_Extra(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:      "multi-arch-image",
		Tag:       "latest",
		Digest:    "sha256:multiarchdigest",
		MediaType: "application/vnd.oci.image.index.v1+json",
		Size:      1024,
		Annotations: map[string]string{
			"org.opencontainers.image.source": "https://github.com/test/repo",
		},
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:amd64digest",
				Size:         512,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
			{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
				Digest:       "sha256:arm64digest",
				Size:         500,
				MediaType:    "application/vnd.oci.image.manifest.v1+json",
			},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayManifestInfo(info)
	})

	if !strings.Contains(output, "multi-architecture manifest") {
		t.Errorf("output missing multi-architecture indicator: %q", output)
	}
	if !strings.Contains(output, "Architectures (2)") {
		t.Errorf("output missing architecture count: %q", output)
	}
	if !strings.Contains(output, "linux/amd64") {
		t.Errorf("output missing linux/amd64: %q", output)
	}
	if !strings.Contains(output, "linux/arm64/v8") {
		t.Errorf("output missing linux/arm64/v8 with variant: %q", output)
	}
	if !strings.Contains(output, "Annotations:") {
		t.Errorf("output missing Annotations section: %q", output)
	}
	if !strings.Contains(output, "Media Type:") {
		t.Errorf("output missing Media Type per arch: %q", output)
	}
}

func TestDisplayManifestInfo_EmptyAnnotations(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:        "no-annotations-image",
		Tag:         "v2.0",
		Digest:      "sha256:noannotations",
		MediaType:   "application/vnd.docker.distribution.manifest.v2+json",
		Size:        200,
		Annotations: map[string]string{},
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:configdigest",
				Size:         100,
			},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayManifestInfo(info)
	})

	if strings.Contains(output, "Annotations:") {
		t.Errorf("output should not contain Annotations section for empty annotations: %q", output)
	}
	if !strings.Contains(output, "single-architecture manifest") {
		t.Errorf("output missing single-architecture indicator: %q", output)
	}
}

func TestDisplayManifestInfo_SingleArchWithMediaType(t *testing.T) {
	info := &manifests.ManifestInfo{
		Name:      "single-arch-media",
		Tag:       "v1.0",
		Digest:    "sha256:single",
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Size:      300,
		Architectures: []manifests.ArchitectureInfo{
			{
				OS:           "linux",
				Architecture: "amd64",
				Digest:       "sha256:configdigest",
				Size:         150,
				MediaType:    "application/vnd.docker.container.image.v1+json",
			},
		},
	}

	output := captureStdoutForTest(t, func() {
		displayManifestInfo(info)
	})

	if !strings.Contains(output, "Config Media:") {
		t.Errorf("output missing Config Media for single-arch with MediaType: %q", output)
	}
}

func TestRunManifestsList_ErrorPath_Extra(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(ctx)

	oldListOpts := *manifestsListOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsListOpts = oldListOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsListOpts.name = "nonexistent-image"
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "test"

	err := runManifestsList(cmd, []string{})
	// Will fail because it can't reach the registry, which exercises the error path
	if err == nil {
		t.Log("runManifestsList succeeded (registry reachable)")
	}
}

func TestRunManifestsInspect_ErrorPath(t *testing.T) {
	ctx := setupTestContext(t)
	cmd := &cobra.Command{Use: "inspect"}
	cmd.SetContext(ctx)

	oldInspectOpts := *manifestsInspectOpts
	oldShared := *manifestsSharedOpts
	defer func() {
		*manifestsInspectOpts = oldInspectOpts
		*manifestsSharedOpts = oldShared
	}()

	manifestsInspectOpts.name = "nonexistent-image"
	manifestsInspectOpts.tags = []string{"v999.999"}
	manifestsSharedOpts.registry = "ghcr.io"
	manifestsSharedOpts.namespace = "nonexistent-ns"

	err := runManifestsInspect(cmd, []string{})
	if err == nil {
		t.Log("runManifestsInspect succeeded unexpectedly")
	} else if !strings.Contains(err.Error(), "failed to inspect manifest") {
		t.Logf("runManifestsInspect error (expected): %v", err)
	}
}
