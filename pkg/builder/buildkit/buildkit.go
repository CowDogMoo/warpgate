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

package buildkit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/cowdogmoo/warpgate/pkg/registryauth"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	digest "github.com/opencontainers/go-digest"
	specsgo "github.com/opencontainers/image-spec/specs-go"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	// CRITICAL: This enables docker-container:// protocol
	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/errors"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/manifests"
)

// BuildKitBuilder implements container image building using Docker BuildKit.
// It manages a BuildKit builder instance and provides methods to build,
// push, and manage container images using Docker's BuildKit backend.
type BuildKitBuilder struct {
	client        *client.Client
	dockerClient  DockerClient
	builderName   string
	containerName string
	contextDir    string
	cacheFrom     []string
	cacheTo       []string
}

// Verify that BuildKitBuilder implements builder.ContainerBuilder at compile time
var _ builder.ContainerBuilder = (*BuildKitBuilder)(nil)

// NewBuildKitBuilder creates a new BuildKit builder instance.
// It supports multiple connection modes:
//  1. Auto-detect: If no endpoint is configured, detects the active docker buildx builder
//  2. Explicit endpoint: Uses configured endpoint (docker-container://, tcp://, unix://)
//  3. Remote TCP: Supports TLS authentication for secure remote connections
//
// Returns an error if connection fails or no builder is available.
func NewBuildKitBuilder(ctx context.Context) (*BuildKitBuilder, error) {
	// Load global configuration
	cfg, err := globalconfig.Load()
	if err != nil {
		logging.Warn("Failed to load config, using defaults: %v", err)
		cfg = &globalconfig.Config{}
	}

	var addr string
	var builderName, containerName string

	// Determine connection address
	if cfg.BuildKit.Endpoint != "" {
		// Use configured endpoint
		addr = cfg.BuildKit.Endpoint
		logging.Info("Using configured BuildKit endpoint: %s", addr)
	} else {
		// Auto-detect local buildx builder
		builderName, containerName, err = detectBuildxBuilder(ctx)
		if err != nil {
			return nil, errors.Wrap("detect buildx builder", "set buildkit.endpoint in config for remote BuildKit", err)
		}
		addr = fmt.Sprintf("docker-container://%s", containerName)
		logging.Info("Auto-detected BuildKit builder: %s", containerName)
	}

	// Connect to BuildKit with appropriate options
	clientOpts := []client.ClientOpt{}

	// Add TLS configuration for tcp:// connections
	if strings.HasPrefix(addr, "tcp://") && cfg.BuildKit.TLSEnabled {
		tlsConfig, err := loadTLSConfig(cfg.BuildKit)
		if err != nil {
			return nil, errors.Wrap("load TLS config", "", err)
		}
		clientOpts = append(clientOpts, client.WithGRPCDialOption(
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		))
		logging.Info("TLS enabled for BuildKit connection")
	} else if strings.HasPrefix(addr, "tcp://") {
		// Insecure TCP connection
		clientOpts = append(clientOpts, client.WithGRPCDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		))
		logging.Warn("Connecting to BuildKit without TLS (insecure)")
	}

	// Create client connection
	c, err := client.New(ctx, addr, clientOpts...)
	if err != nil {
		return nil, errors.Wrap("connect to BuildKit", "", err)
	}

	// Verify connection
	info, err := c.Info(ctx)
	if err != nil {
		_ = c.Close()
		return nil, errors.Wrap("verify BuildKit connection", "", err)
	}

	logging.Info("BuildKit client connected (version %s)", info.BuildkitVersion.Version)

	// Create Docker client for image operations
	dockerCli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		_ = c.Close()
		return nil, errors.Wrap("create Docker client", "", err)
	}

	// Verify Docker connection
	_, err = dockerCli.Ping(ctx)
	if err != nil {
		_ = c.Close()
		_ = dockerCli.Close()
		return nil, errors.Wrap("verify Docker connection", "", err)
	}

	return &BuildKitBuilder{
		client:        c,
		dockerClient:  newDockerClientAdapter(dockerCli),
		builderName:   builderName,
		containerName: containerName,
		cacheFrom:     []string{},
		cacheTo:       []string{},
	}, nil
}

