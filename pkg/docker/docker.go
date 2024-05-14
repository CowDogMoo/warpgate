/*
Copyright © 2024-present, Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	log "github.com/l50/goutils/v2/logging"
)

// DockerLogin authenticates with a Docker registry using the provided username
// and token. It executes the 'docker login' command.
//
// **Parameters:**
//
// username: The username for the Docker registry.
// token: The access token for the Docker registry.
//
// **Returns:**
//
// error: An error if any issue occurs during the login process.
func DockerLogin(username, token string) error {
	cmd := exec.Command("docker", "login", "ghcr.io", "-u", username, "-p", token)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker login failed: %s", out.String())
	}
	return nil
}

// DockerPush pushes a Docker image to a registry. It executes the 'docker push'
// command with the specified image name.
//
// **Parameters:**
//
// image: The name of the image to push.
//
// **Returns:**
//
// error: An error if the push operation fails.
func DockerPush(image string) error {
	cmd := exec.Command("docker", "push", image)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed for %s: %s", image, out.String())
	}
	return nil
}

// DockerManifestCreate creates a Docker manifest that references multiple
// platform-specific versions of an image. It builds the manifest using the
// 'docker manifest create' command.
//
// **Parameters:**
//
// manifest: The name of the manifest to create.
// images: A slice of image names to include in the manifest.
//
// **Returns:**
//
// error: An error if the manifest creation fails.
func DockerManifestCreate(manifest string, images []string) error {
	args := []string{"manifest", "create", manifest}
	for _, image := range images {
		args = append(args, "--amend", image)
	}
	cmd := exec.Command("docker", args...)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker manifest create failed for %s: %s", manifest, out.String())
	}
	return nil
}

// DockerManifestPush pushes a Docker manifest to a registry. It uses the
// 'docker manifest push' command.
//
// **Parameters:**
//
// manifest: The name of the manifest to push.
//
// **Returns:**
//
// error: An error if the push operation fails.
func DockerManifestPush(manifest string) error {
	cmd := exec.Command("docker", "manifest", "push", manifest)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker manifest push failed for %s: %s", manifest, out.String())
	}
	return nil
}

// DockerTag tags a Docker image with a new name. It performs the operation
// using the 'docker tag' command.
//
// **Parameters:**
//
// sourceImage: The current name of the image.
// targetImage: The new name to assign to the image.
//
// **Returns:**
//
// error: An error if the tagging operation fails.
func DockerTag(sourceImage, targetImage string) error {
	cmd := exec.Command("docker", "tag", sourceImage, targetImage)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker tag failed for %s to %s: %s", sourceImage, targetImage, out.String())
	}
	return nil
}

// ParseImageHashes extracts the image hashes from the output of a Packer build
// command and updates the provided Packer blueprint with the new hashes.
//
// **Parameters:**
//
// output: The output from the Packer build command.
// pTmpl: The Packer blueprint to update with the new image hashes.
func ParseImageHashes(output string, pTmpl *packer.BlueprintPacker) {
	if strings.Contains(output, "Imported Docker image: sha256:") {
		parts := strings.Split(output, " ")
		if len(parts) > 4 {
			archParts := strings.Split(parts[1], ".")
			if len(archParts) > 1 {
				arch := strings.TrimSuffix(archParts[1], ":")
				for _, part := range parts {
					if strings.HasPrefix(part, "sha256:") {
						hash := strings.TrimPrefix(part, "sha256:")
						if pTmpl.ImageHashes == nil {
							pTmpl.ImageHashes = make(map[string]string)
						}
						pTmpl.ImageHashes[arch] = hash
						log.L().Debug("Updated ImageHashes: %v\n", pTmpl.ImageHashes)
						break
					}
				}
			}
		}
	}
}
