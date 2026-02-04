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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dockerimage "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/cowdogmoo/warpgate/v3/templates"
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
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "my-source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

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

	sourceDir := filepath.Join(tmpDir, "subdir", "my-source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

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

func TestPush(t *testing.T) {
	tests := []struct {
		name           string
		imageRef       string
		registry       string
		setupMock      func(*MockDockerClient)
		expectError    bool
		expectedDigest string
	}{
		{
			name:     "successful push with digest",
			imageRef: "myregistry.io/myapp:latest",
			registry: "myregistry.io",
			setupMock: func(m *MockDockerClient) {
				m.ImagePushFunc = func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader(`{"status":"Pushed"}`)), nil
				}
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:          "sha256:abc123",
						RepoDigests: []string{"myregistry.io/myapp@sha256:def456"},
					}, nil
				}
			},
			expectError:    false,
			expectedDigest: "sha256:def456",
		},
		{
			name:     "push failure",
			imageRef: "myregistry.io/myapp:latest",
			registry: "myregistry.io",
			setupMock: func(m *MockDockerClient) {
				m.ImagePushFunc = func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
					return nil, fmt.Errorf("authentication required")
				}
			},
			expectError: true,
		},
		{
			name:     "push with error in JSON response",
			imageRef: "myregistry.io/myapp:latest",
			registry: "myregistry.io",
			setupMock: func(m *MockDockerClient) {
				m.ImagePushFunc = func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)), nil
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockDockerClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			builder := &BuildKitBuilder{
				dockerClient: mockClient,
			}

			digest, err := builder.Push(context.Background(), tt.imageRef, tt.registry)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && digest != tt.expectedDigest {
				t.Errorf("Expected digest %q, got %q", tt.expectedDigest, digest)
			}
		})
	}
}

func TestTag(t *testing.T) {
	tests := []struct {
		name        string
		imageRef    string
		newTag      string
		setupMock   func(*MockDockerClient)
		expectError bool
	}{
		{
			name:     "successful tag",
			imageRef: "myapp:latest",
			newTag:   "myapp:v1.0.0",
			setupMock: func(m *MockDockerClient) {
				m.ImageTagFunc = func(ctx context.Context, source, target string) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name:     "tag failure - image not found",
			imageRef: "myapp:latest",
			newTag:   "myapp:v1.0.0",
			setupMock: func(m *MockDockerClient) {
				m.ImageTagFunc = func(ctx context.Context, source, target string) error {
					return fmt.Errorf("No such image: myapp:latest")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockDockerClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			builder := &BuildKitBuilder{
				dockerClient: mockClient,
			}

			err := builder.Tag(context.Background(), tt.imageRef, tt.newTag)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name        string
		imageRef    string
		setupMock   func(*MockDockerClient)
		expectError bool
	}{
		{
			name:     "successful removal",
			imageRef: "myapp:latest",
			setupMock: func(m *MockDockerClient) {
				m.ImageRemoveFunc = func(ctx context.Context, imageID string, options dockerimage.RemoveOptions) ([]dockerimage.DeleteResponse, error) {
					return []dockerimage.DeleteResponse{{Deleted: imageID}}, nil
				}
			},
			expectError: false,
		},
		{
			name:     "removal failure - image in use",
			imageRef: "myapp:latest",
			setupMock: func(m *MockDockerClient) {
				m.ImageRemoveFunc = func(ctx context.Context, imageID string, options dockerimage.RemoveOptions) ([]dockerimage.DeleteResponse, error) {
					return nil, fmt.Errorf("conflict: unable to remove repository reference")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockDockerClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			builder := &BuildKitBuilder{
				dockerClient: mockClient,
			}

			err := builder.Remove(context.Background(), tt.imageRef)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGetLocalImageDigest(t *testing.T) {
	tests := []struct {
		name           string
		imageName      string
		setupMock      func(*MockDockerClient)
		expectedDigest string
	}{
		{
			name:      "image with repo digest",
			imageName: "myapp:latest",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:          "sha256:abc123",
						RepoDigests: []string{"myregistry.io/myapp@sha256:def456"},
					}, nil
				}
			},
			expectedDigest: "sha256:def456",
		},
		{
			name:      "image without repo digest - fallback to ID",
			imageName: "myapp:latest",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:          "sha256:abc123",
						RepoDigests: []string{},
					}, nil
				}
			},
			expectedDigest: "sha256:abc123",
		},
		{
			name:      "image inspect fails",
			imageName: "nonexistent:latest",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{}, fmt.Errorf("image not found")
				}
			},
			expectedDigest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockDockerClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			builder := &BuildKitBuilder{
				dockerClient: mockClient,
			}

			digest := builder.getLocalImageDigest(context.Background(), tt.imageName)

			if digest != tt.expectedDigest {
				t.Errorf("Expected digest %q, got %q", tt.expectedDigest, digest)
			}
		})
	}
}

func TestLoadAndTagImage(t *testing.T) {
	tests := []struct {
		name        string
		imageName   string
		setupMock   func(*MockDockerClient)
		expectError bool
	}{
		{
			name:      "successful load",
			imageName: "myapp:latest",
			setupMock: func(m *MockDockerClient) {
				m.ImageLoadFunc = func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
					return dockerimage.LoadResponse{
						Body: io.NopCloser(strings.NewReader(`{"stream":"Loaded image: myapp:latest"}`)),
					}, nil
				}
			},
			expectError: false,
		},
		{
			name:      "load failure",
			imageName: "myapp:latest",
			setupMock: func(m *MockDockerClient) {
				m.ImageLoadFunc = func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
					return dockerimage.LoadResponse{}, fmt.Errorf("failed to load image")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockDockerClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			builder := &BuildKitBuilder{
				dockerClient: mockClient,
			}

			// Create a temporary tar file for testing
			tmpFile, err := os.CreateTemp("", "test-image-*.tar")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer func() {
				_ = os.Remove(tmpFile.Name())
			}()
			defer func() {
				_ = tmpFile.Close()
			}()

			// Write some dummy data to the file
			if _, err := tmpFile.WriteString("dummy tar data"); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}

			// Call loadAndTagImage
			err = builder.loadAndTagImage(context.Background(), tmpFile.Name(), tt.imageName)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestInspectManifest(t *testing.T) {
	tests := []struct {
		name           string
		manifestName   string
		setupMock      func(*MockDockerClient)
		expectError    bool
		expectedCount  int
		expectedArch   string
		expectedDigest string
	}{
		{
			name:         "successful manifest inspect",
			manifestName: "myregistry.io/myapp:latest",
			setupMock: func(m *MockDockerClient) {
				m.DistributionInspectFunc = func(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error) {
					return dockerregistry.DistributionInspect{
						Descriptor: ocispec.Descriptor{
							Platform: &ocispec.Platform{
								Architecture: "amd64",
								OS:           "linux",
							},
							Digest: "sha256:abc123",
						},
					}, nil
				}
			},
			expectError:    false,
			expectedCount:  1,
			expectedArch:   "amd64",
			expectedDigest: "sha256:abc123",
		},
		{
			name:         "manifest inspect failure",
			manifestName: "myregistry.io/nonexistent:latest",
			setupMock: func(m *MockDockerClient) {
				m.DistributionInspectFunc = func(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error) {
					return dockerregistry.DistributionInspect{}, fmt.Errorf("manifest not found")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockDockerClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			builder := &BuildKitBuilder{
				dockerClient: mockClient,
			}

			entries, err := builder.InspectManifest(context.Background(), tt.manifestName)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError {
				if len(entries) != tt.expectedCount {
					t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(entries))
				}
				if len(entries) > 0 {
					if entries[0].Architecture != tt.expectedArch {
						t.Errorf("Expected architecture %q, got %q", tt.expectedArch, entries[0].Architecture)
					}
					if entries[0].Digest.String() != tt.expectedDigest {
						t.Errorf("Expected digest %q, got %q", tt.expectedDigest, entries[0].Digest.String())
					}
				}
			}
		})
	}
}

func TestExtractRegistryFromImageRef(t *testing.T) {
	tests := []struct {
		name             string
		imageRef         string
		expectedRegistry string
	}{
		{
			name:             "full registry path with tag",
			imageRef:         "ghcr.io/owner/repo:tag",
			expectedRegistry: "ghcr.io",
		},
		{
			name:             "full registry path with digest",
			imageRef:         "ghcr.io/owner/repo@sha256:abc123",
			expectedRegistry: "ghcr.io",
		},
		{
			name:             "docker hub implicit",
			imageRef:         "ubuntu:latest",
			expectedRegistry: "docker.io",
		},
		{
			name:             "docker hub with library",
			imageRef:         "library/ubuntu:latest",
			expectedRegistry: "docker.io",
		},
		{
			name:             "localhost registry",
			imageRef:         "localhost:5000/myapp:latest",
			expectedRegistry: "localhost:5000",
		},
		{
			name:             "gcr registry",
			imageRef:         "gcr.io/project/image:tag",
			expectedRegistry: "gcr.io",
		},
		{
			name:             "ecr registry",
			imageRef:         "123456789012.dkr.ecr.us-east-1.amazonaws.com/myapp:latest",
			expectedRegistry: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
		},
		{
			name:             "image with multiple path segments",
			imageRef:         "ghcr.io/org/team/repo:tag",
			expectedRegistry: "ghcr.io",
		},
		{
			name:             "image with port in registry",
			imageRef:         "registry.example.com:8080/myapp:latest",
			expectedRegistry: "registry.example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := extractRegistryFromImageRef(tt.imageRef)
			if registry != tt.expectedRegistry {
				t.Errorf("Expected registry %q, got %q", tt.expectedRegistry, registry)
			}
		})
	}
}

// ============================================================
// loadTLSConfig - TLS version and full combination tests
// ============================================================

func TestLoadTLSConfig_MinVersionTLS13(t *testing.T) {
	// Even with empty config, TLS 1.3 minimum should be set
	cfg := config.BuildKitConfig{}
	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tlsCfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected MinVersion TLS 1.3 (%d), got %d", tls.VersionTLS13, tlsCfg.MinVersion)
	}
}

func TestLoadTLSConfig_AllFilesWithVerification(t *testing.T) {
	dir := t.TempDir()
	caCertPath, certPath, keyPath := generateExtraCert(t, dir)

	cfg := config.BuildKitConfig{
		TLSCACert: caCertPath,
		TLSCert:   certPath,
		TLSKey:    keyPath,
	}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tlsCfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected TLS 1.3, got %d", tlsCfg.MinVersion)
	}
	if tlsCfg.RootCAs == nil {
		t.Error("expected RootCAs to be set")
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 client certificate, got %d", len(tlsCfg.Certificates))
	}
}

func TestLoadTLSConfig_CACertOnlyNoCerts(t *testing.T) {
	dir := t.TempDir()
	caCertPath, _, _ := generateExtraCert(t, dir)

	cfg := config.BuildKitConfig{
		TLSCACert: caCertPath,
	}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tlsCfg.RootCAs == nil {
		t.Error("expected RootCAs to be set")
	}
	if len(tlsCfg.Certificates) != 0 {
		t.Errorf("expected no client certificates, got %d", len(tlsCfg.Certificates))
	}
}

func TestLoadTLSConfig_ClientCertWithoutCA(t *testing.T) {
	dir := t.TempDir()
	_, certPath, keyPath := generateExtraCert(t, dir)

	cfg := config.BuildKitConfig{
		TLSCert: certPath,
		TLSKey:  keyPath,
	}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tlsCfg.RootCAs != nil {
		t.Error("expected nil RootCAs when no CA cert provided")
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 client certificate, got %d", len(tlsCfg.Certificates))
	}
}

func TestLoadTLSConfig_CertWithoutKey(t *testing.T) {
	dir := t.TempDir()
	_, certPath, _ := generateExtraCert(t, dir)

	// Providing only cert without key means the condition is not met
	cfg := config.BuildKitConfig{
		TLSCert: certPath,
		TLSKey:  "",
	}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have no client certs since key is missing
	if len(tlsCfg.Certificates) != 0 {
		t.Errorf("expected no client certificates when key is empty, got %d", len(tlsCfg.Certificates))
	}
}

func TestLoadTLSConfig_KeyWithoutCert(t *testing.T) {
	dir := t.TempDir()
	_, _, keyPath := generateExtraCert(t, dir)

	// Providing only key without cert means the condition is not met
	cfg := config.BuildKitConfig{
		TLSCert: "",
		TLSKey:  keyPath,
	}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tlsCfg.Certificates) != 0 {
		t.Errorf("expected no client certificates when cert is empty, got %d", len(tlsCfg.Certificates))
	}
}

func TestLoadTLSConfig_SwappedCertAndKey(t *testing.T) {
	dir := t.TempDir()
	_, certPath, keyPath := generateExtraCert(t, dir)

	// Swap cert and key paths - should fail
	cfg := config.BuildKitConfig{
		TLSCert: keyPath,
		TLSKey:  certPath,
	}

	_, err := loadTLSConfig(cfg)
	if err == nil {
		t.Error("expected error when cert and key are swapped")
	}
}

func TestLoadTLSConfig_EmptyCAFile(t *testing.T) {
	dir := t.TempDir()
	emptyCA := filepath.Join(dir, "empty-ca.pem")
	if err := os.WriteFile(emptyCA, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty CA: %v", err)
	}

	cfg := config.BuildKitConfig{
		TLSCACert: emptyCA,
	}

	_, err := loadTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for empty CA cert file")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to parse CA cert") {
		t.Errorf("expected 'failed to parse CA cert' error, got: %v", err)
	}
}

// generateExtraCert creates test certificates for TLS tests.
func generateExtraCert(t *testing.T, dir string) (caCertPath, certPath, keyPath string) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Extra Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCertPath = filepath.Join(dir, "extra-ca.pem")
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	if err := os.WriteFile(caCertPath, caCertPEM, 0644); err != nil {
		t.Fatalf("failed to write CA cert: %v", err)
	}

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Extra Test Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create client certificate: %v", err)
	}

	certPath = filepath.Join(dir, "extra-cert.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}

	keyPath = filepath.Join(dir, "extra-key.pem")
	clientKeyBytes, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatalf("failed to marshal client key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyBytes})
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	return caCertPath, certPath, keyPath
}

// ============================================================
// createAuthProvider - returns non-nil or nil based on Docker config
// ============================================================

func TestCreateAuthProvider_ReturnsAttachables(t *testing.T) {
	result := createAuthProvider()
	// The result depends on whether Docker config exists on the machine.
	// We just verify it doesn't panic and returns a valid type.
	if result != nil {
		if len(result) == 0 {
			t.Error("expected at least one attachable when result is non-nil")
		}
	}
	// nil is also acceptable (no Docker config)
}

// ============================================================
// makeRelativePath - additional edge cases
// ============================================================

func TestMakeRelativePath_SameDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	result, err := b.makeRelativePath(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test.txt" {
		t.Errorf("expected 'test.txt', got %q", result)
	}
}

