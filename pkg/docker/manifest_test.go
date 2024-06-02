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
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (m *MockDockerClient) CreateAndPushManifest(blueprint *bp.Blueprint, imageTags []string) error {
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

func (m *MockDockerClient) PushManifest(imageName string, manifestList v1.ImageIndex) error {
	args := m.Called(imageName, manifestList)
	return args.Error(0)
}

func (m *MockDockerClient) GetAuthToken(repo, tag string) (string, error) {
	args := m.Called(repo, tag)
	return args.String(0), args.Error(1)
}

type MockRemoteClient struct {
	mock.Mock
}

func (m *MockRemoteClient) Image(ref name.Reference, options ...remote.Option) (v1.Image, error) {
	args := m.Called(ref)
	img := args.Get(0)
	if img == nil {
		return nil, args.Error(1)
	}
	return img.(v1.Image), args.Error(1)
}

func mockImage() v1.Image {
	return empty.Image
}

func TestCreateManifest(t *testing.T) {
	tests := []struct {
		name          string
		targetImage   string
		imageTags     []string
		setupMocks    func(*docker.DockerClient, *MockRemoteClient)
		expectedIndex v1.ImageIndex
		expectErr     bool
	}{
		{
			name:        "successful manifest creation",
			targetImage: "example/latest",
			imageTags: []string{
				"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			setupMocks: func(dc *docker.DockerClient, mockRemote *MockRemoteClient) {
				dc.Container.ImageHashes = []packer.ImageHash{
					{Hash: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", OS: "linux", Arch: "amd64"},
					{Hash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", OS: "linux", Arch: "arm64"},
				}
				mockRemote.On("Image", mock.Anything).Return(mockImage(), nil)
			},
			expectedIndex: func() v1.ImageIndex {
				index := empty.Index
				withMediaType := mutate.IndexMediaType(index, types.OCIImageIndex)
				digest := v1.Hash{
					Algorithm: "sha256",
					Hex:       "732112270d7e59418a8c080b134b24cabd67d250d0d0147a97ed95ba5c280aa4",
				}
				descriptor := v1.Descriptor{
					MediaType: "application/vnd.docker.distribution.manifest.v2+json",
					Size:      264,
					Digest:    digest,
				}
				withMediaType = mutate.AppendManifests(withMediaType, mutate.IndexAddendum{Add: mockImage(), Descriptor: descriptor})
				withMediaType = mutate.AppendManifests(withMediaType, mutate.IndexAddendum{Add: mockImage(), Descriptor: descriptor})
				return withMediaType
			}(),
			expectErr: false,
		},
		{
			name:        "error fetching image size",
			targetImage: "example/latest",
			imageTags: []string{
				"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			setupMocks: func(dc *docker.DockerClient, mockRemote *MockRemoteClient) {
				dc.Container.ImageHashes = []packer.ImageHash{
					{Hash: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", OS: "linux", Arch: "amd64"},
					{Hash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", OS: "linux", Arch: "arm64"},
				}
				mockRemote.On("Image", mock.Anything).Return(nil, errors.New("unauthorized"))
			},
			expectedIndex: empty.Index,
			expectErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRemote := &MockRemoteClient{}
			dockerClient := &docker.DockerClient{
				Remote: mockRemote,
			}
			tc.setupMocks(dockerClient, mockRemote)

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.45/images/sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/json":
					if tc.expectErr {
						w.WriteHeader(http.StatusUnauthorized)
					} else {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"Size": 1024}`))
					}
				case "/v1.45/images/sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890/json":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"Size": 2048}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer ts.Close()

			var err error
			dockerClient.CLI, err = client.NewClientWithOpts(client.WithHost(ts.URL), client.WithAPIVersionNegotiation())
			require.NoError(t, err)

			keychain := authn.NewMultiKeychain(
				authn.DefaultKeychain,
				github.Keychain,
				authn.NewKeychainFromHelper(authnHelperMock{}),
			)

			actualIndex, err := dockerClient.CreateManifest(context.Background(), tc.targetImage, tc.imageTags, keychain)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Compare the manifests in a more suitable way
				expectedManifest, err := tc.expectedIndex.IndexManifest()
				require.NoError(t, err)
				actualManifest, err := actualIndex.IndexManifest()
				require.NoError(t, err)
				require.Equal(t, expectedManifest, actualManifest)
			}

			mockRemote.AssertExpectations(t)
		})
	}
}

type authnHelperMock struct {
	authn.Helper
}

func (authnHelperMock) Authenticator(repo name.Repository) (authn.Authenticator, error) {
	return authn.FromConfig(authn.AuthConfig{
		Username: "user",
		Password: "password",
	}), nil
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
		manifestList v1.ImageIndex
		setupMocks   func(*MockDockerClient)
		expectErr    bool
	}{
		{
			name:         "successful manifest push",
			imageName:    "ghcr.io/example/latest",
			manifestList: empty.Index,
			setupMocks: func(m *MockDockerClient) {
				m.On("GetAuthToken", "example", "latest").Return("token", nil)
				m.On("PushManifest", "ghcr.io/example/latest", empty.Index).Return(nil)
			},
			expectErr: false,
		},
		{
			name:         "error during manifest push",
			imageName:    "ghcr.io/example/latest",
			manifestList: empty.Index,
			setupMocks: func(m *MockDockerClient) {
				m.On("GetAuthToken", "example", "latest").Return("", errors.New("failed to get auth token, status: 403 Forbidden"))
				m.On("PushManifest", "ghcr.io/example/latest", empty.Index).Return(errors.New("forbidden"))
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