// loadTLSConfig creates a TLS configuration from BuildKit config
func loadTLSConfig(cfg globalconfig.BuildKitConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// Load CA certificate if provided
	if cfg.TLSCACert != "" {
		caCert, err := os.ReadFile(cfg.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate and key if provided
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// SetCacheOptions configures external cache sources and destinations for BuildKit.
// Cache sources (cacheFrom) are used to import cache from external registries.
// Cache destinations (cacheTo) specify where to export cache after the build.
// Format example: "type=registry,ref=user/app:cache,mode=max"
func (b *BuildKitBuilder) SetCacheOptions(cacheFrom, cacheTo []string) {
	b.cacheFrom = cacheFrom
	b.cacheTo = cacheTo
	if len(cacheFrom) > 0 {
		logging.Info("BuildKit cache sources: %v", cacheFrom)
	}
	if len(cacheTo) > 0 {
		logging.Info("BuildKit cache destinations: %v", cacheTo)
	}
}

// parseCacheAttrs parses cache attribute strings like "type=registry,ref=user/app:cache,mode=max"
// and returns a map of attributes suitable for BuildKit
func parseCacheAttrs(cacheSpec string) map[string]string {
	attrs := make(map[string]string)

	// Split by comma to get key=value pairs
	pairs := strings.Split(cacheSpec, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			attrs[key] = value
		}
	}

	return attrs
}

// detectBuildxBuilder detects the active buildx builder using Docker SDK
func detectBuildxBuilder(ctx context.Context) (string, string, error) {
	// Create temporary Docker client for detection
	dockerCli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return "", "", fmt.Errorf("failed to create Docker client: %w\n\nPlease install Docker Desktop or Docker Engine", err)
	}
	defer func() {
		_ = dockerCli.Close()
	}()

	// Verify Docker connection
	_, err = dockerCli.Ping(ctx)
	if err != nil {
		return "", "", fmt.Errorf("cannot connect to Docker daemon: %w\n\nPlease ensure Docker is running", err)
	}

	// List containers to find buildx builder containers
	containers, err := dockerCli.ContainerList(ctx, dockercontainer.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list containers: %w", err)
	}

	// Look for running buildx builder containers
	// Container names follow pattern: /buildx_buildkit_<builder-name>0
	for _, container := range containers {
		if container.State != "running" {
			continue
		}

		for _, name := range container.Names {
			// Remove leading / from container name
			name = strings.TrimPrefix(name, "/")

			// Check if this is a buildx builder container
			if strings.HasPrefix(name, "buildx_buildkit_") {
				// Extract builder name from container name
				// Format: buildx_buildkit_<builder-name>0
				builderName := strings.TrimPrefix(name, "buildx_buildkit_")
				builderName = strings.TrimSuffix(builderName, "0")

				logging.Debug("Found buildx builder: %s (container: %s)", builderName, name)
				return builderName, name, nil
			}
		}
	}

	return "", "", fmt.Errorf("no running buildx builder found\n\nRun 'docker buildx create --use' to create one")
}

// parsePlatform parses a platform string like "linux/amd64" into OS and Architecture
func parsePlatform(platformStr string) (os string, arch string, err error) {
	if platformStr == "" {
		return "", "", fmt.Errorf("platform string is empty")
	}

	parts := strings.Split(platformStr, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid platform format: %s (expected 'os/arch')", platformStr)
	}

	return parts[0], parts[1], nil
}

// convertToLLB converts a Warpgate config to BuildKit LLB
func (b *BuildKitBuilder) convertToLLB(cfg builder.Config) (llb.State, error) {
	// Check if this is a Dockerfile-based build
	if cfg.IsDockerfileBased() {
		return llb.State{}, fmt.Errorf("dockerfile-based builds should use BuildDockerfile, not convertToLLB")
	}

	// Parse platform from cfg.Base.Platform
	platformOS, platformArch, err := parsePlatform(cfg.Base.Platform)
	if err != nil {
		// Fallback to cfg.Architectures[0] if Platform is not set
		if len(cfg.Architectures) > 0 {
			platformOS = "linux"
			platformArch = cfg.Architectures[0]
		} else {
			return llb.State{}, fmt.Errorf("no platform specified in config: %w", err)
		}
	}

	// Start with base image
	platform := specs.Platform{
		OS:           platformOS,
		Architecture: platformArch,
	}

	state := llb.Image(cfg.Base.Image, llb.Platform(platform))

	// Apply base environment variables
	for key, value := range cfg.Base.Env {
		state = state.AddEnv(key, value)
	}

	// Apply build arguments as environment variables (standard Docker behavior)
	for key, value := range cfg.BuildArgs {
		state = state.AddEnv(key, value)
	}

	// Apply base changes (from base.changes in template)
	if len(cfg.Base.Changes) > 0 {
		state = b.applyPostChanges(state, cfg.Base.Changes)
	}

	// Apply provisioners
	for i, prov := range cfg.Provisioners {
		var err error
		state, err = b.applyProvisioner(state, prov, cfg)
		if err != nil {
			return llb.State{}, fmt.Errorf("provisioner %d failed: %w", i, err)
		}
	}

	// Apply post-changes
	state = b.applyPostChanges(state, cfg.PostChanges)

	return state, nil
}

// applyProvisioner applies a provisioner to the LLB state
func (b *BuildKitBuilder) applyProvisioner(state llb.State, prov builder.Provisioner, cfg builder.Config) (llb.State, error) {
	switch prov.Type {
	case "shell":
		return b.applyShellProvisioner(state, prov)
	case "file":
		return b.applyFileProvisioner(state, prov)
	case "ansible":
		return b.applyAnsibleProvisioner(state, prov)
	default:
		logging.Warn("Unsupported provisioner type: %s", prov.Type)
		return state, nil
	}
}

// applyShellProvisioner applies shell commands to LLB state
func (b *BuildKitBuilder) applyShellProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if len(prov.Inline) == 0 {
		return state, nil
	}

	// Combine commands into single RUN (like Dockerfile does)
	combinedCmd := strings.Join(prov.Inline, " && ")

	// Build run options with cache mounts
	runOpts := []llb.RunOption{
		llb.Shlex(fmt.Sprintf("sh -c '%s'", combinedCmd)),
	}

	// Add common cache mounts for package managers to improve build speed
	// These are shared across builds and help avoid re-downloading packages
	if strings.Contains(combinedCmd, "apt-get") {
		// APT cache for Debian/Ubuntu
		runOpts = append(runOpts,
			llb.AddMount("/var/cache/apt", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-apt-cache", llb.CacheMountShared)),
			llb.AddMount("/var/lib/apt/lists", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-apt-lists", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "yum") || strings.Contains(combinedCmd, "dnf") {
		// YUM/DNF cache for RHEL/Fedora/CentOS
		runOpts = append(runOpts,
			llb.AddMount("/var/cache/yum", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-yum-cache", llb.CacheMountShared)),
			llb.AddMount("/var/cache/dnf", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-dnf-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "apk") {
		// APK cache for Alpine
		runOpts = append(runOpts,
			llb.AddMount("/var/cache/apk", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-apk-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "pip") {
		// Pip cache for Python packages
		runOpts = append(runOpts,
			llb.AddMount("/root/.cache/pip", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-pip-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "npm") || strings.Contains(combinedCmd, "yarn") {
		// NPM/Yarn cache for Node.js packages
		runOpts = append(runOpts,
			llb.AddMount("/root/.npm", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-npm-cache", llb.CacheMountShared)),
			llb.AddMount("/root/.yarn", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-yarn-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "go ") || strings.Contains(combinedCmd, "go get") || strings.Contains(combinedCmd, "go build") {
		// Go module cache
		runOpts = append(runOpts,
			llb.AddMount("/go/pkg/mod", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-go-mod-cache", llb.CacheMountShared)),
			llb.AddMount("/root/.cache/go-build", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-go-build-cache", llb.CacheMountShared)),
		)
	}

	// Execute as shell command with cache mounts
	state = state.Run(runOpts...).Root()

	return state, nil
}

// applyFileProvisioner applies file copy operations to LLB state
func (b *BuildKitBuilder) applyFileProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if prov.Source == "" || prov.Destination == "" {
		return state, nil
	}

	// Convert source path to be relative to build context
	sourcePath, err := b.makeRelativePath(prov.Source)
	if err != nil {
		return state, fmt.Errorf("failed to resolve source path: %w", err)
	}

	// Copy file from local context
	state = state.File(
		llb.Copy(
			llb.Local("context"),
			sourcePath,
			prov.Destination,
		),
	)

	// Apply permissions if specified
	if prov.Mode != "" {
		state = state.Run(
			llb.Shlexf("chmod %s %s", prov.Mode, prov.Destination),
		).Root()
	}

	return state, nil
}

// applyAnsibleProvisioner applies Ansible playbook to LLB state
func (b *BuildKitBuilder) applyAnsibleProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if prov.PlaybookPath == "" {
		return state, nil
	}

	// Convert playbook path to be relative to build context
	playbookPath, err := b.makeRelativePath(prov.PlaybookPath)
	if err != nil {
		return state, fmt.Errorf("failed to resolve playbook path: %w", err)
	}

	// Copy playbook
	state = state.File(
		llb.Copy(
			llb.Local("context"),
			playbookPath,
			"/tmp/playbook.yml",
		),
	)

	// Check for collection (use absolute path for detection)
	absPlaybookPath := filepath.Join(b.contextDir, playbookPath)
	collectionRoot := detectCollectionRoot(absPlaybookPath)
	if collectionRoot != "" {
		// Make collection root relative to context
		relCollectionRoot, err := b.makeRelativePath(collectionRoot)
		if err != nil {
			logging.Warn("Failed to resolve collection root, skipping: %v", err)
		} else {
			// Copy entire collection
			state = state.File(
				llb.Copy(
					llb.Local("context"),
					relCollectionRoot,
					"/tmp/ansible-collection",
				),
			)
			// Install collection
			state = state.Run(
				llb.Shlex("ansible-galaxy collection install /tmp/ansible-collection/ -p /usr/share/ansible/collections"),
			).Root()
		}
	}

	// Copy galaxy requirements if specified
	if prov.GalaxyFile != "" {
		galaxyPath, err := b.makeRelativePath(prov.GalaxyFile)
		if err != nil {
			return state, fmt.Errorf("failed to resolve galaxy file path: %w", err)
		}
		state = state.File(
			llb.Copy(
				llb.Local("context"),
				galaxyPath,
				"/tmp/requirements.yml",
			),
		)
		state = state.Run(
			llb.Shlex("ansible-galaxy install -r /tmp/requirements.yml"),
		).Root()
	}

	// Build ansible-playbook command
	cmd := "ansible-playbook /tmp/playbook.yml -i localhost, -c local"
	for key, value := range prov.ExtraVars {
		cmd += fmt.Sprintf(" -e %s=%s", key, value)
	}

	// Run playbook with cache mounts for package managers
	// This helps when the playbook installs packages
	runOpts := []llb.RunOption{
		llb.Shlex(cmd),
		// Add cache for APT (common in Ansible playbooks)
		llb.AddMount("/var/cache/apt", llb.Scratch(),
			llb.AsPersistentCacheDir("warpgate-apt-cache", llb.CacheMountShared)),
		llb.AddMount("/var/lib/apt/lists", llb.Scratch(),
			llb.AsPersistentCacheDir("warpgate-apt-lists", llb.CacheMountShared)),
	}

	state = state.Run(runOpts...).Root()

	return state, nil
}

// applyPostChanges applies post-build changes to LLB state
func (b *BuildKitBuilder) applyPostChanges(state llb.State, postChanges []string) llb.State {
	// Track environment for variable expansion
	env := map[string]string{
		// Default PATH from standard Ubuntu base image
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	for _, change := range postChanges {
		parts := strings.Fields(change)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "ENV":
			// ENV KEY=value or ENV KEY value
			var key, value string
			switch {
			case strings.Contains(parts[1], "="):
				kv := strings.SplitN(parts[1], "=", 2)
				key, value = kv[0], kv[1]
			case len(parts) >= 3:
				key, value = parts[1], strings.Join(parts[2:], " ")
			default:
				continue
			}

			// Expand $VAR references using tracked environment
			expandedValue := b.expandContainerVars(value, env)
			state = state.AddEnv(key, expandedValue)

			// Update tracked environment
			env[key] = expandedValue

		case "WORKDIR":
			state = state.Dir(parts[1])
		case "USER":
			state = state.User(parts[1])
			// ENTRYPOINT and CMD handled via image metadata
		}
	}
	return state
}

// expandContainerVars expands $VAR references in a string using container environment
func (b *BuildKitBuilder) expandContainerVars(s string, env map[string]string) string {
	result := s
	// Simple expansion of $VAR patterns (unbraced only - braced already handled by loader)
	for key, value := range env {
		// Replace $KEY with value (but not ${KEY} which was already expanded)
		result = strings.ReplaceAll(result, "$"+key, value)
	}
	return result
}

// detectCollectionRoot detects if a playbook is from an Ansible collection source directory
// Returns the collection root directory if detected, empty string otherwise
func detectCollectionRoot(playbookPath string) string {
	// Check if path contains /playbooks/, /roles/, or similar collection structure
	if !strings.Contains(playbookPath, "/playbooks/") && !strings.Contains(playbookPath, "/roles/") {
		return ""
	}

	// Walk up the directory tree to find galaxy.yml
	dir := filepath.Dir(playbookPath)
	for dir != "/" && dir != "." {
		galaxyPath := filepath.Join(dir, "galaxy.yml")
		if _, err := os.Stat(galaxyPath); err == nil {
			// Found galaxy.yml, this is the collection root
			return dir
		}
		dir = filepath.Dir(dir)
	}

	return ""
}

// buildExportAttributes creates export attributes for BuildKit including labels
func buildExportAttributes(imageName string, labels map[string]string) map[string]string {
	exportAttrs := map[string]string{
		"name": imageName,
	}

	// Add labels to image metadata
	if len(labels) > 0 {
		for key, value := range labels {
			// BuildKit expects labels in the format "label:key=value"
			labelKey := fmt.Sprintf("label:%s", key)
			exportAttrs[labelKey] = value
			logging.Debug("Adding label to image: %s=%s", key, value)
		}
	}

	return exportAttrs
}

// configureCacheOptions configures cache import/export for BuildKit
func (b *BuildKitBuilder) configureCacheOptions(solveOpt *client.SolveOpt, noCache bool) {
	// Add cache import sources if specified (only if caching is enabled)
	if !noCache {
		if len(b.cacheFrom) > 0 {
			logging.Info("Configuring cache import from %d source(s)", len(b.cacheFrom))
			for _, cacheSource := range b.cacheFrom {
				solveOpt.CacheImports = append(solveOpt.CacheImports, client.CacheOptionsEntry{
					Type:  "registry",
					Attrs: parseCacheAttrs(cacheSource),
				})
			}
		}

		// Add cache export destinations if specified
		if len(b.cacheTo) > 0 {
			logging.Info("Configuring cache export to %d destination(s)", len(b.cacheTo))
			for _, cacheDest := range b.cacheTo {
				solveOpt.CacheExports = append(solveOpt.CacheExports, client.CacheOptionsEntry{
					Type:  "registry",
					Attrs: parseCacheAttrs(cacheDest),
				})
			}
		}
	} else {
		logging.Info("Caching disabled - building from scratch")
	}
}

// getLocalImageDigest retrieves the digest of a local Docker image using the Docker SDK
func (b *BuildKitBuilder) getLocalImageDigest(ctx context.Context, imageName string) string {
	inspect, err := b.dockerClient.ImageInspect(ctx, imageName)
	if err != nil {
		logging.Warn("Failed to inspect image %s: %v", imageName, err)
		return ""
	}

	// Try to get RepoDigest first (if image was pushed)
	if len(inspect.RepoDigests) > 0 {
		repoDigest := inspect.RepoDigests[0]
		// Extract just the digest part (format: registry/image@sha256:...)
		if strings.Contains(repoDigest, "@") {
			parts := strings.Split(repoDigest, "@")
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}

	// Fall back to image ID
	if inspect.ID != "" {
		return inspect.ID
	}

	return ""
}

// loadAndTagImage loads the built Docker image tar into Docker and tags it using Docker SDK
func (b *BuildKitBuilder) loadAndTagImage(ctx context.Context, imageTarPath, imageName string) error {
	// Open the tar file
	logging.Info("Loading image into Docker...")
	imageFile, err := os.Open(imageTarPath)
	if err != nil {
		return fmt.Errorf("failed to open image tar: %w", err)
	}
	defer func() {
		_ = imageFile.Close()
	}()

	// Load Docker image tar into Docker
	resp, err := b.dockerClient.ImageLoad(ctx, imageFile)
	if err != nil {
		return fmt.Errorf("failed to load image into Docker: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read and log the response
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		logging.Warn("Failed to read load response: %v", err)
	} else {
		logging.Debug("Image load response: %s", buf.String())
	}

	logging.Info("Image loaded successfully: %s", imageName)
	return nil
}

// getPlatformString extracts platform string from config
func getPlatformString(cfg builder.Config) string {
	switch {
	case cfg.Base.Platform != "":
		return cfg.Base.Platform
	case len(cfg.Architectures) > 0:
		return fmt.Sprintf("linux/%s", cfg.Architectures[0])
	default:
		return "unknown"
	}
}

// extractArchFromPlatform extracts architecture from platform string (e.g., "linux/amd64" -> "amd64")
func extractArchFromPlatform(platform string) string {
	parts := strings.Split(platform, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// calculateBuildContext finds the common parent directory of all files referenced in the config
// This allows builds to work from any directory by automatically determining the appropriate context
func (b *BuildKitBuilder) calculateBuildContext(cfg builder.Config) (string, error) {
	var paths []string

	// Collect all file paths from provisioners
	for _, prov := range cfg.Provisioners {
		switch prov.Type {
		case "ansible":
			if prov.PlaybookPath != "" {
				paths = append(paths, os.ExpandEnv(prov.PlaybookPath))
			}
			if prov.GalaxyFile != "" {
				paths = append(paths, os.ExpandEnv(prov.GalaxyFile))
			}
		case "file":
			if prov.Source != "" {
				paths = append(paths, os.ExpandEnv(prov.Source))
			}
		case "script":
			for _, script := range prov.Scripts {
				paths = append(paths, os.ExpandEnv(script))
			}
		}
	}

	// If no files referenced, use current directory
	if len(paths) == 0 {
		return ".", nil
	}

	// Convert all paths to absolute
	absPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			logging.Warn("Failed to get absolute path for %s: %v", p, err)
			continue
		}
		absPaths = append(absPaths, absPath)
	}

	if len(absPaths) == 0 {
		return ".", nil
	}

	// Find common parent directory
	commonParent := filepath.Dir(absPaths[0])
	for _, p := range absPaths[1:] {
		commonParent = findCommonParent(commonParent, p)
	}

	logging.Info("Calculated build context from %d file(s): %s", len(absPaths), commonParent)
	return commonParent, nil
}

// findCommonParent finds the common parent directory of two paths
func findCommonParent(path1, path2 string) string {
	// Ensure both are absolute
	abs1, err1 := filepath.Abs(path1)
	abs2, err2 := filepath.Abs(path2)
	if err1 != nil || err2 != nil {
		return "/"
	}

	// Split into components
	parts1 := strings.Split(filepath.Clean(abs1), string(filepath.Separator))
	parts2 := strings.Split(filepath.Clean(abs2), string(filepath.Separator))

	// Find common prefix
	var common []string
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] == parts2[i] {
			common = append(common, parts1[i])
		} else {
			break
		}
	}

	if len(common) == 0 {
		return "/"
	}

	// On Unix systems, the first element after split is empty (from leading /)
	// filepath.Join will handle this correctly and produce an absolute path
	result := filepath.Join(common...)

	// If result doesn't start with separator, it means we lost the root
	if !strings.HasPrefix(result, string(filepath.Separator)) {
		result = string(filepath.Separator) + result
	}

	return result
}

// makeRelativePath converts an absolute path to be relative to the build context
func (b *BuildKitBuilder) makeRelativePath(path string) (string, error) {
	absPath, err := filepath.Abs(os.ExpandEnv(path))
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	absContext, err := filepath.Abs(b.contextDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute context: %w", err)
	}

	relPath, err := filepath.Rel(absContext, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to make relative path: %w", err)
	}

	return relPath, nil
}

// Build creates a container image using BuildKit's Low-Level Build (LLB) primitives.
// It converts the Warpgate configuration to LLB, executes the build with BuildKit, and loads
// the resulting image into Docker's image store. The build process includes intelligent caching
// for package managers and proper handling of file copies and provisioners.
func (b *BuildKitBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	// Check if this is a Dockerfile-based build
	if cfg.IsDockerfileBased() {
		return b.BuildDockerfile(ctx, cfg)
	}

	startTime := time.Now()
	logging.Info("Building image: %s (native LLB)", cfg.Name)

	// Step 1: Calculate intelligent build context
	contextDir, err := b.calculateBuildContext(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate build context: %w", err)
	}
	logging.Debug("Using build context: %s", contextDir)

	// Step 2: Store context for use in provisioners
	b.contextDir = contextDir

	// Step 3: Convert to LLB
	state, err := b.convertToLLB(cfg)
	if err != nil {
		return nil, fmt.Errorf("LLB conversion failed: %w", err)
	}

	// Step 4: Marshal LLB definition
	def, err := state.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("LLB marshal failed: %w", err)
	}

	logging.Debug("LLB definition: %d bytes", len(def.Def))

	// Step 5: Prepare image name
	imageName := fmt.Sprintf("%s:%s", cfg.Name, cfg.Version)
	if cfg.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", cfg.Registry, imageName)
	}

	// Step 5: Export to Docker image tar and load into Docker
	imageTarPath := filepath.Join(os.TempDir(), fmt.Sprintf("warpgate-image-%d.tar", time.Now().Unix()))
	defer func() {
		if err := os.Remove(imageTarPath); err != nil {
			logging.Warn("Failed to remove temporary image tar: %v", err)
		}
	}()

	// Build export attributes
	exportAttrs := buildExportAttributes(imageName, cfg.Labels)

	solveOpt := client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type:   client.ExporterDocker,
				Output: fixedWriteCloser(imageTarPath),
				Attrs:  exportAttrs,
			},
		},
		LocalDirs: map[string]string{
			"context": contextDir,
		},
	}

	// Configure cache options
	b.configureCacheOptions(&solveOpt, cfg.NoCache)

	// Step 6: Execute with progress streaming
	ch := make(chan *client.SolveStatus)
	done := make(chan struct{})

	go b.displayProgress(ch, done)

	_, err = b.client.Solve(ctx, def, solveOpt, ch)
	<-done

	if err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	// Step 7: Load and tag image
	if err := b.loadAndTagImage(ctx, imageTarPath, imageName); err != nil {
		return nil, err
	}

	// Step 8: Get digest from local image
	digest := b.getLocalImageDigest(ctx, imageName)

	duration := time.Since(startTime)
	platform := getPlatformString(cfg)

	return &builder.BuildResult{
		ImageRef:     imageName,
		Digest:       digest,
		Architecture: extractArchFromPlatform(platform),
		Platform:     platform,
		Duration:     duration.String(),
		Notes:        []string{"Built with native BuildKit LLB", "Image loaded to Docker"},
	}, nil
}

// fixedWriteCloser returns a WriteCloser that exports to a file
func fixedWriteCloser(filepath string) func(map[string]string) (io.WriteCloser, error) {
	return func(m map[string]string) (io.WriteCloser, error) {
		f, err := os.Create(filepath)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
}

// displayProgress displays build progress
func (b *BuildKitBuilder) displayProgress(ch <-chan *client.SolveStatus, done chan<- struct{}) {
	defer close(done)

	for status := range ch {
		// codespell:ignore vertexes
		for _, vertex := range status.Vertexes {
			if vertex.Name != "" {
				logging.Debug("[%s] %s", vertex.Digest.String()[:12], vertex.Name)
			}
		}
		for _, log := range status.Logs {
			fmt.Print(string(log.Data))
		}
	}
}

// extractRegistryFromImageRef extracts the registry hostname from an image reference.
// For example, "ghcr.io/owner/repo:tag" returns "ghcr.io".
// If no registry is specified (e.g., "ubuntu:latest"), returns "docker.io".
func extractRegistryFromImageRef(imageRef string) string {
	// First, remove digest if present (format: image@sha256:...)
	parts := strings.SplitN(imageRef, "@", 2)
	imageWithoutDigest := parts[0]

	// Split by slash to get registry
	slashParts := strings.Split(imageWithoutDigest, "/")

	// If there's only one part (no slashes), it's Docker Hub
	if len(slashParts) == 1 {
		return "docker.io"
	}

	// The first part before the first slash is the registry candidate
	registryCandidate := slashParts[0]

	// Check if it's a registry by looking for:
	// 1. Contains a dot (e.g., ghcr.io, registry.example.com)
	// 2. Contains a colon (e.g., localhost:5000, registry.example.com:8080)
	// 3. Is "localhost"
	if strings.Contains(registryCandidate, ".") || strings.Contains(registryCandidate, ":") || registryCandidate == "localhost" {
		return registryCandidate
	}

	// Otherwise, it's likely a Docker Hub image with a namespace (e.g., library/ubuntu)
	return "docker.io"
}

// Push pushes the built image to a container registry using the Docker SDK.
// The imageRef should include the full registry path (e.g., "registry.io/org/image:tag").
// This method automatically reads credentials using go-containerregistry's DefaultKeychain
// which supports Docker config, credential helpers, and environment variables.
// It streams output for progress visibility and returns the image digest.
func (b *BuildKitBuilder) Push(ctx context.Context, imageRef, registry string) (string, error) {
	logging.Info("Pushing image: %s", imageRef)

	// Extract registry from image reference if not provided
	if registry == "" {
		registry = extractRegistryFromImageRef(imageRef)
		logging.Debug("Extracted registry from image ref: %s", registry)
	}

	// Extract just the registry hostname for auth lookup
	// The registry parameter may include namespace (e.g., "ghcr.io/l50")
	// but auth lookup needs just the hostname (e.g., "ghcr.io")
	registryHostname := extractRegistryFromImageRef(registry)
	logging.Debug("Using registry hostname for auth: %s", registryHostname)

	// Get authentication credentials using unified adapter
	registryAuth, err := registryauth.ToDockerSDKAuth(ctx, registryHostname)
	if err != nil {
		logging.Warn("Failed to get registry credentials: %v (attempting push anyway)", err)
		registryAuth = ""
	}

	// Create push options with authentication
	pushOpts := dockerimage.PushOptions{
		RegistryAuth: registryAuth,
	}

	// Push the image
	resp, err := b.dockerClient.ImagePush(ctx, imageRef, pushOpts)
	if err != nil {
		return "", fmt.Errorf("docker push failed: %w", err)
	}
	defer func() {
		_ = resp.Close()
	}()

	// Stream the output
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp); err != nil {
		logging.Warn("Failed to read push response: %v", err)
	} else {
		// Parse JSON output for errors
		output := buf.String()
		logging.Debug("Push response: %s", output)

		// Check for errors in the output
		if strings.Contains(output, "\"error\"") {
			return "", fmt.Errorf("push failed: %s", output)
		}
	}

	// Get the digest from the pushed image
	inspect, err := b.dockerClient.ImageInspect(ctx, imageRef)
	if err != nil {
		logging.Warn("Failed to inspect image after push: %v", err)
		return "", nil // Return empty digest, not an error - push was successful
	}

	// Extract digest from RepoDigests
	if len(inspect.RepoDigests) > 0 {
		repoDigest := inspect.RepoDigests[0]
		// Extract just the digest part (sha256:...)
		digestParts := strings.Split(repoDigest, "@")
		if len(digestParts) == 2 {
			digest := digestParts[1]
			logging.Info("Image digest: %s", digest)
			return digest, nil
		}
	}

	logging.Warn("No digest found for %s", imageRef)
	return "", nil
}

// Tag creates an additional tag for an existing image using the Docker SDK.
// The newTag should be the complete tag reference including registry if needed.
// This is useful for creating architecture-specific tags or version aliases.
func (b *BuildKitBuilder) Tag(ctx context.Context, imageRef, newTag string) error {
	if err := b.dockerClient.ImageTag(ctx, imageRef, newTag); err != nil {
		return fmt.Errorf("docker tag failed: %w", err)
	}

	logging.Debug("Tagged %s as %s", imageRef, newTag)
	return nil
}

// Remove deletes an image from the local Docker image store using the Docker SDK.
// This frees up disk space but does not affect images already pushed to registries.
// Returns an error if the image is in use by a running container.
func (b *BuildKitBuilder) Remove(ctx context.Context, imageRef string) error {
	removeOpts := dockerimage.RemoveOptions{
		Force:         false,
		PruneChildren: true,
	}

	_, err := b.dockerClient.ImageRemove(ctx, imageRef, removeOpts)
	if err != nil {
		return fmt.Errorf("docker rmi failed: %w", err)
	}

	logging.Debug("Removed image: %s", imageRef)
	return nil
}

// CreateAndPushManifest creates and pushes a multi-architecture manifest list to a registry.
// It uses BuildKit's native manifest creation capabilities to combine multiple architecture-specific
// images into a single manifest that allows Docker to automatically select the correct architecture.
// The manifestName should include the registry path (e.g., "registry.io/org/image:tag").
func (b *BuildKitBuilder) CreateAndPushManifest(ctx context.Context, manifestName string, entries []manifests.ManifestEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no manifest entries provided")
	}

	logging.Info("Creating multi-arch manifest: %s", manifestName)
	logging.Info("Manifest %s will include %d architectures:", manifestName, len(entries))

	// Build the list of manifests for the manifest list
	manifestDescriptors := make([]specs.Descriptor, 0, len(entries))

	for _, entry := range entries {
		logging.Info("  - %s/%s (digest: %s)", entry.OS, entry.Architecture, entry.Digest.String())

		// Validate that we have a digest
		if entry.Digest.String() == "" {
			return fmt.Errorf("no digest found for %s/%s", entry.OS, entry.Architecture)
		}

		// Parse the platform from the entry
		platform := specs.Platform{
			OS:           entry.OS,
			Architecture: entry.Architecture,
		}
		if entry.Variant != "" {
			platform.Variant = entry.Variant
		}

		// We need to get the image size from the registry
		// Try to inspect the image (Docker will fetch from registry if needed)
		inspect, err := b.dockerClient.ImageInspect(ctx, entry.ImageRef)
		var imageSize int64
		if err != nil {
			// If we can't inspect, log a warning and use 0 for size
			// The manifest will still work, but size won't be accurate
			logging.Warn("Failed to inspect image %s for size (using 0): %v", entry.ImageRef, err)
			imageSize = 0
		} else {
			imageSize = inspect.Size
		}

		// Create manifest descriptor using the digest from the entry
		manifestDescriptors = append(manifestDescriptors, specs.Descriptor{
			MediaType: "application/vnd.docker.distribution.manifest.v2+json",
			Digest:    entry.Digest,
			Size:      imageSize,
			Platform:  &platform,
		})
	}

	// Create the manifest list
	manifestList := specs.Index{
		Versioned: specsgo.Versioned{
			SchemaVersion: 2,
		},
		MediaType: "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests: manifestDescriptors,
	}

	// Marshal to JSON
	manifestJSON, err := json.Marshal(manifestList)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest list: %w", err)
	}

	logging.Debug("Manifest list JSON: %s", string(manifestJSON))
	logging.Info("Successfully created multi-arch manifest: %s", manifestName)

	// Push the manifest list to the registry using go-containerregistry
	ref, err := name.ParseReference(manifestName)
	if err != nil {
		return fmt.Errorf("failed to parse manifest name: %w", err)
	}

	// Build an image index with the manifest descriptors
	adds := make([]mutate.IndexAddendum, 0, len(entries))
	for _, entry := range entries {
		// Parse the image reference to get the remote descriptor
		imgRef, err := name.ParseReference(entry.ImageRef)
		if err != nil {
			logging.Warn("Failed to parse image reference %s: %v (skipping)", entry.ImageRef, err)
			continue
		}

		// Get the remote descriptor for this image
		desc, err := remote.Get(imgRef, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
		if err != nil {
			logging.Warn("Failed to get remote descriptor for %s: %v (skipping)", entry.ImageRef, err)
			continue
		}

		// Convert descriptor to image
		img, err := desc.Image()
		if err != nil {
			logging.Warn("Failed to convert descriptor to image for %s: %v (skipping)", entry.ImageRef, err)
			continue
		}

		// Create the platform spec
		platform := v1.Platform{
			OS:           entry.OS,
			Architecture: entry.Architecture,
		}
		if entry.Variant != "" {
			platform.Variant = entry.Variant
		}

		// Add to the index with platform information
		// Use the descriptor from remote.Get but override the platform
		adds = append(adds, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				MediaType: desc.MediaType,
				Size:      desc.Size,
				Digest:    desc.Digest,
				Platform:  &platform,
			},
		})
	}

	// Create an empty index and append all manifests
	mediaType := types.MediaType(manifestList.MediaType)
	idx := mutate.IndexMediaType(mutate.AppendManifests(
		mutate.IndexMediaType(empty.Index, mediaType),
		adds...,
	), mediaType)

	// Push the index to the registry
	logging.Info("Pushing manifest list to registry: %s", manifestName)
	if err := remote.WriteIndex(ref, idx,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	); err != nil {
		return fmt.Errorf("failed to push manifest list: %w", err)
	}

	logging.Info("Successfully pushed multi-arch manifest: %s", manifestName)
	return nil
}

// InspectManifest retrieves information about a manifest list from Docker.
// It uses the Docker SDK to inspect the manifest and extract platform information.
// This can be used to verify that a multi-arch manifest was created correctly.
func (b *BuildKitBuilder) InspectManifest(ctx context.Context, manifestName string) ([]manifests.ManifestEntry, error) {
	logging.Debug("Inspecting manifest: %s", manifestName)

	// Use Docker SDK to get distribution inspect (this provides manifest info)
	inspect, err := b.dockerClient.DistributionInspect(ctx, manifestName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to inspect manifest: %w", err)
	}

	logging.Debug("Manifest descriptor: %+v", inspect.Descriptor)

	// Create manifest entries from the inspection
	var entries []manifests.ManifestEntry

	// If this is a manifest list, the platforms would be in the descriptor
	if inspect.Descriptor.Platform != nil {
		entries = append(entries, manifests.ManifestEntry{
			ImageRef:     manifestName,
			OS:           inspect.Descriptor.Platform.OS,
			Architecture: inspect.Descriptor.Platform.Architecture,
			Variant:      inspect.Descriptor.Platform.Variant,
			Digest:       digest.Digest(inspect.Descriptor.Digest.String()),
		})
	}

	logging.Debug("Found %d manifest entries", len(entries))
	return entries, nil
}

// BuildDockerfile builds a container image using a Dockerfile with BuildKit's native client.
// This method is used when the config specifies a dockerfile section instead of provisioners.
// It uses BuildKit's dockerfile frontend to build directly without shelling out to docker CLI.
func (b *BuildKitBuilder) BuildDockerfile(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	startTime := time.Now()
	logging.Info("Building image from Dockerfile: %s (native BuildKit)", cfg.Name)

	// Prepare image name
	imageName := fmt.Sprintf("%s:%s", cfg.Name, cfg.Version)
	if cfg.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", cfg.Registry, imageName)
	}

	// Get Dockerfile configuration (paths are already absolute from config loader)
	dockerfileCfg := cfg.Dockerfile
	dockerfilePath := dockerfileCfg.GetDockerfilePath()
	buildContext := dockerfileCfg.GetBuildContext()

	logging.Debug("Dockerfile: %s, Context: %s", dockerfilePath, buildContext)

	// Calculate relative Dockerfile path from context
	relDockerfilePath, err := filepath.Rel(buildContext, dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate relative Dockerfile path: %w", err)
	}

	// Prepare frontend attributes for BuildKit
	frontendAttrs := map[string]string{
		"filename": relDockerfilePath,
	}

	// Add target if specified
	if dockerfileCfg.Target != "" {
		frontendAttrs["target"] = dockerfileCfg.Target
	}

	// Add build arguments from dockerfile config
	for key, value := range dockerfileCfg.Args {
		frontendAttrs[fmt.Sprintf("build-arg:%s", key)] = value
	}

	// Add build arguments from CLI flags
	for key, value := range cfg.BuildArgs {
		frontendAttrs[fmt.Sprintf("build-arg:%s", key)] = value
	}

	// Add platform for single-arch builds
	if len(cfg.Architectures) == 1 {
		frontendAttrs["platform"] = fmt.Sprintf("linux/%s", cfg.Architectures[0])
	}

	// Add no-cache if specified
	if cfg.NoCache {
		frontendAttrs["no-cache"] = ""
	}

	// Export to Docker image tar and load into Docker
	imageTarPath := filepath.Join(os.TempDir(), fmt.Sprintf("warpgate-image-%d.tar", time.Now().Unix()))
	defer func() {
		if err := os.Remove(imageTarPath); err != nil {
			logging.Warn("Failed to remove temporary image tar: %v", err)
		}
	}()

	// Build export attributes
	exportAttrs := buildExportAttributes(imageName, cfg.Labels)

	// Prepare solve options
	solveOpt := client.SolveOpt{
		Frontend:      "dockerfile.v0", // Use BuildKit's dockerfile frontend
		FrontendAttrs: frontendAttrs,
		Exports: []client.ExportEntry{
			{
				Type:   client.ExporterDocker,
				Output: fixedWriteCloser(imageTarPath),
				Attrs:  exportAttrs,
			},
		},
		LocalDirs: map[string]string{
			"context":    buildContext,
			"dockerfile": filepath.Dir(dockerfilePath),
		},
	}

	// Configure cache options
	b.configureCacheOptions(&solveOpt, cfg.NoCache)

	// Execute build with progress streaming
	ch := make(chan *client.SolveStatus)
	done := make(chan struct{})

	go b.displayProgress(ch, done)

	_, err = b.client.Solve(ctx, nil, solveOpt, ch)
	<-done

	if err != nil {
		return nil, fmt.Errorf("dockerfile build failed: %w", err)
	}

	// Load and tag image
	if err := b.loadAndTagImage(ctx, imageTarPath, imageName); err != nil {
		return nil, err
	}

	// Get digest from local image
	digest := b.getLocalImageDigest(ctx, imageName)

	duration := time.Since(startTime)

	return &builder.BuildResult{
		ImageRef:     imageName,
		Digest:       digest,
		Architecture: cfg.Architectures[0],
		Platform:     fmt.Sprintf("linux/%s", cfg.Architectures[0]),
		Duration:     duration.String(),
		Notes:        []string{"Built from Dockerfile with native BuildKit", "Image loaded to Docker"},
	}, nil
}

// Close releases resources and closes connections to BuildKit and Docker daemons.
// This should be called when the builder is no longer needed, typically via defer.
// Calling Close multiple times is safe and will not cause errors.
func (b *BuildKitBuilder) Close() error {
	var errs []error

	if b.client != nil {
		if err := b.client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close BuildKit client: %w", err))
		}
	}

	if b.dockerClient != nil {
		if err := b.dockerClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close Docker client: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}
