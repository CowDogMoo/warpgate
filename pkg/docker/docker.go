package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"
)

// DockerClientInterface represents an interface for Docker client
// operations.
//
// **Methods:**
//
// DockerLogin: Authenticates with a Docker registry.
// DockerPush: Pushes a Docker image to a registry.
// DockerTag: Tags a Docker image with a new name.
type DockerClientInterface interface {
	DockerLogin(username, password, server string) (string, error)
	DockerPush(image, authStr string) error
	DockerTag(sourceImage, targetImage string) error
}

// DockerClient represents a Docker client.
//
// **Attributes:**
//
// CLI: API client for Docker operations.
type DockerClient struct {
	CLI client.APIClient
}

// NewDockerClient creates a new Docker client.
//
// **Returns:**
//
// *DockerClient: A DockerClient instance.
// error: An error if any issue occurs while creating the client.
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{CLI: cli}, nil
}

// DockerLogin authenticates with a Docker registry using the provided
// username, password, and server. It constructs an auth string for
// the registry.
//
// **Parameters:**
//
// username: The username for the Docker registry.
// password: The password for the Docker registry.
// server: The server address of the Docker registry.
//
// **Returns:**
//
// string: The base64 encoded auth string.
// error: An error if any issue occurs during the login process.
func (d *DockerClient) DockerLogin(username, password, server string) (string, error) {
	authConfig := map[string]string{
		"username":      username,
		"password":      password,
		"serveraddress": server,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	return authStr, nil
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
	ctx := context.Background()
	return d.CLI.ImageTag(ctx, sourceImage, targetImage)
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
func (d *DockerClient) DockerPush(containerImage, authStr string) error {
	ctx := context.Background()
	resp, err := d.CLI.ImagePush(ctx, containerImage, image.PushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		return err
	}
	defer resp.Close()
	_, err = io.ReadAll(resp)
	return err
}

// TagAndPushImages tags and pushes images specified in the packer templates.
//
// **Parameters:**
//
// packerTemplates: A slice of BlueprintPacker containing the images to tag
// and push.
//
// **Returns:**
//
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) TagAndPushImages(packerTemplates []packer.BlueprintPacker) error {
	registryServer := viper.GetString("container.registry.server")
	registryUsername := viper.GetString("container.registry.username")
	githubToken := viper.GetString("container.registry.token")

	authStr, err := d.DockerLogin(registryUsername, githubToken, registryServer)
	if err != nil {
		return err
	}

	for _, pTmpl := range packerTemplates {
		imageName := pTmpl.Tag.Name

		var imageTags []string

		for arch, hash := range pTmpl.ImageHashes {
			localTag := fmt.Sprintf("sha256:%s", hash)
			remoteTag := fmt.Sprintf("%s/%s:%s", registryServer, imageName, arch)

			if err := d.DockerTag(localTag, remoteTag); err != nil {
				return err
			}

			if err := d.DockerPush(remoteTag, authStr); err != nil {
				return err
			}

			imageTags = append(imageTags, remoteTag)
		}

		if len(imageTags) > 1 {
			manifestName := fmt.Sprintf("%s/%s:latest", registryServer, imageName)
			fmt.Printf("manifest creation and push needs to be handled by CLI or other means: %s\n", manifestName)
		} else {
			fmt.Printf("not enough images for manifest creation: %v\n", imageTags)
		}
	}
	return nil
}
