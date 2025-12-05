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
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/provisioner"
	"go.podman.io/image/v5/transports/alltransports"
	imagetypes "go.podman.io/image/v5/types"
	"go.podman.io/storage"
)

// BuildahBuilder implements the ContainerBuilder interface using Buildah
type BuildahBuilder struct {
	store         storage.Store
	systemContext *imagetypes.SystemContext
	storageConfig *StorageConfig
	workDir       string
	builder       *buildah.Builder
	containerID   string
	globalConfig  *globalconfig.Config
}

// BuildahConfig holds configuration for BuildahBuilder
type BuildahConfig struct {
	StorageDriver string
	StorageRoot   string
	RunRoot       string
	WorkDir       string
}

// NewBuildahBuilder creates a new Buildah-based builder
func NewBuildahBuilder(cfg BuildahConfig) (*BuildahBuilder, error) {
	logging.Info("Initializing Buildah builder")

	// Load global config
	globalCfg, err := globalconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	// Set up work directory
	workDir := cfg.WorkDir
	if workDir == "" {
		tmpDir, err := os.MkdirTemp("", "warpgate-build-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create work directory: %w", err)
		}
		workDir = tmpDir
	}

	// Initialize storage configuration
	storageConfig := NewStorageConfig()
	if cfg.StorageRoot != "" {
		storageConfig.SetRoot(cfg.StorageRoot)
	}
	if cfg.RunRoot != "" {
		storageConfig.SetRunRoot(cfg.RunRoot)
	}
	if cfg.StorageDriver != "" {
		storageConfig.SetDriver(cfg.StorageDriver)
	}

	// Configure storage
	if err := storageConfig.Configure(); err != nil {
		return nil, fmt.Errorf("failed to configure storage: %w", err)
	}

	// Set up storage options
	storeOpts, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to get default store options: %w", err)
	}

	storeOpts.GraphRoot = storageConfig.GetRoot()
	storeOpts.RunRoot = storageConfig.GetRunRoot()
	storeOpts.GraphDriverName = storageConfig.GetDriver()

	// Initialize storage
	store, err := storage.GetStore(storeOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Set up system context
	systemContext := &imagetypes.SystemContext{}

	logging.Info("Buildah builder initialized successfully")
	return &BuildahBuilder{
		store:         store,
		systemContext: systemContext,
		storageConfig: storageConfig,
		workDir:       workDir,
		globalConfig:  globalCfg,
	}, nil
}

// Build creates a container image from the given configuration
func (b *BuildahBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	startTime := time.Now()
	logging.Info("Starting build for %s", cfg.Name)

	// Pull or use base image
	if err := b.fromImage(ctx, cfg.Base); err != nil {
		return nil, fmt.Errorf("failed to create from base image: %w", err)
	}

	// Run provisioners
	if err := b.runProvisioners(ctx, cfg.Provisioners); err != nil {
		return nil, fmt.Errorf("failed to run provisioners: %w", err)
	}

	// Commit the image
	imageRef, digest, err := b.commit(ctx, cfg.Name, cfg.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to commit image: %w", err)
	}

	duration := time.Since(startTime)
	logging.Info("Build completed in %s", duration)

	// Determine platform and architecture
	platform := cfg.Base.Platform
	if platform == "" {
		platform = b.globalConfig.Container.DefaultPlatform
	}
	arch := strings.Split(platform, "/")[1] // Extract architecture from platform

	return &builder.BuildResult{
		ImageRef:     imageRef,
		Digest:       digest,
		Platform:     platform,
		Architecture: arch,
		Duration:     duration.String(),
		Notes:        []string{fmt.Sprintf("Built with Buildah on %s", b.storageConfig.GetDriver())},
	}, nil
}

// fromImage creates a builder from a base image
func (b *BuildahBuilder) fromImage(ctx context.Context, base builder.BaseImage) error {
	logging.Info("Pulling base image: %s", base.Image)

	// Build options
	options := buildah.BuilderOptions{
		FromImage:     base.Image,
		PullPolicy:    define.PullIfMissing,
		SystemContext: b.systemContext,
	}

	if base.Pull {
		options.PullPolicy = define.PullAlways
	}

	// Create builder
	bldr, err := buildah.NewBuilder(ctx, b.store, options)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}

	b.builder = bldr
	b.containerID = bldr.ContainerID

	logging.Info("Created builder container: %s", b.containerID)

	// Set environment variables if provided
	if len(base.Env) > 0 {
		for key, value := range base.Env {
			bldr.SetEnv(key, value)
		}
	}

	// Apply Changes directives (USER, WORKDIR, ENV, ENTRYPOINT, CMD, etc.)
	if len(base.Changes) > 0 {
		if err := b.applyChanges(base.Changes); err != nil {
			return fmt.Errorf("failed to apply changes: %w", err)
		}
	}

	return nil
}

