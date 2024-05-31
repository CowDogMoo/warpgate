package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/storage"

	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/types"
	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/docker/api/types/image"
	dockerRegistry "github.com/docker/docker/api/types/registry"
	dockerClient "github.com/docker/docker/client"
)

// GetStoreFunc represents a function that returns a storage.Store
// instance.
//
// **Parameters:**
//
// options: Storage options for the store.
//
// **Returns:**
//
// storage.Store: A storage.Store instance.
// error: An error if any issue occurs while getting the store.
type GetStoreFunc func(options storage.StoreOptions) (storage.Store, error)

// DockerClientInterface represents an interface for Docker client
// operations.
//
// **Methods:**
//
// DockerLogin: Authenticates with a Docker registry.
// DockerPush: Pushes a Docker image to a registry.
// DockerTag: Tags a Docker image with a new name.
type DockerClientInterface interface {
	DockerLogin() error
	DockerPush(image string) error
	DockerTag(sourceImage, targetImage string) error
}

// DockerClient represents a Docker client.
//
// **Attributes:**
//
// AuthStr: The base64 encoded auth string for the Docker registry.
// CLI: API client for Docker operations.
// Container: A packer.Container instance.
// Registry: A distribution.Namespace instance.
type DockerClient struct {
	AuthStr   string
	CLI       *dockerClient.Client
	Container packer.Container
	Registry  *DockerRegistry
}

// DockerRegistry represents a Docker registry with runtime and
// storage information.
//
// **Attributes:**
//
// Runtime: A libimage.Runtime instance.
// Store: A storage.Store instance.
// RegistryURL: URL of the Docker registry.
// AuthToken: Authentication token for the registry.
type DockerRegistry struct {
	Runtime     *libimage.Runtime
	Store       storage.Store
	RegistryURL string
	AuthToken   string
}

// NewDockerRegistry creates a new Docker registry.
//
// **Parameters:**
//
// registryURL: The URL of the Docker registry.
// authToken: The authentication token for the registry.
//
// **Returns:**
//
// *DockerRegistry: A DockerRegistry instance.
// error: An error if any issue occurs while creating the registry.
func NewDockerRegistry(registryURL, authToken string, getStore GetStoreFunc, ignoreChownErrors bool) (*DockerRegistry, error) {
	if registryURL == "" {
		return nil, errors.New("registry URL must not be empty")
	}

	storeOpts, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("error getting default store options: %v", err)
	}

	// Check if the driver is vfs and remove ignore_chown_errors if necessary
	if storeOpts.GraphDriverName == "vfs" {
		ignoreChownErrors = false
	}

	runtimeOpts := &libimage.RuntimeOptions{
		SystemContext: &types.SystemContext{},
	}

	runtime, err := libimage.RuntimeFromStoreOptions(runtimeOpts, &storeOpts)
	if err != nil {
		return nil, fmt.Errorf("error getting runtime from store options: %v", err)
	}

	store, err := getStore(storeOpts)
	if err != nil {
		if ignoreChownErrors && (os.IsPermission(err) || strings.Contains(err.Error(), "operation not permitted")) {
			fmt.Println("Warning: Ignoring chown errors as configured.")
			store, err = getStore(storeOpts)
			if err != nil {
				return nil, fmt.Errorf("retry error getting storage store: %v", err)
			}
		} else {
			return nil, fmt.Errorf("error getting storage store: %v", err)
		}
	}

	return &DockerRegistry{
		Runtime:     runtime,
		Store:       store,
		RegistryURL: registryURL,
		AuthToken:   authToken,
	}, nil
}

// DefaultGetStore returns a storage.Store instance with the provided
// options.
//
// **Parameters:**
//
// options: Storage options for the store.
//
// **Returns:**
//
// storage.Store: A storage.Store instance.
// error: An error if any issue occurs while getting the store.
func DefaultGetStore(options storage.StoreOptions) (storage.Store, error) {
	return storage.GetStore(options)
}

// NewDockerClient creates a new Docker client.
//
// **Returns:**
//
// *DockerClient: A DockerClient instance.
// error: An error if any issue occurs while creating the client.
func NewDockerClient(registryURL, authToken string) (*DockerClient, error) {
	cli, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv, dockerClient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating Docker client: %v", err)
	}

	dockerRegistry, err := NewDockerRegistry(registryURL, authToken, DefaultGetStore, true)
	if err != nil {
		return nil, fmt.Errorf("error creating Docker registry: %v", err)
	}

	return &DockerClient{
		CLI: cli,
		Container: packer.Container{
			ImageRegistry: packer.ContainerImageRegistry{},
			ImageHashes:   []packer.ImageHash{},
		},
		Registry: dockerRegistry,
	}, nil
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

