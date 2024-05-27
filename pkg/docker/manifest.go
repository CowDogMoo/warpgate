package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/libimage/manifests"
	"github.com/containers/storage"
	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ManifestList struct {
	image *libimage.Image
	list  manifests.List
	store storage.Store
}

// Remove removes the manifest list.
func (m *ManifestList) Remove(ctx context.Context) error {
	_, err := m.store.DeleteImage(m.image.ID(), true)
	return err
}

func (m *ManifestList) saveAndReload() error {
	newID, err := m.list.SaveToImage(m.store, m.image.ID(), nil, "")
	if err != nil {
		return err
	}

	_, list, err := manifests.LoadFromImage(m.store, newID)
	if err != nil {
		return err
	}
	m.list = list
	return nil
}

// RemoveInstance removes the instance specified by `d` from the manifest list.
func (m *ManifestList) RemoveInstance(ctx context.Context, d digest.Digest) error {
	if err := m.list.Remove(d); err != nil {
		return err
	}

	// Write the changes to disk.
	return m.saveAndReload()
}

// RemoveManifestList removes a manifest list specified by `name`.
// func (d *DockerClient) RemoveManifestList(ctx context.Context, name string) error {
// 	mList, err := d.Registry.Runtime.LookupManifestList(name)
// 	if err != nil {
// 		// If the manifest list is not found, consider it removed
// 		if errors.Is(err, storage.ErrImageUnknown) || errors.Is(err, libimage.ErrNotAManifestList) {
// 			fmt.Printf("Manifest list '%s' not found, skipping removal.\n", name)
// 			return nil
// 		}
// 		return err
// 	}

// 	for _, result := range results {
// 		if result != nil {
// 			return fmt.Errorf("failed to remove manifest list: %v", result)
// 		}
// 	}

// 	for _, result := range results {
// 		if result.Error != nil {
// 			return fmt.Errorf("failed to remove manifest list: %v", result.Error)
// 		}
// 	}

// 	// listInspect, err := mList.Inspect()
// 	// if err != nil {
// 	// 	return err
// 	// }

// 	// if len(listInspect.Manifests) == 0 {
// 	// 	fmt.Printf("Manifest list '%s' not found, skipping removal.\n", name)
// 	// 	return nil
// 	// }

// 	// Lock the image record where this list lives.
// 	locker, err := manifests.LockerForImage(*d.Registry.Store, mList.ID())
// 	if err != nil {
// 		return err
// 	}
// 	locker.Lock()
// 	defer locker.Unlock()

// 	// for _, instance := range listInspect.Manifests {
// 	// 	if err := mList.RemoveInstance(digest.Digest(instance.Digest)); err != nil {
// 	// 		return err
// 	// 	}
// 	// }

// 	// Remove the manifest list itself
// 	// if err := mList.RemoveInstance(digest.Digest(mList.ID())); err != nil {
// 	// 	return err
// 	// }

// 	return nil
// }

func (d *DockerClient) removeImageIfExists(imageName string) error {
	_, err := d.CLI.ImageRemove(context.Background(), imageName, types.ImageRemoveOptions{Force: true})
	if err != nil {
		rootErr := unwrapError(err)
		if strings.Contains(rootErr.Error(), "No such image") {
			fmt.Printf("No such image: %s, continuing...\n", imageName)
			return nil
		}
		return fmt.Errorf("failed to remove existing image: %s, root error: %v", imageName, rootErr)
	}
	fmt.Printf("Successfully removed existing image: %s\n", imageName)
	return nil
}

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
	manifestList, err := d.ManifestCreate(context.Background(), targetImage, imageTags)
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

	// Push the manifest list
	fmt.Printf("Pushing manifest list for %s\n", targetImage)
	if err := d.PushManifest(targetImage, manifestList); err != nil {
		rootErr := unwrapError(err)
		return fmt.Errorf("failed to push manifest list for %s, error: %v", targetImage, rootErr)
	}

	// Remove existing manifest list and image with the same name
	// if err := d.RemoveManifestList(context.Background(), targetImage); err != nil {
	// 	return fmt.Errorf("error removing existing manifest list: %v", err)
	// }

	// if err := d.removeImageIfExists(targetImage); err != nil {
	// 	return fmt.Errorf("error removing existing image: %v", err)
	// }

	// for _, instance := range listInspect.Manifests {
	// 	fmt.Printf("  Digest: %s, Platform: %s/%s\n", instance.Digest, instance.Platform.Architecture, instance.Platform.OS)
	// }

	// fmt.Printf("Created and pushed manifest list: %s\n", manifestDigest.String())

	return nil
}

