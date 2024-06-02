package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
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
		return fmt.Errorf("no image tags provided for manifest creation")
	}

	targetImage := fmt.Sprintf("%s/%s:%s",
		strings.TrimPrefix(d.Container.ImageRegistry.Server, "https://"),
		blueprint.Tag.Name, blueprint.Tag.Version)

	fmt.Printf("Creating manifest list for %s with %v tags\n", targetImage, imageTags)

	manifestList, err := d.CreateManifest(context.Background(), targetImage, imageTags)
	if err != nil {
		return fmt.Errorf("failed to create manifest list for %s: %v", targetImage, unwrapError(err))
	}

	fmt.Println("Pushing manifest list")
	if err := d.PushManifest(targetImage, manifestList); err != nil {
		return fmt.Errorf("failed to push manifest list for %s, error: %v", targetImage, unwrapError(err))
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
func (d *DockerClient) CreateManifest(ctx context.Context, targetImage string, imageTags []string) (v1.ImageIndex, error) {
	index := empty.Index
	withMediaType := mutate.IndexMediaType(index, types.OCIImageIndex)

	// Set up keychain for authentication
	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		github.Keychain,
	)

	for _, tag := range imageTags {
		// Use the tag directly instead of constructing the full image name
		fullImageName := tag
		ref, err := name.NewTag(fullImageName)
		if err != nil {
			return nil, fmt.Errorf("creating reference for image %s: %v", fullImageName, err)
		}

		image, err := remote.Image(ref, remote.WithAuthFromKeychain(keychain))
		if err != nil {
			return nil, fmt.Errorf("getting image %s: %v", fullImageName, err)
		}

		withMediaType = mutate.AppendManifests(withMediaType, mutate.IndexAddendum{
			Add: image,
		})
	}

	return withMediaType, nil
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
func (d *DockerClient) PushManifest(imageName string, manifestList v1.ImageIndex) error {
	targetRef, err := name.ParseReference(imageName)
	if err != nil {
		return fmt.Errorf("failed to parse target reference: %v", err)
	}

	// Set up keychain for authentication
	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		github.Keychain,
	)

	options := []remote.Option{
		remote.WithAuthFromKeychain(keychain),
	}

	if err := remote.WriteIndex(targetRef, manifestList, options...); err != nil {
		return fmt.Errorf("failed to push manifest list: %v", err)
	}

	fmt.Printf("Manifest list for %s pushed successfully\n", imageName)
	return nil
}
