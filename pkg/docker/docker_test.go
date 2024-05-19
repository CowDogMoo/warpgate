package docker_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"
)

type MockDockerClient struct {
	client.Client
	ImageTagFunc             func(ctx context.Context, source, target string) error
	ImagePushFunc            func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error)
	DockerManifestCreateFunc func(manifest string, images []string) error
	DockerManifestPushFunc   func(manifest string) error
	DockerLoginFunc          func(username, password, server string) (string, error)
	authStr                  string
}

func (m *MockDockerClient) ImageTag(ctx context.Context, source, target string) error {
	if m.ImageTagFunc != nil {
		return m.ImageTagFunc(ctx, source, target)
	}
	return nil
}

func (m *MockDockerClient) ImagePush(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
	if m.ImagePushFunc != nil {
		return m.ImagePushFunc(ctx, ref, options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockDockerClient) DockerManifestCreate(manifest string, images []string) error {
	if m.DockerManifestCreateFunc != nil {
		return m.DockerManifestCreateFunc(manifest, images)
	}
	return nil
}

func (m *MockDockerClient) DockerManifestPush(manifest string) error {
	if m.DockerManifestPushFunc != nil {
		return m.DockerManifestPushFunc(manifest)
	}
	return nil
}

func (m *MockDockerClient) DockerLogin(username, password, server string) (string, error) {
	if m.DockerLoginFunc != nil {
		authStr, err := m.DockerLoginFunc(username, password, server)
		m.authStr = authStr
		return authStr, err
	}
	if username == "" || password == "" || server == "" {
		return "", errors.New("login error")
	}
	authStr := "eyJ"
	m.authStr = authStr
	return authStr, nil
}

func (m *MockDockerClient) Info(ctx context.Context) (system.Info, error) {
	return system.Info{}, nil
}

func NewMockDockerClient() *docker.DockerClient {
	mockClient := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error { return nil },
		ImagePushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		},
		DockerManifestCreateFunc: func(manifest string, images []string) error { return nil },
		DockerManifestPushFunc: func(manifest string) error {
			if manifest == "testserver/test-image:latest" {
				return errors.New("denied: requested access to the resource is denied")
			}
			return nil
		},
		DockerLoginFunc: func(username, password, server string) (string, error) {
			if username == "" || password == "" || server == "" {
				return "", errors.New("login error")
			}
			return "eyJ", nil
		},
	}
	return &docker.DockerClient{
		CLI:     mockClient,
		AuthStr: mockClient.authStr,
	}
}

func TestDockerLogin(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		server   string
		want     string
		wantErr  bool
	}{
		{
			name:     "valid login",
			username: "user",
			password: "pass",
			server:   "server",
			want:     "eyJ",
			wantErr:  false,
		},
		{
			name:     "invalid login",
			username: "",
			password: "",
			server:   "",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewMockDockerClient()
			client.CLI.(*MockDockerClient).DockerLoginFunc = func(username, password, server string) (string, error) {
				if tc.wantErr {
					return "", errors.New("login error")
				}
				return "eyJ", nil
			}

			err := client.DockerLogin(tc.username, tc.password, tc.server)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerLogin() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr && !strings.Contains(client.AuthStr, "eyJ") {
				t.Errorf("DockerLogin() = %v, want substring 'eyJ'", client.AuthStr)
			}
			if tc.wantErr && err == nil {
				t.Errorf("DockerLogin() error = <nil>, wantErr %v", tc.wantErr)
			}
		})
	}
}

func ensureImagePulled(t *testing.T, cli *client.Client, containerImage string) {
	_, _, err := cli.ImageInspectWithRaw(context.Background(), containerImage)
	if client.IsErrNotFound(err) {
		_, err = cli.ImagePull(context.Background(), containerImage, image.PullOptions{})
		if err != nil {
			t.Fatalf("Failed to pull image %s: %v", containerImage, err)
		}
	}
}

