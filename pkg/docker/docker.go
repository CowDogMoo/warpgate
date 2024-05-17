package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"

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
// DockerManifestCreate: Creates a Docker manifest.
// DockerManifestPush: Pushes a Docker manifest to a registry.
type DockerClientInterface interface {
	DockerLogin(username, password, server string) (string, error)
	DockerPush(image, authStr string) error
	DockerTag(sourceImage, targetImage string) error
	DockerManifestCreate(manifest string, images []string) error
	DockerManifestPush(manifest string) error
}

// DockerClient represents a Docker client.
//
// **Attributes:**
//
// CLI: API client for Docker operations.
type DockerClient struct {
	CLI     client.APIClient
	AuthStr string // Auth string to track the login session
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
func (d *DockerClient) DockerLogin(username, password, server string) error {
	if username == "" || password == "" || server == "" {
		return errors.New("username, password, and server must not be empty")
	}

	authConfig := map[string]string{
		"username":      username,
		"password":      password,
		"serveraddress": server,
	}

	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}

	d.AuthStr = base64.URLEncoding.EncodeToString(encodedJSON)

	return nil
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
func (d *DockerClient) DockerPush(containerImage string) error {
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
		return err
	}
	defer resp.Close()

	_, err = io.ReadAll(resp)
	return err
}

// DockerManifestCreate creates a Docker manifest that references multiple
// platform-specific versions of an image. It builds the manifest using the
// 'docker manifest create' command.
//
// **Parameters:**
//
// manifest: The name of the manifest to create.
// images: A slice of image names to include in the manifest.
//
// **Returns:**
//
// error: An error if the manifest creation fails.
func (d *DockerClient) DockerManifestCreate(manifest string, images []string) error {
	if manifest == "" {
		return errors.New("manifest must not be empty")
	}

	if len(images) == 0 {
		return errors.New("images must not be empty")
	}

	args := []string{"manifest", "create", manifest}
	for _, image := range images {
		args = append(args, "--amend", image)
	}

	cmd := exec.Command("docker", args...)
	var out bytes.Buffer
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker manifest create failed for %s: %s", manifest, out.String())
	}

	return nil
}

// DockerManifestPush pushes a Docker manifest to a registry. It uses the
// 'docker manifest push' command.
//
// **Parameters:**
//
// manifest: The name of the manifest to push.
//
// **Returns:**
//
// error: An error if the push operation fails.
func (d *DockerClient) DockerManifestPush(manifest string) error {
	if manifest == "" {
		return errors.New("manifest must not be empty")
	}

	cmd := exec.Command("docker", "manifest", "push", manifest)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker manifest push failed for %s: %s", manifest, out.String())
	}

	return nil
}

// TagAndPushImages tags and pushes images specified in packer templates.
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
	if len(packerTemplates) == 0 {
		return errors.New("packerTemplates must not be empty")
	}

	registryServer := viper.GetString("container.registry.server")
	registryUsername := viper.GetString("container.registry.username")
	githubToken := viper.GetString("container.registry.token")

	if registryServer == "" || registryUsername == "" || githubToken == "" {
		return errors.New("registry server, username, and token must not be empty")
	}

	if d.AuthStr == "" {
		if err := d.DockerLogin(registryUsername, githubToken, registryServer); err != nil {
			return fmt.Errorf("failed to login to %s: %v", registryServer, err)
		}
	}

	for _, pTmpl := range packerTemplates {
		if err := d.processTemplate(pTmpl, registryServer); err != nil {
			return err
		}
	}
	return nil
}

func (d *DockerClient) processTemplate(pTmpl packer.BlueprintPacker, registryServer string) error {
	imageName := pTmpl.Tag.Name
	if imageName == "" {
		return errors.New("image name in packer template must not be empty")
	}

	var imageTags []string

	for arch, hash := range pTmpl.ImageHashes {
		if err := d.processImageTag(imageName, arch, hash, registryServer, &imageTags); err != nil {
			return err
		}
	}

	if len(imageTags) > 1 {
		manifestName := fmt.Sprintf("%s/%s:latest", registryServer, imageName)
		if err := d.DockerManifestCreate(manifestName, imageTags); err != nil {
			return err
		}
		if err := d.DockerManifestPush(manifestName); err != nil {
			return err
		}
	} else {
		fmt.Printf("not enough images for manifest creation: %v\n", imageTags)
	}

	return nil
}

func (d *DockerClient) processImageTag(imageName, arch, hash, registryServer string, imageTags *[]string) error {
	if arch == "" || hash == "" {
		return errors.New("arch and hash must not be empty")
	}

	localTag := fmt.Sprintf("sha256:%s", hash)
	remoteTag := fmt.Sprintf("%s/%s:%s", registryServer, imageName, arch)

	if err := d.DockerTag(localTag, remoteTag); err != nil {
		return err
	}

	if remoteTag == "" || d.AuthStr == "" {
		return errors.New("containerImage and authStr must not be empty")
	}

	if err := d.DockerPush(remoteTag); err != nil {
		return err
	}

	*imageTags = append(*imageTags, remoteTag)
	return nil
}
