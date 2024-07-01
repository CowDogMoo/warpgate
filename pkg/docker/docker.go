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
func NewDockerRegistry(registryConfig packer.ContainerImageRegistry, getStore GetStoreFunc, ignoreChownErrors bool) (*DockerRegistry, error) {
	if registryConfig.Server == "" {
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
		RegistryURL:   registryConfig.Server,
		AuthToken:     registryConfig.Credential,
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