// applyChanges applies a list of Dockerfile-style change directives
func (b *BuildahBuilder) applyChanges(changes []string) error {
	logging.Info("Applying %d change directives", len(changes))

	for _, change := range changes {
		if err := b.applyChange(change); err != nil {
			return fmt.Errorf("failed to apply change '%s': %w", change, err)
		}
	}

	return nil
}

// applyChange applies a single Dockerfile-style change directive
func (b *BuildahBuilder) applyChange(change string) error {
	// Trim whitespace
	change = strings.TrimSpace(change)
	if change == "" {
		return nil
	}

	// Parse directive (e.g., "USER sliver" or "ENV KEY=VALUE")
	parts := strings.SplitN(change, " ", 2)
	if len(parts) < 1 {
		return fmt.Errorf("invalid change directive: %s", change)
	}

	directive := strings.ToUpper(parts[0])
	var value string
	if len(parts) > 1 {
		value = strings.TrimSpace(parts[1])
	}

	logging.Debug("Applying change: %s %s", directive, value)

	switch directive {
	case "USER":
		if value == "" {
			return fmt.Errorf("USER directive requires a value")
		}
		b.builder.SetUser(value)
		logging.Info("Set user to: %s", value)

	case "WORKDIR":
		if value == "" {
			return fmt.Errorf("WORKDIR directive requires a value")
		}
		b.builder.SetWorkDir(value)
		logging.Info("Set working directory to: %s", value)

	case "ENV":
		if value == "" {
			return fmt.Errorf("ENV directive requires a value")
		}
		// Parse ENV KEY=VALUE or ENV KEY VALUE
		var key, val string
		if strings.Contains(value, "=") {
			envParts := strings.SplitN(value, "=", 2)
			key = strings.TrimSpace(envParts[0])
			if len(envParts) > 1 {
				val = envParts[1]
			}
		} else {
			envParts := strings.SplitN(value, " ", 2)
			key = strings.TrimSpace(envParts[0])
			if len(envParts) > 1 {
				val = envParts[1]
			}
		}
		b.builder.SetEnv(key, val)
		logging.Info("Set environment variable: %s=%s", key, val)

	case "ENTRYPOINT":
		if value == "" {
			// Empty ENTRYPOINT clears it
			b.builder.SetEntrypoint([]string{})
			logging.Info("Cleared entrypoint")
		} else {
			// Parse as JSON array or shell form
			entrypoint := b.parseCommandValue(value)
			b.builder.SetEntrypoint(entrypoint)
			logging.Info("Set entrypoint to: %v", entrypoint)
		}

	case "CMD":
		if value == "" {
			// Empty CMD clears it
			b.builder.SetCmd([]string{})
			logging.Info("Cleared command")
		} else {
			// Parse as JSON array or shell form
			cmd := b.parseCommandValue(value)
			b.builder.SetCmd(cmd)
			logging.Info("Set command to: %v", cmd)
		}

	case "LABEL":
		if value == "" {
			return fmt.Errorf("LABEL directive requires a value")
		}
		// Parse LABEL key=value
		if strings.Contains(value, "=") {
			labelParts := strings.SplitN(value, "=", 2)
			key := strings.TrimSpace(labelParts[0])
			val := ""
			if len(labelParts) > 1 {
				val = strings.Trim(labelParts[1], "\"")
			}
			b.builder.SetLabel(key, val)
			logging.Info("Set label: %s=%s", key, val)
		} else {
			return fmt.Errorf("invalid LABEL format: %s", value)
		}

	case "EXPOSE":
		if value == "" {
			return fmt.Errorf("EXPOSE directive requires a value")
		}
		b.builder.SetPort(value)
		logging.Info("Exposed port: %s", value)

	case "VOLUME":
		if value == "" {
			return fmt.Errorf("VOLUME directive requires a value")
		}
		// Parse as JSON array or single path
		volumes := b.parseCommandValue(value)
		for _, vol := range volumes {
			b.builder.AddVolume(vol)
			logging.Info("Added volume: %s", vol)
		}

	default:
		logging.Warn("Unsupported change directive: %s (skipping)", directive)
	}

	return nil
}