// ManifestCreate creates a manifest list and adds the image tags.
func (d *DockerClient) ManifestCreate(ctx context.Context, targetImage string, imageTags []string) (ocispec.Index, error) {
	fmt.Printf("Creating manifest list for %s with %v tags\n", targetImage, imageTags)

	manifestList := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{},
	}

	// Fetch digests for each image tag and add to the manifest list
	for arch, hash := range d.Container.ImageHashes {
		size, err := d.getImageSize(hash)
		if err != nil {
			return manifestList, fmt.Errorf("failed to get size for %s: %v", hash, err)
		}

		manifestList.Manifests = append(manifestList.Manifests, ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    digest.Digest(hash),
			Size:      size,
			Platform:  &ocispec.Platform{OS: "linux", Architecture: arch},
		})
	}

	return manifestList, nil
}

// getImageSize fetches the image size for the given image reference.
func (d *DockerClient) getImageSize(imageRef string) (int64, error) {
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

// FetchDigest dynamically fetches the image digest using Docker CLI.
func FetchDigest(image string) (string, error) {
	cmd := exec.Command("docker", "buildx", "imagetools", "inspect", image, "--format", "{{.Digest}}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to inspect image: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// PushManifestList pushes the manifest list to the registry.
//
// **Parameters:**
//
// ctx: The context within which the manifest list is pushed.
// manifestID: The ID of the manifest list to push.
// destination: The destination image name.
// all, quiet, rm: Additional push options.
//
// **Returns:**
//
// string: The manifest digest of the pushed manifest list.
// error: An error if any operation fails during the push.
func (d *DockerClient) PushManifest(imageName string, manifestList ocispec.Index) error {
	manifestBytes, err := json.Marshal(manifestList)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest list: %v", err)
	}

	// Extract repository and tag from imageName
	parts := strings.Split(imageName, "/")
	if len(parts) < 3 {
		return fmt.Errorf("invalid repository name: %s", imageName)
	}
	// repo := strings.Join(parts[1:], "/")
	// tag := parts[len(parts)-1]

	token, err := d.getAuthToken(parts[1], parts[2])
	if err != nil {
		return fmt.Errorf("failed to get auth token: %v", err)
	}
	// token := d.AuthStr

	// URL to push the manifest
	// url := fmt.Sprintf("https://ghcr.io/v2/%s/manifests/%s", repo, tag)
	// url := "https://ghcr.io/v2/l50/atomic-red-team/latest"
	url := "https://ghcr.io/v2/l50/atomic-red-team/manifests/latest"
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(manifestBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	// url := "https://ghcr.io/v2/l50/manifests/atomic-red-team:latest"
	// req, err := http.NewRequest("PUT", url, bytes.NewBuffer(manifestBytes))
	// if err != nil {
	// 	return fmt.Errorf("failed to create request: %v", err)
	// }

	// Set headers
	req.Header.Set("Content-Type", "application/vnd.oci.image.index.v1+json")
	req.Header.Set("Authorization", "Bearer "+token)

	fmt.Println("TOKEN:", token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to push manifest list: %v", err)
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to push manifest list: %s, error reading response body: %v", resp.Status, err)
		}
		return fmt.Errorf("failed to push manifest list: %s, response: %s", resp.Status, body)
	}

	fmt.Println("Manifest list pushed successfully")
	return nil
}

func (d *DockerClient) getAuthToken(repo, tag string) (string, error) {
	authURL := fmt.Sprintf("https://ghcr.io/token?service=ghcr.io&scope=repository:%s/%s:pull,push", repo, tag)
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(d.Container.ImageRegistry.Username, d.Container.ImageRegistry.Credential)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get auth token, status: %s", resp.Status)
	}

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	return authResp.Token, nil
}
