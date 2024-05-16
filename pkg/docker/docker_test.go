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
	"github.com/docker/docker/client"
)

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
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := docker.NewDockerClient()
			got, err := client.DockerLogin(tc.username, tc.password, tc.server)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerLogin() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr && !strings.Contains(got, tc.want) {
				t.Errorf("DockerLogin() = %v, want substring %v", got, tc.want)
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

	// Ensure busybox:latest is available locally
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

type MockDockerClient struct {
	client.Client
	ImageTagFunc  func(ctx context.Context, source, target string) error
	ImagePushFunc func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error)
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

func NewMockDockerClient() *docker.DockerClient {
	return &docker.DockerClient{
		CLI: &MockDockerClient{
			ImageTagFunc: func(ctx context.Context, source, target string) error { return nil },
			ImagePushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("")), nil
			},
		},
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
			client.CLI.(*MockDockerClient).ImagePushFunc = tc.mockPushFunc

			err := client.DockerPush(tc.containerImage, tc.authStr)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerPush() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestTagAndPushImages(t *testing.T) {
	tests := []struct {
		name            string
		packerTemplates []packer.BlueprintPacker
		mockTagFunc     func(ctx context.Context, source, target string) error
		mockPushFunc    func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error)
		wantErr         bool
	}{
		{
			name: "valid push images",
			packerTemplates: []packer.BlueprintPacker{
				{
					Tag: packer.BlueprintTag{Name: "test-image"},
					ImageHashes: map[string]string{
						"amd64": "hash1",
						"arm64": "hash2",
					},
				},
			},
			mockTagFunc: func(ctx context.Context, source, target string) error { return nil },
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("")), nil
			},
			wantErr: false,
		},
		{
			name: "invalid push images",
			packerTemplates: []packer.BlueprintPacker{
				{
					Tag: packer.BlueprintTag{Name: "test-image"},
					ImageHashes: map[string]string{
						"amd64": "hash1",
						"arm64": "hash2",
					},
				},
			},
			mockTagFunc: func(ctx context.Context, source, target string) error { return nil },
			mockPushFunc: func(ctx context.Context, ref string, options image.PushOptions) (io.ReadCloser, error) {
				return nil, errors.New("push error")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewMockDockerClient()
			client.CLI.(*MockDockerClient).ImageTagFunc = tc.mockTagFunc
			client.CLI.(*MockDockerClient).ImagePushFunc = tc.mockPushFunc

			err := client.TagAndPushImages(tc.packerTemplates)
			if (err != nil) != tc.wantErr {
				t.Errorf("PushImages() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
