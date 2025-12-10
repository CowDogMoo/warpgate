package buildkit

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	dockerimage "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/cowdogmoo/warpgate/pkg/manifests"
)

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

func TestCreateAndPushManifest(t *testing.T) {
	tests := []struct {
		name         string
		manifestName string
		entries      []manifests.ManifestEntry
		setupMock    func(*MockDockerClient)
		expectError  bool
	}{
		{
			name:         "successful manifest creation",
			manifestName: "myregistry.io/myapp:latest",
			entries: []manifests.ManifestEntry{
				{
					ImageRef:     "myregistry.io/myapp:amd64",
					OS:           "linux",
					Architecture: "amd64",
					Digest:       digest.Digest("sha256:abc123"),
				},
				{
					ImageRef:     "myregistry.io/myapp:arm64",
					OS:           "linux",
					Architecture: "arm64",
					Digest:       digest.Digest("sha256:def456"),
				},
			},
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:   "sha256:test",
						Size: 1024,
					}, nil
				}
			},
			expectError: false,
		},
		{
			name:         "manifest creation with no entries",
			manifestName: "myregistry.io/myapp:latest",
			entries:      []manifests.ManifestEntry{},
			setupMock:    func(m *MockDockerClient) {},
			expectError:  true,
		},
		{
			name:         "manifest creation with empty digest",
			manifestName: "myregistry.io/myapp:latest",
			entries: []manifests.ManifestEntry{
				{
					ImageRef:     "myregistry.io/myapp:amd64",
					OS:           "linux",
					Architecture: "amd64",
					Digest:       digest.Digest(""),
				},
			},
			setupMock:   func(m *MockDockerClient) {},
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

			err := builder.CreateAndPushManifest(context.Background(), tt.manifestName, tt.entries)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
