package buildkit

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
)

// DockerClient defines the interface for Docker operations needed by BuildKit builder.
// This interface allows for easier testing by enabling mock implementations.
type DockerClient interface {
	// Image operations
	ImagePush(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error)
	ImageTag(ctx context.Context, source, target string) error
	ImageRemove(ctx context.Context, imageID string, options dockerimage.RemoveOptions) ([]dockerimage.DeleteResponse, error)
	ImageLoad(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error)
	ImageInspect(ctx context.Context, imageID string) (dockerimage.InspectResponse, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (dockerimage.InspectResponse, []byte, error)

	// Distribution operations
	DistributionInspect(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error)

	// Container operations (for builder detection)
	ContainerList(ctx context.Context, options dockercontainer.ListOptions) ([]dockercontainer.Summary, error)
	Ping(ctx context.Context) (types.Ping, error)

	// Lifecycle
	Close() error
}

// dockerClientAdapter wraps the real Docker client to match our interface.
// The adapter is necessary because the Docker SDK uses variadic options for
// some methods, while our interface uses fixed parameters for simplicity.
type dockerClientAdapter struct {
	*dockerclient.Client
}

// ImageInspect adapts the Docker SDK's variadic signature to our fixed interface.
func (a *dockerClientAdapter) ImageInspect(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
	return a.Client.ImageInspect(ctx, imageID)
}

// ImageLoad adapts the Docker SDK's variadic signature to our fixed interface.
func (a *dockerClientAdapter) ImageLoad(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
	return a.Client.ImageLoad(ctx, input)
}

// newDockerClientAdapter wraps a real Docker client to satisfy the DockerClient interface.
func newDockerClientAdapter(c *dockerclient.Client) DockerClient {
	return &dockerClientAdapter{Client: c}
}
