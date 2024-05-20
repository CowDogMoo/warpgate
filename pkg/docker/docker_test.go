package docker_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
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
	DockerLoginFunc          func(username, password, server string) error
	RegistryLoginFunc        func(ctx context.Context, auth registry.AuthConfig) (registry.AuthenticateOKBody, error)
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

func (m *MockDockerClient) RegistryLogin(ctx context.Context, auth registry.AuthConfig) (registry.AuthenticateOKBody, error) {
	if m.RegistryLoginFunc != nil {
		return m.RegistryLoginFunc(ctx, auth)
	}
	return registry.AuthenticateOKBody{}, nil
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
			return nil
		},
		DockerLoginFunc: func(username, password, server string) error {
			if username == "" || password == "" || server == "" {
				return errors.New("login error")
			}
			return nil
		},
		RegistryLoginFunc: func(ctx context.Context, auth registry.AuthConfig) (registry.AuthenticateOKBody, error) {
			if auth.Username == "" || auth.Password == "" || auth.ServerAddress == "" {
				return registry.AuthenticateOKBody{}, errors.New("login error")
			}
			return registry.AuthenticateOKBody{IdentityToken: "mockToken"}, nil
		},
	}
	return &docker.DockerClient{
		CLI:         mockClient,
		ExecCommand: exec.Command,
		AuthStr:     mockClient.authStr,
	}
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
			client := NewMockDockerClient()
			err := client.DockerLogin(tc.username, tc.password, tc.server)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerLogin() error = %v, wantErr %v", err, tc.wantErr)
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

	execCommand := func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	tests := []struct {
		name                   string
		packerTemplates        []packer.PackerTemplate
		mockLoginFunc          func(username, password, server string) error
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
							"amd64": "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d",
							"arm64": "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d",
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
			mockLoginFunc: func(username, password, server string) error { return nil },
			mockTagFunc:   func(ctx context.Context, source, target string) error { return nil },
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("")), nil
			},
			mockManifestCreateFunc: func(manifest string, images []string) error {
				return nil
			},
			mockManifestPushFunc: func(manifest string) error { return nil },
			wantErr:              false,
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
							"amd64": "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d",
							"arm64": "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d",
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
			mockLoginFunc: func(username, password, server string) error {
				return errors.New("login error")
			},
			mockTagFunc: func(ctx context.Context, source, target string) error { return nil },
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return nil, errors.New("push error")
			},
			mockManifestCreateFunc: func(manifest string, images []string) error {
				return errors.New("manifest create error")
			},
			mockManifestPushFunc: func(manifest string) error {
				return errors.New("manifest push error")
			},
			wantErr:        true,
			expectedErrMsg: "push error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &docker.DockerClient{
				CLI:         &MockDockerClient{},
				ExecCommand: execCommand,
				AuthStr:     "test-auth-token",
			}

			client.CLI.(*MockDockerClient).DockerLoginFunc = tc.mockLoginFunc
			client.CLI.(*MockDockerClient).ImageTagFunc = tc.mockTagFunc
			client.CLI.(*MockDockerClient).ImagePushFunc = tc.mockPushFunc
			client.CLI.(*MockDockerClient).DockerManifestCreateFunc = tc.mockManifestCreateFunc
			client.CLI.(*MockDockerClient).DockerManifestPushFunc = tc.mockManifestPushFunc

			err := client.TagAndPushImages(tc.packerTemplates, client.AuthStr, "test-image", tc.packerTemplates[0].Container.ImageHashes)
			if (err != nil) != tc.wantErr {
				t.Errorf("TagAndPushImages() error = %v, wantErr %v", err, tc.wantErr)
			} else if tc.wantErr && err != nil && !strings.Contains(err.Error(), tc.expectedErrMsg) {
				t.Errorf("TagAndPushImages() error = %v, expectedErrMsg %v", err, tc.expectedErrMsg)
			}
		})
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	if len(args) > 3 && args[3] == "manifest" {
		if args[4] == "create" || args[4] == "push" {
			fmt.Fprintln(os.Stderr, "error")
			os.Exit(1)
		}
	}
	os.Exit(0)
}
