package docker_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/containers/common/libimage"
	"github.com/containers/storage"
	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDockerAPIClient simulates the Docker API client.
type MockDockerAPIClient struct {
	mock.Mock
	client.Client
}

// RegistryLogin is the mock implementation of the Docker RegistryLogin function.
func (m *MockDockerAPIClient) RegistryLogin(ctx context.Context, authConfig registry.AuthConfig) (registry.AuthenticateOKBody, error) {
	args := m.Called(ctx, authConfig)
	return args.Get(0).(registry.AuthenticateOKBody), args.Error(1)
}

// ImageTag is the mock implementation of the Docker ImageTag function.
func (m *MockDockerAPIClient) ImageTag(ctx context.Context, sourceImage, targetImage string) error {
	args := m.Called(ctx, sourceImage, targetImage)
	return args.Error(0)
}

// MockDockerRegistry is a mock implementation of DockerRegistry.
type MockDockerRegistry struct {
	Runtime     *libimage.Runtime
	Store       storage.Store
	RegistryURL string
	AuthToken   string
}

func TestNewDockerRegistry(t *testing.T) {
	tests := []struct {
		name        string
		registryURL string
		authToken   string
		wantErr     bool
	}{
		{
			name:        "valid registry",
			registryURL: "https://example.com",
			authToken:   "testToken",
			wantErr:     false,
		},
		{
			name:        "invalid registry URL",
			registryURL: "",
			authToken:   "testToken",
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := docker.NewDockerRegistry(tc.registryURL, tc.authToken)
			if (err != nil) != tc.wantErr {
				t.Errorf("NewDockerRegistry() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if registry != nil {
				if registry.RegistryURL != tc.registryURL {
					t.Errorf("Unexpected registry URL. Got: %s, Want: %s", registry.RegistryURL, tc.registryURL)
				}

				if registry.AuthToken != tc.authToken {
					t.Errorf("Unexpected auth token. Got: %s, Want: %s", registry.AuthToken, tc.authToken)
				}
			}
		})
	}
}

type MockHTTPRoundTripper struct {
	mock.Mock
}

func (m *MockHTTPRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(*http.Response)
	return resp, args.Error(1)
}

type MockDockerClient struct {
	client.Client
	ImageTagFunc      func(ctx context.Context, source, target string) error
	ImagePushFunc     func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error)
	DockerLoginFunc   func(username, password, server string) error
	RegistryLoginFunc func(ctx context.Context, auth registry.AuthConfig) (registry.AuthenticateOKBody, error)
	authStr           string
}

// NewMockDockerClient creates a mock Docker client for testing.
func NewMockDockerClient() (*docker.DockerClient, *MockDockerAPIClient, *MockHTTPRoundTripper) {
	mockRoundTripper := new(MockHTTPRoundTripper)
	httpClient := &http.Client{Transport: mockRoundTripper}
	cli, _ := client.NewClientWithOpts(client.WithHTTPClient(httpClient))

	mockAPIClient := new(MockDockerAPIClient)
	dockerClient := &docker.DockerClient{
		CLI:     cli,
		AuthStr: "",
	}

	return dockerClient, mockAPIClient, mockRoundTripper
}

func (m *MockDockerClient) ImageTag(ctx context.Context, source, target string) error {
	if m.ImageTagFunc != nil {
		return m.ImageTagFunc(ctx, source, target)
	}
	return nil
}

func (m *MockDockerClient) DockerLogin(username, password, server string) error {
	if m.DockerLoginFunc != nil {
		err := m.DockerLoginFunc(username, password, server)
		if err == nil {
			m.authStr = "mockAuthString"
		}
		return err
	}
	if username == "" || password == "" || server == "" {
		return errors.New("login error")
	}
	m.authStr = "mockAuthString"
	return nil
}