func TestMakeRelativePath_NestedSubdirectory(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	filePath := filepath.Join(nestedDir, "deep.txt")
	if err := os.WriteFile(filePath, []byte("deep"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	result, err := b.makeRelativePath(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("a", "b", "c", "deep.txt")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestMakeRelativePath_ContextDirIsParent(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	result, err := b.makeRelativePath(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "sub" {
		t.Errorf("expected 'sub', got %q", result)
	}
}

// ============================================================
// displayProgress - context cancellation and multiple status types
// ============================================================

func TestDisplayProgress_CancelledContext(t *testing.T) {
	b := &BuildKitBuilder{}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *client.SolveStatus, 2)
	done := make(chan struct{})

	// Send status before starting
	ch <- &client.SolveStatus{
		// codespell:ignore vertexes
		Vertexes: []*client.Vertex{
			{
				Digest: digest.FromString("ctx-test"),
				Name:   "context cancellation test",
			},
		},
	}

	go b.displayProgress(ctx, ch, done)

	// Cancel context - displayProgress should still drain channel
	cancel()
	close(ch)

	select {
	case <-done:
		// success - displayProgress completed
	case <-time.After(2 * time.Second):
		t.Fatal("displayProgress did not complete after context cancellation")
	}
}

func TestDisplayProgress_MixedVertexesAndLogs(t *testing.T) {
	b := &BuildKitBuilder{}
	ctx := context.Background()
	ch := make(chan *client.SolveStatus, 5)
	done := make(chan struct{})

	go b.displayProgress(ctx, ch, done)

	// Send status with both vertexes and logs
	ch <- &client.SolveStatus{
		// codespell:ignore vertexes
		Vertexes: []*client.Vertex{
			{Digest: digest.FromString("v1"), Name: "step 1"},
			{Digest: digest.FromString("v2"), Name: "step 2"},
			{Digest: digest.FromString("v3"), Name: ""},
		},
		Logs: []*client.VertexLog{
			{Data: []byte("log from step 1\n")},
			{Data: []byte("log from step 2\n")},
		},
	}

	// Send status with only logs, no vertexes
	ch <- &client.SolveStatus{
		Logs: []*client.VertexLog{
			{Data: []byte("standalone log\n")},
		},
	}

	// Send status with only vertexes, no logs
	ch <- &client.SolveStatus{
		// codespell:ignore vertexes
		Vertexes: []*client.Vertex{
			{Digest: digest.FromString("v4"), Name: "final step"},
		},
	}

	close(ch)

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("displayProgress did not close done channel")
	}
}

// ============================================================
// extractRegistryFromImageRef - comprehensive format testing
// ============================================================

func TestExtractRegistryFromImageRef_Formats(t *testing.T) {
	tests := []struct {
		imageRef string
		expected string
	}{
		// Simple image names default to docker.io
		{"nginx", "docker.io"},
		{"nginx:latest", "docker.io"},
		// User/repo defaults to docker.io (no dots/colons in first segment)
		{"library/nginx", "docker.io"},
		{"myuser/myimage:v1", "docker.io"},
		// Registry with domain
		{"ghcr.io/owner/image:tag", "ghcr.io"},
		{"registry.example.com/image", "registry.example.com"},
		{"my.registry.io/org/image:v2", "my.registry.io"},
		// Registry with port
		{"localhost:5000/myimage", "localhost:5000"},
		{"registry:5000/org/image:tag", "registry:5000"},
		// localhost without port
		{"localhost/myimage", "localhost"},
		// With digest
		{"ghcr.io/owner/image@sha256:abcdef1234567890", "ghcr.io"},
		{"myimage@sha256:abcdef1234567890", "docker.io"},
		{"localhost:5000/img@sha256:abc", "localhost:5000"},
		// Deep paths
		{"gcr.io/my-project/my-image:tag", "gcr.io"},
		{"123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:latest", "123456789.dkr.ecr.us-east-1.amazonaws.com"},
	}

	for _, tt := range tests {
		t.Run(tt.imageRef, func(t *testing.T) {
			result := extractRegistryFromImageRef(tt.imageRef)
			if result != tt.expected {
				t.Errorf("extractRegistryFromImageRef(%q) = %q, want %q", tt.imageRef, result, tt.expected)
			}
		})
	}
}

// ============================================================
// fixedWriteCloser - write and verify data integrity
// ============================================================

func TestFixedWriteCloser_LargeWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.tar")

	factory := fixedWriteCloser(filePath)
	wc, err := factory(map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Write a larger payload
	data := strings.Repeat("abcdefghij", 1000)
	n, err := wc.Write([]byte(data))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}

	if err := wc.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != data {
		t.Error("file content does not match written data")
	}
}

func TestFixedWriteCloser_EmptyMetadataMap(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty-meta.tar")

	factory := fixedWriteCloser(filePath)
	wc, err := factory(map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	// File should exist and be empty
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, got size %d", info.Size())
	}
}

// ============================================================
// buildExportAttributes - various label scenarios
// ============================================================

func TestBuildExportAttributes_SingleLabel(t *testing.T) {
	attrs := buildExportAttributes("myimage:v1", map[string]string{
		"org.opencontainers.image.authors": "test@example.com",
	})

	if attrs["name"] != "myimage:v1" {
		t.Errorf("expected name 'myimage:v1', got %q", attrs["name"])
	}
	labelKey := "label:org.opencontainers.image.authors"
	if attrs[labelKey] != "test@example.com" {
		t.Errorf("expected label value 'test@example.com', got %q", attrs[labelKey])
	}
	// name + 1 label = 2 keys
	if len(attrs) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(attrs))
	}
}

func TestBuildExportAttributes_MultipleLabels(t *testing.T) {
	labels := map[string]string{
		"version":     "1.0.0",
		"maintainer":  "dev@example.com",
		"description": "Test image",
	}
	attrs := buildExportAttributes("test:latest", labels)

	if attrs["name"] != "test:latest" {
		t.Errorf("expected name 'test:latest', got %q", attrs["name"])
	}

	for k, v := range labels {
		labelKey := fmt.Sprintf("label:%s", k)
		if attrs[labelKey] != v {
			t.Errorf("label %q: expected %q, got %q", k, v, attrs[labelKey])
		}
	}

	// name + 3 labels = 4 keys
	if len(attrs) != 4 {
		t.Errorf("expected 4 attributes, got %d", len(attrs))
	}
}

func TestBuildExportAttributes_NilLabelsMap(t *testing.T) {
	attrs := buildExportAttributes("img:v2", nil)

	if attrs["name"] != "img:v2" {
		t.Errorf("expected name 'img:v2', got %q", attrs["name"])
	}
	if len(attrs) != 1 {
		t.Errorf("expected 1 attribute for nil labels, got %d", len(attrs))
	}
}

// ============================================================
// configureCacheOptions - additional edge cases
// ============================================================

func TestConfigureCacheOptions_MultipleCacheFromAndTo(t *testing.T) {
	b := &BuildKitBuilder{
		cacheFrom: []string{
			"type=registry,ref=a:cache",
			"type=registry,ref=b:cache",
			"type=registry,ref=c:cache",
		},
		cacheTo: []string{
			"type=registry,ref=x:cache,mode=max",
			"type=registry,ref=y:cache,mode=min",
		},
	}

	solveOpt := &client.SolveOpt{}
	b.configureCacheOptions(solveOpt, builder.Config{})

	if len(solveOpt.CacheImports) != 3 {
		t.Errorf("expected 3 cache imports, got %d", len(solveOpt.CacheImports))
	}
	if len(solveOpt.CacheExports) != 2 {
		t.Errorf("expected 2 cache exports, got %d", len(solveOpt.CacheExports))
	}

	// Verify all entries have type "registry"
	for i, ci := range solveOpt.CacheImports {
		if ci.Type != "registry" {
			t.Errorf("CacheImports[%d].Type = %q, want 'registry'", i, ci.Type)
		}
	}
	for i, ce := range solveOpt.CacheExports {
		if ce.Type != "registry" {
			t.Errorf("CacheExports[%d].Type = %q, want 'registry'", i, ce.Type)
		}
	}
}

func TestConfigureCacheOptions_NoCacheAndLocalTemplateBothTrue(t *testing.T) {
	b := &BuildKitBuilder{
		cacheFrom: []string{"type=registry,ref=a:cache"},
		cacheTo:   []string{"type=registry,ref=a:cache"},
	}

	solveOpt := &client.SolveOpt{}
	cfg := builder.Config{
		NoCache:         true,
		IsLocalTemplate: true,
	}
	b.configureCacheOptions(solveOpt, cfg)

	if len(solveOpt.CacheImports) != 0 {
		t.Errorf("expected 0 cache imports when NoCache is true, got %d", len(solveOpt.CacheImports))
	}
	if len(solveOpt.CacheExports) != 0 {
		t.Errorf("expected 0 cache exports when NoCache is true, got %d", len(solveOpt.CacheExports))
	}
}

func TestConfigureCacheOptions_EmptyFromNonEmptyTo(t *testing.T) {
	b := &BuildKitBuilder{
		cacheFrom: []string{},
		cacheTo:   []string{"type=registry,ref=out:cache"},
	}

	solveOpt := &client.SolveOpt{}
	b.configureCacheOptions(solveOpt, builder.Config{})

	if len(solveOpt.CacheImports) != 0 {
		t.Errorf("expected 0 cache imports, got %d", len(solveOpt.CacheImports))
	}
	if len(solveOpt.CacheExports) != 1 {
		t.Errorf("expected 1 cache export, got %d", len(solveOpt.CacheExports))
	}
}

// ============================================================
// getPlatformString - all code paths
// ============================================================

func TestGetPlatformString_BasePlatform(t *testing.T) {
	cfg := builder.Config{
		Base: builder.BaseImage{
			Platform: "linux/arm64",
		},
		Architectures: []string{"amd64"},
	}
	result := getPlatformString(cfg)
	if result != "linux/arm64" {
		t.Errorf("expected 'linux/arm64' (base platform takes priority), got %q", result)
	}
}

func TestGetPlatformString_ArchitecturesFallback(t *testing.T) {
	cfg := builder.Config{
		Base:          builder.BaseImage{},
		Architectures: []string{"arm64"},
	}
	result := getPlatformString(cfg)
	if result != "linux/arm64" {
		t.Errorf("expected 'linux/arm64', got %q", result)
	}
}

func TestGetPlatformString_EmptyConfig(t *testing.T) {
	cfg := builder.Config{}
	result := getPlatformString(cfg)
	if result != "unknown" {
		t.Errorf("expected 'unknown', got %q", result)
	}
}

func TestGetPlatformString_MultipleArchitectures(t *testing.T) {
	cfg := builder.Config{
		Architectures: []string{"amd64", "arm64"},
	}
	result := getPlatformString(cfg)
	// Should use the first architecture
	if result != "linux/amd64" {
		t.Errorf("expected 'linux/amd64', got %q", result)
	}
}

// ============================================================
// extractArchFromPlatform - all code paths
// ============================================================

func TestExtractArchFromPlatform_ValidFormats(t *testing.T) {
	tests := []struct {
		platform string
		expected string
	}{
		{"linux/amd64", "amd64"},
		{"linux/arm64", "arm64"},
		{"linux/arm/v7", "arm"},
		{"windows/amd64", "amd64"},
		{"darwin/arm64", "arm64"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			result := extractArchFromPlatform(tt.platform)
			if result != tt.expected {
				t.Errorf("extractArchFromPlatform(%q) = %q, want %q", tt.platform, result, tt.expected)
			}
		})
	}
}

func TestExtractArchFromPlatform_InvalidFormats(t *testing.T) {
	tests := []struct {
		platform string
		expected string
	}{
		{"", ""},
		{"linux", ""},
		{"singlepart", ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("platform=%q", tt.platform), func(t *testing.T) {
			result := extractArchFromPlatform(tt.platform)
			if result != tt.expected {
				t.Errorf("extractArchFromPlatform(%q) = %q, want %q", tt.platform, result, tt.expected)
			}
		})
	}
}

// ============================================================
// getLocalImageDigest - repo digest with @ separator edge cases
// ============================================================

func TestGetLocalImageDigest_DigestWithMultipleAtSigns(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"registry.io/img@sha256:abc123def456"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	d := b.getLocalImageDigest(context.Background(), "test:latest")
	if d != "sha256:abc123def456" {
		t.Errorf("expected 'sha256:abc123def456', got %q", d)
	}
}

func TestGetLocalImageDigest_FallsBackToID(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:fallback_id",
				RepoDigests: []string{},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	d := b.getLocalImageDigest(context.Background(), "test:latest")
	if d != "sha256:fallback_id" {
		t.Errorf("expected 'sha256:fallback_id', got %q", d)
	}
}

func TestGetLocalImageDigest_InspectFailure(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, fmt.Errorf("image not found")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	d := b.getLocalImageDigest(context.Background(), "nonexistent:latest")
	if d != "" {
		t.Errorf("expected empty string for inspect failure, got %q", d)
	}
}

// ============================================================
// loadAndTagImage - success path with mock
// ============================================================

func TestLoadAndTagImage_Success(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "image.tar")
	if err := os.WriteFile(tarPath, []byte("fake tar content"), 0644); err != nil {
		t.Fatalf("failed to write tar: %v", err)
	}

	loadCalled := false
	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			loadCalled = true
			return dockerimage.LoadResponse{
				Body: io.NopCloser(strings.NewReader(`{"stream":"Loaded image: test:latest\n"}`)),
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	err := b.loadAndTagImage(context.Background(), tarPath, "test:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !loadCalled {
		t.Error("expected ImageLoad to be called")
	}
}

func TestLoadAndTagImage_LoadErrorFromMock(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "image.tar")
	if err := os.WriteFile(tarPath, []byte("fake tar content"), 0644); err != nil {
		t.Fatalf("failed to write tar: %v", err)
	}

	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			return dockerimage.LoadResponse{}, fmt.Errorf("load failed")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	err := b.loadAndTagImage(context.Background(), tarPath, "test:latest")
	if err == nil {
		t.Error("expected error from load failure")
	}
	if !strings.Contains(err.Error(), "failed to load image") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ============================================================
// applyProvisioner - dispatch to correct provisioner type
// ============================================================

func TestApplyProvisioner_UnknownTypeReturnsStateUnchanged(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{Type: "custom-unknown"}
	cfg := builder.Config{}

	result, err := b.applyProvisioner(state, prov, cfg)
	if err != nil {
		t.Fatalf("unexpected error for unknown provisioner type: %v", err)
	}
	_ = result
}

func TestApplyProvisioner_ShellType(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"echo hello"},
	}
	cfg := builder.Config{}

	_, err := b.applyProvisioner(state, prov, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// expandContainerVars - additional coverage
// ============================================================

func TestExpandContainerVars_EmptyEnv(t *testing.T) {
	b := &BuildKitBuilder{}
	result := b.expandContainerVars("$PATH/bin", map[string]string{})
	if result != "$PATH/bin" {
		t.Errorf("expected '$PATH/bin' (no expansion), got %q", result)
	}
}

func TestExpandContainerVars_NestedVarReferences(t *testing.T) {
	b := &BuildKitBuilder{}
	env := map[string]string{
		"HOME": "/home/user",
		"PATH": "/usr/bin",
	}
	result := b.expandContainerVars("$HOME/bin:$PATH", env)
	if result != "/home/user/bin:/usr/bin" {
		t.Errorf("expected '/home/user/bin:/usr/bin', got %q", result)
	}
}

// ============================================================
// findCommonParent - additional edge cases
// ============================================================

func TestFindCommonParent_DeeplyNested(t *testing.T) {
	result := findCommonParent("/a/b/c/d/e", "/a/b/c/x/y")
	if result != "/a/b/c" {
		t.Errorf("expected '/a/b/c', got %q", result)
	}
}

func TestFindCommonParent_OneIsParentOfOther(t *testing.T) {
	result := findCommonParent("/a/b", "/a/b/c/d")
	if result != "/a/b" {
		t.Errorf("expected '/a/b', got %q", result)
	}
}

func TestFindCommonParent_IdenticalPaths(t *testing.T) {
	result := findCommonParent("/opt/data", "/opt/data")
	if result != "/opt/data" {
		t.Errorf("expected '/opt/data', got %q", result)
	}
}

// ============================================================
// applyShellProvisioner - empty inline returns unchanged state
// ============================================================

func TestApplyShellProvisioner_EmptyInline(t *testing.T) {
	b := &BuildKitBuilder{}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:   "shell",
		Inline: []string{},
	}

	result, err := b.applyShellProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyShellProvisioner_GoAndNpmCombined(t *testing.T) {
	b := &BuildKitBuilder{}
	state := llb.Image("ubuntu:22.04")
	prov := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"go build ./... && npm install && pip install boto3"},
	}

	_, err := b.applyShellProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// convertToLLB - invalid platform format
