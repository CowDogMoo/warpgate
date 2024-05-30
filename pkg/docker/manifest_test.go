package docker_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func (m *MockDockerClient) CreateAndPushManifest(
	blueprint *bp.Blueprint, imageTags []string,
) error {
	args := m.Called(blueprint, imageTags)
	return args.Error(0)
}

func convertToInt64(val interface{}) (int64, error) {
	switch v := val.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unsupported type: %v", reflect.TypeOf(val))
	}
}

func (m *MockDockerClient) GetImageSize(imageRef string) (int64, error) {
	args := m.Called(imageRef)
	val := args.Get(0)
	i64Val, err := convertToInt64(val)
	if err != nil {
		return 0, err
	}
	return i64Val, args.Error(1)
}

func (m *MockDockerClient) PushManifest(
	imageName string, manifestList ocispec.Index,
) error {
	args := m.Called(imageName, manifestList)
	return args.Error(0)
}

func (m *MockDockerClient) GetAuthToken(repo, tag string) (string, error) {
	args := m.Called(repo, tag)
	return args.String(0), args.Error(1)
}

func TestCreateManifest(t *testing.T) {
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

			actualIndex, err := dockerClient.CreateManifest(context.Background(), tc.targetImage, tc.imageTags)
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
		manifestList v1.Index
		setupMocks   func(*MockDockerClient)
		expectErr    bool
	}{
		{
			name:         "successful manifest push",
			imageName:    "ghcr.io/example/latest",
			manifestList: v1.Index{},
			setupMocks: func(m *MockDockerClient) {
				m.On("GetAuthToken", "example", "latest").Return("token", nil)
				m.On("PushManifest", "ghcr.io/example/latest", v1.Index{}).Return(nil)
			},
			expectErr: false,
		},
		{
			name:         "error during manifest push",
			imageName:    "ghcr.io/example/latest",
			manifestList: v1.Index{},
			setupMocks: func(m *MockDockerClient) {
				m.On("GetAuthToken", "example", "latest").Return("", errors.New("failed to get auth token, status: 403 Forbidden"))
				m.On("PushManifest", "ghcr.io/example/latest", v1.Index{}).Return(errors.New("forbidden"))
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
				require.Contains(t, err.Error(), "forbidden")
			} else {
				require.NoError(t, err)
			}

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
				m.On("GetAuthToken", "example", "latest").Return("token", nil)
			},
			expectedToken: "token",
			expectErr:     false,
		},
		{
			name: "error during auth token retrieval",
			repo: "example",
			tag:  "latest",
			setupMocks: func(m *MockDockerClient) {
				m.On("GetAuthToken", "example", "latest").Return("", errors.New("auth error"))
			},
			expectedToken: "",
			expectErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			tc.setupMocks(mockClient)

			token, err := mockClient.GetAuthToken(tc.repo, tc.tag)
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
