package docker_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func (m *MockDockerClient) CreateAndPushManifest(
	blueprint *bp.Blueprint, imageTags []string,
) error {
	args := m.Called(blueprint, imageTags)
	return args.Error(0)
}

func (m *MockDockerClient) ManifestCreate(
	ctx context.Context, targetImage string, imageTags []string,
) (ocispec.Index, error) {
	args := m.Called(ctx, targetImage, imageTags)
	return args.Get(0).(ocispec.Index), args.Error(1)
}

func (m *MockDockerClient) GetImageSize(imageRef string) (int64, error) {
	args := m.Called(imageRef)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDockerClient) PushManifest(
	imageName string, manifestList ocispec.Index,
) error {
	args := m.Called(imageName, manifestList)
	return args.Error(0)
}

func (m *MockDockerClient) getAuthToken(repo, tag string) (string, error) {
	args := m.Called(repo, tag)
	return args.String(0), args.Error(1)
}

func TestManifestCreate(t *testing.T) {
	tests := []struct {
		name          string
		targetImage   string
		imageTags     []string
		setupMocks    func(*docker.DockerClient)
		expectedIndex ocispec.Index
		expectErr     bool
	}{
		{
			name:        "successful manifest creation",
			targetImage: "example/latest",
			imageTags:   []string{"sha256:1234", "sha256:5678"},
			setupMocks: func(dc *docker.DockerClient) {
				dc.Container.ImageHashes = []packer.ImageHash{
					{Hash: "sha256:1234", OS: "linux", Arch: "amd64"},
					{Hash: "sha256:5678", OS: "linux", Arch: "arm64"},
				}
			},
			expectedIndex: ocispec.Index{
				Versioned: specs.Versioned{
					SchemaVersion: 2,
				},
				MediaType: ocispec.MediaTypeImageIndex,
				Manifests: []ocispec.Descriptor{
					{
						MediaType: ocispec.MediaTypeImageManifest,
						Digest:    digest.Digest("sha256:1234"),
						Size:      1024,
						Platform: &ocispec.Platform{
							Architecture: "amd64",
							OS:           "linux",
						},
					},
					{
						MediaType: ocispec.MediaTypeImageManifest,
						Digest:    digest.Digest("sha256:5678"),
						Size:      2048,
						Platform: &ocispec.Platform{
							Architecture: "arm64",
							OS:           "linux",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name:        "error fetching image size",
			targetImage: "example/latest",
			imageTags:   []string{"sha256:1234", "sha256:5678"},
			setupMocks: func(dc *docker.DockerClient) {
				dc.Container.ImageHashes = []packer.ImageHash{
					{Hash: "sha256:1234", OS: "linux", Arch: "amd64"},
					{Hash: "sha256:5678", OS: "linux", Arch: "arm64"},
				}
			},
			expectedIndex: ocispec.Index{},
			expectErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dockerClient := &docker.DockerClient{}
			tc.setupMocks(dockerClient)

			// Set up a test HTTP server for mocking the Docker API responses
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.45/images/sha256:1234/json":
					if tc.expectErr {
						w.WriteHeader(http.StatusNotFound)
					} else {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"Size": 1024}`))
					}
				case "/v1.45/images/sha256:5678/json":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"Size": 2048}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer ts.Close()

			// Override the Docker client's URL to use the test server
			dockerClient.CLI, _ = client.NewClientWithOpts(client.WithHost(ts.URL), client.WithAPIVersionNegotiation())

			actualIndex, err := dockerClient.ManifestCreate(context.Background(), tc.targetImage, tc.imageTags)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedIndex, actualIndex)
			}
		})
	}
}

func TestGetImageSize(t *testing.T) {
	tests := []struct {
		name         string
		imageRef     string
		setupMocks   func(*MockDockerClient)
		expectedSize int64
		expectErr    bool
	}{
		{
			name:     "successful image size retrieval",
			imageRef: "sha256:1234",
			setupMocks: func(m *MockDockerClient) {
				m.On("GetImageSize", "sha256:1234").Return(int64(1024), nil)
			},
			expectedSize: int64(1024),
			expectErr:    false,
		},
		{
			name:     "error during image size retrieval",
			imageRef: "sha256:1234",
			setupMocks: func(m *MockDockerClient) {
				m.On("GetImageSize", "sha256:1234").Return(int64(0), errors.New("inspect error"))
			},
			expectedSize: int64(0),
			expectErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			tc.setupMocks(mockClient)

			size, err := mockClient.GetImageSize(tc.imageRef)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedSize, size)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestPushManifest(t *testing.T) {
	tests := []struct {
		name         string
		imageName    string
		manifestList ocispec.Index
		setupMocks   func(*MockDockerClient)
		expectErr    bool
	}{
		{
			name:         "successful manifest push",
			imageName:    "example/latest",
			manifestList: ocispec.Index{},
			setupMocks: func(m *MockDockerClient) {
				m.On("PushManifest", "example/latest", ocispec.Index{}).Return(nil)
			},
			expectErr: false,
		},
		{
			name:         "error during manifest push",
			imageName:    "example/latest",
			manifestList: ocispec.Index{},
			setupMocks: func(m *MockDockerClient) {
				m.On("PushManifest", "example/latest", ocispec.Index{}).Return(errors.New("push error"))
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			tc.setupMocks(mockClient)

			err := mockClient.PushManifest(tc.imageName, tc.manifestList)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestGetAuthToken(t *testing.T) {
	tests := []struct {
		name          string
		repo          string
		tag           string
		setupMocks    func(*MockDockerClient)
		expectedToken string
		expectErr     bool
	}{
		{
			name: "successful auth token retrieval",
			repo: "example",
			tag:  "latest",
			setupMocks: func(m *MockDockerClient) {
				m.On("getAuthToken", "example", "latest").Return("token", nil)
			},
			expectedToken: "token",
			expectErr:     false,
		},
		{
			name: "error during auth token retrieval",
			repo: "example",
			tag:  "latest",
			setupMocks: func(m *MockDockerClient) {
				m.On("getAuthToken", "example", "latest").Return("", errors.New("auth error"))
			},
			expectedToken: "",
			expectErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			tc.setupMocks(mockClient)

			token, err := mockClient.getAuthToken(tc.repo, tc.tag)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedToken, token)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
