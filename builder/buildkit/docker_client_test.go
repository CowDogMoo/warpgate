package buildkit

import (
	"context"
	"io"
	"strings"

	dockercontainer "github.com/moby/moby/api/types/container"
	dockerimage "github.com/moby/moby/api/types/image"
	dockerregistry "github.com/moby/moby/api/types/registry"
)

// MockDockerClient is a mock implementation of DockerClient for testing.
type MockDockerClient struct {
	ImagePushFunc           func(ctx context.Context, image string, options ImagePushOptions) (io.ReadCloser, error)
	ImageTagFunc            func(ctx context.Context, source, target string) error
	ImageRemoveFunc         func(ctx context.Context, imageID string, options ImageRemoveOptions) ([]dockerimage.DeleteResponse, error)
	ImageLoadFunc           func(ctx context.Context, input io.Reader) (ImageLoadResponse, error)
	ImageInspectFunc        func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error)
	ImageInspectWithRawFunc func(ctx context.Context, imageID string) (dockerimage.InspectResponse, []byte, error)
	DistributionInspectFunc func(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error)
	ContainerListFunc       func(ctx context.Context, options ContainerListOptions) ([]dockercontainer.Summary, error)
	PingFunc                func(ctx context.Context) (PingResponse, error)
}

func (m *MockDockerClient) ImagePush(ctx context.Context, image string, options ImagePushOptions) (io.ReadCloser, error) {
	if m.ImagePushFunc != nil {
		return m.ImagePushFunc(ctx, image, options)
	}
	return io.NopCloser(strings.NewReader("{}")), nil
}

func (m *MockDockerClient) ImageTag(ctx context.Context, source, target string) error {
	if m.ImageTagFunc != nil {
		return m.ImageTagFunc(ctx, source, target)
	}
	return nil
}

func (m *MockDockerClient) ImageRemove(ctx context.Context, imageID string, options ImageRemoveOptions) ([]dockerimage.DeleteResponse, error) {
	if m.ImageRemoveFunc != nil {
		return m.ImageRemoveFunc(ctx, imageID, options)
	}
	return []dockerimage.DeleteResponse{{Deleted: imageID}}, nil
}

func (m *MockDockerClient) ImageLoad(ctx context.Context, input io.Reader) (ImageLoadResponse, error) {
	if m.ImageLoadFunc != nil {
		return m.ImageLoadFunc(ctx, input)
	}
	return ImageLoadResponse{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (m *MockDockerClient) ImageInspect(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
	if m.ImageInspectFunc != nil {
		return m.ImageInspectFunc(ctx, imageID)
	}
	return dockerimage.InspectResponse{}, nil
}

func (m *MockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (dockerimage.InspectResponse, []byte, error) {
	if m.ImageInspectWithRawFunc != nil {
		return m.ImageInspectWithRawFunc(ctx, imageID)
	}
	return dockerimage.InspectResponse{}, nil, nil
}

func (m *MockDockerClient) DistributionInspect(ctx context.Context, image, encodedRegistryAuth string) (dockerregistry.DistributionInspect, error) {
	if m.DistributionInspectFunc != nil {
		return m.DistributionInspectFunc(ctx, image, encodedRegistryAuth)
	}
	return dockerregistry.DistributionInspect{}, nil
}

func (m *MockDockerClient) ContainerList(ctx context.Context, options ContainerListOptions) ([]dockercontainer.Summary, error) {
	if m.ContainerListFunc != nil {
		return m.ContainerListFunc(ctx, options)
	}
	return []dockercontainer.Summary{}, nil
}

func (m *MockDockerClient) Ping(ctx context.Context) (PingResponse, error) {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return PingResponse{}, nil
}

func (m *MockDockerClient) Close() error {
	return nil
}