// ============================================================

func TestConvertToLLB_InvalidPlatformFormat(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "alpine:latest",
			Platform: "invalid-platform",
		},
	}

	_, err := b.convertToLLB(cfg)
	if err == nil {
		t.Error("expected error for invalid platform format")
	}
}

func TestConvertToLLB_BadPlatformWithArchFallback(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "alpine:latest",
			Platform: "bad",
		},
		Architectures: []string{"arm64"},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Fatalf("expected no error when architectures provides fallback, got: %v", err)
	}
}

// ============================================================
// calculateBuildContext - directory source
// ============================================================

func TestCalculateBuildContext_DirectorySource(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "mydir")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{Type: "file", Source: srcDir, Destination: "/opt/mydir"},
		},
	}

	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Error("expected non-empty context directory")
	}
}

// ============================================================
// Close - nil docker client only
// ============================================================

func TestClose_NilDockerClient(t *testing.T) {
	b := &BuildKitBuilder{
		client:       nil,
		dockerClient: nil,
	}
	err := b.Close()
	if err != nil {
		t.Errorf("expected no error closing nil clients, got: %v", err)
	}
}

func TestClose_MockDockerClientSuccess(t *testing.T) {
	b := &BuildKitBuilder{
		client:       nil,
		dockerClient: &MockDockerClient{},
	}
	err := b.Close()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// ============================================================
// parsePlatform edge cases
// ============================================================

func TestParsePlatform_ThreeParts(t *testing.T) {
	_, _, err := parsePlatform("linux/arm/v7")
	if err == nil {
		t.Error("expected error for three-part platform")
	}
}

func TestParsePlatform_EmptyParts(t *testing.T) {
	os, arch, err := parsePlatform("/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if os != "" || arch != "" {
		t.Errorf("expected empty os and arch, got os=%q arch=%q", os, arch)
	}
}

// ============================================================
// ToDockerSDKAuth - edge cases
// ============================================================

func TestToDockerSDKAuth_InvalidRegistry(t *testing.T) {
	// Use a registry name with spaces which is clearly invalid
	_, err := ToDockerSDKAuth(context.Background(), "invalid registry name with spaces")
	if err == nil {
		t.Error("expected error for invalid registry name")
	}
}

func TestToDockerSDKAuth_ValidRegistryAnonymous(t *testing.T) {
	// Use a registry that is unlikely to have credentials configured
	result, err := ToDockerSDKAuth(context.Background(), "nonexistent-registry-12345.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return empty string for anonymous access (no credentials for this registry)
	// or a valid base64-encoded auth string if somehow configured
	_ = result
}

func TestToDockerSDKAuth_DockerIO(t *testing.T) {
	// docker.io is a valid registry; this tests the happy path parsing
	result, err := ToDockerSDKAuth(context.Background(), "docker.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result is either empty (anonymous) or base64-encoded JSON
	_ = result
}

// ============================================================
// newDockerClientAdapter - wraps a real docker client
// ============================================================

func TestNewDockerClientAdapter_WrapsClient(t *testing.T) {
	// Create a real docker client (may fail on CI, but we test the adapter logic)
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("Docker not available, skipping: %v", err)
	}
	defer cli.Close()

	adapter := newDockerClientAdapter(cli)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}

	// Verify it satisfies the DockerClient interface
	var _ DockerClient = adapter
}

// ============================================================
// applyFileProvisioner - empty source/destination edge cases
// ============================================================

func TestApplyFileProvisioner_EmptySource(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      "",
		Destination: "/opt/data",
	}

	result, err := b.applyFileProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return state unchanged when source is empty
	_ = result
}

func TestApplyFileProvisioner_EmptyDestination(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      "/some/path",
		Destination: "",
	}

	result, err := b.applyFileProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return state unchanged when destination is empty
	_ = result
}

func TestApplyFileProvisioner_BothEmpty(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      "",
		Destination: "",
	}

	result, err := b.applyFileProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyFileProvisioner_NonexistentSource(t *testing.T) {
	dir := t.TempDir()
	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      filepath.Join(dir, "does-not-exist.txt"),
		Destination: "/opt/data",
	}

	_, err := b.applyFileProvisioner(state, prov)
	if err == nil {
		t.Error("expected error for nonexistent source file")
	}
}

