package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func unwrapError(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			break
		}
		err = unwrapped
	}
	return err
}

// CreateAndPushManifest creates a manifest list and pushes it to a registry.
//
// **Parameters:**
//
// blueprint: The blueprint containing image tag information.
// imageTags: A slice of image tags to include in the manifest list.
//
// **Returns:**
//
// error: An error if any operation fails during manifest creation or pushing.
func (d *DockerClient) CreateAndPushManifest(blueprint *bp.Blueprint, imageTags []string) error {
	if len(imageTags) == 0 {
		return errors.New("no image tags provided for manifest creation")
	}

	targetImage := fmt.Sprintf("%s/%s:%s",
		strings.TrimPrefix(d.Container.ImageRegistry.Server, "https://"),
		blueprint.Tag.Name, blueprint.Tag.Version)

	fmt.Printf("Creating manifest list for %s with %v tags\n", targetImage, imageTags)
	manifestList, err := d.CreateManifest(context.Background(), targetImage, imageTags)
	if err != nil {
		rootErr := unwrapError(err)
		return fmt.Errorf("failed to create manifest list for %s: %v", targetImage, rootErr)
	}

	fmt.Println("Manifest list contents:")
	for _, instance := range manifestList.Manifests {
		fmt.Printf("  Digest: %s\n", instance.Digest)
		fmt.Printf("  Platform: %s/%s\n", instance.Platform.Architecture, instance.Platform.OS)
		fmt.Printf("  Size: %d\n", instance.Size)
	}

	fmt.Printf("Pushing manifest list for %s\n", targetImage)
	if err := d.PushManifest(targetImage, manifestList); err != nil {
		rootErr := unwrapError(err)
		return fmt.Errorf("failed to push manifest list for %s, error: %v", targetImage, rootErr)
	}

	return nil
}

// CreateManifest creates a manifest list with the input image tags
// and the specified target image.
//
// **Parameters:**
//
// ctx: The context within which the manifest list is created.
// targetImage: The name of the image to create the manifest list for.
// imageTags: A slice of image tags to include in the manifest list.
//
// **Returns:**
//
// ocispec.Index: The manifest list created with the input image tags.
// error: An error if any operation fails during the manifest list creation.
func (d *DockerClient) CreateManifest(ctx context.Context, targetImage string, imageTags []string) (ocispec.Index, error) {
	fmt.Printf("Creating manifest list for %s with %v tags\n", targetImage, imageTags)

	manifestList := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{},
	}

	fmt.Printf("Image hashes: %+v\n", d.Container.ImageHashes)
	for _, imgHash := range d.Container.ImageHashes {
		parsedDigest, err := digest.Parse(fmt.Sprintf("sha256:%s", imgHash.Hash))
		if err != nil {
			return manifestList, fmt.Errorf("failed to parse digest sha256:%s: %v", imgHash.Hash, err)
		}

		size, err := d.GetImageSize(parsedDigest.String())
		if err != nil {
			return manifestList, fmt.Errorf("failed to get size for sha256:%s: %v", imgHash.Hash, err)
		}

		manifestList.Manifests = append(manifestList.Manifests, ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    parsedDigest,
			Size:      size,
			Platform:  &ocispec.Platform{OS: imgHash.OS, Architecture: imgHash.Arch},
		})
	}

	return manifestList, nil
}

// GetImageSize returns the size of the image with the input reference.
//
// **Parameters:**
//
// imageRef: The reference of the image to get the size of.
//
// **Returns:**
//
// int64: The size of the image in bytes
// error: An error if any operation fails during the size retrieval
func (d *DockerClient) GetImageSize(imageRef string) (int64, error) {
	ctx := context.Background()
	imageInspect, contents, err := d.CLI.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		return 0, err
	}
	if contents == nil {
		return 0, fmt.Errorf("image contents are nil")
	}

	return imageInspect.Size, nil
}

// PushManifest pushes the input manifest list to the registry.
//
// **Parameters:**
//
// imageName: The name of the image to push the manifest list for.
// manifestList: The manifest list to push.
//
// **Returns:**
//
// error: An error if any operation fails during the push.
func (d *DockerClient) PushManifest(imageName string, manifestList ocispec.Index) error {
	targetRef, err := name.ParseReference(imageName)
	if err != nil {
		return fmt.Errorf("failed to parse target reference: %v", err)
	}

	idx, err := remote.Index(targetRef, remote.WithAuth(&authn.Basic{
		Username: d.Container.ImageRegistry.Username,
		Password: d.Container.ImageRegistry.Credential,
	}))
	if err != nil {
		return fmt.Errorf("failed to parse manifest list: %v", err)
	}

	options := []remote.Option{
		remote.WithAuth(authn.FromConfig(authn.AuthConfig{
			Username: d.Container.ImageRegistry.Username,
			Password: d.Container.ImageRegistry.Credential,
		})),
	}

	if err := remote.WriteIndex(targetRef, idx, options...); err != nil {
		return fmt.Errorf("failed to push manifest list: %v", err)
	}

	fmt.Printf("Manifest list for %s pushed successfully\n", imageName)
	return nil
}
