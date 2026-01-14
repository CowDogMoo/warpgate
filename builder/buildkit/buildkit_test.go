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

package buildkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/moby/buildkit/client/llb"
)

func llbState() llb.State {
	return llb.Scratch()
}

// TestParsePlatformEdgeCases tests edge cases in platform parsing
func TestParsePlatformEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		expectError bool
	}{
		{"valid linux/amd64", "linux/amd64", false},
		{"valid linux/arm64", "linux/arm64", false},
		{"empty string", "", true},
		{"only os", "linux", true},
		{"too many parts", "linux/amd64/extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parsePlatform(tt.platform)
			if (err != nil) != tt.expectError {
				t.Errorf("parsePlatform(%q) error = %v, expectError = %v", tt.platform, err, tt.expectError)
			}
		})
	}
}

// TestBuildExportAttributesLogic tests the logic of building export attributes
func TestBuildExportAttributesLogic(t *testing.T) {
	tests := []struct {
		name           string
		imageName      string
		labels         map[string]string
		expectedKeys   []string
		expectedValues map[string]string
	}{
		{
			name:           "basic image",
			imageName:      "test:latest",
			labels:         nil,
			expectedKeys:   []string{"name"},
			expectedValues: map[string]string{"name": "test:latest"},
		},
		{
			name:      "with labels",
			imageName: "test:v1",
			labels: map[string]string{
				"version": "1.0",
				"author":  "test",
			},
			expectedKeys: []string{"name", "label:version", "label:author"},
			expectedValues: map[string]string{
				"name":          "test:v1",
				"label:version": "1.0",
				"label:author":  "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := buildExportAttributes(tt.imageName, tt.labels)

			// Check expected keys exist
			for _, key := range tt.expectedKeys {
				if _, ok := attrs[key]; !ok {
					t.Errorf("missing expected key: %s", key)
				}
			}

			// Check expected values
			for key, expectedVal := range tt.expectedValues {
				if actualVal, ok := attrs[key]; !ok || actualVal != expectedVal {
					t.Errorf("key %s: expected %q, got %q", key, expectedVal, actualVal)
				}
			}
		})
	}
}

