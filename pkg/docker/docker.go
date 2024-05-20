package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/ocischema"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockerClient "github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
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
	DockerLogin(containerImageRegistry packer.ContainerImageRegistry) error
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
// ExecCommand: Command for executing Docker commands.
// AuthStr: Auth string for the Docker registry.
type DockerClient struct {
	AuthStr                string
	CLI                    dockerClient.APIClient
	ExecCommand            func(name string, arg ...string) *exec.Cmd
	ManifestList           distribution.Manifest
	ContainerImageRegistry packer.ContainerImageRegistry
}

// NewDockerClient creates a new Docker client.
//
// **Returns:**
//
// *DockerClient: A DockerClient instance.
// error: An error if any issue occurs while creating the client.
func NewDockerClient() (*DockerClient, error) {
	cli, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv, dockerClient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{CLI: cli, ExecCommand: exec.Command}, nil
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
func (d *DockerClient) DockerLogin(containerImageRegistry packer.ContainerImageRegistry) error {
	if containerImageRegistry.Username == "" || containerImageRegistry.Credential == "" || containerImageRegistry.Server == "" {
		return errors.New("username, password, and server must not be empty")
	}

	authConfig := registry.AuthConfig{
		Username:      containerImageRegistry.Username,
		Password:      containerImageRegistry.Credential,
		ServerAddress: containerImageRegistry.Server,
	}

	authBytes, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	d.AuthStr = base64.URLEncoding.EncodeToString(authBytes)

	ctx := context.Background()
	_, err = d.CLI.RegistryLogin(ctx, authConfig)
	if err != nil {
		return err
	}

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
// credentialStore implements auth.CredentialStore
type credentialStore struct {
	authConfig registry.AuthConfig
}

func (cs *credentialStore) Basic(u *url.URL) (string, string) {
	return cs.authConfig.Username, cs.authConfig.Password
}

func (cs *credentialStore) RefreshToken(u *url.URL, service string) string {
	return ""
}

func (cs *credentialStore) SetRefreshToken(u *url.URL, service, refreshToken string) {
}

// DockerManifestCreate creates a Docker manifest that references multiple platform-specific versions of an image.
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
	ctx := context.Background()

	builder := ocischema.NewManifestBuilder(nil, nil, nil)

	for _, img := range images {
		imgRef, err := reference.ParseNormalizedNamed(img)
		if err != nil {
			return fmt.Errorf("failed to parse image reference: %v", err)
		}

		imgRepo, err := client.NewRepository(imgRef, d.ContainerImageRegistry.Server, http.DefaultTransport)
		if err != nil {
			return fmt.Errorf("failed to create image repository: %v", err)
		}

		imgManifestService, err := imgRepo.Manifests(ctx)
		if err != nil {
			return fmt.Errorf("failed to get image manifest service: %v", err)
		}

		tagsService := imgRepo.Tags(ctx)
		imgDigest, err := tagsService.Get(ctx, "latest")
		if err != nil {
			return fmt.Errorf("failed to get image digest: %v", err)
		}

		digestValue := digest.Digest(imgDigest.Digest)
		imgDescriptor, err := imgManifestService.Get(ctx, digestValue)
		if err != nil {
			return fmt.Errorf("failed to get image manifest: %v", err)
		}

		deserializedManifest, ok := imgDescriptor.(*ocischema.DeserializedManifest)
		if !ok {
			return fmt.Errorf("failed to assert type: %v", err)
		}

		for _, descriptor := range deserializedManifest.References() {
			err = builder.AppendReference(descriptor)
			if err != nil {
				return fmt.Errorf("failed to append image reference: %v", err)
			}
		}
	}

	finalManifest, err := builder.Build(ctx)
	if err != nil {
		return fmt.Errorf("failed to build manifest: %v", err)
	}

	d.ManifestList = finalManifest
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
	ctx := context.Background()

	ref, err := reference.ParseNormalizedNamed(manifest)
	if err != nil {
		return fmt.Errorf("failed to parse manifest reference: %v", err)
	}

	authConfig := registry.AuthConfig{
		Username:      d.ContainerImageRegistry.Username,
		Password:      d.ContainerImageRegistry.Credential,
		ServerAddress: d.ContainerImageRegistry.Server,
	}
	creds := &credentialStore{authConfig: authConfig}
	challengeManager := challenge.NewSimpleManager()
	tokenHandler := auth.NewTokenHandler(nil, creds, "repository", "pull", "push")
	basicHandler := auth.NewBasicHandler(creds)
	authorizer := auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)
	tr := transport.NewTransport(http.DefaultTransport, authorizer)
	repo, err := client.NewRepository(ref, d.ContainerImageRegistry.Server, tr)
	if err != nil {
		return fmt.Errorf("failed to create repository: %v", err)
	}

	manifestService, err := repo.Manifests(ctx)
	if err != nil {
		return fmt.Errorf("failed to get manifest service: %v", err)
	}

	builder := ocischema.NewManifestBuilder(nil, nil, nil)

	finalManifest, err := builder.Build(ctx)
	if err != nil {
		return fmt.Errorf("failed to build manifest: %v", err)
	}

	_, err = manifestService.Put(ctx, finalManifest)
	if err != nil {
		return fmt.Errorf("failed to put manifest: %v", err)
	}

	return nil
}

