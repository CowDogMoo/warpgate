package buildkit

import (
	"bytes"
	"context"
	"io"

	dockercontainer "github.com/moby/moby/api/types/container"
	dockerimage "github.com/moby/moby/api/types/image"
	dockerregistry "github.com/moby/moby/api/types/registry"
	dockerclient "github.com/moby/moby/client"
)

// ImagePushOptions configures image push operations.
// Defined locally to decouple from Docker SDK internal type changes.
type ImagePushOptions struct {
	All          bool
	RegistryAuth string
}

// ImageRemoveOptions configures image removal operations.
type ImageRemoveOptions struct {
	Force         bool
	PruneChildren bool
}

// ImageLoadResponse holds the response from an image load operation.
type ImageLoadResponse struct {
	Body io.ReadCloser
	JSON bool
}

// ContainerListOptions configures container listing operations.
type ContainerListOptions struct {
	All bool
}

// PingResponse represents a Docker daemon ping response.
type PingResponse struct{}

// DockerClient defines the interface for Docker operations needed by BuildKit builder.
type DockerClient interface {
	// Image operations
	ImagePush(ctx context.Context, image string, options ImagePushOptions) (io.ReadCloser, error)
	ImageTag(ctx context.Context, source, target string) error
	ImageRemove(ctx context.Context, imageID string, options ImageRemoveOptions) ([]dockerimage.DeleteResponse, error)
	ImageLoad(ctx context.Context, input io.Reader) (ImageLoadResponse, error)
	ImageInspect(ctx context.Context, imageID string) (dockerimage.InspectResponse, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (dockerimage.InspectResponse, []byte, error)

	// Distribution operations
	DistributionInspect(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error)

	// Container operations (for builder detection)
	ContainerList(ctx context.Context, options ContainerListOptions) ([]dockercontainer.Summary, error)
	Ping(ctx context.Context) (PingResponse, error)

	// Lifecycle
	Close() error
}

// dockerClientAdapter wraps the real Docker client to match our interface.
// The adapter is necessary because the Docker SDK uses variadic options for
// some methods, while our interface uses fixed parameters for simplicity.
type dockerClientAdapter struct {
	*dockerclient.Client
}

// Verify that dockerClientAdapter implements DockerClient at compile time
var _ DockerClient = (*dockerClientAdapter)(nil)

// ImagePush adapts the Docker SDK's new signature to our fixed interface.
func (a *dockerClientAdapter) ImagePush(ctx context.Context, image string, options ImagePushOptions) (io.ReadCloser, error) {
	return a.Client.ImagePush(ctx, image, dockerclient.ImagePushOptions{
		All:          options.All,
		RegistryAuth: options.RegistryAuth,
	})
}

// ImageTag adapts the Docker SDK's new signature to our fixed interface.
func (a *dockerClientAdapter) ImageTag(ctx context.Context, source, target string) error {
	_, err := a.Client.ImageTag(ctx, dockerclient.ImageTagOptions{Source: source, Target: target})
	return err
}

// ImageRemove adapts the Docker SDK's new signature to our fixed interface.
func (a *dockerClientAdapter) ImageRemove(ctx context.Context, imageID string, options ImageRemoveOptions) ([]dockerimage.DeleteResponse, error) {
	result, err := a.Client.ImageRemove(ctx, imageID, dockerclient.ImageRemoveOptions{
		Force:         options.Force,
		PruneChildren: options.PruneChildren,
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ImageLoad adapts the Docker SDK's variadic signature to our fixed interface.
func (a *dockerClientAdapter) ImageLoad(ctx context.Context, input io.Reader) (ImageLoadResponse, error) {
	result, err := a.Client.ImageLoad(ctx, input)
	if err != nil {
		return ImageLoadResponse{}, err
	}
	return ImageLoadResponse{Body: result}, nil
}

// ImageInspect adapts the Docker SDK's variadic signature to our fixed interface.
func (a *dockerClientAdapter) ImageInspect(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
	result, err := a.Client.ImageInspect(ctx, imageID)
	if err != nil {
		return dockerimage.InspectResponse{}, err
	}
	return result.InspectResponse, nil
}

// ImageInspectWithRaw adapts the Docker SDK's new signature to our fixed interface.
func (a *dockerClientAdapter) ImageInspectWithRaw(ctx context.Context, imageID string) (dockerimage.InspectResponse, []byte, error) {
	var raw bytes.Buffer
	result, err := a.Client.ImageInspect(ctx, imageID, dockerclient.ImageInspectWithRawResponse(&raw))
	if err != nil {
		return dockerimage.InspectResponse{}, nil, err
	}
	return result.InspectResponse, raw.Bytes(), nil
}

// DistributionInspect adapts the Docker SDK's new signature to our fixed interface.
func (a *dockerClientAdapter) DistributionInspect(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error) {
	result, err := a.Client.DistributionInspect(ctx, image, dockerclient.DistributionInspectOptions{
		EncodedRegistryAuth: encodedRegistryAuth,
	})
	if err != nil {
		return dockerregistry.DistributionInspect{}, err
	}
	return result.DistributionInspect, nil
}

// ContainerList adapts the Docker SDK's new signature to our fixed interface.
func (a *dockerClientAdapter) ContainerList(ctx context.Context, options ContainerListOptions) ([]dockercontainer.Summary, error) {
	result, err := a.Client.ContainerList(ctx, dockerclient.ContainerListOptions{All: options.All})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Ping adapts the Docker SDK's new signature to our fixed interface.
func (a *dockerClientAdapter) Ping(ctx context.Context) (PingResponse, error) {
	_, err := a.Client.Ping(ctx, dockerclient.PingOptions{})
	return PingResponse{}, err
}

// newDockerClientAdapter wraps a real Docker client to satisfy the DockerClient interface.
func newDockerClientAdapter(c *dockerclient.Client) DockerClient {
	return &dockerClientAdapter{Client: c}
}