// TestExpandContainerVarsLogic tests variable expansion logic
func TestExpandContainerVarsLogic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		env      map[string]string
		expected string
	}{
		{
			name:     "simple expansion",
			input:    "/usr/local/bin:$PATH",
			env:      map[string]string{"PATH": "/usr/bin:/bin"},
			expected: "/usr/local/bin:/usr/bin:/bin",
		},
		{
			name:     "no expansion needed",
			input:    "/usr/local/bin",
			env:      map[string]string{},
			expected: "/usr/local/bin",
		},
		{
			name:     "multiple vars",
			input:    "$HOME/bin:$PATH",
			env:      map[string]string{"HOME": "/home/user", "PATH": "/usr/bin"},
			expected: "/home/user/bin:/usr/bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			result := b.expandContainerVars(tt.input, tt.env)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestFindCommonParentLogic tests common parent directory finding
func TestFindCommonParentLogic(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected string
	}{
		{
			name:     "sibling directories",
			path1:    "/home/user/project1",
			path2:    "/home/user/project2",
			expected: "/home/user",
		},
		{
			name:     "nested directories",
			path1:    "/home/user/project",
			path2:    "/home/user/project/subdir",
			expected: "/home/user/project",
		},
		{
			name:     "completely different",
			path1:    "/home/user",
			path2:    "/opt/app",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findCommonParent(tt.path1, tt.path2)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestExtractArchFromPlatformLogic tests architecture extraction
func TestExtractArchFromPlatformLogic(t *testing.T) {
	tests := []struct {
		platform string
		expected string
	}{
		{"linux/amd64", "amd64"},
		{"linux/arm64", "arm64"},
		{"linux/arm64/v8", "arm64"},
		{"amd64", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			result := extractArchFromPlatform(tt.platform)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestGetPlatformStringLogic tests platform string extraction from config
func TestGetPlatformStringLogic(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		arches   []string
		expected string
	}{
		{
			name:     "platform specified",
			platform: "linux/amd64",
			arches:   []string{"arm64"},
			expected: "linux/amd64",
		},
		{
			name:     "from architectures",
			platform: "",
			arches:   []string{"amd64"},
			expected: "linux/amd64",
		},
		{
			name:     "no platform or arches",
			platform: "",
			arches:   []string{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := builder.Config{
				Base:          builder.BaseImage{Platform: tt.platform},
				Architectures: tt.arches,
			}
			result := getPlatformString(cfg)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestApplyProvisionerTypes(t *testing.T) {
	tests := []struct {
		name        string
		provisioner builder.Provisioner
		expectWarn  bool
	}{
		{
			name: "shell provisioner",
			provisioner: builder.Provisioner{
				Type:   "shell",
				Inline: []string{"echo hello"},
			},
			expectWarn: false,
		},
		{
			name: "script provisioner empty",
			provisioner: builder.Provisioner{
				Type:    "script",
				Scripts: []string{},
			},
			expectWarn: false,
		},
		{
			name: "powershell provisioner empty",
			provisioner: builder.Provisioner{
				Type:      "powershell",
				PSScripts: []string{},
			},
			expectWarn: false,
		},
		{
			name: "unknown provisioner",
			provisioner: builder.Provisioner{
				Type: "unknown",
			},
			expectWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{contextDir: t.TempDir()}
			// We can't easily test LLB state, but we can verify no panics occur
			_, _ = b.applyProvisioner(llbState(), tt.provisioner, builder.Config{})
		})
	}
}

func TestCalculateBuildContextWithPowerShell(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.ps1")
	if err := os.WriteFile(scriptPath, []byte("Write-Host 'test'"), 0644); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:      "powershell",
				PSScripts: []string{scriptPath},
			},
		},
	}

	b := &BuildKitBuilder{}
	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("calculateBuildContext() error = %v", err)
	}

	if ctx == "" {
		t.Error("expected non-empty context path")
	}
}

func TestCalculateBuildContextWithDirectorySource(t *testing.T) {
	// Create a temp directory structure to simulate a source directory
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "my-source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Create a file inside the source directory
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:        "file",
				Source:      sourceDir,
				Destination: "/app",
			},
		},
	}

	b := &BuildKitBuilder{}
	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("calculateBuildContext() error = %v", err)
	}

	// When source is a directory, context should be the directory itself, not its parent
	absSourceDir, _ := filepath.Abs(sourceDir)
	if ctx != absSourceDir {
		t.Errorf("expected context to be source directory %q, got %q", absSourceDir, ctx)
	}
}

func TestCalculateBuildContextWithFileSource(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(sourceFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:        "file",
				Source:      sourceFile,
				Destination: "/etc/config.yaml",
			},
		},
	}

	b := &BuildKitBuilder{}
	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("calculateBuildContext() error = %v", err)
	}

	// When source is a file, context should be the parent directory
	absTmpDir, _ := filepath.Abs(tmpDir)
	if ctx != absTmpDir {
		t.Errorf("expected context to be parent directory %q, got %q", absTmpDir, ctx)
	}
}

func TestCalculateBuildContextWithMixedSources(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory source
	sourceDir := filepath.Join(tmpDir, "subdir", "my-source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Create a file source in the same parent
	sourceFile := filepath.Join(tmpDir, "subdir", "config.yaml")
	if err := os.WriteFile(sourceFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:        "file",
				Source:      sourceDir,
				Destination: "/app",
			},
			{
				Type:        "file",
				Source:      sourceFile,
				Destination: "/etc/config.yaml",
			},
		},
	}

	b := &BuildKitBuilder{}
	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("calculateBuildContext() error = %v", err)
	}

	// Common parent should be tmpDir/subdir (parent of config.yaml and my-source directory)
	expectedCtx := filepath.Join(tmpDir, "subdir")
	absExpected, _ := filepath.Abs(expectedCtx)
	if ctx != absExpected {
		t.Errorf("expected context to be common parent %q, got %q", absExpected, ctx)
	}
}
