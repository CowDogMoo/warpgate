package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/api/types/image"
	dockerRegistry "github.com/docker/docker/api/types/registry"
	dockerClient "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// DockerClientInterface represents an interface for Docker client
// operations.
//
// **Methods:**
//
// DockerLogin: Authenticates with a Docker registry.
// DockerPush: Pushes a Docker image to a registry.
// DockerTag: Tags a Docker image with a new name.
// CreateAndPushManifest: Creates and pushes a Docker manifest list to a registry.
// CreateManifest: Creates a Docker manifest list from image tags.
// GetImageSize: Retrieves the size of a Docker image.
type DockerClientInterface interface {
	DockerLogin() error
	DockerPush(image string) error
	DockerTag(sourceImage, targetImage string) error
	CreateAndPushManifest(blueprint *bp.Blueprint, imageTags []string) error
	CreateManifest(ctx context.Context, targetImage string, imageTags []string) (v1.ImageIndex, error)
	GetImageSize(imageRef string) (int64, error)
}

// DockerClient represents a Docker client.
//
// **Attributes:**
//
// AuthStr: The base64 encoded auth string for the Docker registry.
// CLI: API client for Docker operations.
// Container: A packer.Container instance.
// Registry: A DockerRegistry instance.
// AuthConfig: Authentication configuration for the Docker registry.
// Remote: A RemoteInterface instance.
type DockerClient struct {
	AuthStr    string
	CLI        *dockerClient.Client
	Container  packer.Container
	Registry   *DockerRegistry
	AuthConfig authn.AuthConfig
	Remote     RemoteInterface
}

// DockerLogin authenticates with a Docker registry using the provided
// credentials.
//
// **Returns:**
//
// error: An error if the login operation fails.
func (d *DockerClient) DockerLogin() error {
	if d.Container.ImageRegistry.Username == "" || d.Container.ImageRegistry.Credential == "" || d.Container.ImageRegistry.Server == "" {
		return errors.New("username, password, and server must not be empty")
	}

	authConfig := dockerRegistry.AuthConfig{
		Username:      d.Container.ImageRegistry.Username,
		Password:      d.Container.ImageRegistry.Credential,
		ServerAddress: d.Container.ImageRegistry.Server,
	}

	resp, err := d.CLI.RegistryLogin(context.Background(), authConfig)
	if err != nil {
		return fmt.Errorf("error logging into Docker registry %s: %v", d.Container.ImageRegistry.Server, err)
	}

	if resp.Status != "Login Succeeded" {
		return fmt.Errorf("failed to login to Docker registry %s:%v", d.Container.ImageRegistry.Server, resp.Status)
	}

	// Use the identity token if it is available
	if resp.IdentityToken != "" {
		d.AuthStr = resp.IdentityToken
		fmt.Printf("Successfully logged in to %s and retrieved identity token as %s\n", d.Container.ImageRegistry.Server, d.AuthStr)
	}

	// If no identity token is available, encode the authConfig to base64
	if d.AuthStr == "" {
		fmt.Println("No identity token retrieved from registry, encoding input token...")
		d.AuthStr, err = encodeAuthToBase64(authConfig)
		if err != nil {
			return fmt.Errorf("error encoding authConfig to base64: %v", err)
		}
	}

	fmt.Printf("Successfully logged in to %s as %s\n", d.Container.ImageRegistry.Server, d.Container.ImageRegistry.Username)
	return nil
}

// NewDockerClient creates a new Docker client.
//
// **Returns:**
//
// *DockerClient: A DockerClient instance.
// error: An error if any issue occurs while creating the client.
func NewDockerClient(registryURL, authToken string, registryConfig packer.ContainerImageRegistry) (*DockerClient, error) {
	cli, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv, dockerClient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating Docker client: %v", err)
	}

	dockerRegistry, err := NewDockerRegistry(registryURL, authToken, registryConfig, DefaultGetStore, true)
	if err != nil {
		return nil, fmt.Errorf("error creating Docker registry: %v", err)
	}

	return &DockerClient{
		CLI: cli,
		Container: packer.Container{
			ImageRegistry: registryConfig,
			ImageHashes:   []packer.ImageHash{},
		},
		Registry: dockerRegistry,
		Remote:   &RemoteClient{},
	}, nil
}

// DockerTag tags a Docker image with a new name.
//
// **Parameters:**
//
// sourceImage: The current name of the image.
// targetImage: The new name to assign to the image.
//
// **Returns:**
//
// error: An error if the tagging operation fails.
func (d *DockerClient) DockerTag(sourceImage, targetImage string) error {
	if sourceImage == "" || targetImage == "" {
		return errors.New("sourceImage and targetImage must not be empty")
	}

	ctx := context.Background()
	return d.CLI.ImageTag(ctx, sourceImage, targetImage)
}

// RemoveImage removes an image from the Docker client.
//
// **Parameters:**
//
// ctx: The context within which the image is to be removed.
// imageID: The ID of the image to be removed.
// options: Options for the image removal operation.
//
// **Returns:**
//
// error: An error if any issue occurs during the image removal process.
// []image.DeleteResponse: A slice of image.DeleteResponse instances.
func (d *DockerClient) RemoveImage(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	fmt.Println("Removing image:", imageID)
	return d.CLI.ImageRemove(ctx, imageID, options)
}

// DockerPush pushes a Docker image to a registry using the provided
// auth string.
//
// **Parameters:**
//
// containerImage: The name of the image to push.
// authStr: The auth string for the Docker registry.
//
// **Returns:**
//
// error: An error if the push operation fails.
func (d *DockerClient) PushImage(containerImage string) error {
	if d.AuthStr == "" {
		return errors.New("error: docker client is not authenticated with a registry")
	}

	if containerImage == "" {
		return errors.New("containerImage must not be empty")
	}

	ctx := context.Background()
	resp, err := d.CLI.ImagePush(ctx, containerImage, image.PushOptions{
		RegistryAuth: d.AuthStr,
	})
	if err != nil {
		return fmt.Errorf("error pushing image %s: %v", containerImage, err)
	}
	defer resp.Close()

	_, err = io.Copy(os.Stdout, resp)
	if err != nil {
		return fmt.Errorf("error copying response to stdout: %v", err)
	}

	return nil
}
