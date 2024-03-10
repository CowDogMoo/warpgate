package registry

import (
	"bytes"
	"fmt"
	"os/exec"
)

// DockerLogin logs in to the Docker registry.
func DockerLogin(username, token string) error {
	cmd := exec.Command("docker", "login", "ghcr.io", "-u", username, "-p", token)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker login failed: %s", out.String())
	}
	return nil
}

// DockerPush pushes a Docker image to the registry.
func DockerPush(image string) error {
	cmd := exec.Command("docker", "push", image)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed for %s: %s", image, out.String())
	}
	return nil
}

// DockerManifestCreate creates a Docker manifest.
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

// DockerManifestPush pushes a Docker manifest to the registry.
func DockerManifestPush(manifest string) error {
	cmd := exec.Command("docker", "manifest", "push", manifest)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker manifest push failed for %s: %s", manifest, out.String())
	}
	return nil
}

// DockerTag tags a Docker image.
func DockerTag(sourceImage, targetImage string) error {
	cmd := exec.Command("docker", "tag", sourceImage, targetImage)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker tag failed for %s to %s: %s", sourceImage, targetImage, out.String())
	}
	return nil
}