func TestDockerTag(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	ensureImagePulled(t, cli, "busybox:latest")

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
			client, _ := docker.NewDockerClient()
			err := client.DockerTag(tc.sourceImage, tc.targetImage)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerTag() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestDockerPush(t *testing.T) {
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
			client := NewMockDockerClient()
			client.AuthStr = tc.authStr
			client.CLI.(*MockDockerClient).ImagePushFunc = tc.mockPushFunc

			err := client.DockerPush(tc.containerImage)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerPush() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
func TestTagAndPushImages(t *testing.T) {
	viper.Set("container.registry.server", "testserver")
	viper.Set("container.registry.username", "testuser")
	viper.Set("container.registry.token", "testtoken")

	tests := []struct {
		name                   string
		packerTemplates        []packer.PackerTemplate
		mockLoginFunc          func(username, password, server string) (string, error)
		mockTagFunc            func(ctx context.Context, source, target string) error
		mockPushFunc           func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error)
		mockManifestCreateFunc func(manifest string, images []string) error
		mockManifestPushFunc   func(manifest string) error
		wantErr                bool
		expectedErrMsg         string
	}{
		{
			name: "valid push images",
			packerTemplates: []packer.PackerTemplate{
				{
					AMI: packer.AMI{
						InstanceType: "t2.micro",
						Region:       "us-west-2",
						SSHUser:      "ec2-user",
					},
					Container: packer.Container{
						ImageHashes: map[string]string{
							"amd64": "hash1",
							"arm64": "hash2",
						},
						Registry: packer.ContainerImageRegistry{
							Server:     "testserver",
							Username:   "testuser",
							Credential: "testtoken",
						},
						Workdir: "/tmp",
					},
					ImageValues: packer.ImageValues{
						Name:    "test-image",
						Version: "latest",
					},
				},
			},
			mockLoginFunc: func(username, password, server string) (string, error) {
				return "test-auth-token", nil
			},
			mockTagFunc: func(ctx context.Context, source, target string) error { return nil },
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("")), nil
			},
			mockManifestCreateFunc: func(manifest string, images []string) error {
				return errors.New("docker manifest create failed for testserver/test-image:latest: errors:\ndenied: requested access to the resource is denied\nunauthorized: authentication required\n")
			},
			mockManifestPushFunc: func(manifest string) error {
				return nil
			},
			wantErr:        true,
			expectedErrMsg: "unauthorized: authentication required",
		},
		{
			name: "invalid push images",
			packerTemplates: []packer.PackerTemplate{
				{
					AMI: packer.AMI{
						InstanceType: "t2.micro",
						Region:       "us-west-2",
						SSHUser:      "ec2-user",
					},
					Container: packer.Container{
						ImageHashes: map[string]string{
							"amd64": "hash1",
							"arm64": "hash2",
						},
						Registry: packer.ContainerImageRegistry{
							Server:     "testserver",
							Username:   "testuser",
							Credential: "testtoken",
						},
						Workdir: "/tmp",
					},
					ImageValues: packer.ImageValues{
						Name:    "test-image",
						Version: "latest",
					},
				},
			},
			mockLoginFunc: func(username, password, server string) (string, error) {
				return "", errors.New("login error")
			},
			mockTagFunc: func(ctx context.Context, source, target string) error { return nil },
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return nil, errors.New("push error")
			},
			mockManifestCreateFunc: func(manifest string, images []string) error { return errors.New("manifest create error") },
			mockManifestPushFunc:   func(manifest string) error { return errors.New("manifest push error") },
			wantErr:                true,
			expectedErrMsg:         "push error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &docker.DockerClient{
				CLI: &MockDockerClient{
					DockerLoginFunc:          tc.mockLoginFunc,
					ImageTagFunc:             tc.mockTagFunc,
					ImagePushFunc:            tc.mockPushFunc,
					DockerManifestCreateFunc: tc.mockManifestCreateFunc,
					DockerManifestPushFunc:   tc.mockManifestPushFunc,
				},
				AuthStr: "test-auth-token",
			}

			err := client.TagAndPushImages(tc.packerTemplates, client.AuthStr, "test-image", tc.packerTemplates[0].Container.ImageHashes)
			if (err != nil) != tc.wantErr {
				t.Errorf("TagAndPushImages() error = %v, wantErr %v", err, tc.wantErr)
			} else if tc.wantErr && err != nil && !strings.Contains(err.Error(), tc.expectedErrMsg) {
				t.Errorf("TagAndPushImages() error = %v, expectedErrMsg %v", err, tc.expectedErrMsg)
			}
		})
	}
}