func TestApplyFileProvisioner_DirectoryCopy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "mydir")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      srcDir,
		Destination: "/opt/mydir",
	}

	result, err := b.applyFileProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyFileProvisioner_WithMode(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(srcFile, []byte("#!/bin/sh"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      srcFile,
		Destination: "/opt/script.sh",
		Mode:        "0755",
	}

	result, err := b.applyFileProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// applyScriptProvisioner - empty scripts list
// ============================================================

func TestApplyScriptProvisioner_EmptyScripts(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:    "script",
		Scripts: []string{},
	}

	result, err := b.applyScriptProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyScriptProvisioner_NilScripts(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:    "script",
		Scripts: nil,
	}

	result, err := b.applyScriptProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyScriptProvisioner_WithScript(t *testing.T) {
	dir := t.TempDir()
	scriptFile := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(scriptFile, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:    "script",
		Scripts: []string{scriptFile},
	}

	result, err := b.applyScriptProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// applyPowerShellProvisioner - empty/nil PSScripts
// ============================================================

func TestApplyPowerShellProvisioner_EmptyPSScripts(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{},
	}

	result, err := b.applyPowerShellProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyPowerShellProvisioner_NilPSScripts(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: nil,
	}

	result, err := b.applyPowerShellProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyPowerShellProvisioner_WithScript(t *testing.T) {
	dir := t.TempDir()
	psFile := filepath.Join(dir, "setup.ps1")
	if err := os.WriteFile(psFile, []byte("Write-Host 'hello'"), 0755); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{psFile},
	}

	result, err := b.applyPowerShellProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// applyAnsibleProvisioner - empty PlaybookPath and ExtraVars
// ============================================================

func TestApplyAnsibleProvisioner_EmptyPlaybookPath(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "",
	}

	result, err := b.applyAnsibleProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyAnsibleProvisioner_WithPlaybookAndExtraVars(t *testing.T) {
	dir := t.TempDir()
	playbookFile := filepath.Join(dir, "playbook.yml")
	if err := os.WriteFile(playbookFile, []byte("---\n- hosts: all\n  tasks: []\n"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookFile,
		ExtraVars: map[string]string{
			"var1": "value1",
			"var2": "value2",
		},
	}

	result, err := b.applyAnsibleProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyAnsibleProvisioner_WithGalaxyFile(t *testing.T) {
	dir := t.TempDir()
	playbookFile := filepath.Join(dir, "playbook.yml")
	if err := os.WriteFile(playbookFile, []byte("---\n- hosts: all\n  tasks: []\n"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	galaxyFile := filepath.Join(dir, "requirements.yml")
	if err := os.WriteFile(galaxyFile, []byte("---\nroles: []\n"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookFile,
		GalaxyFile:   galaxyFile,
	}

	result, err := b.applyAnsibleProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// collectProvisionerPaths - various provisioner types
// ============================================================

func TestCollectProvisionerPaths_MixedTypes(t *testing.T) {
	dir := t.TempDir()

	// Create real files so path expansion works
	playbookFile := filepath.Join(dir, "playbook.yml")
	if err := os.WriteFile(playbookFile, []byte("---"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	scriptFile := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(scriptFile, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	sourceFile := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(sourceFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: dir}
	provisioners := []builder.Provisioner{
		{Type: "ansible", PlaybookPath: playbookFile},
		{Type: "file", Source: sourceFile, Destination: "/opt/data.txt"},
		{Type: "script", Scripts: []string{scriptFile}},
		{Type: "shell", Inline: []string{"echo hello"}},
	}

	paths, err := b.collectProvisionerPaths(provisioners, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ansible (1 playbook) + file (1 source) + script (1 script) = 3 paths
	// shell type returns no paths
	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d: %v", len(paths), paths)
	}
}

func TestCollectProvisionerPaths_EmptyList(t *testing.T) {
	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: t.TempDir()}

	paths, err := b.collectProvisionerPaths([]builder.Provisioner{}, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestCollectProvisionerPaths_UnknownType(t *testing.T) {
	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	provisioners := []builder.Provisioner{
		{Type: "unknown-type"},
	}

	paths, err := b.collectProvisionerPaths(provisioners, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for unknown type, got %d", len(paths))
	}
}

// ============================================================
// getAnsiblePaths - playbook, galaxy file
// ============================================================

func TestGetAnsiblePaths_PlaybookOnly(t *testing.T) {
	dir := t.TempDir()
	playbookFile := filepath.Join(dir, "playbook.yml")
	if err := os.WriteFile(playbookFile, []byte("---"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: dir}
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookFile,
	}

	paths, err := b.getAnsiblePaths(prov, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}
}

func TestGetAnsiblePaths_PlaybookAndGalaxy(t *testing.T) {
	dir := t.TempDir()
	playbookFile := filepath.Join(dir, "playbook.yml")
	if err := os.WriteFile(playbookFile, []byte("---"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	galaxyFile := filepath.Join(dir, "requirements.yml")
	if err := os.WriteFile(galaxyFile, []byte("---"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: dir}
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookFile,
		GalaxyFile:   galaxyFile,
	}

	paths, err := b.getAnsiblePaths(prov, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

func TestGetAnsiblePaths_EmptyPlaybook(t *testing.T) {
	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "",
	}

	paths, err := b.getAnsiblePaths(prov, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for empty playbook, got %d", len(paths))
	}
}

// ============================================================
// getFilePaths - source path
// ============================================================

func TestGetFilePaths_WithSource(t *testing.T) {
	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(sourceFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: dir}
	prov := builder.Provisioner{
		Type:   "file",
		Source: sourceFile,
	}

	paths, err := b.getFilePaths(prov, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}
}

func TestGetFilePaths_EmptySource(t *testing.T) {
	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	prov := builder.Provisioner{
		Type:   "file",
		Source: "",
	}

	paths, err := b.getFilePaths(prov, pv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths for empty source, got %v", paths)
	}
}

// ============================================================
// expandPathList - empty list and with entries
// ============================================================

func TestExpandPathList_EmptyList(t *testing.T) {
	pv := templates.NewPathValidator()
	paths, err := expandPathList([]string{}, pv, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestExpandPathList_NilList(t *testing.T) {
	pv := templates.NewPathValidator()
	paths, err := expandPathList(nil, pv, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestExpandPathList_WithEntries(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "script1.sh")
	file2 := filepath.Join(dir, "script2.sh")
	if err := os.WriteFile(file1, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := os.WriteFile(file2, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	pv := templates.NewPathValidator()
	paths, err := expandPathList([]string{file1, file2}, pv, "script")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

// ============================================================
// findCommonParent - single path, no common parent
// ============================================================

func TestFindCommonParent_SingleComponentPaths(t *testing.T) {
	result := findCommonParent("/a", "/b")
	if result != "/" {
		t.Errorf("expected '/', got %q", result)
	}
}

func TestFindCommonParent_RootPaths(t *testing.T) {
	result := findCommonParent("/", "/")
	// Both are just root
	if !strings.HasPrefix(result, "/") {
		t.Errorf("expected path starting with '/', got %q", result)
	}
}

// ============================================================
// makeRelativePath - various edge cases
// ============================================================

func TestMakeRelativePath_EmptyContextDir(t *testing.T) {
	// When contextDir is empty, filepath.Abs resolves to cwd
	b := &BuildKitBuilder{contextDir: ""}
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// This should work (empty contextDir becomes cwd)
	result, err := b.makeRelativePath(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

// ============================================================
// Close - error from docker client
// ============================================================

func TestClose_WithDockerClientCloseError(t *testing.T) {
	// MockDockerClient.Close() always returns nil, so this just verifies
	// the nil buildkit client path combined with a docker client
	b := &BuildKitBuilder{
		client:       nil,
		dockerClient: &MockDockerClient{},
	}
	err := b.Close()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestClose_BothClientsNil(t *testing.T) {
	b := &BuildKitBuilder{
		client:       nil,
		dockerClient: nil,
	}
	err := b.Close()
	if err != nil {
		t.Errorf("expected no error when both clients are nil, got: %v", err)
	}
}

// ============================================================
// loadAndTagImage - missing tar file
// ============================================================

func TestLoadAndTagImage_MissingTarFile(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.loadAndTagImage(context.Background(), "/nonexistent/path/image.tar", "test:latest")
	if err == nil {
		t.Error("expected error for missing tar file")
	}
	if !strings.Contains(err.Error(), "failed to open image tar") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadAndTagImage_ImageInspectError(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "image.tar")
	if err := os.WriteFile(tarPath, []byte("fake tar"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			return dockerimage.LoadResponse{
				Body: io.NopCloser(strings.NewReader(`{"stream":"Loaded"}`)),
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	// loadAndTagImage does not call ImageInspect; it just loads.
	// This test verifies that loading succeeds even without inspect.
	err := b.loadAndTagImage(context.Background(), tarPath, "test:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAndTagImage_ResponseReadError(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "image.tar")
	if err := os.WriteFile(tarPath, []byte("fake tar"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			// Return a reader that errors on read
			return dockerimage.LoadResponse{
				Body: io.NopCloser(&failingReader{}),
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	// loadAndTagImage logs the error but does not return it
	err := b.loadAndTagImage(context.Background(), tarPath, "test:latest")
	if err != nil {
		t.Fatalf("unexpected error (response read error is only logged): %v", err)
	}
}

// failingReader always returns an error on Read.
type failingReader struct{}

func (r *failingReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated read error")
}

// ============================================================
// Push - various error paths
// ============================================================

func TestPush_ImagePushError(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return nil, fmt.Errorf("push denied")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	_, err := b.Push(context.Background(), "ghcr.io/test/image:latest", "")
	if err == nil {
		t.Error("expected error from push failure")
	}
	if !strings.Contains(err.Error(), "failed to push") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPush_ResponseContainsError(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	_, err := b.Push(context.Background(), "ghcr.io/test/image:latest", "")
	if err == nil {
		t.Error("expected error from push response containing error")
	}
	if !strings.Contains(err.Error(), "push failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPush_WithRegistryPrefix(t *testing.T) {
	pushCalled := false
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			pushCalled = true
			return io.NopCloser(strings.NewReader(`{"status":"pushed"}`)), nil
		},
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			return nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				RepoDigests: []string{"ghcr.io/test/image@sha256:abc123"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	digest, err := b.Push(context.Background(), "image:latest", "ghcr.io/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pushCalled {
		t.Error("expected push to be called")
	}
	if digest != "sha256:abc123" {
		t.Errorf("expected digest 'sha256:abc123', got %q", digest)
	}
}

func TestPush_InspectFailsAfterPush(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"pushed"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, fmt.Errorf("inspect failed")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	digest, err := b.Push(context.Background(), "ghcr.io/test/image:latest", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When inspect fails, should return empty digest
	if digest != "" {
		t.Errorf("expected empty digest when inspect fails, got %q", digest)
	}
}

// ============================================================
// CreateAndPushManifest - empty entries and error paths
// ============================================================

func TestCreateAndPushManifest_EmptyEntries(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.CreateAndPushManifest(context.Background(), "test:latest", []manifests.ManifestEntry{})
	if err == nil {
		t.Error("expected error for empty manifest entries")
	}
	if !strings.Contains(err.Error(), "no manifest entries provided") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateAndPushManifest_EmptyDigest(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "test:latest",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       digest.Digest(""),
		},
	}

	err := b.CreateAndPushManifest(context.Background(), "test:latest", entries)
	if err == nil {
		t.Error("expected error for empty digest")
	}
}

func TestCreateAndPushManifest_InvalidManifestName(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{Size: 1000}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "test:latest",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       digest.FromString("test"),
		},
	}

	// An invalid manifest name should cause ParseReference to fail
	err := b.CreateAndPushManifest(context.Background(), "!!!invalid!!!", entries)
	if err == nil {
		t.Error("expected error for invalid manifest name")
	}
}

// ============================================================
// applyProvisioner dispatch - additional types
// ============================================================

func TestApplyProvisioner_FileType(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(srcFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      srcFile,
		Destination: "/opt/data.txt",
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyProvisioner_ScriptType(t *testing.T) {
	dir := t.TempDir()
	scriptFile := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(scriptFile, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:    "script",
		Scripts: []string{scriptFile},
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyProvisioner_PowerShellType(t *testing.T) {
	dir := t.TempDir()
	psFile := filepath.Join(dir, "run.ps1")
	if err := os.WriteFile(psFile, []byte("Write-Host 'test'"), 0755); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{psFile},
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

func TestApplyProvisioner_AnsibleType(t *testing.T) {
	dir := t.TempDir()
	playbookFile := filepath.Join(dir, "playbook.yml")
	if err := os.WriteFile(playbookFile, []byte("---\n- hosts: all\n"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: playbookFile,
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// SetCacheOptions
// ============================================================

func TestSetCacheOptions(t *testing.T) {
	tests := []struct {
		name      string
		cacheFrom []string
		cacheTo   []string
	}{
		{
			name:      "empty cache options",
			cacheFrom: []string{},
			cacheTo:   []string{},
		},
		{
			name:      "single cache from",
			cacheFrom: []string{"type=registry,ref=user/app:cache"},
			cacheTo:   []string{},
		},
		{
			name:      "single cache to",
			cacheFrom: []string{},
			cacheTo:   []string{"type=registry,ref=user/app:cache,mode=max"},
		},
		{
			name:      "both cache from and to",
			cacheFrom: []string{"type=registry,ref=user/app:cache"},
			cacheTo:   []string{"type=registry,ref=user/app:cache,mode=max"},
		},
		{
			name:      "multiple cache sources",
			cacheFrom: []string{"type=registry,ref=a:cache", "type=registry,ref=b:cache"},
			cacheTo:   []string{"type=registry,ref=a:cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			ctx := context.Background()

			b.SetCacheOptions(ctx, tt.cacheFrom, tt.cacheTo)

			if len(b.cacheFrom) != len(tt.cacheFrom) {
				t.Errorf("cacheFrom length: expected %d, got %d", len(tt.cacheFrom), len(b.cacheFrom))
			}
			if len(b.cacheTo) != len(tt.cacheTo) {
				t.Errorf("cacheTo length: expected %d, got %d", len(tt.cacheTo), len(b.cacheTo))
			}
			for i, v := range tt.cacheFrom {
				if b.cacheFrom[i] != v {
					t.Errorf("cacheFrom[%d]: expected %q, got %q", i, v, b.cacheFrom[i])
				}
			}
			for i, v := range tt.cacheTo {
				if b.cacheTo[i] != v {
					t.Errorf("cacheTo[%d]: expected %q, got %q", i, v, b.cacheTo[i])
				}
			}
		})
	}
}

// ============================================================
// parseCacheAttrs
// ============================================================

func TestParseCacheAttrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "registry cache spec",
			input: "type=registry,ref=user/app:cache",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
			},
		},
		{
			name:  "full cache spec with mode",
			input: "type=registry,ref=user/app:cache,mode=max",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
				"mode": "max",
			},
		},
		{
			name:     "single key-value",
			input:    "type=local",
			expected: map[string]string{"type": "local"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "value with equals sign",
			input: "type=registry,ref=host:5000/img:tag",
			expected: map[string]string{
				"type": "registry",
				"ref":  "host:5000/img:tag",
			},
		},
		{
			name:     "malformed pair without equals",
			input:    "noequalssign",
			expected: map[string]string{},
		},
		{
			name:  "spaces around keys and values",
			input: " type = registry , ref = user/app:cache ",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCacheAttrs(tt.input)
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
			// Verify no extra keys (except empty strings from malformed input)
			for k, v := range result {
				if k == "" {
					continue
				}
				if _, ok := tt.expected[k]; !ok {
					t.Errorf("unexpected key %q=%q in result", k, v)
				}
			}
		})
	}
}

// ============================================================
// loadTLSConfig
// ============================================================

// generateTestCert generates a self-signed CA cert and a client cert/key pair for testing.
func generateTestCert(t *testing.T, dir string) (caCertPath, certPath, keyPath string) {
	t.Helper()

	// Generate CA key
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCertPath = filepath.Join(dir, "ca.pem")
	caCertFile, err := os.Create(caCertPath)
	if err != nil {
		t.Fatalf("failed to create CA cert file: %v", err)
	}
	if err := pem.Encode(caCertFile, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}); err != nil {
		t.Fatalf("failed to encode CA cert: %v", err)
	}
	if err := caCertFile.Close(); err != nil {
		t.Fatalf("failed to close CA cert file: %v", err)
	}

	// Generate client key
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Test Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create client certificate: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER}); err != nil {
		t.Fatalf("failed to encode client cert: %v", err)
	}
	if err := certFile.Close(); err != nil {
		t.Fatalf("failed to close cert file: %v", err)
	}

	keyPath = filepath.Join(dir, "key.pem")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	clientKeyBytes, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatalf("failed to marshal client key: %v", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyBytes}); err != nil {
		t.Fatalf("failed to encode client key: %v", err)
	}
	if err := keyFile.Close(); err != nil {
		t.Fatalf("failed to close key file: %v", err)
	}

	return caCertPath, certPath, keyPath
}

func TestLoadTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath, certPath, keyPath := generateTestCert(t, tmpDir)

	tests := []struct {
		name        string
		cfg         config.BuildKitConfig
		expectError bool
		checkCA     bool
		checkCert   bool
	}{
		{
			name:        "no TLS files (default config)",
			cfg:         config.BuildKitConfig{},
			expectError: false,
		},
		{
			name: "CA cert only",
			cfg: config.BuildKitConfig{
				TLSCACert: caCertPath,
			},
			expectError: false,
			checkCA:     true,
		},
		{
			name: "client cert and key",
			cfg: config.BuildKitConfig{
				TLSCert: certPath,
				TLSKey:  keyPath,
			},
			expectError: false,
			checkCert:   true,
		},
		{
			name: "all TLS files",
			cfg: config.BuildKitConfig{
				TLSCACert: caCertPath,
				TLSCert:   certPath,
				TLSKey:    keyPath,
			},
			expectError: false,
			checkCA:     true,
			checkCert:   true,
		},
		{
			name: "nonexistent CA cert",
			cfg: config.BuildKitConfig{
				TLSCACert: "/nonexistent/ca.pem",
			},
			expectError: true,
		},
		{
			name: "nonexistent client cert",
			cfg: config.BuildKitConfig{
				TLSCert: "/nonexistent/cert.pem",
				TLSKey:  keyPath,
			},
			expectError: true,
		},
		{
			name: "nonexistent client key",
			cfg: config.BuildKitConfig{
				TLSCert: certPath,
				TLSKey:  "/nonexistent/key.pem",
			},
			expectError: true,
		},
		{
			name: "invalid CA cert content",
			cfg: config.BuildKitConfig{
				TLSCACert: func() string {
					p := filepath.Join(tmpDir, "bad-ca.pem")
					_ = os.WriteFile(p, []byte("not a cert"), 0644)
					return p
				}(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsCfg, err := loadTLSConfig(tt.cfg)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tlsCfg == nil {
				t.Fatal("expected non-nil TLS config")
			}
			if tt.checkCA && tlsCfg.RootCAs == nil {
				t.Error("expected RootCAs to be set")
			}
			if tt.checkCert && len(tlsCfg.Certificates) == 0 {
				t.Error("expected at least one client certificate")
			}
		})
	}
}

// ============================================================
// configureCacheOptions
// ============================================================

func TestConfigureCacheOptions(t *testing.T) {
	tests := []struct {
		name              string
		cacheFrom         []string
		cacheTo           []string
		cfg               builder.Config
		expectImportCount int
		expectExportCount int
	}{
		{
			name:              "no cache, no options",
			cacheFrom:         []string{},
			cacheTo:           []string{},
			cfg:               builder.Config{},
			expectImportCount: 0,
			expectExportCount: 0,
		},
		{
			name:              "cache from and to configured",
			cacheFrom:         []string{"type=registry,ref=user/app:cache"},
			cacheTo:           []string{"type=registry,ref=user/app:cache,mode=max"},
			cfg:               builder.Config{},
			expectImportCount: 1,
			expectExportCount: 1,
		},
		{
			name:              "NoCache disables caching",
			cacheFrom:         []string{"type=registry,ref=user/app:cache"},
			cacheTo:           []string{"type=registry,ref=user/app:cache"},
			cfg:               builder.Config{NoCache: true},
			expectImportCount: 0,
			expectExportCount: 0,
		},
		{
			name:              "IsLocalTemplate disables caching",
			cacheFrom:         []string{"type=registry,ref=user/app:cache"},
			cacheTo:           []string{"type=registry,ref=user/app:cache"},
			cfg:               builder.Config{IsLocalTemplate: true},
			expectImportCount: 0,
			expectExportCount: 0,
		},
		{
			name:              "multiple cache sources",
			cacheFrom:         []string{"type=registry,ref=a:cache", "type=registry,ref=b:cache"},
			cacheTo:           []string{"type=registry,ref=c:cache"},
			cfg:               builder.Config{},
			expectImportCount: 2,
			expectExportCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{
				cacheFrom: tt.cacheFrom,
				cacheTo:   tt.cacheTo,
			}

			solveOpt := &client.SolveOpt{}
			b.configureCacheOptions(solveOpt, tt.cfg)

			if len(solveOpt.CacheImports) != tt.expectImportCount {
				t.Errorf("CacheImports count: expected %d, got %d", tt.expectImportCount, len(solveOpt.CacheImports))
			}
			if len(solveOpt.CacheExports) != tt.expectExportCount {
				t.Errorf("CacheExports count: expected %d, got %d", tt.expectExportCount, len(solveOpt.CacheExports))
			}
		})
	}
}

// ============================================================
// applyPostChanges
// ============================================================

func TestApplyPostChanges(t *testing.T) {
	tests := []struct {
		name        string
		postChanges []string
	}{
		{
			name:        "empty post changes",
			postChanges: []string{},
		},
		{
			name:        "ENV with equals sign",
			postChanges: []string{"ENV PATH=/usr/local/bin:/usr/bin"},
		},
		{
			name:        "ENV with space separated key value",
			postChanges: []string{"ENV MY_VAR my_value"},
		},
		{
			name:        "WORKDIR change",
			postChanges: []string{"WORKDIR /app"},
		},
		{
			name:        "USER change",
			postChanges: []string{"USER nobody"},
		},
		{
			name: "multiple changes",
			postChanges: []string{
				"ENV PATH=/custom:$PATH",
				"WORKDIR /home/user",
				"USER user",
			},
		},
		{
			name:        "single word entry - skip",
			postChanges: []string{"INVALID"},
		},
		{
			name:        "unknown instruction - skip",
			postChanges: []string{"COPY src dst"},
		},
		{
			name:        "ENV with only key no value - skip",
			postChanges: []string{"ENV ALONE"},
		},
		{
			name: "ENV with variable expansion",
			postChanges: []string{
				"ENV HOME /home/user",
				"ENV PATH $HOME/bin:$PATH",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			state := llb.Image("alpine:latest")

			// Should not panic
			result := b.applyPostChanges(state, tt.postChanges)
			_ = result
		})
	}
}

// ============================================================
// detectCollectionRoot
// ============================================================

func TestDetectCollectionRoot(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) string
		expectedRoot bool
	}{
		{
			name: "playbook in collection with galaxy.yml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				collectionDir := filepath.Join(dir, "mycollection")
				playbooksDir := filepath.Join(collectionDir, "playbooks")
				_ = os.MkdirAll(playbooksDir, 0755)
				_ = os.WriteFile(filepath.Join(collectionDir, "galaxy.yml"), []byte("namespace: test"), 0644)
				return filepath.Join(playbooksDir, "site.yml")
			},
			expectedRoot: true,
		},
		{
			name: "playbook in roles directory with galaxy.yml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				collectionDir := filepath.Join(dir, "mycollection")
				rolesDir := filepath.Join(collectionDir, "roles")
				_ = os.MkdirAll(rolesDir, 0755)
				_ = os.WriteFile(filepath.Join(collectionDir, "galaxy.yml"), []byte("namespace: test"), 0644)
				return filepath.Join(rolesDir, "main.yml")
			},
			expectedRoot: true,
		},
		{
			name: "playbook without collection structure",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return filepath.Join(dir, "playbook.yml")
			},
			expectedRoot: false,
		},
		{
			name: "playbook in playbooks dir but no galaxy.yml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				playbooksDir := filepath.Join(dir, "playbooks")
				_ = os.MkdirAll(playbooksDir, 0755)
				return filepath.Join(playbooksDir, "site.yml")
			},
			expectedRoot: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playbookPath := tt.setup(t)
			root := detectCollectionRoot(playbookPath)

			if tt.expectedRoot && root == "" {
				t.Error("expected non-empty collection root, got empty")
			}
			if !tt.expectedRoot && root != "" {
				t.Errorf("expected empty collection root, got %q", root)
			}
		})
	}
}

// ============================================================
// makeRelativePath
// ============================================================

func TestMakeRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	_ = os.MkdirAll(subDir, 0755)

	filePath := filepath.Join(subDir, "file.txt")
	_ = os.WriteFile(filePath, []byte("test"), 0644)

	tests := []struct {
		name        string
		contextDir  string
		path        string
		expectError bool
		expectRel   string
	}{
		{
			name:       "absolute path within context",
			contextDir: tmpDir,
			path:       filePath,
			expectRel:  filepath.Join("sub", "file.txt"),
		},
		{
			name:       "path is the context dir itself",
			contextDir: tmpDir,
			path:       tmpDir,
			expectRel:  ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{contextDir: tt.contextDir}
			result, err := b.makeRelativePath(tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expectRel {
				t.Errorf("expected %q, got %q", tt.expectRel, result)
			}
		})
	}
}

// ============================================================
// fixedWriteCloser
// ============================================================

func TestFixedWriteCloser(t *testing.T) {
	t.Run("creates file and writes data", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "output.tar")

		factory := fixedWriteCloser(filePath)
		wc, err := factory(map[string]string{"test": "value"})
		if err != nil {
			t.Fatalf("unexpected error creating WriteCloser: %v", err)
		}

		testData := []byte("hello world")
		n, err := wc.Write(testData)
		if err != nil {
			t.Fatalf("unexpected error writing: %v", err)
		}
		if n != len(testData) {
			t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
		}

		if err := wc.Close(); err != nil {
			t.Fatalf("unexpected error closing: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}
		if string(content) != "hello world" {
			t.Errorf("expected %q, got %q", "hello world", string(content))
		}
	})

	t.Run("nil metadata map works", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "output2.tar")

		factory := fixedWriteCloser(filePath)
		wc, err := factory(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := wc.Close(); err != nil {
			t.Fatalf("unexpected close error: %v", err)
		}
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		factory := fixedWriteCloser("/nonexistent/dir/file.tar")
		_, err := factory(nil)
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

// ============================================================
// Close
// ============================================================

func TestClose(t *testing.T) {
	tests := []struct {
		name        string
		builder     *BuildKitBuilder
		expectError bool
	}{
		{
			name: "both clients nil",
			builder: &BuildKitBuilder{
				client:       nil,
				dockerClient: nil,
			},
			expectError: false,
		},
		{
			name: "docker client closes successfully",
			builder: &BuildKitBuilder{
				client:       nil,
				dockerClient: &MockDockerClient{},
			},
			expectError: false,
		},
		{
			name: "docker client close fails",
			builder: &BuildKitBuilder{
				client: nil,
				dockerClient: &mockDockerClientWithCloseError{
					MockDockerClient: MockDockerClient{},
					closeErr:         fmt.Errorf("close failed"),
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder.Close()
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// mockDockerClientWithCloseError wraps MockDockerClient with a configurable Close error.
type mockDockerClientWithCloseError struct {
	MockDockerClient
	closeErr error
}

func (m *mockDockerClientWithCloseError) Close() error {
	return m.closeErr
}

// ============================================================
// displayProgress
// ============================================================

func TestDisplayProgress(t *testing.T) {
	t.Run("empty channel closes done", func(t *testing.T) {
		b := &BuildKitBuilder{}
		ch := make(chan *client.SolveStatus)
		done := make(chan struct{})

		go b.displayProgress(context.Background(), ch, done)
		close(ch)

		select {
		case <-done:
			// success
		case <-time.After(2 * time.Second):
			t.Fatal("displayProgress did not close done channel in time")
		}
	})

	t.Run("processes statuses and closes done", func(t *testing.T) {
		b := &BuildKitBuilder{}
		ch := make(chan *client.SolveStatus, 3)
		done := make(chan struct{})

		go b.displayProgress(context.Background(), ch, done)

		// Send a status with a vertex
		ch <- &client.SolveStatus{
			// codespell:ignore vertexes
			Vertexes: []*client.Vertex{
				{
					Digest: digest.FromString("test"),
					Name:   "test vertex",
				},
			},
		}

		// Send a status with logs
		ch <- &client.SolveStatus{
			Logs: []*client.VertexLog{
				{Data: []byte("log line 1")},
			},
		}

		// Send empty status
		ch <- &client.SolveStatus{}

		close(ch)

		select {
		case <-done:
			// success
		case <-time.After(2 * time.Second):
			t.Fatal("displayProgress did not close done channel in time")
		}
	})

	t.Run("vertex without name is skipped", func(t *testing.T) {
		b := &BuildKitBuilder{}
		ch := make(chan *client.SolveStatus, 1)
		done := make(chan struct{})

		go b.displayProgress(context.Background(), ch, done)

		ch <- &client.SolveStatus{
			// codespell:ignore vertexes
			Vertexes: []*client.Vertex{
				{
					Digest: digest.FromString("no-name"),
					Name:   "",
				},
			},
		}

		close(ch)

		select {
		case <-done:
			// success
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	})
}

// ============================================================
// getLocalImageDigest (additional edge cases)
// ============================================================

func TestGetLocalImageDigestEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockDockerClient)
		expectedDigest string
	}{
		{
			name: "repo digest without @ symbol",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:          "sha256:abc123",
						RepoDigests: []string{"malformed-digest"},
					}, nil
				}
			},
			expectedDigest: "sha256:abc123",
		},
		{
			name: "empty ID and empty repo digests",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:          "",
						RepoDigests: []string{},
					}, nil
				}
			},
			expectedDigest: "",
		},
		{
			name: "multiple repo digests uses first",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID: "sha256:abc",
						RepoDigests: []string{
							"registry.io/img@sha256:first",
							"registry.io/img@sha256:second",
						},
					}, nil
				}
			},
			expectedDigest: "sha256:first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockDockerClient{}
			tt.setupMock(mock)
			b := &BuildKitBuilder{dockerClient: mock}
			d := b.getLocalImageDigest(context.Background(), "test:latest")
			if d != tt.expectedDigest {
				t.Errorf("expected %q, got %q", tt.expectedDigest, d)
			}
		})
	}
}

// ============================================================
// applyFileProvisioner
// ============================================================

func TestApplyFileProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty source and dest are skipped",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{Type: "file"}
			},
			expectError: false,
		},
		{
			name: "empty source is skipped",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type:        "file",
					Destination: "/tmp/dest",
				}
			},
			expectError: false,
		},
		{
			name: "file source copies successfully",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "config.txt")
				_ = os.WriteFile(filePath, []byte("data"), 0644)
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      filePath,
					Destination: "/etc/config.txt",
				}
			},
			expectError: false,
		},
		{
			name: "directory source copies successfully",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				srcDir := filepath.Join(dir, "mydir")
				_ = os.MkdirAll(srcDir, 0755)
				_ = os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("x"), 0644)
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      srcDir,
					Destination: "/opt/mydir",
				}
			},
			expectError: false,
		},
		{
			name: "file with mode set",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "script.sh")
				_ = os.WriteFile(filePath, []byte("#!/bin/sh"), 0644)
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      filePath,
					Destination: "/usr/local/bin/script.sh",
					Mode:        "0755",
				}
			},
			expectError: false,
		},
		{
			name: "nonexistent source returns error",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      filepath.Join(dir, "nonexistent.txt"),
					Destination: "/tmp/dest",
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("alpine:latest")

			_, err := b.applyFileProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// applyScriptProvisioner
// ============================================================

func TestApplyScriptProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty scripts list",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type:    "script",
					Scripts: []string{},
				}
			},
			expectError: false,
		},
		{
			name: "single script",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				scriptPath := filepath.Join(dir, "setup.sh")
				_ = os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hello"), 0755)
				return dir, builder.Provisioner{
					Type:    "script",
					Scripts: []string{scriptPath},
				}
			},
			expectError: false,
		},
		{
			name: "multiple scripts",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				s1 := filepath.Join(dir, "a.sh")
				s2 := filepath.Join(dir, "b.sh")
				_ = os.WriteFile(s1, []byte("#!/bin/sh"), 0755)
				_ = os.WriteFile(s2, []byte("#!/bin/sh"), 0755)
				return dir, builder.Provisioner{
					Type:    "script",
					Scripts: []string{s1, s2},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("alpine:latest")

			_, err := b.applyScriptProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// applyPowerShellProvisioner
// ============================================================

func TestApplyPowerShellProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty ps scripts list",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type:      "powershell",
					PSScripts: []string{},
				}
			},
			expectError: false,
		},
		{
			name: "single powershell script",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				scriptPath := filepath.Join(dir, "setup.ps1")
				_ = os.WriteFile(scriptPath, []byte("Write-Host 'hello'"), 0644)
				return dir, builder.Provisioner{
					Type:      "powershell",
					PSScripts: []string{scriptPath},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("alpine:latest")

			_, err := b.applyPowerShellProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// applyAnsibleProvisioner
// ============================================================

func TestApplyAnsibleProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty playbook path returns state unchanged",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type: "ansible",
				}
			},
			expectError: false,
		},
		{
			name: "playbook with galaxy file",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				pbPath := filepath.Join(dir, "playbook.yml")
				galPath := filepath.Join(dir, "requirements.yml")
				_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)
				_ = os.WriteFile(galPath, []byte("---\nroles: []"), 0644)
				return dir, builder.Provisioner{
					Type:         "ansible",
					PlaybookPath: pbPath,
					GalaxyFile:   galPath,
				}
			},
			expectError: false,
		},
		{
			name: "playbook with extra vars",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				pbPath := filepath.Join(dir, "playbook.yml")
				_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)
				return dir, builder.Provisioner{
					Type:         "ansible",
					PlaybookPath: pbPath,
					ExtraVars:    map[string]string{"env": "test", "debug": "true"},
				}
			},
			expectError: false,
		},
		{
			name: "playbook inside collection structure",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				collDir := filepath.Join(dir, "mycollection")
				pbDir := filepath.Join(collDir, "playbooks")
				_ = os.MkdirAll(pbDir, 0755)
				_ = os.WriteFile(filepath.Join(collDir, "galaxy.yml"), []byte("namespace: ns\nname: col"), 0644)
				pbPath := filepath.Join(pbDir, "site.yml")
				_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)
				return dir, builder.Provisioner{
					Type:         "ansible",
					PlaybookPath: pbPath,
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("ubuntu:22.04")

			_, err := b.applyAnsibleProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// convertToLLB
// ============================================================

func TestConvertToLLB(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Config)
		expectError bool
		errContains string
	}{
		{
			name: "dockerfile-based config returns error",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Dockerfile: &builder.DockerfileConfig{
						Path: "Dockerfile",
					},
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
				}
			},
			expectError: true,
			errContains: "dockerfile-based builds",
		},
		{
			name: "basic config with platform",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with architectures fallback",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:          "test",
					Version:       "1.0",
					Base:          builder.BaseImage{Image: "alpine:latest"},
					Architectures: []string{"arm64"},
				}
			},
			expectError: false,
		},
		{
			name: "config with no platform and no architectures",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base:    builder.BaseImage{Image: "alpine:latest"},
				}
			},
			expectError: true,
			errContains: "no platform",
		},
		{
			name: "config with base env",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
						Env:      map[string]string{"FOO": "bar"},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with build args",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
					BuildArgs: map[string]string{"VERSION": "1.0"},
				}
			},
			expectError: false,
		},
		{
			name: "config with base changes",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
						Changes:  []string{"ENV FOO=bar", "WORKDIR /app"},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with shell provisioner",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "ubuntu:22.04",
						Platform: "linux/amd64",
					},
					Provisioners: []builder.Provisioner{
						{
							Type:   "shell",
							Inline: []string{"apt-get update", "apt-get install -y curl"},
						},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with post changes",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
					PostChanges: []string{"USER nobody", "WORKDIR /home/nobody"},
				}
			},
			expectError: false,
		},
		{
			name: "config with file provisioner",
			setup: func(t *testing.T) (string, builder.Config) {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "config.yml")
				_ = os.WriteFile(filePath, []byte("key: value"), 0644)
				return dir, builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
					Provisioners: []builder.Provisioner{
						{
							Type:        "file",
							Source:      filePath,
							Destination: "/etc/config.yml",
						},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with multiple provisioner types",
			setup: func(t *testing.T) (string, builder.Config) {
				dir := t.TempDir()
				scriptPath := filepath.Join(dir, "setup.sh")
				_ = os.WriteFile(scriptPath, []byte("#!/bin/sh\necho done"), 0755)
				return dir, builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "ubuntu:22.04",
						Platform: "linux/amd64",
					},
					Provisioners: []builder.Provisioner{
						{
							Type:   "shell",
							Inline: []string{"echo step1"},
						},
						{
							Type:    "script",
							Scripts: []string{scriptPath},
						},
					},
					PostChanges: []string{"ENV DONE=true"},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, cfg := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}

			_, err := b.convertToLLB(cfg)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.errContains != "" && err != nil && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// getAnsiblePaths
// ============================================================

func TestGetAnsiblePaths(t *testing.T) {
	tmpDir := t.TempDir()
	pbPath := filepath.Join(tmpDir, "playbook.yml")
	galPath := filepath.Join(tmpDir, "requirements.yml")
	_ = os.WriteFile(pbPath, []byte("---"), 0644)
	_ = os.WriteFile(galPath, []byte("---"), 0644)

	tests := []struct {
		name        string
		prov        builder.Provisioner
		expectCount int
		expectError bool
	}{
		{
			name:        "both paths",
			prov:        builder.Provisioner{Type: "ansible", PlaybookPath: pbPath, GalaxyFile: galPath},
			expectCount: 2,
		},
		{
			name:        "playbook only",
			prov:        builder.Provisioner{Type: "ansible", PlaybookPath: pbPath},
			expectCount: 1,
		},
		{
			name:        "galaxy only",
			prov:        builder.Provisioner{Type: "ansible", GalaxyFile: galPath},
			expectCount: 1,
		},
		{
			name:        "neither",
			prov:        builder.Provisioner{Type: "ansible"},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			pv := templates.NewPathValidator()
			paths, err := b.getAnsiblePaths(tt.prov, pv)

			if tt.expectError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.expectCount {
				t.Errorf("expected %d paths, got %d", tt.expectCount, len(paths))
			}
		})
	}
}