func TestDockerLogin(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		server   string
		wantErr  bool
	}{
		{
			name:     "valid login with protocol",
			username: "user",
			password: "pass",
			server:   "https://ghcr.io",
			wantErr:  false,
		},
		{
			name:     "valid login without protocol",
			username: "user",
			password: "pass",
			server:   "ghcr.io",
			wantErr:  false,
		},
		{
			name:     "invalid login - empty credentials",
			username: "",
			password: "",
			server:   "",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, mockAPIClient, mockRoundTripper := NewMockDockerClient()
			client.Container = packer.Container{
				ImageRegistry: packer.ContainerImageRegistry{
					Username:   tc.username,
					Credential: tc.password,
					Server:     tc.server,
				},
				ImageHashes: []packer.ImageHash{},
			}

			authConfig := registry.AuthConfig{
				Username:      tc.username,
				Password:      tc.password,
				ServerAddress: tc.server,
			}

			authBytes, err := json.Marshal(authConfig)
			require.NoError(t, err)

			client.AuthStr = base64.URLEncoding.EncodeToString(authBytes)

			if tc.wantErr {
				mockAPIClient.On("RegistryLogin", mock.Anything, authConfig).
					Return(registry.AuthenticateOKBody{}, errors.New("invalid credentials")).Once()
			} else {
				mockAPIClient.On("RegistryLogin", mock.Anything, authConfig).
					Return(registry.AuthenticateOKBody{IdentityToken: "mockToken"}, nil).Once()
			}

			mockRoundTripper.On("RoundTrip", mock.Anything).
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{}`)),
				}, nil).Once()

			err = client.DockerLogin()
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerLogin() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestDockerTag(t *testing.T) {
	tests := []struct {
		name        string
		sourceImage string
		targetImage string
		wantErr     bool
	}{
		{
			name:        "valid tag",
			sourceImage: "busybox:latest",
			targetImage: "busybox:tagged",
			wantErr:     false,
		},
		{
			name:        "invalid tag",
			sourceImage: "",
			targetImage: "busybox:tagged",
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, mockAPIClient, mockRoundTripper := NewMockDockerClient()

			if tc.wantErr {
				mockAPIClient.On("ImageTag", mock.Anything, tc.sourceImage, tc.targetImage).
					Return(errors.New("invalid source image")).Once()
			} else {
				mockAPIClient.On("ImageTag", mock.Anything, tc.sourceImage, tc.targetImage).
					Return(nil).Once()
			}

			mockRoundTripper.On("RoundTrip", mock.Anything).
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{}`)),
				}, nil).Once()

			err := client.CLI.ImageTag(context.Background(), tc.sourceImage, tc.targetImage)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerTag() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestPushImage(t *testing.T) {
	tests := []struct {
		name           string
		containerImage string
		authStr        string
		mockPushFunc   func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error)
		wantErr        bool
	}{
		{
			name:           "valid push",
			containerImage: "busybox:latest",
			authStr:        "dGVzdDp0ZXN0",
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("")), nil
			},
			wantErr: false,
		},
		{
			name:           "invalid push",
			containerImage: "invalidimage",
			authStr:        "dGVzdDp0ZXN0",
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return nil, errors.New("push error")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, mockAPIClient, mockRoundTripper := NewMockDockerClient()
			client.AuthStr = tc.authStr
			dockerClient := &docker.DockerClient{
				CLI:     client.CLI,
				AuthStr: tc.authStr,
			}

			mockAPIClient.On("ImagePush", mock.Anything, tc.containerImage, image.PushOptions{RegistryAuth: tc.authStr}).
				Return(tc.mockPushFunc(context.Background(), tc.containerImage, image.PushOptions{RegistryAuth: tc.authStr})).Once()

			if tc.wantErr {
				mockRoundTripper.On("RoundTrip", mock.Anything).
					Return(&http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(strings.NewReader(`{"message": "push error"}`)),
					}, nil).Once()
			} else {
				mockRoundTripper.On("RoundTrip", mock.Anything).
					Return(&http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil).Once()
			}

			err := dockerClient.PushImage(tc.containerImage)
			if (err != nil) != tc.wantErr {
				t.Errorf("PushImage() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// func TestProcessPackerTemplates(t *testing.T) {
// 	tests := []struct {
// 		name            string
// 		packerTemplates []packer.PackerTemplate
// 		mockLoginFunc   func() error
// 		mockTagFunc     func(ctx context.Context, source, target string) error
// 		mockPushFunc    func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error)
// 		wantErr         bool
// 		expectedErrMsg  string
// 	}{
// 		{
// 			name: "valid push images",
// 			packerTemplates: []packer.PackerTemplate{
// 				{
// 					AMI: packer.AMI{
// 						InstanceType: "t2.micro",
// 						Region:       "us-west-2",
// 						SSHUser:      "ec2-user",
// 					},
// 					Container: packer.Container{
// 						ImageHashes: []packer.ImageHash{
// 							{Arch: "amd64", OS: "linux", Hash: "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d"},
// 							{Arch: "arm64", OS: "linux", Hash: "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d"},
// 						},
// 						ImageRegistry: packer.ContainerImageRegistry{
// 							Server:     "testserver",
// 							Username:   "testuser",
// 							Credential: "testtoken",
// 						},
// 						Workdir: "/tmp",
// 					},
// 					ImageValues: packer.ImageValues{
// 						Name:    "test-image",
// 						Version: "latest",
// 					},
// 				},
// 			},
// 			mockLoginFunc: func() error { return nil },
// 			mockTagFunc:   func(ctx context.Context, source, target string) error { return nil },
// 			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
// 				return io.NopCloser(strings.NewReader("")), nil
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "invalid push images",
// 			packerTemplates: []packer.PackerTemplate{
// 				{
// 					AMI: packer.AMI{
// 						InstanceType: "t2.micro",
// 						Region:       "us-west-2",
// 						SSHUser:      "ec2-user",
// 					},
// 					Container: packer.Container{
// 						ImageHashes: []packer.ImageHash{
// 							{Arch: "amd64", OS: "linux", Hash: "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d"},
// 							{Arch: "arm64", OS: "linux", Hash: "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d"},
// 						},
// 						ImageRegistry: packer.ContainerImageRegistry{
// 							Server:     "testserver",
// 							Username:   "testuser",
// 							Credential: "testtoken",
// 						},
// 						Workdir: "/tmp",
// 					},
// 					ImageValues: packer.ImageValues{
// 						Name:    "test-image",
// 						Version: "latest",
// 					},
// 				},
// 			},
// 			mockLoginFunc: func() error {
// 				return errors.New("login error")
// 			},
// 			mockTagFunc: func(ctx context.Context, source, target string) error { return nil },
// 			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
// 				return nil, errors.New("push error")
// 			},
// 			wantErr:        true,
// 			expectedErrMsg: "push error",
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			client := NewMockDockerClient()
// 			dockerClient := &docker.DockerClient{
// 				CLI: client,
// 			}

// 			client.On("DockerLogin").Return(tc.mockLoginFunc())
// 			client.On("DockerTag", mock.Anything, mock.Anything).Return(tc.mockTagFunc)
// 			client.On("ImagePush", mock.Anything, mock.Anything, mock.Anything).Return(tc.mockPushFunc)

// 			err := dockerClient.ProcessPackerTemplates(tc.packerTemplates, packer.Blueprint{Name: "test-image"})
// 			if (err != nil) != tc.wantErr {
// 				t.Errorf("ProcessPackerTemplates() error = %v, wantErr %v", err, tc.wantErr)
// 			} else if tc.wantErr && err != nil && !strings.Contains(err.Error(), tc.expectedErrMsg) {
// 				t.Errorf("ProcessPackerTemplates() error = %v, expectedErrMsg %v", err, tc.expectedErrMsg)
// 			}
// 		})
// 	}
// }
