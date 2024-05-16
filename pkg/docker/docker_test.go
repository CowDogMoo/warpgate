package docker_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
)

// Updated MockDockerClient to implement DockerClientInterface
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) DockerLogin(username, password, server string) (string, error) {
	args := m.Called(username, password, server)
	return args.String(0), args.Error(1)
}

func (m *MockDockerClient) DockerPush(image, authStr string) error {
	args := m.Called(image, authStr)
	return args.Error(0)
}

func (m *MockDockerClient) DockerTag(sourceImage, targetImage string) error {
	args := m.Called(sourceImage, targetImage)
	return args.Error(0)
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
			want:     "eyJ1c2VybmFtZSI6ICJ1c2VyIiwgInBhc3N3b3JkIjogInBhc3MiLCAic2VydmVyYWRkcmVzcyI6ICJzZXJ2ZXIifQ==",
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
			mockClient := new(MockDockerClient)
			expectedError := error(nil)
			if tc.wantErr {
				expectedError = fmt.Errorf("login error")
			}
			mockClient.On("DockerLogin", tc.username, tc.password, tc.server).Return(tc.want, expectedError)

			got, err := mockClient.DockerLogin(tc.username, tc.password, tc.server)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerLogin() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if got != tc.want {
				t.Errorf("DockerLogin() = %v, want %v", got, tc.want)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerPush(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		authStr   string
		pushError error
		wantErr   bool
	}{
		{
			name:      "successful push",
			image:     "image",
			authStr:   "auth",
			pushError: nil,
			wantErr:   false,
		},
		{
			name:      "failed push",
			image:     "image",
			authStr:   "auth",
			pushError: fmt.Errorf("push error"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			mockClient.On("DockerPush", tc.image, tc.authStr).Return(tc.pushError)
			client := mockClient

			err := client.DockerPush(tc.image, tc.authStr)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerPush() error = %v, wantErr %v", err, tc.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerTag(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		target   string
		tagError error
		wantErr  bool
	}{
		{
			name:     "successful tag",
			source:   "source",
			target:   "target",
			tagError: nil,
			wantErr:  false,
		},
		{
			name:     "failed tag",
			source:   "source",
			target:   "target",
			tagError: fmt.Errorf("tag error"),
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			mockClient.On("DockerTag", tc.source, tc.target).Return(tc.tagError)
			client := mockClient

			err := client.DockerTag(tc.source, tc.target)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerTag() error = %v, wantErr %v", err, tc.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestPushDockerImages(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		authStr   string
		pushError error
		wantErr   bool
	}{
		{
			name:      "successful push",
			image:     "image",
			authStr:   "auth",
			pushError: nil,
			wantErr:   false,
		},
		{
			name:      "failed push",
			image:     "image",
			authStr:   "auth",
			pushError: fmt.Errorf("push error"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			mockClient.On("DockerPush", tc.image, tc.authStr).Return(tc.pushError)

			err := mockClient.DockerPush(tc.image, tc.authStr)
			if (err != nil) != tc.wantErr {
				t.Errorf("DockerPush() error = %v, wantErr %v", err, tc.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}
