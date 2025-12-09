/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// BuildKitBuilder implements container image building using Docker BuildKit
type BuildKitBuilder struct {
	buildxAvailable bool
}

// NewBuildKitBuilder creates a new BuildKit builder instance
func NewBuildKitBuilder(ctx context.Context) (*BuildKitBuilder, error) {
	// Check if docker buildx is available
	cmd := exec.CommandContext(ctx, "docker", "buildx", "version")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker buildx not available: %w", err)
	}

	logging.Info("BuildKit builder initialized (using docker buildx)")
	return &BuildKitBuilder{
		buildxAvailable: true,
	}, nil
}

// Build creates a container image using BuildKit
func (b *BuildKitBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	logging.Info("Building image: %s", cfg.Name)

	// Generate Dockerfile from config
	dockerfile, err := b.generateDockerfile(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Write Dockerfile to temp location
	tmpDir, err := os.MkdirTemp("", "warpgate-buildkit-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			logging.Warn("Failed to remove temp directory: %v", err)
		}
	}()

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return nil, fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Copy Ansible files to build context
	if err := b.copyAnsibleFiles(cfg, tmpDir); err != nil {
		return nil, fmt.Errorf("failed to copy Ansible files: %w", err)
	}

	logging.Debug("Generated Dockerfile:\n%s", dockerfile)

	// Build the image
	platform := "linux/amd64"
	if len(cfg.Architectures) > 0 {
		platform = fmt.Sprintf("linux/%s", cfg.Architectures[0])
	}

	imageName := fmt.Sprintf("%s:%s", cfg.Name, cfg.Version)
	if cfg.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", cfg.Registry, imageName)
	}

	args := []string{
		"buildx", "build",
		"--platform", platform,
		"--load",
		"-t", imageName,
		"-f", dockerfilePath,
		tmpDir,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logging.Info("Executing: docker %s", strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker buildx build failed: %w", err)
	}

	return &builder.BuildResult{
		ImageRef: imageName,
		Platform: platform,
		Duration: "unknown",
		Notes:    []string{"Built with BuildKit via docker buildx"},
	}, nil
}

// generateDockerfile converts a Warpgate config to a Dockerfile
func (b *BuildKitBuilder) generateDockerfile(cfg builder.Config) (string, error) {
	var dockerfile strings.Builder

	// FROM statement
	dockerfile.WriteString(fmt.Sprintf("FROM %s\n\n", cfg.Base.Image))

	// Base environment variables
	if cfg.Base.Env != nil {
		for key, value := range cfg.Base.Env {
			dockerfile.WriteString(fmt.Sprintf("ENV %s=%s\n", key, value))
		}
		dockerfile.WriteString("\n")
	}

	// Provisioners
	for i, prov := range cfg.Provisioners {
		logging.Debug("Processing provisioner %d: type=%s", i+1, prov.Type)

		switch prov.Type {
		case "shell":
			if len(prov.Inline) > 0 {
				// Combine inline commands into a single RUN statement
				dockerfile.WriteString("RUN ")
				for idx, cmd := range prov.Inline {
					if idx > 0 {
						dockerfile.WriteString(" && \\\n    ")
					}
					dockerfile.WriteString(cmd)
				}
				dockerfile.WriteString("\n\n")
			}

		case "ansible":
			// Copy playbook and run ansible
			if prov.PlaybookPath != "" {
				// Resolve the playbook path (handle env vars)
				playbookPath := os.ExpandEnv(prov.PlaybookPath)
				playbookFilename := filepath.Base(playbookPath)

				dockerfile.WriteString("# Ansible provisioner\n")
				dockerfile.WriteString(fmt.Sprintf("COPY %s /tmp/playbook.yml\n", playbookFilename))

				// Copy galaxy file if specified
				if prov.GalaxyFile != "" {
					galaxyPath := os.ExpandEnv(prov.GalaxyFile)
					galaxyFilename := filepath.Base(galaxyPath)
					dockerfile.WriteString(fmt.Sprintf("COPY %s /tmp/requirements.yml\n", galaxyFilename))
					dockerfile.WriteString("RUN ansible-galaxy install -r /tmp/requirements.yml\n")
				}

				// Build ansible-playbook command
				var ansibleCmd strings.Builder
				ansibleCmd.WriteString("ansible-playbook /tmp/playbook.yml")

				// Add connection settings
				ansibleCmd.WriteString(" -c local")

				// Add extra vars
				if len(prov.ExtraVars) > 0 {
					for key, value := range prov.ExtraVars {
						ansibleCmd.WriteString(fmt.Sprintf(" -e %s=%s", key, value))
					}
				}

				dockerfile.WriteString(fmt.Sprintf("RUN %s\n\n", ansibleCmd.String()))
			}

		default:
			logging.Warn("Unsupported provisioner type in BuildKit backend: %s", prov.Type)
		}
	}

	// Post-changes (ENV, WORKDIR, USER, ENTRYPOINT, CMD)
	if len(cfg.PostChanges) > 0 {
		dockerfile.WriteString("# Post-build changes\n")
		for _, change := range cfg.PostChanges {
			dockerfile.WriteString(fmt.Sprintf("%s\n", change))
		}
	}

	return dockerfile.String(), nil
}

// copyAnsibleFiles copies Ansible playbooks and related files to the build context
func (b *BuildKitBuilder) copyAnsibleFiles(cfg builder.Config, destDir string) error {
	for _, prov := range cfg.Provisioners {
		if prov.Type != "ansible" {
			continue
		}

		// Copy playbook
		if prov.PlaybookPath != "" {
			playbookPath := os.ExpandEnv(prov.PlaybookPath)
			destPath := filepath.Join(destDir, filepath.Base(playbookPath))

			if err := copyFile(playbookPath, destPath); err != nil {
				return fmt.Errorf("failed to copy playbook %s: %w", playbookPath, err)
			}
			logging.Debug("Copied playbook: %s -> %s", playbookPath, destPath)
		}

		// Copy galaxy requirements file
		if prov.GalaxyFile != "" {
			galaxyPath := os.ExpandEnv(prov.GalaxyFile)
			destPath := filepath.Join(destDir, filepath.Base(galaxyPath))

			if err := copyFile(galaxyPath, destPath); err != nil {
				return fmt.Errorf("failed to copy galaxy file %s: %w", galaxyPath, err)
			}
			logging.Debug("Copied galaxy file: %s -> %s", galaxyPath, destPath)
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			logging.Warn("Failed to close source file: %v", err)
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			logging.Warn("Failed to close destination file: %v", err)
		}
	}()

	if _, err := destFile.ReadFrom(sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// Push pushes the image to a registry
func (b *BuildKitBuilder) Push(ctx context.Context, imageRef, registry string) error {
	cmd := exec.CommandContext(ctx, "docker", "push", imageRef)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logging.Info("Pushing image: %s", imageRef)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}

	return nil
}

// Tag adds additional tags to an image
func (b *BuildKitBuilder) Tag(ctx context.Context, imageRef, newTag string) error {
	cmd := exec.CommandContext(ctx, "docker", "tag", imageRef, newTag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker tag failed: %w", err)
	}

	logging.Debug("Tagged %s as %s", imageRef, newTag)
	return nil
}

// Remove removes an image from local storage
func (b *BuildKitBuilder) Remove(ctx context.Context, imageRef string) error {
	cmd := exec.CommandContext(ctx, "docker", "rmi", imageRef)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker rmi failed: %w", err)
	}

	logging.Debug("Removed image: %s", imageRef)
	return nil
}

// Close cleans up any resources
func (b *BuildKitBuilder) Close() error {
	// No persistent resources to clean up
	return nil
}