// parseCommandValue parses a command value that can be either JSON array format or shell form
// Examples:
//   - JSON: ["executable", "param1", "param2"]
//   - Shell: /bin/sh -c "echo hello"
func (b *BuildahBuilder) parseCommandValue(value string) []string {
	value = strings.TrimSpace(value)

	// Check if it's JSON array format (starts with '[')
	if strings.HasPrefix(value, "[") {
		// Parse JSON array
		var result []string
		// Simple JSON array parser (handles basic cases)
		value = strings.TrimPrefix(value, "[")
		value = strings.TrimSuffix(value, "]")

		// Split by comma and clean up
		parts := strings.Split(value, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			part = strings.Trim(part, "\"")
			if part != "" {
				result = append(result, part)
			}
		}
		return result
	}

	// Shell form - return as single string with /bin/sh -c
	return []string{"/bin/sh", "-c", value}
}

// runProvisioners executes all provisioners in order
func (b *BuildahBuilder) runProvisioners(ctx context.Context, provisioners []builder.Provisioner) error {
	logging.Info("Running %d provisioners", len(provisioners))

	for i, prov := range provisioners {
		logging.Info("Running provisioner %d/%d: %s", i+1, len(provisioners), prov.Type)

		switch prov.Type {
		case "shell":
			if err := b.runShellProvisioner(ctx, prov); err != nil {
				return fmt.Errorf("shell provisioner failed: %w", err)
			}

		case "script":
			if err := b.runScriptProvisioner(ctx, prov); err != nil {
				return fmt.Errorf("script provisioner failed: %w", err)
			}

		case "ansible":
			if err := b.runAnsibleProvisioner(ctx, prov); err != nil {
				return fmt.Errorf("ansible provisioner failed: %w", err)
			}

		default:
			return fmt.Errorf("unknown provisioner type: %s", prov.Type)
		}
	}

	logging.Info("All provisioners completed successfully")
	return nil
}

// runShellProvisioner runs shell commands inside the container
func (b *BuildahBuilder) runShellProvisioner(ctx context.Context, prov builder.Provisioner) error {
	shellProv := provisioner.NewShellProvisioner(b.builder)
	return shellProv.Provision(ctx, prov)
}

// runScriptProvisioner runs script files inside the container
func (b *BuildahBuilder) runScriptProvisioner(ctx context.Context, prov builder.Provisioner) error {
	scriptProv := provisioner.NewScriptProvisioner(b.builder)
	return scriptProv.Provision(ctx, prov)
}

// runAnsibleProvisioner runs Ansible playbooks
func (b *BuildahBuilder) runAnsibleProvisioner(ctx context.Context, prov builder.Provisioner) error {
	ansibleProv := provisioner.NewAnsibleProvisioner(b.builder)
	return ansibleProv.Provision(ctx, prov)
}

