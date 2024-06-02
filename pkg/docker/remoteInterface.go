package docker

import (
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// RemoteInterface represents an interface for remote operations.
//
// **Methods:**
//
// Image: Retrieves a container image from a registry.
type RemoteInterface interface {
	Image(ref name.Reference, options ...remote.Option) (v1.Image, error)
}

// RemoteClient represents a remote client.
type RemoteClient struct{}

// Image retrieves a container image from a registry.
//
// **Parameters:**
//
// ref: The name.Reference instance for the image.
// options: Options for the remote operation.
//
// **Returns:**
//
// v1.Image: The container image.
// error: An error if the operation fails.
func (rc *RemoteClient) Image(ref name.Reference, options ...remote.Option) (v1.Image, error) {
	return remote.Image(ref, options...)
}
