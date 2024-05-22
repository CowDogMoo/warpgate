package docker_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

type MockDockerClient struct {
	mock.Mock
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

func (m *MockDockerClient) NewRepository(ref reference.Named, registry string, tr http.RoundTripper) (distribution.Repository, error) {
	args := m.Called(ref, registry, tr)
	return args.Get(0).(distribution.Repository), args.Error(1)
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
		DockerManifestPushFunc:   func(manifest string) error { return nil },
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
		AuthStr:     "test-auth-token",
		Container: packer.Container{
			ImageRegistry: packer.ContainerImageRegistry{
				Server:     "https://ghcr.io",
				Username:   "testuser",
				Credential: "testtoken",
			},
		},
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
			client.Container.ImageRegistry.Username = tc.username
			client.Container.ImageRegistry.Credential = tc.password
			client.Container.ImageRegistry.Server = tc.server
			err := client.DockerLogin()
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
			client := NewMockDockerClient()
			client.AuthStr = tc.authStr
			client.CLI.(*MockDockerClient).ImagePushFunc = tc.mockPushFunc

			err := client.PushImage(tc.containerImage)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerPush() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestProcessPackerTemplates(t *testing.T) {
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
						ImageRegistry: packer.ContainerImageRegistry{
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
						ImageRegistry: packer.ContainerImageRegistry{
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

			for i := range tc.packerTemplates {
				client.Container = tc.packerTemplates[i].Container
				err := client.ProcessPackerTemplates(tc.packerTemplates, "test-image")
				if (err != nil) != tc.wantErr {
					t.Errorf("ProcessPackerTemplates() error = %v, wantErr %v", err, tc.wantErr)
				} else if tc.wantErr && err != nil && !strings.Contains(err.Error(), tc.expectedErrMsg) {
					t.Errorf("ProcessPackerTemplates() error = %v, expectedErrMsg %v", err, tc.expectedErrMsg)
				}
			}
		})
	}
}

func TestProcessImageTag(t *testing.T) {
	client := NewMockDockerClient()
	imageName := "test-image"
	arch := "amd64"
	hash := "51e3e95c15772272fe39b628cd825352add77c782d1f3cfdf8a0131c16a78f4d"
	imageTags := []string{}

	err := client.TagAndPushImages(imageName, arch, hash, &imageTags)
	if err != nil {
		t.Errorf("ProcessImageTag() error = %v", err)
	}

	// Verify that the image tag was added to the list
	if len(imageTags) != 1 {
		t.Errorf("ProcessImageTag() expected 1 image tag, got %d", len(imageTags))
	}

	// Verify the correctness of the image tag
	expectedTag := "https://ghcr.io/testuser/test-image:amd64"
	if imageTags[0] != expectedTag {
		t.Errorf("ProcessImageTag() expected image tag %s, got %s", expectedTag, imageTags[0])
	}
}

// type MockTagsService struct {
// 	mock.Mock
// }

// // Tag implements distribution.TagService.
// func (m *MockTagsService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
// 	panic("unimplemented")
// }

// // Untag implements distribution.TagService.
// func (m *MockTagsService) Untag(ctx context.Context, tag string) error {
// 	panic("unimplemented")
// }

// func (m *MockTagsService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
// 	args := m.Called(ctx, tag)
// 	return args.Get(0).(distribution.Descriptor), args.Error(1)
// }

// type MockManifestsService struct {
// 	mock.Mock
// }

// // Delete implements distribution.ManifestService.
// func (m *MockManifestsService) Delete(ctx context.Context, dgst digest.Digest) error {
// 	panic("unimplemented")
// }

// // Exists implements distribution.ManifestService.
// func (m *MockManifestsService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
// 	panic("unimplemented")
// }

// // Put implements distribution.ManifestService.
// func (m *MockManifestsService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
// 	panic("unimplemented")
// }

// func (m *MockManifestsService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
// 	args := m.Called(ctx, dgst)
// 	return args.Get(0).(distribution.Manifest), args.Error(1)
// }

// func (m *MockTagsService) All(ctx context.Context) ([]string, error) {
// 	args := m.Called(ctx)
// 	return args.Get(0).([]string), args.Error(1)
// }

// func (m *MockTagsService) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
// 	args := m.Called(ctx, digest)
// 	return args.Get(0).([]string), args.Error(1)
// }

// type MockRepository struct {
// 	distribution.Repository
// 	TagsService      *MockTagsService
// 	ManifestsService *MockManifestsService
// }

// func (r *MockRepository) Tags(ctx context.Context) distribution.TagService {
// 	return r.TagsService
// }

// func (r *MockRepository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
// 	return r.ManifestsService, nil
// }

// func TestDockerManifestCreate(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		imageTags   []string
// 		wantErr     bool
// 		expectedErr string
// 	}{
// 		{
// 			name:      "valid manifest creation",
// 			imageTags: []string{"docker.io/library/testimage:latest", "docker.io/library/testimage:arm64"},
// 			wantErr:   false,
// 		},
// 		{
// 			name:        "invalid manifest creation",
// 			imageTags:   []string{"docker.io/library/invalidimage"},
// 			wantErr:     true,
// 			expectedErr: "failed to get image digest",
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			mockTagsService := new(MockTagsService)
// 			mockManifestsService := new(MockManifestsService)
// 			mockTagsService.On("Get", mock.Anything, "latest").Return(distribution.Descriptor{Digest: digest.Digest("sha256:mockdigest")}, nil)
// 			mockTagsService.On("Get", mock.Anything, "arm64").Return(distribution.Descriptor{Digest: digest.Digest("sha256:mockdigest")}, nil)
// 			mockTagsService.On("Get", mock.Anything, "invalidimage").Return(distribution.Descriptor{}, errors.New("failed to get image digest"))

// 			mockRepo := &MockRepository{
// 				TagsService:      mockTagsService,
// 				ManifestsService: mockManifestsService,
// 			}

// 			mockDockerClient := new(MockDockerClient)
// 			repoName, _ := reference.WithName("docker.io/library/testimage")
// 			mockDockerClient.On("NewRepository", repoName, "https://docker.io", mock.Anything).Return(mockRepo, nil)

// 			client := &docker.DockerClient{
// 				CLI: mockDockerClient,
// 			}

// 			err := client.DockerManifestCreate("testmanifest", &tc.imageTags)

// 			if (err != nil) != tc.wantErr {
// 				t.Errorf("DockerManifestCreate() error = %v, wantErr %v", err, tc.wantErr)
// 			}
// 			if tc.wantErr && err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
// 				t.Errorf("DockerManifestCreate() error = %v, expectedErr %v", err, tc.expectedErr)
// 			}

// 			// mockTagsService.AssertExpectations(t)
// 			// mockManifestsService.AssertExpectations(t)
// 			// mockDockerClient.AssertExpectations(t)
// 		})
// 	}
// }
