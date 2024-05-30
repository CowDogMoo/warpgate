package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
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
		// Ensure the digest has the correct format
		parsedDigest, err := digest.Parse(fmt.Sprintf("sha256:%s", imgHash.Hash))
		if err != nil {
			return manifestList, fmt.Errorf("failed to parse digest sha256:%s: %v", imgHash.Hash, err)
		}

		size, err := d.GetImageSize(parsedDigest.String())
		if err != nil {
			return manifestList, fmt.Errorf("failed to get size for %s: %v", imgHash.Hash, err)
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

// PushManifestList pushes the input manifest list to the registry.
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
	manifestBytes, err := json.Marshal(manifestList)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest list: %v", err)
	}

	parts := strings.Split(imageName, "/")
	if len(parts) < 3 {
		return fmt.Errorf("invalid repository name: %s", imageName)
	}

	repoParts := strings.Split(parts[len(parts)-1], ":")
	repo := fmt.Sprintf("%s/%s", parts[1], repoParts[0])
	tag := "latest"
	if len(repoParts) > 1 {
		tag = repoParts[1]
	}

	token, err := d.getAuthToken(repo, tag)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %v", err)
	}

	url := fmt.Sprintf("https://ghcr.io/v2/%s/manifests/%s", repo, tag)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(manifestBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/vnd.oci.image.index.v1+json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to push manifest list: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to push manifest list: %s, error reading response body: %v", resp.Status, err)
		}
		return fmt.Errorf("failed to push manifest list: %s, response: %s", resp.Status, body)
	}

	fmt.Printf("Manifest list for %s pushed successfully\n", imageName)
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