// TagAndPushImages tags and pushes images specified in packer templates.
//
// **Parameters:**
//
// packerTemplates: A slice of PackerTemplate containing the images to tag
// and push.
//
// **Returns:**
//
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) TagAndPushImages(packerTemplates []packer.PackerTemplate, token, bpName string, imageHashes map[string]string) error {
	if len(packerTemplates) == 0 {
		return errors.New("packer templates must be provided for the blueprint")
	}

	if token == "" {
		return errors.New("token used to authenticate with the registry must not be empty")
	}

	for _, pTmpl := range packerTemplates {
		pTmpl.Container.ImageHashes = imageHashes
		pTmpl.Container.Registry.Credential = token
		if err := d.processTemplate(pTmpl, bpName); err != nil {
			return err
		}
	}

	return nil
}

func (d *DockerClient) processTemplate(pTmpl packer.PackerTemplate, bpName string) error {
	pTmpl.Container.Registry.Server = viper.GetString("container.registry.server")
	pTmpl.Container.Registry.Username = viper.GetString("container.registry.username")

	if bpName == "" {
		return errors.New("image name in packer template must not be empty")
	}

	if pTmpl.Container.Registry.Username == "" || pTmpl.Container.Registry.Credential == "" || pTmpl.Container.Registry.Server == "" {
		return errors.New("registry server, username, and token must not be empty")
	}

	if d.AuthStr == "" {
		if err := d.DockerLogin(pTmpl.Container.Registry); err != nil {
			return fmt.Errorf("failed to login to %s: %v", pTmpl.Container.Registry.Server, err)
		}
	}

	fmt.Printf("Processing %s image...\n", bpName)

	var imageTags []string

	for arch, hash := range pTmpl.Container.ImageHashes {
		fmt.Printf("Processing image name: %s, arch: %s, hash: %s\n", bpName, arch, hash)
		err := d.processImageTag(bpName, arch, hash, pTmpl.Container.Registry, &imageTags)
		if err != nil {
			return err
		}
	}

	fmt.Printf("Image tags: %v\n", imageTags)
	if len(imageTags) > 0 { // Ensure manifest creation proceeds with one or more tags
		manifestName := fmt.Sprintf("%s/%s/%s:latest", pTmpl.Container.Registry.Server, pTmpl.Container.Registry.Username, bpName)
		if err := d.DockerManifestCreate(manifestName, imageTags); err != nil {
			return err
		}
		if err := d.DockerManifestPush(manifestName); err != nil {
			return err
		}
	} else {
		fmt.Printf("Not enough images for manifest creation: %v\n", imageTags)
	}

	return nil
}

func (d *DockerClient) processImageTag(imageName, arch, hash string, containerImageRegistry packer.ContainerImageRegistry, imageTags *[]string) error {
	if arch == "" || hash == "" {
		return errors.New("arch and hash must not be empty")
	}

	localTag := fmt.Sprintf("sha256:%s", hash)
	remoteTag := fmt.Sprintf("%s/%s/%s:%s", containerImageRegistry.Server, containerImageRegistry.Username, imageName, arch)

	fmt.Printf("Tagging image: %s as %s\n", localTag, remoteTag)

	if err := d.DockerTag(localTag, remoteTag); err != nil {
		return err
	}

	if remoteTag == "" || d.AuthStr == "" {
		return errors.New("containerImage and authStr must not be empty")
	}

	fmt.Printf("Pushing image: %s\n", remoteTag)

	if err := d.DockerPush(remoteTag); err != nil {
		return err
	}

	// Add the tag to the list for the manifest
	*imageTags = append(*imageTags, remoteTag)
	return nil
}