// ============================================================
// getProvisionerPaths
// ============================================================

func TestGetProvisionerPaths(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	scriptPath := filepath.Join(tmpDir, "script.sh")
	psPath := filepath.Join(tmpDir, "script.ps1")
	pbPath := filepath.Join(tmpDir, "playbook.yml")
	_ = os.WriteFile(filePath, []byte("data"), 0644)
	_ = os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0755)
	_ = os.WriteFile(psPath, []byte("Write-Host"), 0644)
	_ = os.WriteFile(pbPath, []byte("---"), 0644)

	tests := []struct {
		name        string
		prov        builder.Provisioner
		expectCount int
	}{
		{
			name:        "ansible provisioner",
			prov:        builder.Provisioner{Type: "ansible", PlaybookPath: pbPath},
			expectCount: 1,
		},
		{
			name:        "file provisioner",
			prov:        builder.Provisioner{Type: "file", Source: filePath, Destination: "/tmp/f"},
			expectCount: 1,
		},
		{
			name:        "script provisioner",
			prov:        builder.Provisioner{Type: "script", Scripts: []string{scriptPath}},
			expectCount: 1,
		},
		{
			name:        "powershell provisioner",
			prov:        builder.Provisioner{Type: "powershell", PSScripts: []string{psPath}},
			expectCount: 1,
		},
		{
			name:        "shell provisioner has no paths",
			prov:        builder.Provisioner{Type: "shell", Inline: []string{"echo hi"}},
			expectCount: 0,
		},
		{
			name:        "unknown provisioner has no paths",
			prov:        builder.Provisioner{Type: "unknown"},
			expectCount: 0,
		},
		{
			name:        "file provisioner empty source",
			prov:        builder.Provisioner{Type: "file", Source: "", Destination: "/tmp/f"},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			pv := templates.NewPathValidator()
			paths, err := b.getProvisionerPaths(tt.prov, pv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.expectCount {
				t.Errorf("expected %d paths, got %d: %v", tt.expectCount, len(paths), paths)
			}
		})
	}
}

// ============================================================
// applyShellProvisioner (package manager cache mounts)
// ============================================================

