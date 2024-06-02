package docker

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/storage"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/types"
	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	dockerRegistry "github.com/docker/docker/api/types/registry"
)

// DockerRegistry represents a Docker registry with runtime and
// storage information.
//
// **Attributes:**
//
// Runtime: A libimage.Runtime instance.
// Store: A storage.Store instance.
// RegistryURL: URL of the Docker registry.
// AuthToken: Authentication token for the registry.
// Authenticator: An instance of an Authenticator for Docker registry.
type DockerRegistry struct {
	Runtime       *libimage.Runtime
	Store         storage.Store
	RegistryURL   string
	AuthToken     string
	Authenticator authn.Authenticator
}

// CustomAuthenticator provides authentication details for Docker registry.
//
// **Attributes:**
//
// Username: Username for the registry.
// Password: Password for the registry.
type CustomAuthenticator struct {
	Username string
	Password string
}

// Authorization returns the value to use in an http transport's Authorization header.
func (h *CustomAuthenticator) Authorization() (*authn.AuthConfig, error) {
	return &authn.AuthConfig{
		Username: h.Username,
		Password: h.Password,
	}, nil
}

// NewDockerRegistry creates a new Docker registry.
//
// **Parameters:**
//
// registryURL: The URL of the Docker registry.
// authToken: The authentication token for the registry.
// registryConfig: A packer.ContainerImageRegistry instance.
// getStore: A function that returns a storage.Store instance.
// ignoreChownErrors: A boolean indicating whether to ignore chown errors.
//
// **Returns:**
//
// *DockerRegistry: A DockerRegistry instance.
// error: An error if any issue occurs while creating the registry.
func NewDockerRegistry(registryURL, authToken string, registryConfig packer.ContainerImageRegistry, getStore GetStoreFunc, ignoreChownErrors bool) (*DockerRegistry, error) {
	if registryURL == "" {
		return nil, errors.New("registry URL must not be empty")
	}

	storeOpts, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("error getting default store options: %v", err)
	}

	if storeOpts.GraphDriverName == "overlay" {
		storeOpts.GraphDriverOptions = append(storeOpts.GraphDriverOptions, "overlay.mount_program=/usr/bin/fuse-overlayfs")
		storeOpts.GraphDriverOptions = append(storeOpts.GraphDriverOptions, "ignore_chown_errors=true")
	} else if storeOpts.GraphDriverName == "vfs" {
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
		Runtime:       runtime,
		Store:         store,
		RegistryURL:   registryURL,
		AuthToken:     authToken,
		Authenticator: &CustomAuthenticator{Username: registryConfig.Username, Password: registryConfig.Credential},
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

// encodeAuthToBase64 encodes the authConfig to a base64 string.
func encodeAuthToBase64(authConfig dockerRegistry.AuthConfig) (string, error) {
	authJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("error marshalling authConfig to JSON: %v", err)
	}
	return base64.URLEncoding.EncodeToString(authJSON), nil
}

// SetRegistry sets the DockerRegistry for the DockerClient.
//
// **Parameters:**
//
// registry: A pointer to the DockerRegistry to be set.
func (d *DockerClient) SetRegistry(registry *DockerRegistry) {
	d.Registry = registry
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

	fmt.Printf("Image hashes: %+v\n", d.Container.ImageHashes)

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
