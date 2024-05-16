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

type DockerClientInterface interface {
	DockerLogin(username, password, server string) (string, error)
	DockerPush(image, authStr string) error
	DockerTag(sourceImage, targetImage string) error
}

type DockerClient struct {
	cli client.APIClient
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{cli: cli}, nil
}

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

func (d *DockerClient) DockerPush(containerImage, authStr string) error {
	ctx := context.Background()
	resp, err := d.cli.ImagePush(ctx, containerImage, image.PushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		return err
	}
	defer resp.Close()
	_, err = io.ReadAll(resp)
	return err
}

func (d *DockerClient) DockerTag(sourceImage, targetImage string) error {
	ctx := context.Background()
	return d.cli.ImageTag(ctx, sourceImage, targetImage)
}

func (d *DockerClient) PushImages(packerTemplates []packer.BlueprintPacker) error {
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
			fmt.Printf("Manifest creation and push needs to be handled by CLI or other means: %s\n", manifestName)
		} else {
			fmt.Printf("Not enough images for manifest creation: %v\n", imageTags)
		}
	}
	return nil
}