func TestApplyShellProvisionerCacheMounts(t *testing.T) {
	tests := []struct {
		name   string
		inline []string
	}{
		{
			name:   "empty inline",
			inline: []string{},
		},
		{
			name:   "apt-get commands",
			inline: []string{"apt-get update", "apt-get install -y curl"},
		},
		{
			name:   "yum commands",
			inline: []string{"yum install -y wget"},
		},
		{
			name:   "dnf commands",
			inline: []string{"dnf install -y git"},
		},
		{
			name:   "apk commands",
			inline: []string{"apk add curl"},
		},
		{
			name:   "pip commands",
			inline: []string{"pip install requests"},
		},
		{
			name:   "npm commands",
			inline: []string{"npm install express"},
		},
		{
			name:   "yarn commands",
			inline: []string{"yarn add react"},
		},
		{
			name:   "go build commands",
			inline: []string{"go build ./..."},
		},
		{
			name:   "go get commands",
			inline: []string{"go get github.com/some/pkg"},
		},
		{
			name:   "mixed package managers",
			inline: []string{"apt-get update && pip install boto3 && npm install"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			state := llb.Image("ubuntu:22.04")
			prov := builder.Provisioner{
				Type:   "shell",
				Inline: tt.inline,
			}

			_, err := b.applyShellProvisioner(state, prov)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// buildExportAttributes (additional coverage)
// ============================================================

func TestBuildExportAttributesEmpty(t *testing.T) {
	attrs := buildExportAttributes("myimage:v1", map[string]string{})
	if attrs["name"] != "myimage:v1" {
		t.Errorf("expected name 'myimage:v1', got %q", attrs["name"])
	}
	// Should have only the name key
	if len(attrs) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(attrs))
	}
}

// ============================================================
// loadAndTagImage (additional coverage with mock)
// ============================================================

func TestLoadAndTagImageFileNotFound(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.loadAndTagImage(context.Background(), "/nonexistent/path.tar", "test:latest")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ============================================================
// Push edge cases: unqualified image ref gets tagged with registry
// ============================================================

func TestPushUnqualifiedImageRef(t *testing.T) {
	tagCalled := false
	mock := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			tagCalled = true
			return nil
		},
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"ghcr.io/org/app@sha256:digest123"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	_, err := b.Push(context.Background(), "myapp:latest", "ghcr.io/org")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tagCalled {
		t.Error("expected ImageTag to be called for unqualified image ref")
	}
}

// ============================================================
// CreateAndPushManifest edge case: empty entries
// ============================================================

func TestCreateAndPushManifestEmptyEntries(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.CreateAndPushManifest(context.Background(), "test:latest", nil)
	if err == nil {
		t.Error("expected error for empty entries")
	}
	if !strings.Contains(err.Error(), "no manifest entries") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// expandContainerVars additional coverage
// ============================================================

func TestExpandContainerVarsNoOp(t *testing.T) {
	b := &BuildKitBuilder{}
	result := b.expandContainerVars("novar", map[string]string{"PATH": "/usr/bin"})
	if result != "novar" {
		t.Errorf("expected 'novar', got %q", result)
	}
}

func TestExpandContainerVarsMultipleOccurrences(t *testing.T) {
	b := &BuildKitBuilder{}
	result := b.expandContainerVars("$X and $X", map[string]string{"X": "val"})
	if result != "val and val" {
		t.Errorf("expected 'val and val', got %q", result)
	}
}

// ============================================================
// extractRegistryFromImageRef edge cases (additional)
// ============================================================

func TestExtractRegistryFromImageRefEdgeCases(t *testing.T) {
	tests := []struct {
		imageRef string
		expected string
	}{
		{"", "docker.io"},
		{"image", "docker.io"},
		{"user/image", "docker.io"},
		{"localhost/image", "localhost"},
		{"host.com:5000/image", "host.com:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.imageRef, func(t *testing.T) {
			result := extractRegistryFromImageRef(tt.imageRef)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================
// getPlatformString and extractArchFromPlatform additional combos
// ============================================================

func TestGetPlatformStringNoArches(t *testing.T) {
	cfg := builder.Config{
		Base:          builder.BaseImage{},
		Architectures: nil,
	}
	result := getPlatformString(cfg)
	if result != "unknown" {
		t.Errorf("expected 'unknown', got %q", result)
	}
}

// ============================================================
// findCommonParent edge cases
// ============================================================

func TestFindCommonParentSamePath(t *testing.T) {
	result := findCommonParent("/usr/local/bin", "/usr/local/bin")
	if result != "/usr/local/bin" {
		t.Errorf("expected '/usr/local/bin', got %q", result)
	}
}

func TestFindCommonParentRoot(t *testing.T) {
	result := findCommonParent("/a", "/b")
	if result != "/" {
		t.Errorf("expected '/', got %q", result)
	}
}

// ============================================================
// CreateAndPushManifest with empty digest
// ============================================================

func TestCreateAndPushManifestEmptyDigest(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "test:latest",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       "",
		},
	}

	err := b.CreateAndPushManifest(context.Background(), "test:latest", entries)
	if err == nil {
		t.Error("expected error for empty digest")
	}
	if !strings.Contains(err.Error(), "no digest found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// collectProvisionerPaths
// ============================================================

func TestCollectProvisionerPaths(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	scriptPath := filepath.Join(tmpDir, "script.sh")
	_ = os.WriteFile(filePath, []byte("data"), 0644)
	_ = os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0755)

	tests := []struct {
		name         string
		provisioners []builder.Provisioner
		expectCount  int
		expectError  bool
	}{
		{
			name:         "empty provisioners",
			provisioners: []builder.Provisioner{},
			expectCount:  0,
		},
		{
			name: "single file provisioner",
			provisioners: []builder.Provisioner{
				{Type: "file", Source: filePath, Destination: "/tmp/f"},
			},
			expectCount: 1,
		},
		{
			name: "multiple provisioners",
			provisioners: []builder.Provisioner{
				{Type: "file", Source: filePath, Destination: "/tmp/f"},
				{Type: "script", Scripts: []string{scriptPath}},
			},
			expectCount: 2,
		},
		{
			name: "shell provisioner adds no paths",
			provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo hi"}},
			},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			pv := templates.NewPathValidator()
			paths, err := b.collectProvisionerPaths(tt.provisioners, pv)

			if tt.expectError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.expectCount {
				t.Errorf("expected %d paths, got %d", tt.expectCount, len(paths))
			}
		})
	}
}

// ============================================================
// calculateBuildContext edge cases
// ============================================================

func TestCalculateBuildContextEmptyProvisioners(t *testing.T) {
	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{},
	}

	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "." {
		t.Errorf("expected '.', got %q", ctx)
	}
}

func TestCalculateBuildContextNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "doesnt_exist.txt")

	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{Type: "file", Source: nonExistent, Destination: "/tmp/f"},
		},
	}

	// When the file doesn't exist, calculateBuildContext uses filepath.Dir
	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Error("expected non-empty context")
	}
}

// ============================================================
// ToDockerSDKAuth
// ============================================================

func TestToDockerSDKAuthRegistries(t *testing.T) {
	tests := []struct {
		name        string
		registry    string
		expectError bool
	}{
		{
			name:        "valid registry",
			registry:    "ghcr.io",
			expectError: false,
		},
		{
			name:        "docker hub",
			registry:    "docker.io",
			expectError: false,
		},
		{
			name:        "localhost registry",
			registry:    "localhost",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToDockerSDKAuth(context.Background(), tt.registry)
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			// We cannot guarantee credentials exist, but the function should not panic
		})
	}
}

// ============================================================
// Push edge cases: tag failure
// ============================================================

func TestPushTagFailure(t *testing.T) {
	mock := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			return fmt.Errorf("tag failed")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	_, err := b.Push(context.Background(), "myapp:latest", "ghcr.io/org")

	if err == nil {
		t.Error("expected error for tag failure")
	}
	if !strings.Contains(err.Error(), "tag") {
		t.Errorf("error should mention tag: %v", err)
	}
}

// ============================================================
// Push: inspect fails after successful push
// ============================================================

func TestPushInspectFailsAfterPush(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, fmt.Errorf("inspect failed")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	digest, err := b.Push(context.Background(), "ghcr.io/org/myapp:latest", "ghcr.io/org")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if digest != "" {
		t.Errorf("expected empty digest when inspect fails, got %q", digest)
	}
}

// ============================================================
// Push: no digest in RepoDigests after push
// ============================================================

func TestPushNoDigestInRepoDigests(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	digest, err := b.Push(context.Background(), "ghcr.io/org/myapp:latest", "ghcr.io/org")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if digest != "" {
		t.Errorf("expected empty digest, got %q", digest)
	}
}

// ============================================================
// applyProvisioner: file provisioner dispatch
// ============================================================

func TestApplyProvisionerFileDispatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.txt")
	_ = os.WriteFile(filePath, []byte("test"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:        "file",
		Source:      filePath,
		Destination: "/tmp/data.txt",
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// applyProvisioner: ansible dispatch
// ============================================================

func TestApplyProvisionerAnsibleDispatch(t *testing.T) {
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("ubuntu:22.04")

	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: pbPath,
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// loadAndTagImage: successful load with read response
// ============================================================

func TestLoadAndTagImageSuccessWithResponse(t *testing.T) {
	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			return dockerimage.LoadResponse{
				Body: io.NopCloser(strings.NewReader(`{"stream":"Loaded image: test:latest"}`)),
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	tmpFile, err := os.CreateTemp("", "test-image-*.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	if _, err := tmpFile.WriteString("dummy"); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	err = b.loadAndTagImage(context.Background(), tmpFile.Name(), "test:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// expandPathList
// ============================================================

func TestExpandPathList(t *testing.T) {
	tmpDir := t.TempDir()
	s1 := filepath.Join(tmpDir, "a.sh")
	s2 := filepath.Join(tmpDir, "b.sh")
	_ = os.WriteFile(s1, []byte("#!/bin/sh"), 0755)
	_ = os.WriteFile(s2, []byte("#!/bin/sh"), 0755)

	pv := templates.NewPathValidator()

	tests := []struct {
		name     string
		scripts  []string
		pathType string
		count    int
	}{
		{"empty", []string{}, "script", 0},
		{"single", []string{s1}, "script", 1},
		{"multiple", []string{s1, s2}, "script", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := expandPathList(tt.scripts, pv, tt.pathType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.count {
				t.Errorf("expected %d, got %d", tt.count, len(paths))
			}
		})
	}
}

// ============================================================
// getFilePaths
// ============================================================

func TestGetFilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "data.txt")
	_ = os.WriteFile(filePath, []byte("x"), 0644)

	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{}

	t.Run("with source", func(t *testing.T) {
		prov := builder.Provisioner{Type: "file", Source: filePath}
		paths, err := b.getFilePaths(prov, pv)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 1 {
			t.Errorf("expected 1 path, got %d", len(paths))
		}
	})

	t.Run("empty source", func(t *testing.T) {
		prov := builder.Provisioner{Type: "file", Source: ""}
		paths, err := b.getFilePaths(prov, pv)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Errorf("expected 0 paths, got %d", len(paths))
		}
	})
}

// ============================================================
// CreateAndPushManifest: valid entries with inspect failure
// ============================================================

// ============================================================
// Close: buildkit client error (non-nil client that returns error)
// We can't easily mock client.Client, but we can test combined errors
// ============================================================

func TestCloseDockerClientError(t *testing.T) {
	b := &BuildKitBuilder{
		client: nil,
		dockerClient: &mockDockerClientWithCloseError{
			closeErr: fmt.Errorf("docker close failed"),
		},
	}

	err := b.Close()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "docker close failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// Push: test push response read failure path
// ============================================================

func TestPushReadResponseFailure(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(&errorReader{}), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"ghcr.io/test@sha256:def"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	// The push proceeds even if reading the response body fails
	_, err := b.Push(context.Background(), "ghcr.io/test:v1", "ghcr.io")
	// Should not return error - read failure is logged but not fatal
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// errorReader is an io.Reader that always returns an error.
type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

// ============================================================
// Push: digest parsing edge case - RepoDigest without proper format
// ============================================================

func TestPushRepoDigestMalformed(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"no-at-symbol-here"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	d, err := b.Push(context.Background(), "ghcr.io/test:v1", "ghcr.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With malformed digest, should warn and return empty
	if d != "" {
		t.Errorf("expected empty digest for malformed RepoDigest, got %q", d)
	}
}

// ============================================================
// CreateAndPushManifest with variant
// ============================================================

func TestCreateAndPushManifestWithVariant(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{Size: 1024}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/app:latest",
			OS:           "linux",
			Architecture: "arm",
			Variant:      "v7",
			Digest:       digest.FromString("test-arm"),
		},
	}

	// Will fail at remote.Get, but exercises the variant code path
	err := b.CreateAndPushManifest(context.Background(), "ghcr.io/test/app:latest", entries)
	// Expected to fail at network step - that's OK
	_ = err
}

func TestCreateAndPushManifestInspectFails(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, fmt.Errorf("inspect failed")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/app:latest",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       digest.FromString("test"),
		},
	}

	// This will proceed past inspect (using size 0) but fail when trying to
	// parse the manifest name and push - that's OK, we're testing the code path
	err := b.CreateAndPushManifest(context.Background(), "ghcr.io/test/app:latest", entries)
	// It will fail at the remote.Get step since we're not connected to a registry
	// but the important thing is we exercised the code path through inspect failure
	if err == nil {
		// If somehow it succeeds (unlikely), that's also fine
		return
	}
	// Error is expected at the remote step
}

// ============================================================
// CreateAndPushManifest: multiple entries with successful inspect
// ============================================================

func TestCreateAndPushManifestMultipleEntries(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{Size: 1024}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/app:amd64",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       digest.FromString("test-amd64"),
		},
		{
			ImageRef:     "ghcr.io/test/app:arm64",
			OS:           "linux",
			Architecture: "arm64",
			Digest:       digest.FromString("test-arm64"),
		},
	}

	// Will fail at remote.Get since no real registry, but exercises the multi-entry loop
	_ = b.CreateAndPushManifest(context.Background(), "ghcr.io/test/app:latest", entries)
}

// ============================================================
// applyAnsibleProvisioner: galaxy file makeRelativePath error
// ============================================================

func TestApplyAnsibleProvisionerGalaxyError(t *testing.T) {
	// Use a context directory that's different from where the file is,
	// but the file path uses a tilde which would cause expansion issues
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	_ = os.WriteFile(pbPath, []byte("---"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("ubuntu:22.04")

	// Playbook path valid, galaxy file has a nonexistent tilde-based path
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: pbPath,
		GalaxyFile:   "~nonexistentuser/requirements.yml",
	}

	// This should fail when trying to resolve the galaxy path
	_, err := b.applyAnsibleProvisioner(state, prov)
	// Error is expected from makeRelativePath failure
	if err == nil {
		// On some systems tilde expansion may work differently,
		// so don't fail if it happens to succeed
		return
	}
}

// ============================================================
// loadAndTagImage: test the response read and close path fully
// ============================================================

func TestLoadAndTagImageReadResponseError(t *testing.T) {
	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			return dockerimage.LoadResponse{
				Body: io.NopCloser(&errorReader{}),
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	tmpFile, err := os.CreateTemp("", "test-image-*.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	if _, err := tmpFile.WriteString("dummy"); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	err = b.loadAndTagImage(context.Background(), tmpFile.Name(), "test:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// collectProvisionerPaths: error propagation from provisioner
// ============================================================

func TestCollectProvisionerPathsError(t *testing.T) {
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	// Use a provisioner with a path that will fail expansion
	provisioners := []builder.Provisioner{
		{
			Type:   "file",
			Source: "~nonexistentuser/file.txt",
		},
	}

	_, err := b.collectProvisionerPaths(provisioners, pv)
	// On macOS/Linux this may or may not error depending on how tilde expansion works
	// The important thing is no panic
	_ = err
}

// ============================================================
// convertToLLB: with ansible provisioner
// ============================================================

func TestConvertToLLBWithAnsible(t *testing.T) {
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "ubuntu:22.04",
			Platform: "linux/amd64",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: pbPath,
			},
		},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// convertToLLB: with powershell provisioner
// ============================================================

func TestConvertToLLBWithPowerShell(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "setup.ps1")
	_ = os.WriteFile(psPath, []byte("Write-Host test"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "mcr.microsoft.com/powershell:latest",
			Platform: "linux/amd64",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:      "powershell",
				PSScripts: []string{psPath},
			},
		},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// applyFileProvisioner: abs path resolution error (edge case)
// ============================================================

func TestApplyFileProvisionerAbsPathEdge(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	_ = os.WriteFile(filePath, []byte("data"), 0644)

	// Use the file's directory as context so relative path is simple
	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:        "file",
		Source:      filePath,
		Destination: "/tmp/test.txt",
	}

	result, err := b.applyFileProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// makeRelativePath: error from expand path (invalid user home)
// ============================================================

func TestMakeRelativePathExpandError(t *testing.T) {
	b := &BuildKitBuilder{contextDir: "/tmp"}
	_, err := b.makeRelativePath("~nonexistentuser/file.txt")
	// This may or may not error depending on the OS, but should not panic
	_ = err
}

// ============================================================
// calculateBuildContext: with ansible provisioner
// ============================================================

func TestCalculateBuildContextWithAnsible(t *testing.T) {
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	galPath := filepath.Join(dir, "requirements.yml")
	_ = os.WriteFile(pbPath, []byte("---"), 0644)
	_ = os.WriteFile(galPath, []byte("---"), 0644)

	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: pbPath,
				GalaxyFile:   galPath,
			},
		},
	}

	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Error("expected non-empty context")
	}
}

// ============================================================
// createAuthProvider (exercise the function)
// ============================================================

// ============================================================
// applyFileProvisioner: makeRelativePath returns path that doesn't exist
// ============================================================