// encodeAuthToBase64 encodes the authConfig to a base64 string.
func encodeAuthToBase64(authConfig dockerRegistry.AuthConfig) (string, error) {
	authJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("error marshalling authConfig to JSON: %v", err)
	}
	return base64.URLEncoding.EncodeToString(authJSON), nil
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

// SetRegistry sets the DockerRegistry for the DockerClient.
//
// **Parameters:**
//
// registry: A pointer to the DockerRegistry to be set.
func (d *DockerClient) SetRegistry(registry *DockerRegistry) {
	d.Registry = registry
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

// ProcessPackerTemplates processes a list of Packer templates by
// tagging and pushing images to a registry.
//
// **Parameters:**
//
// pTmpl: A slice of PackerTemplate instances to process.
// blueprint: The blueprint containing tag information.
//
// **Returns:**
//
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) ProcessPackerTemplates(pTmpl []packer.PackerTemplate, blueprint bp.Blueprint) error {
	if len(pTmpl) == 0 {
		return errors.New("packer templates must be provided for the blueprint")
	}

	for _, p := range pTmpl {
		if err := d.ProcessTemplate(p, blueprint); err != nil {
			return fmt.Errorf("error processing Packer template: %v", err)
		}
	}

	return nil
}

// TagAndPushImages tags and pushes images to a registry based on
// the provided blueprint.
//
// **Parameters:**
//
// blueprint: The blueprint containing tag information.
//
// **Returns:**
//
// []string: A slice of image tags that were successfully pushed.
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) TagAndPushImages(blueprint *bp.Blueprint) ([]string, error) {
	var imageTags []string

	fmt.Printf("Image hashes: %+v\n", d.Container.ImageHashes) // Debugging line

	for _, hash := range d.Container.ImageHashes {
		if hash.Arch == "" || hash.Hash == "" || hash.OS == "" {
			return imageTags, errors.New("arch, hash, and OS must not be empty")
		}

		localTag := fmt.Sprintf("sha256:%s", hash.Hash)
		remoteTag := fmt.Sprintf("%s/%s:%s",
			strings.TrimPrefix(d.Container.ImageRegistry.Server, "https://"),
			blueprint.Tag.Name, hash.Arch)
		fmt.Printf("Tagging image: %s as %s\n", localTag, remoteTag)

		if err := d.DockerTag(localTag, remoteTag); err != nil {
			return imageTags, err
		}

		fmt.Printf("Pushing image: %s\n", remoteTag)

		if err := d.PushImage(remoteTag); err != nil {
			return imageTags, err
		}

		imageTags = append(imageTags, remoteTag)
	}

	if len(imageTags) == 0 {
		return imageTags, errors.New("no images were tagged and pushed")
	}

	return imageTags, nil
}

// ProcessTemplate processes a Packer template by tagging and pushing images
// to a registry.
//
// **Parameters:**
//
// pTmpl: A PackerTemplate containing the image to process.
// blueprint: The blueprint containing tag information.
//
// **Returns:**
//
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) ProcessTemplate(pTmpl packer.PackerTemplate, blueprint bp.Blueprint) error {
	if blueprint.Name == "" {
		return errors.New("blueprint name must not be empty")
	}

	if blueprint.Tag.Name == "" || blueprint.Tag.Version == "" {
		return errors.New("blueprint tag name and version must not be empty")
	}

	if pTmpl.Container.ImageRegistry.Server == "" || pTmpl.Container.ImageRegistry.Username == "" || pTmpl.Container.ImageRegistry.Credential == "" {
		return fmt.Errorf("registry server '%s', username '%s', and credential must not be empty", pTmpl.Container.ImageRegistry.Server, pTmpl.Container.ImageRegistry.Username)
	}

	d.Container.ImageRegistry = pTmpl.Container.ImageRegistry
	d.Container.ImageHashes = pTmpl.Container.ImageHashes

	if d.AuthStr == "" {
		if err := d.DockerLogin(); err != nil {
			return fmt.Errorf("failed to login to %s: %v", pTmpl.Container.ImageRegistry.Server, err)
		}
	}

	// Remove any entries with empty ImageHashes
	for i := 0; i < len(d.Container.ImageHashes); i++ {
		if d.Container.ImageHashes[i].Hash == "" {
			d.Container.ImageHashes = append(d.Container.ImageHashes[:i], d.Container.ImageHashes[i+1:]...)
			i--
		}
	}
	fmt.Printf("Processing %s image with the following hashes: %+v\n", blueprint.Name, d.Container.ImageHashes) // Debugging line

	imageTags, err := d.TagAndPushImages(&blueprint)
	if err != nil {
		return err
	}

	if err := d.CreateAndPushManifest(&blueprint, imageTags); err != nil {
		return err
	}

	return nil
}
