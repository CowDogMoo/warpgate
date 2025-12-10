//go:build linux

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

package buildah

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
	workDir       string
	builder       *buildah.Builder
	containerID   string
	globalConfig  *globalconfig.Config
}

// Verify that BuildahBuilder implements builder.ContainerBuilder at compile time
var _ builder.ContainerBuilder = (*BuildahBuilder)(nil)

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

	// Use containers/storage default options - respects system configuration
	// This delegates storage configuration to the containers/storage library
	// which properly handles /etc/containers/storage.conf and ~/.config/containers/storage.conf
	storeOpts, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to get default storage options: %w", err)
	}

	// Allow user overrides from warpgate config or BuildahConfig
	// Only override if explicitly provided - otherwise trust system defaults
	if cfg.StorageRoot != "" {
		storeOpts.GraphRoot = cfg.StorageRoot
		logging.Debug("Using custom storage root: %s", cfg.StorageRoot)
	}
	if cfg.RunRoot != "" {
		storeOpts.RunRoot = cfg.RunRoot
		logging.Debug("Using custom run root: %s", cfg.RunRoot)
	}
	if cfg.StorageDriver != "" {
		storeOpts.GraphDriverName = cfg.StorageDriver
		logging.Debug("Using custom storage driver: %s", cfg.StorageDriver)
	}

	// Enable ignore_chown_errors for rootless VFS builds
	// This allows rootless containers to work with VFS driver despite permission limitations
	if storeOpts.GraphDriverName == "vfs" {
		storeOpts.GraphDriverOptions = append(storeOpts.GraphDriverOptions, "vfs.ignore_chown_errors=true")
		logging.Debug("Enabled ignore_chown_errors for VFS driver")
	}

	logging.Info("Storage configuration: driver=%s, root=%s, runroot=%s",
		storeOpts.GraphDriverName, storeOpts.GraphRoot, storeOpts.RunRoot)

	// Initialize storage
	store, err := storage.GetStore(storeOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Ensure container configuration files exist
	if err := ensureContainerConfig(); err != nil {
		return nil, fmt.Errorf("failed to ensure container config: %w", err)
	}

	// Set up system context with Docker authentication
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	systemContext := &imagetypes.SystemContext{
		// Point to Docker's config.json for registry authentication
		AuthFilePath: filepath.Join(homeDir, ".docker", "config.json"),
	}

	logging.Info("Buildah builder initialized successfully")
	return &BuildahBuilder{
		store:         store,
		systemContext: systemContext,
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

	// Apply post-provisioner changes (USER, WORKDIR, etc.)
	if len(cfg.PostChanges) > 0 {
		if err := b.applyChanges(cfg.PostChanges); err != nil {
			return nil, fmt.Errorf("failed to apply post-changes: %w", err)
		}
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
		Notes:        []string{"Built with Buildah"},
	}, nil
}

// fromImage creates a builder from a base image
func (b *BuildahBuilder) fromImage(ctx context.Context, base builder.BaseImage) error {
	logging.Debug("entered fromImage - base.Image='%s', base.Platform='%s'", base.Image, base.Platform)
	logging.Info("Pulling base image: %s", base.Image)

	// Configure system context for the platform
	systemContext := &imagetypes.SystemContext{
		OSChoice: "linux",
	}

	// If a platform is specified in the base config, extract the architecture
	if base.Platform != "" {
		parts := strings.Split(base.Platform, "/")
		if len(parts) >= 1 {
			systemContext.OSChoice = parts[0]
		}
		if len(parts) >= 2 {
			systemContext.ArchitectureChoice = parts[1]
		}
		if len(parts) >= 3 && parts[2] != "" {
			systemContext.VariantChoice = parts[2]
		}
	}

	// Log the platform settings being used (before buildah.NewBuilder)
	logging.Debug("SystemContext settings - OS: '%s', Arch: '%s', Variant: '%s', Platform: '%s'",
		systemContext.OSChoice, systemContext.ArchitectureChoice, systemContext.VariantChoice, base.Platform)

	// Build options with OCI isolation for full container capabilities
	options := buildah.BuilderOptions{
		FromImage:     base.Image,
		PullPolicy:    define.PullIfMissing,
		SystemContext: systemContext,
		Isolation:     define.IsolationOCI,
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
		return b.applyUserChange(value)
	case "WORKDIR":
		return b.applyWorkdirChange(value)
	case "ENV":
		return b.applyEnvChange(value)
	case "ENTRYPOINT":
		return b.applyEntrypointChange(value)
	case "CMD":
		return b.applyCmdChange(value)
	case "LABEL":
		return b.applyLabelChange(value)
	case "EXPOSE":
		return b.applyExposeChange(value)
	case "VOLUME":
		return b.applyVolumeChange(value)
	default:
		logging.Warn("Unsupported change directive: %s (skipping)", directive)
		return nil
	}
}

func (b *BuildahBuilder) applyUserChange(value string) error {
	if value == "" {
		return fmt.Errorf("dockerfile: USER directive requires a value")
	}
	b.builder.SetUser(value)
	logging.Info("Set user to: %s", value)
	return nil
}

func (b *BuildahBuilder) applyWorkdirChange(value string) error {
	if value == "" {
		return fmt.Errorf("dockerfile: WORKDIR directive requires a value")
	}
	b.builder.SetWorkDir(value)
	logging.Info("Set working directory to: %s", value)
	return nil
}

func (b *BuildahBuilder) applyEnvChange(value string) error {
	if value == "" {
		return fmt.Errorf("dockerfile: ENV directive requires a value")
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
	return nil
}

func (b *BuildahBuilder) applyEntrypointChange(value string) error {
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
	return nil
}

func (b *BuildahBuilder) applyCmdChange(value string) error {
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
	return nil
}

func (b *BuildahBuilder) applyLabelChange(value string) error {
	if value == "" {
		return fmt.Errorf("dockerfile: LABEL directive requires a value")
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
	return nil
}

func (b *BuildahBuilder) applyExposeChange(value string) error {
	if value == "" {
		return fmt.Errorf("dockerfile: EXPOSE directive requires a value")
	}
	b.builder.SetPort(value)
	logging.Info("Exposed port: %s", value)
	return nil
}

func (b *BuildahBuilder) applyVolumeChange(value string) error {
	if value == "" {
		return fmt.Errorf("dockerfile: VOLUME directive requires a value")
	}
	// Parse as JSON array or single path
	volumes := b.parseCommandValue(value)
	for _, vol := range volumes {
		b.builder.AddVolume(vol)
		logging.Info("Added volume: %s", vol)
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
	runtime := b.globalConfig.Container.Runtime
	shellProv := provisioner.NewShellProvisioner(b.builder, runtime)
	return shellProv.Provision(ctx, prov)
}

// runScriptProvisioner runs script files inside the container
func (b *BuildahBuilder) runScriptProvisioner(ctx context.Context, prov builder.Provisioner) error {
	scriptProv := provisioner.NewScriptProvisioner(b.builder)
	return scriptProv.Provision(ctx, prov)
}

// runAnsibleProvisioner runs Ansible playbooks
func (b *BuildahBuilder) runAnsibleProvisioner(ctx context.Context, prov builder.Provisioner) error {
	runtime := b.globalConfig.Container.Runtime
	ansibleProv := provisioner.NewAnsibleProvisioner(b.builder, runtime)
	return ansibleProv.Provision(ctx, prov)
}

// commit commits the container to an image and returns the image reference and digest
func (b *BuildahBuilder) commit(ctx context.Context, name, version string) (imageRef, digest string, err error) {
	// Format image reference conditionally based on whether a default registry is set
	var imageRefStr string
	if b.globalConfig.Container.DefaultRegistry == "" {
		// No registry prefix - local image reference
		imageRefStr = fmt.Sprintf("%s:%s", name, version)
	} else {
		// Include registry prefix
		imageRefStr = fmt.Sprintf("%s/%s:%s", b.globalConfig.Container.DefaultRegistry, name, version)
	}
	logging.Info("Committing image: %s", imageRefStr)

	// Parse image reference
	imageRefParsed, err := alltransports.ParseImageName("containers-storage:" + imageRefStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	options := buildah.CommitOptions{
		Squash: false,
	}

	imageID, _, digestParsed, err := b.builder.Commit(ctx, imageRefParsed, options)
	if err != nil {
		return "", "", fmt.Errorf("failed to commit: %w", err)
	}

	digestStr := digestParsed.String()

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
	// SystemContext is configured with AuthFilePath to use Docker credentials
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

// ensureContainerConfig ensures that required container configuration files exist
func ensureContainerConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "containers")

	// Create the config directory if it doesn't exist
	if err := os.MkdirAll(configDir, globalconfig.DirPermReadWriteExec); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ensure registries.conf exists
	registriesPath := filepath.Join(configDir, "registries.conf")
	if _, err := os.Stat(registriesPath); os.IsNotExist(err) {
		defaultRegistries := `# Generated by Warpgate
# Container registries configuration

[registries.search]
registries = ['docker.io']

[registries.insecure]
registries = []

[registries.block]
registries = []
`
		if err := os.WriteFile(registriesPath, []byte(defaultRegistries), globalconfig.FilePermReadWrite); err != nil {
			return fmt.Errorf("failed to write registries config: %w", err)
		}
		logging.Info("Created default registries config at: %s", registriesPath)
	} else {
		logging.Debug("Registries config already exists at: %s", registriesPath)
	}

	// Ensure policy.json exists
	policyPath := filepath.Join(configDir, "policy.json")
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		defaultPolicy := `{
  "default": [
    {
      "type": "insecureAcceptAnything"
    }
  ],
  "transports": {
    "docker-daemon": {
      "": [
        {
          "type": "insecureAcceptAnything"
        }
      ]
    }
  }
}
`
		if err := os.WriteFile(policyPath, []byte(defaultPolicy), globalconfig.FilePermReadWrite); err != nil {
			return fmt.Errorf("failed to write policy config: %w", err)
		}
		logging.Info("Created default policy config at: %s", policyPath)
	} else {
		logging.Debug("Policy config already exists at: %s", policyPath)
	}

	return nil
}

// GetDefaultConfig returns a default BuildahConfig
// This now returns minimal overrides - storage.DefaultStoreOptions() handles the rest
func GetDefaultConfig() BuildahConfig {
	// Load global config to check for user overrides
	globalCfg, err := globalconfig.Load()
	if err != nil {
		logging.Warn("Failed to load global config, using system defaults: %v", err)
		// Return empty config - storage.DefaultStoreOptions() will provide sensible defaults
		return BuildahConfig{}
	}

	// Only populate fields if user explicitly configured them
	// Empty values allow storage.DefaultStoreOptions() to use system defaults
	return BuildahConfig{
		StorageDriver: globalCfg.Storage.Driver, // Empty = use system default
		StorageRoot:   globalCfg.Storage.Root,   // Empty = use system default
		// RunRoot intentionally not set - system handles this
	}
}