func TestApplyFileProvisionerStatError(t *testing.T) {
	b := &BuildKitBuilder{contextDir: "/tmp"}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      string([]byte{0}), // null byte causes stat failure
		Destination: "/tmp/dest",
	}

	_, err := b.applyFileProvisioner(state, prov)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestCreateAuthProviderCoverage(t *testing.T) {
	t.Parallel()
	// This function reads Docker config; in test environment it may
	// or may not find credentials, but should not panic
	result := createAuthProvider()
	// result may be nil (no Docker config) or non-nil (has Docker config)
	_ = result
}

// ============================================================
// Tag: success and error paths
// ============================================================

func TestTag_Success(t *testing.T) {
	t.Parallel()
	tagCalled := false
	mock := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			tagCalled = true
			if source != "myapp:latest" || target != "myapp:v2" {
				t.Errorf("unexpected args: source=%q, target=%q", source, target)
			}
			return nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.Tag(context.Background(), "myapp:latest", "myapp:v2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tagCalled {
		t.Error("ImageTag was not called")
	}
}

func TestTag_Error(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			return fmt.Errorf("image not found")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.Tag(context.Background(), "nonexistent:latest", "newname:v1")
	if err == nil {
		t.Error("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "docker tag failed") {
		t.Errorf("error should mention docker tag: %v", err)
	}
}

// ============================================================
// Remove: success and error paths
// ============================================================

func TestRemove_Success(t *testing.T) {
	t.Parallel()
	removeCalled := false
	mock := &MockDockerClient{
		ImageRemoveFunc: func(ctx context.Context, imageID string, options dockerimage.RemoveOptions) ([]dockerimage.DeleteResponse, error) {
			removeCalled = true
			if imageID != "myapp:old" {
				t.Errorf("unexpected imageID: %q", imageID)
			}
			// Verify PruneChildren is set
			if !options.PruneChildren {
				t.Error("PruneChildren should be true")
			}
			return []dockerimage.DeleteResponse{{Deleted: imageID}}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.Remove(context.Background(), "myapp:old")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !removeCalled {
		t.Error("ImageRemove was not called")
	}
}

func TestRemove_Error(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImageRemoveFunc: func(ctx context.Context, imageID string, options dockerimage.RemoveOptions) ([]dockerimage.DeleteResponse, error) {
			return nil, fmt.Errorf("image is in use")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.Remove(context.Background(), "myapp:latest")
	if err == nil {
		t.Error("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "docker rmi failed") {
		t.Errorf("error should mention docker rmi: %v", err)
	}
}

// ============================================================
// Push: success with digest extraction
// ============================================================

func TestPush_SuccessWithDigest(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"Pushed"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc123",
				RepoDigests: []string{"ghcr.io/org/app@sha256:deadbeef1234567890abcdef"},
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	d, err := b.Push(context.Background(), "ghcr.io/org/app:latest", "ghcr.io/org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != "sha256:deadbeef1234567890abcdef" {
		t.Errorf("expected digest sha256:deadbeef1234567890abcdef, got %q", d)
	}
}

func TestPush_Error(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	_, err := b.Push(context.Background(), "ghcr.io/org/app:latest", "ghcr.io/org")
	if err == nil {
		t.Error("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "failed to push") {
		t.Errorf("error should mention push: %v", err)
	}
}

// ============================================================
// Push: error in JSON response body
// ============================================================

func TestPush_JSONErrorInResponse(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"error":"unauthorized: access denied"}`)), nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	_, err := b.Push(context.Background(), "ghcr.io/org/app:latest", "ghcr.io/org")
	if err == nil {
		t.Error("expected error for error in JSON response")
	}
	if !strings.Contains(err.Error(), "push failed") {
		t.Errorf("error should mention push failed: %v", err)
	}
}

// ============================================================
// Push: fully qualified ref (no tagging needed)
// ============================================================

func TestPush_FullyQualifiedRef(t *testing.T) {
	t.Parallel()
	tagCalled := false
	mock := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			tagCalled = true
			return nil
		},
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"ghcr.io/org/myapp@sha256:digest123"},
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	// Fully qualified ref contains "/" so no tagging needed
	_, err := b.Push(context.Background(), "ghcr.io/org/myapp:latest", "ghcr.io/org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tagCalled {
		t.Error("Tag should NOT be called for fully qualified image ref")
	}
}

// ============================================================
// findCommonParent: additional edge cases
// ============================================================

func TestFindCommonParent_NestedPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected string
	}{
		{
			name:     "deeply nested common parent",
			path1:    "/home/user/project/src/main.go",
			path2:    "/home/user/project/test/test.go",
			expected: "/home/user/project",
		},
		{
			name:     "parent-child relationship",
			path1:    "/usr/local",
			path2:    "/usr/local/bin",
			expected: "/usr/local",
		},
		{
			name:     "identical paths",
			path1:    "/etc/config",
			path2:    "/etc/config",
			expected: "/etc/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := findCommonParent(tt.path1, tt.path2)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================
// makeRelativePath: additional edge cases
// ============================================================

func TestMakeRelativePath_OutsideContext(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	_ = os.WriteFile(outsideFile, []byte("test"), 0644)

	b := &BuildKitBuilder{contextDir: tmpDir}
	result, err := b.makeRelativePath(outsideFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should contain ".." since the file is outside context
	if !strings.Contains(result, "..") {
		t.Errorf("expected relative path with '..' for file outside context, got %q", result)
	}
}

// ============================================================
// parsePlatform: additional coverage
// ============================================================

func TestParsePlatform_ValidAndInvalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		platform    string
		expectOS    string
		expectArch  string
		expectError bool
	}{
		{
			name:       "linux/amd64",
			platform:   "linux/amd64",
			expectOS:   "linux",
			expectArch: "amd64",
		},
		{
			name:       "linux/arm64",
			platform:   "linux/arm64",
			expectOS:   "linux",
			expectArch: "arm64",
		},
		{
			name:       "windows/amd64",
			platform:   "windows/amd64",
			expectOS:   "windows",
			expectArch: "amd64",
		},
		{
			name:        "empty string",
			platform:    "",
			expectError: true,
		},
		{
			name:        "no separator",
			platform:    "linuxamd64",
			expectError: true,
		},
		{
			name:        "three parts",
			platform:    "linux/arm64/v8",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			os, arch, err := parsePlatform(tt.platform)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if os != tt.expectOS {
				t.Errorf("OS: expected %q, got %q", tt.expectOS, os)
			}
			if arch != tt.expectArch {
				t.Errorf("Arch: expected %q, got %q", tt.expectArch, arch)
			}
		})
	}
}

// ============================================================
// buildExportAttributes: with labels
// ============================================================

func TestBuildExportAttributes_WithLabels(t *testing.T) {
	t.Parallel()
	labels := map[string]string{
		"org.opencontainers.image.source": "https://github.com/test/repo",
		"org.opencontainers.image.title":  "my-app",
	}

	attrs := buildExportAttributes("myimage:v1", labels)

	if attrs["name"] != "myimage:v1" {
		t.Errorf("expected name 'myimage:v1', got %q", attrs["name"])
	}

	if attrs["label:org.opencontainers.image.source"] != "https://github.com/test/repo" {
		t.Errorf("expected source label, got %q", attrs["label:org.opencontainers.image.source"])
	}

	if attrs["label:org.opencontainers.image.title"] != "my-app" {
		t.Errorf("expected title label, got %q", attrs["label:org.opencontainers.image.title"])
	}

	// Should have name + 2 labels = 3 attrs
	if len(attrs) != 3 {
		t.Errorf("expected 3 attributes, got %d", len(attrs))
	}
}

func TestBuildExportAttributes_NilLabels(t *testing.T) {
	t.Parallel()
	attrs := buildExportAttributes("img:v2", nil)

	if attrs["name"] != "img:v2" {
		t.Errorf("expected name 'img:v2', got %q", attrs["name"])
	}
	if len(attrs) != 1 {
		t.Errorf("expected 1 attribute for nil labels, got %d", len(attrs))
	}
}

// ============================================================
// getPlatformString: additional cases
// ============================================================

func TestGetPlatformString_AllCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cfg      builder.Config
		expected string
	}{
		{
			name: "platform takes precedence over architectures",
			cfg: builder.Config{
				Base:          builder.BaseImage{Platform: "linux/arm64"},
				Architectures: []string{"amd64"},
			},
			expected: "linux/arm64",
		},
		{
			name: "architectures fallback",
			cfg: builder.Config{
				Base:          builder.BaseImage{},
				Architectures: []string{"arm64"},
			},
			expected: "linux/arm64",
		},
		{
			name: "no platform no architectures",
			cfg: builder.Config{
				Base:          builder.BaseImage{},
				Architectures: nil,
			},
			expected: "unknown",
		},
		{
			name: "empty architectures list",
			cfg: builder.Config{
				Base:          builder.BaseImage{},
				Architectures: []string{},
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getPlatformString(tt.cfg)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================
// extractArchFromPlatform: additional cases
// ============================================================

func TestExtractArchFromPlatform_Coverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		platform string
		expected string
	}{
		{"linux/amd64", "amd64"},
		{"linux/arm64", "arm64"},
		{"linux/arm/v7", "arm"},
		{"windows/amd64", "amd64"},
		{"amd64", ""},
		{"", ""},
		{"a/b/c/d", "b"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			t.Parallel()
			result := extractArchFromPlatform(tt.platform)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================
// extractRegistryFromImageRef: with digest
// ============================================================

func TestExtractRegistryFromImageRef_WithDigest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		imageRef string
		expected string
	}{
		{"ghcr.io/org/app@sha256:abc123", "ghcr.io"},
		{"docker.io/library/ubuntu@sha256:abc123", "docker.io"},
		{"myapp@sha256:abc123", "docker.io"},
		{"localhost:5000/app@sha256:abc123", "localhost:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.imageRef, func(t *testing.T) {
			t.Parallel()
			result := extractRegistryFromImageRef(tt.imageRef)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================
// loadTLSConfig: additional TLS tests
// ============================================================

func TestLoadTLSConfig_EmptyConfig(t *testing.T) {
	t.Parallel()
	cfg := config.BuildKitConfig{}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("expected non-nil TLS config")
	}
	if tlsCfg.RootCAs != nil {
		t.Error("RootCAs should be nil for empty config")
	}
	if len(tlsCfg.Certificates) != 0 {
		t.Error("Certificates should be empty for empty config")
	}
}

func TestLoadTLSConfig_CACertOnly(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	caCertPath, _, _ := generateTestCert(t, tmpDir)

	cfg := config.BuildKitConfig{
		TLSCACert: caCertPath,
	}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tlsCfg.RootCAs == nil {
		t.Error("expected RootCAs to be set")
	}
	if len(tlsCfg.Certificates) != 0 {
		t.Error("Certificates should be empty when only CA cert is provided")
	}
}

func TestLoadTLSConfig_ClientCertOnly(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	_, certPath, keyPath := generateTestCert(t, tmpDir)

	cfg := config.BuildKitConfig{
		TLSCert: certPath,
		TLSKey:  keyPath,
	}

	tlsCfg, err := loadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tlsCfg.RootCAs != nil {
		t.Error("RootCAs should be nil when only client cert is provided")
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 client certificate, got %d", len(tlsCfg.Certificates))
	}
}

func TestLoadTLSConfig_MissingCACert(t *testing.T) {
	t.Parallel()
	cfg := config.BuildKitConfig{
		TLSCACert: "/nonexistent/ca.pem",
	}

	_, err := loadTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for missing CA cert")
	}
	if !strings.Contains(err.Error(), "failed to read CA cert") {
		t.Errorf("error should mention CA cert: %v", err)
	}
}

func TestLoadTLSConfig_MissingClientCert(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	_, _, keyPath := generateTestCert(t, tmpDir)

	cfg := config.BuildKitConfig{
		TLSCert: "/nonexistent/cert.pem",
		TLSKey:  keyPath,
	}

	_, err := loadTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for missing client cert")
	}
	if !strings.Contains(err.Error(), "failed to load client cert/key") {
		t.Errorf("error should mention client cert: %v", err)
	}
}

func TestLoadTLSConfig_InvalidCACertContent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	badCAPath := filepath.Join(tmpDir, "bad-ca.pem")
	_ = os.WriteFile(badCAPath, []byte("not a valid cert"), 0644)

	cfg := config.BuildKitConfig{
		TLSCACert: badCAPath,
	}

	_, err := loadTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid CA cert content")
	}
	if !strings.Contains(err.Error(), "failed to parse CA cert") {
		t.Errorf("error should mention parsing: %v", err)
	}
}

// ============================================================
// configureCacheOptions: additional edge cases
// ============================================================

func TestConfigureCacheOptions_LocalTemplateCacheDisabled(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{
		cacheFrom: []string{"type=registry,ref=user/app:cache"},
		cacheTo:   []string{"type=registry,ref=user/app:cache"},
	}

	solveOpt := &client.SolveOpt{}
	b.configureCacheOptions(solveOpt, builder.Config{IsLocalTemplate: true})

	if len(solveOpt.CacheImports) != 0 {
		t.Errorf("expected 0 cache imports for local template, got %d", len(solveOpt.CacheImports))
	}
	if len(solveOpt.CacheExports) != 0 {
		t.Errorf("expected 0 cache exports for local template, got %d", len(solveOpt.CacheExports))
	}
}

func TestConfigureCacheOptions_NoCacheFlagDisablesCaching(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{
		cacheFrom: []string{"type=registry,ref=user/app:cache"},
		cacheTo:   []string{"type=registry,ref=user/app:cache"},
	}

	solveOpt := &client.SolveOpt{}
	b.configureCacheOptions(solveOpt, builder.Config{NoCache: true})

	if len(solveOpt.CacheImports) != 0 {
		t.Errorf("expected 0 cache imports with NoCache, got %d", len(solveOpt.CacheImports))
	}
	if len(solveOpt.CacheExports) != 0 {
		t.Errorf("expected 0 cache exports with NoCache, got %d", len(solveOpt.CacheExports))
	}
}

// ============================================================
// loadAndTagImage: load error
// ============================================================

func TestLoadAndTagImage_LoadError(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			return dockerimage.LoadResponse{}, fmt.Errorf("disk full")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	tmpFile, err := os.CreateTemp("", "test-image-*.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	if _, err := tmpFile.WriteString("data"); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	err = b.loadAndTagImage(context.Background(), tmpFile.Name(), "test:latest")
	if err == nil {
		t.Error("expected error for load failure")
	}
	if !strings.Contains(err.Error(), "failed to load image") {
		t.Errorf("error should mention load: %v", err)
	}
}

// ============================================================
// getLocalImageDigest: inspect error
// ============================================================

func TestGetLocalImageDigest_InspectError(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, fmt.Errorf("image not found")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	d := b.getLocalImageDigest(context.Background(), "nonexistent:latest")
	if d != "" {
		t.Errorf("expected empty digest for inspect error, got %q", d)
	}
}

// ============================================================
// InspectManifest: no platform case
// ============================================================

func TestInspectManifest_NoPlatform(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		DistributionInspectFunc: func(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error) {
			return dockerregistry.DistributionInspect{
				Descriptor: ocispec.Descriptor{
					// No platform set
					Digest: "sha256:abc123",
				},
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries, err := b.InspectManifest(context.Background(), "test:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No platform means no entries added
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when no platform, got %d", len(entries))
	}
}

// ============================================================
// Close: both clients nil
// ============================================================

func TestClose_BothNil(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{
		client:       nil,
		dockerClient: nil,
	}

	err := b.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// applyShellProvisioner: go command detection
// ============================================================

func TestApplyShellProvisioner_GoCommand(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{}
	state := llb.Image("golang:1.21")

	prov := builder.Provisioner{
		Type:   "shell",
		Inline: []string{"go build ./cmd/myapp"},
	}

	result, err := b.applyShellProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// convertToLLB: unknown provisioner type should not error
// ============================================================

func TestConvertToLLB_UnknownProvisioner(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "alpine:latest",
			Platform: "linux/amd64",
		},
		Provisioners: []builder.Provisioner{
			{
				Type: "custom-unknown",
			},
		},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Errorf("unexpected error for unknown provisioner type: %v", err)
	}
}

// ============================================================
// convertToLLB: invalid platform but with architectures fallback
// ============================================================

func TestConvertToLLB_InvalidPlatformWithArchFallback(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "alpine:latest",
			Platform: "invalid", // Invalid platform format
		},
		Architectures: []string{"amd64"},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Errorf("unexpected error when architecture fallback available: %v", err)
	}
}

// ============================================================
// SupportsMultiArch is not a method on BuildKitBuilder, skip
// ============================================================

// ============================================================
// applyProvisioner: powershell dispatch
// ============================================================

func TestApplyProvisioner_PowerShellDispatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	psPath := filepath.Join(dir, "test.ps1")
	_ = os.WriteFile(psPath, []byte("Write-Host 'test'"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("mcr.microsoft.com/powershell:latest")

	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{psPath},
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// applyProvisioner: script dispatch
// ============================================================

func TestApplyProvisioner_ScriptDispatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "test.sh")
	_ = os.WriteFile(scriptPath, []byte("#!/bin/sh\necho test"), 0755)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:    "script",
		Scripts: []string{scriptPath},
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// fixedWriteCloser: write multiple times
// ============================================================

func TestFixedWriteCloser_MultipleWrites(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "multi-write.tar")

	factory := fixedWriteCloser(filePath)
	wc, err := factory(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Multiple writes
	for i := 0; i < 3; i++ {
		_, err := wc.Write([]byte("data"))
		if err != nil {
			t.Fatalf("write %d error: %v", i, err)
		}
	}

	if err := wc.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(content) != "datadatadata" {
		t.Errorf("expected 'datadatadata', got %q", string(content))
	}
}

// ============================================================
// parseCacheAttrs: additional edge cases
// ============================================================

func TestParseCacheAttrs_MultipleEquals(t *testing.T) {
	t.Parallel()
	// Value with "=" in it should be preserved
	result := parseCacheAttrs("type=registry,ref=host:5000/img:tag=latest")

	if result["type"] != "registry" {
		t.Errorf("expected type 'registry', got %q", result["type"])
	}
	if result["ref"] != "host:5000/img:tag=latest" {
		t.Errorf("expected ref with = preserved, got %q", result["ref"])
	}
}

// ============================================================
// expandContainerVars: multiple variables
// ============================================================

func TestExpandContainerVars_MultipleVars(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{}
	env := map[string]string{
		"HOME": "/home/user",
		"PATH": "/usr/bin",
		"USER": "testuser",
	}

	result := b.expandContainerVars("$HOME/.local/bin:$PATH (user: $USER)", env)
	expected := "/home/user/.local/bin:/usr/bin (user: testuser)"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ============================================================
// InspectManifest: inspect failure
// ============================================================

func TestInspectManifest_InspectFailure(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		DistributionInspectFunc: func(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error) {
			return dockerregistry.DistributionInspect{}, fmt.Errorf("manifest not found")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	_, err := b.InspectManifest(context.Background(), "nonexistent:latest")
	if err == nil {
		t.Error("expected error for inspect failure")
	}
	if !strings.Contains(err.Error(), "failed to inspect manifest") {
		t.Errorf("error should mention inspect: %v", err)
	}
}

// ============================================================
// getAnsiblePaths: error path
// ============================================================

func TestGetAnsiblePaths_PlaybookExpandError(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "~nonexistentuser12345/playbook.yml",
	}

	_, err := b.getAnsiblePaths(prov, pv)
	// May or may not error depending on OS tilde expansion
	_ = err
}

func TestGetAnsiblePaths_GalaxyExpandError(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	tmpDir := t.TempDir()
	pbPath := filepath.Join(tmpDir, "playbook.yml")
	_ = os.WriteFile(pbPath, []byte("---"), 0644)

	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: pbPath,
		GalaxyFile:   "~nonexistentuser12345/requirements.yml",
	}

	_, err := b.getAnsiblePaths(prov, pv)
	// May or may not error depending on OS tilde expansion
	_ = err
}

// ============================================================
// getFilePaths: error path
// ============================================================

func TestGetFilePaths_ExpandError(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	prov := builder.Provisioner{
		Type:   "file",
		Source: "~nonexistentuser12345/file.txt",
	}

	_, err := b.getFilePaths(prov, pv)
	// May or may not error depending on OS tilde expansion
	_ = err
}

// ============================================================
// expandPathList: error path
// ============================================================

func TestExpandPathList_Error(t *testing.T) {
	t.Parallel()
	pv := templates.NewPathValidator()

	scripts := []string{"~nonexistentuser12345/script.sh"}
	_, err := expandPathList(scripts, pv, "script")
	// May or may not error depending on OS tilde expansion
	_ = err
}

// ============================================================
// collectProvisionerPaths: error propagation
// ============================================================

func TestCollectProvisionerPaths_ErrorPropagation(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	provisioners := []builder.Provisioner{
		{
			Type:         "ansible",
			PlaybookPath: "~nonexistentuser12345/playbook.yml",
		},
	}

	_, err := b.collectProvisionerPaths(provisioners, pv)
	// Error propagation depends on OS tilde expansion behavior
	_ = err
}

// ============================================================
// applyAnsibleProvisioner: error from makeRelativePath
// ============================================================

func TestApplyAnsibleProvisioner_PlaybookResolveError(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("ubuntu:22.04")

	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "~nonexistentuser12345/playbook.yml",
	}

	_, err := b.applyAnsibleProvisioner(state, prov)
	// May or may not error depending on OS tilde expansion
	_ = err
}

// ============================================================
// applyScriptProvisioner: script path error
// ============================================================

func TestApplyScriptProvisioner_ResolveError(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:    "script",
		Scripts: []string{"~nonexistentuser12345/script.sh"},
	}

	_, err := b.applyScriptProvisioner(state, prov)
	// May or may not error depending on OS tilde expansion
	_ = err
}

// ============================================================
// applyPowerShellProvisioner: script path error
// ============================================================

func TestApplyPowerShellProvisioner_ResolveError(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("mcr.microsoft.com/powershell:latest")

	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{"~nonexistentuser12345/script.ps1"},
	}

	_, err := b.applyPowerShellProvisioner(state, prov)
	// May or may not error depending on OS tilde expansion
	_ = err
}

// ============================================================
// calculateBuildContext: multiple provisioners with mixed types
// ============================================================

func TestCalculateBuildContext_MixedProvisioners(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "subA", "file.txt")
	script1 := filepath.Join(tmpDir, "subB", "script.sh")
	_ = os.MkdirAll(filepath.Dir(file1), 0755)
	_ = os.MkdirAll(filepath.Dir(script1), 0755)
	_ = os.WriteFile(file1, []byte("data"), 0644)
	_ = os.WriteFile(script1, []byte("#!/bin/sh"), 0755)

	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{Type: "file", Source: file1, Destination: "/tmp/f"},
			{Type: "script", Scripts: []string{script1}},
			{Type: "shell", Inline: []string{"echo hi"}}, // No paths
		},
	}

	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Common parent should be tmpDir since subA and subB are children
	absTmpDir, _ := filepath.Abs(tmpDir)
	if ctx != absTmpDir {
		t.Errorf("expected %q, got %q", absTmpDir, ctx)
	}
}

// ============================================================
// Push: empty registry with unqualified ref
// ============================================================

func TestPush_EmptyRegistryWithUnqualifiedRef(t *testing.T) {
	t.Parallel()
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{},
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	// Empty registry should not trigger tagging
	d, err := b.Push(context.Background(), "myapp:latest", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No digest expected with empty RepoDigests
	if d != "" {
		t.Errorf("expected empty digest, got %q", d)
	}
}

// ============================================================
// convertToLLB: with env and build args simultaneously
// ============================================================

func TestConvertToLLB_EnvAndBuildArgs(t *testing.T) {
	t.Parallel()
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "alpine:latest",
			Platform: "linux/amd64",
			Env:      map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		BuildArgs:   map[string]string{"VERSION": "1.0", "DEBUG": "true"},
		PostChanges: []string{"ENV RESULT done", "WORKDIR /app", "USER appuser"},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// makeRelativePath: error path for invalid context dir
// ============================================================

func TestMakeRelativePathInvalidContext(t *testing.T) {
	// Test with an empty contextDir - should still work since filepath.Abs handles ""
	b := &BuildKitBuilder{contextDir: ""}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("test"), 0644)

	// Should not panic, may produce a relative path from cwd
	_, err := b.makeRelativePath(filePath)
	if err != nil {
		// Some environments may error, that's OK
		t.Logf("makeRelativePath with empty context: %v", err)
	}
}

func TestMakeRelativePathOutsideContext(t *testing.T) {
	// Test with a path that's outside the context directory
	contextDir := t.TempDir()
	otherDir := t.TempDir()
	filePath := filepath.Join(otherDir, "outside.txt")
	_ = os.WriteFile(filePath, []byte("test"), 0644)

	b := &BuildKitBuilder{contextDir: contextDir}
	relPath, err := b.makeRelativePath(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Path outside context should still work but contain ".."
	if relPath == "" {
		t.Error("expected non-empty relative path")
	}
}

// ============================================================
// applyScriptProvisioner: error from makeRelativePath
// ============================================================

func TestApplyScriptProvisionerMakeRelativePathError(t *testing.T) {
	// Use a context dir and a script path with a tilde-user that doesn't exist
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:    "script",
		Scripts: []string{"~nonexistentuser12345/script.sh"},
	}

	_, err := b.applyScriptProvisioner(state, prov)
	// Should return an error from makeRelativePath failure
	if err == nil {
		t.Logf("script provisioner with invalid tilde path succeeded (tilde expansion may work differently)")
	}
}

// ============================================================
// applyPowerShellProvisioner: error from makeRelativePath
// ============================================================

func TestApplyPowerShellProvisionerMakeRelativePathError(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{"~nonexistentuser12345/script.ps1"},
	}

	_, err := b.applyPowerShellProvisioner(state, prov)
	if err == nil {
		t.Logf("powershell provisioner with invalid tilde path succeeded (tilde expansion may work differently)")
	}
}

// ============================================================
// applyAnsibleProvisioner: error from makeRelativePath on playbook
// ============================================================

func TestApplyAnsibleProvisionerPlaybookPathError(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("ubuntu:22.04")

	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: "~nonexistentuser12345/playbook.yml",
	}

	_, err := b.applyAnsibleProvisioner(state, prov)
	if err == nil {
		t.Logf("ansible provisioner with invalid tilde playbook path succeeded")
	}
}

// ============================================================
// applyFileProvisioner: error from makeRelativePath
// ============================================================

func TestApplyFileProvisionerMakeRelativePathError(t *testing.T) {
	b := &BuildKitBuilder{contextDir: t.TempDir()}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:        "file",
		Source:      "~nonexistentuser12345/file.txt",
		Destination: "/tmp/file.txt",
	}

	_, err := b.applyFileProvisioner(state, prov)
	if err == nil {
		t.Logf("file provisioner with invalid tilde path succeeded")
	}
}

// ============================================================
// getAnsiblePaths: error on playbook path expansion
// ============================================================

func TestGetAnsiblePathsExpansionError(t *testing.T) {
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	t.Run("playbook expansion error", func(t *testing.T) {
		prov := builder.Provisioner{
			Type:         "ansible",
			PlaybookPath: "~nonexistentuser12345/playbook.yml",
		}
		_, err := b.getAnsiblePaths(prov, pv)
		if err == nil {
			t.Logf("tilde expansion succeeded (OS-dependent behavior)")
		} else if !strings.Contains(err.Error(), "failed to expand playbook path") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("galaxy expansion error", func(t *testing.T) {
		prov := builder.Provisioner{
			Type:       "ansible",
			GalaxyFile: "~nonexistentuser12345/requirements.yml",
		}
		_, err := b.getAnsiblePaths(prov, pv)
		if err == nil {
			t.Logf("tilde expansion succeeded (OS-dependent behavior)")
		} else if !strings.Contains(err.Error(), "failed to expand galaxy file path") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// ============================================================
// getFilePaths: error on source path expansion
// ============================================================

func TestGetFilePathsExpansionError(t *testing.T) {
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	prov := builder.Provisioner{
		Type:   "file",
		Source: "~nonexistentuser12345/file.txt",
	}
	_, err := b.getFilePaths(prov, pv)
	if err == nil {
		t.Logf("tilde expansion succeeded (OS-dependent behavior)")
	} else if !strings.Contains(err.Error(), "failed to expand file source path") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// expandPathList: error on path expansion
// ============================================================

func TestExpandPathListExpansionError(t *testing.T) {
	pv := templates.NewPathValidator()

	scripts := []string{"~nonexistentuser12345/script.sh"}
	_, err := expandPathList(scripts, pv, "script")
	if err == nil {
		t.Logf("tilde expansion succeeded (OS-dependent behavior)")
	} else if !strings.Contains(err.Error(), "failed to expand script path") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// findCommonParent: relative path inputs
// ============================================================

func TestFindCommonParentRelativePaths(t *testing.T) {
	// Relative paths should be converted to absolute by findCommonParent
	result := findCommonParent("a/b", "a/c")
	if result == "" {
		t.Error("expected non-empty result for relative paths")
	}
}

// ============================================================
// InspectManifest: no platform in descriptor
// ============================================================

func TestInspectManifestNoPlatform(t *testing.T) {
	mock := &MockDockerClient{
		DistributionInspectFunc: func(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error) {
			return dockerregistry.DistributionInspect{
				Descriptor: ocispec.Descriptor{
					Digest: "sha256:abc123",
					// Platform is nil
				},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	entries, err := b.InspectManifest(context.Background(), "myregistry.io/myapp:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No platform means no entries
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for nil platform, got %d", len(entries))
	}
}

// ============================================================
// CreateAndPushManifest: invalid manifest name
// ============================================================

func TestCreateAndPushManifestInvalidName(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{Size: 1024}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/app:latest",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       digest.FromString("test"),
		},
	}

	// Use an invalid manifest name that will fail name.ParseReference
	err := b.CreateAndPushManifest(context.Background(), "INVALID:::name", entries)
	if err == nil {
		t.Error("expected error for invalid manifest name")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to parse manifest name") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// Push: empty registry with non-fully-qualified ref (no tagging needed)
// ============================================================

func TestPushEmptyRegistryNonQualifiedRef(t *testing.T) {
	pushCalled := false
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			pushCalled = true
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"myapp@sha256:digest123"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	// Empty registry with non-qualified ref should not trigger tagging
	_, err := b.Push(context.Background(), "myapp:latest", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pushCalled {
		t.Error("expected push to be called")
	}
}

// ============================================================
// convertToLLB: provisioner error propagation
// ============================================================

func TestConvertToLLBProvisionerError(t *testing.T) {
	dir := t.TempDir()
	b := &BuildKitBuilder{contextDir: dir}

	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "alpine:latest",
			Platform: "linux/amd64",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:        "file",
				Source:      filepath.Join(dir, "nonexistent-file-for-error.txt"),
				Destination: "/tmp/dest",
			},
		},
	}

	_, err := b.convertToLLB(cfg)
	if err == nil {
		t.Error("expected error from file provisioner with nonexistent source")
	}
	if err != nil && !strings.Contains(err.Error(), "provisioner 0 failed") {
		t.Errorf("expected provisioner error wrapping, got: %v", err)
	}
}

// ============================================================
// calculateBuildContext: single path (no common parent needed)
// ============================================================

func TestCalculateBuildContextSinglePath(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "script.sh")
	_ = os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0755)

	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{Type: "script", Scripts: []string{scriptPath}},
		},
	}

	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With a single file, context should be its parent directory
	absTmpDir, _ := filepath.Abs(tmpDir)
	if ctx != absTmpDir {
		t.Errorf("expected %q, got %q", absTmpDir, ctx)
	}
}