// commit commits the container to an image and returns the image reference and digest
func (b *BuildahBuilder) commit(ctx context.Context, name, version string) (string, string, error) {
	imageRefStr := fmt.Sprintf("%s/%s:%s", b.globalConfig.Container.DefaultRegistry, name, version)
	logging.Info("Committing image: %s", imageRefStr)

	// Parse image reference
	imageRef, err := alltransports.ParseImageName("containers-storage:" + imageRefStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	options := buildah.CommitOptions{
		Squash: false,
	}

	imageID, _, digest, err := b.builder.Commit(ctx, imageRef, options)
	if err != nil {
		return "", "", fmt.Errorf("failed to commit: %w", err)
	}

	digestStr := digest.String()

	logging.Info("Image committed successfully: %s (ID: %s, Digest: %s)", imageRefStr, imageID, digestStr)
	return imageRefStr, digestStr, nil
}

// Push pushes the image to a registry using Buildah's native push
func (b *BuildahBuilder) Push(ctx context.Context, imageRef, destination string) error {
	logging.Info("Pushing image %s to %s", imageRef, destination)

	// Parse the destination reference
	// Buildah expects format like: docker://ghcr.io/org/image:tag
	destRefStr := destination
	if !strings.HasPrefix(destination, "docker://") {
		destRefStr = "docker://" + destination
	}

	destRef, err := alltransports.ParseImageName(destRefStr)
	if err != nil {
		return fmt.Errorf("failed to parse destination: %w", err)
	}

	// Configure push options
	// SystemContext with nil uses defaults which auto-loads ~/.docker/config.json
	pushOpts := buildah.PushOptions{
		Store:         b.store,
		SystemContext: b.systemContext,
		// Compression, SignBy, etc. can be configured here if needed
	}

	// Use buildah.Push() - it handles authentication, compression, etc.
	imageID, digest, err := buildah.Push(ctx, imageRef, destRef, pushOpts)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	logging.Info("Successfully pushed image to %s", destination)
	logging.Info("Image ID: %s", imageID)
	logging.Info("Digest: %s", digest.String())

	return nil
}

// Tag adds a tag to an image
func (b *BuildahBuilder) Tag(ctx context.Context, imageRef, newTag string) error {
	logging.Info("Tagging image %s as %s", imageRef, newTag)

	// Get the image
	img, err := b.store.Image(imageRef)
	if err != nil {
		return fmt.Errorf("failed to find image: %w", err)
	}

	// Add the new tag
	if err := b.store.SetNames(img.ID, append(img.Names, newTag)); err != nil {
		return fmt.Errorf("failed to add tag: %w", err)
	}

	logging.Info("Successfully tagged image")
	return nil
}

// Remove removes an image from local storage
func (b *BuildahBuilder) Remove(ctx context.Context, imageRef string) error {
	logging.Info("Removing image: %s", imageRef)

	// Get the image
	img, err := b.store.Image(imageRef)
	if err != nil {
		return fmt.Errorf("failed to find image: %w", err)
	}

	// Delete the image
	if _, err := b.store.DeleteImage(img.ID, true); err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	logging.Info("Successfully removed image")
	return nil
}

// GetManifestManager returns a ManifestManager for multi-arch operations
func (b *BuildahBuilder) GetManifestManager() *ManifestManager {
	return NewManifestManager(b.store, b.systemContext)
}

// Close cleans up resources
func (b *BuildahBuilder) Close() error {
	logging.Info("Cleaning up Buildah builder")

	// Delete the builder container if it exists
	if b.builder != nil {
		if err := b.builder.Delete(); err != nil {
			logging.Warn("Failed to delete builder container: %v", err)
		}
	}

	// Shutdown storage
	if b.store != nil {
		if _, err := b.store.Shutdown(false); err != nil {
			logging.Warn("Failed to shutdown storage: %v", err)
		}
	}

	// Clean up work directory
	if b.workDir != "" {
		if err := os.RemoveAll(b.workDir); err != nil {
			logging.Warn("Failed to remove work directory: %v", err)
		}
	}

	logging.Info("Cleanup completed")
	return nil
}

// GetDefaultConfig returns a default BuildahConfig
func GetDefaultConfig() BuildahConfig {
	// Load global config for defaults
	globalCfg, err := globalconfig.Load()
	if err != nil {
		logging.Warn("Failed to load global config, using hardcoded defaults: %v", err)
		// Fallback to hardcoded defaults if config load fails
		homeDir, _ := os.UserHomeDir()
		return BuildahConfig{
			StorageDriver: "vfs",
			StorageRoot:   filepath.Join(homeDir, ".local", "share", "containers", "storage"),
			RunRoot:       filepath.Join(homeDir, ".local", "share", "containers", "runroot"),
		}
	}

	homeDir, _ := os.UserHomeDir()

	// Use storage driver from config
	storageDriver := globalCfg.Storage.Driver

	// Use platform-appropriate paths
	var storageRoot, runRoot string

	// Check if config has custom paths
	if globalCfg.Storage.Root != "" {
		storageRoot = globalCfg.Storage.Root
	} else if os.Getenv("HOME") != "" && filepath.Base(homeDir) != "" {
		storageRoot = filepath.Join(homeDir, ".local", "share", "containers", "storage")
	}

	// On Linux, try to use /run if available and writable
	if os.Getenv("HOME") != "" && filepath.Base(homeDir) != "" {
		runRoot = filepath.Join(homeDir, ".local", "share", "containers", "runroot")

		runDir := filepath.Join("/run", "user", fmt.Sprintf("%d", os.Getuid()), "containers")
		if _, err := os.Stat(filepath.Dir(runDir)); err == nil {
			// Check if we can create the directory
			if err := os.MkdirAll(runDir, 0755); err == nil {
				runRoot = runDir
			}
		}
	}

	return BuildahConfig{
		StorageDriver: storageDriver,
		StorageRoot:   storageRoot,
		RunRoot:       runRoot,
	}
}
