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
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	dockerconfig "github.com/docker/cli/cli/config"
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

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	digest "github.com/opencontainers/go-digest"
	specsgo "github.com/opencontainers/image-spec/specs-go"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	// CRITICAL: This enables docker-container:// protocol
	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/errors"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/cowdogmoo/warpgate/v3/templates"
)

// BuildKitBuilder implements container image building using Docker BuildKit.
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
// Supports auto-detect, explicit endpoint (docker-container://, tcp://, unix://), and remote TCP with TLS.
func NewBuildKitBuilder(ctx context.Context) (*BuildKitBuilder, error) {
	cfg, err := config.Load()
	if err != nil {
		logging.WarnContext(ctx, "Failed to load config, using defaults: %v", err)
		cfg = &config.Config{}
	}

	var addr string
	var builderName, containerName string

	// Determine connection address
	if cfg.BuildKit.Endpoint != "" {
		// Use configured endpoint
		addr = cfg.BuildKit.Endpoint
		logging.InfoContext(ctx, "Using configured BuildKit endpoint: %s", addr)
	} else {
		// Auto-detect local buildx builder
		builderName, containerName, err = detectBuildxBuilder(ctx)
		if err != nil {
			return nil, errors.Wrap("detect buildx builder", "set buildkit.endpoint in config for remote BuildKit", err)
		}
		addr = fmt.Sprintf("docker-container://%s", containerName)
		logging.InfoContext(ctx, "Auto-detected BuildKit builder: %s", containerName)
	}

	clientOpts := []client.ClientOpt{}

	if strings.HasPrefix(addr, "tcp://") && cfg.BuildKit.TLSEnabled {
		tlsConfig, err := loadTLSConfig(cfg.BuildKit)
		if err != nil {
			return nil, errors.Wrap("load TLS config", "", err)
		}
		clientOpts = append(clientOpts, client.WithGRPCDialOption(
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		))
		logging.InfoContext(ctx, "TLS enabled for BuildKit connection")
	} else if strings.HasPrefix(addr, "tcp://") {
		clientOpts = append(clientOpts, client.WithGRPCDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		))
		logging.WarnContext(ctx, "Connecting to BuildKit without TLS (insecure)")
	}

	c, err := client.New(ctx, addr, clientOpts...)
	if err != nil {
		return nil, errors.Wrap("connect to BuildKit", "", err)
	}

	info, err := c.Info(ctx)
	if err != nil {
		_ = c.Close()
		return nil, errors.Wrap("verify BuildKit connection", "", err)
	}

	logging.InfoContext(ctx, "BuildKit client connected (version %s)", info.BuildkitVersion.Version)

	dockerCli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		_ = c.Close()
		return nil, errors.Wrap("create Docker client", "", err)
	}

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

// loadTLSConfig creates a TLS configuration for secure BuildKit connections.
// This is used when connecting to remote BuildKit instances over TCP with TLS.
//
// The function configures:
//   - TLS 1.3 minimum version for security
//   - Optional CA certificate for server verification (cfg.TLSCACert)
//   - Optional client certificate for mutual TLS (cfg.TLSCert + cfg.TLSKey)
//
// Certificate files should be PEM-encoded. Returns an error if any specified
// certificate file cannot be read or parsed.
func loadTLSConfig(cfg config.BuildKitConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

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

// createAuthProvider creates a BuildKit session auth provider from Docker config.
// This enables authentication for base image pulls from private registries.
// Returns nil if no Docker config is available (falls back to anonymous access).
func createAuthProvider() []session.Attachable {
	ctx := context.Background()
	dockerCfg, err := dockerconfig.Load(dockerconfig.Dir())
	if err != nil {
		logging.DebugContext(ctx, "Failed to load Docker config for auth: %v (using anonymous access)", err)
		return nil
	}

	ap := authprovider.NewDockerAuthProvider(authprovider.DockerAuthProviderConfig{
		ConfigFile: dockerCfg,
	})

	logging.DebugContext(ctx, "Created auth provider from Docker config for base image pulls")
	return []session.Attachable{ap}
}

// SetCacheOptions configures external cache sources and destinations for BuildKit.
// Cache sources (cacheFrom) are used to import cache from external registries.
// Cache destinations (cacheTo) specify where to export cache after the build.
// Format example: "type=registry,ref=user/app:cache,mode=max"
func (b *BuildKitBuilder) SetCacheOptions(ctx context.Context, cacheFrom, cacheTo []string) {
	b.cacheFrom = cacheFrom
	b.cacheTo = cacheTo
	if len(cacheFrom) > 0 {
		logging.InfoContext(ctx, "BuildKit cache sources: %v", cacheFrom)
	}
	if len(cacheTo) > 0 {
		logging.InfoContext(ctx, "BuildKit cache destinations: %v", cacheTo)
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

// detectBuildxBuilder detects the active buildx builder using Docker SDK.
// It scans running containers for buildx builder instances (containers with
// names prefixed "buildx_buildkit_") and returns the builder name and container name.
//
// This function creates a temporary Docker client connection which is properly
// closed before returning. The context is used for the Docker API calls and
// should have an appropriate timeout for container listing operations.
//
// Returns:
//   - builderName: The name of the buildx builder (e.g., "default")
//   - containerName: The full container name (e.g., "buildx_buildkit_default0")
//   - error: Non-nil if no builder found or Docker communication fails
func detectBuildxBuilder(ctx context.Context) (string, string, error) {
	dockerCli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return "", "", fmt.Errorf("failed to create Docker client: %w\n\nPlease install Docker Desktop or Docker Engine", err)
	}
	defer func() {
		if err := dockerCli.Close(); err != nil {
			logging.WarnContext(ctx, "Failed to close Docker client: %v", err)
		}
	}()

	_, err = dockerCli.Ping(ctx)
	if err != nil {
		return "", "", fmt.Errorf("cannot connect to Docker daemon: %w\n\nPlease ensure Docker is running", err)
	}

	containers, err := dockerCli.ContainerList(ctx, dockercontainer.ListOptions{
		All: true,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		if container.State != "running" {
			continue
		}

		for _, name := range container.Names {
			name = strings.TrimPrefix(name, "/")

			if strings.HasPrefix(name, "buildx_buildkit_") {
				builderName := strings.TrimPrefix(name, "buildx_buildkit_")
				builderName = strings.TrimSuffix(builderName, "0")

				logging.DebugContext(ctx, "Found buildx builder: %s (container: %s)", builderName, name)
				return builderName, name, nil
			}
		}
	}

	return "", "", fmt.Errorf("no running buildx builder found\n\nRun 'docker buildx create --use' to create one")
}

// parsePlatform parses a platform string into OS and architecture.
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

// convertToLLB converts a Warpgate config to BuildKit LLB.
func (b *BuildKitBuilder) convertToLLB(cfg builder.Config) (llb.State, error) {
	if cfg.IsDockerfileBased() {
		return llb.State{}, fmt.Errorf("dockerfile-based builds should use BuildDockerfile, not convertToLLB")
	}

	platformOS, platformArch, err := parsePlatform(cfg.Base.Platform)
	if err != nil {
		if len(cfg.Architectures) > 0 {
			platformOS = "linux"
			platformArch = cfg.Architectures[0]
		} else {
			return llb.State{}, fmt.Errorf("no platform specified in config: %w", err)
		}
	}

	platform := specs.Platform{
		OS:           platformOS,
		Architecture: platformArch,
	}

	state := llb.Image(cfg.Base.Image, llb.Platform(platform))

	for key, value := range cfg.Base.Env {
		state = state.AddEnv(key, value)
	}

	for key, value := range cfg.BuildArgs {
		state = state.AddEnv(key, value)
	}

	if len(cfg.Base.Changes) > 0 {
		state = b.applyPostChanges(state, cfg.Base.Changes)
	}

	for i, prov := range cfg.Provisioners {
		var err error
		state, err = b.applyProvisioner(state, prov, cfg)
		if err != nil {
			return llb.State{}, fmt.Errorf("provisioner %d failed: %w", i, err)
		}
	}

	state = b.applyPostChanges(state, cfg.PostChanges)

	return state, nil
}

// applyProvisioner applies a provisioner to the LLB state.
func (b *BuildKitBuilder) applyProvisioner(state llb.State, prov builder.Provisioner, cfg builder.Config) (llb.State, error) {
	ctx := context.Background()
	switch prov.Type {
	case "shell":
		return b.applyShellProvisioner(state, prov)
	case "file":
		return b.applyFileProvisioner(state, prov)
	case "ansible":
		return b.applyAnsibleProvisioner(state, prov)
	case "script":
		return b.applyScriptProvisioner(state, prov)
	case "powershell":
		return b.applyPowerShellProvisioner(state, prov)
	default:
		logging.WarnContext(ctx, "Unsupported provisioner type: %s", prov.Type)
		return state, nil
	}
}

// applyShellProvisioner applies shell commands to LLB state.
func (b *BuildKitBuilder) applyShellProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if len(prov.Inline) == 0 {
		return state, nil
	}

	combinedCmd := strings.Join(prov.Inline, " && ")

	runOpts := []llb.RunOption{
		llb.Shlex(fmt.Sprintf("sh -c '%s'", combinedCmd)),
	}

	if strings.Contains(combinedCmd, "apt-get") {
		runOpts = append(runOpts,
			llb.AddMount("/var/cache/apt", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-apt-cache", llb.CacheMountShared)),
			llb.AddMount("/var/lib/apt/lists", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-apt-lists", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "yum") || strings.Contains(combinedCmd, "dnf") {
		runOpts = append(runOpts,
			llb.AddMount("/var/cache/yum", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-yum-cache", llb.CacheMountShared)),
			llb.AddMount("/var/cache/dnf", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-dnf-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "apk") {
		runOpts = append(runOpts,
			llb.AddMount("/var/cache/apk", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-apk-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "pip") {
		runOpts = append(runOpts,
			llb.AddMount("/root/.cache/pip", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-pip-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "npm") || strings.Contains(combinedCmd, "yarn") {
		runOpts = append(runOpts,
			llb.AddMount("/root/.npm", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-npm-cache", llb.CacheMountShared)),
			llb.AddMount("/root/.yarn", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-yarn-cache", llb.CacheMountShared)),
		)
	}

	if strings.Contains(combinedCmd, "go ") || strings.Contains(combinedCmd, "go get") || strings.Contains(combinedCmd, "go build") {
		runOpts = append(runOpts,
			llb.AddMount("/go/pkg/mod", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-go-mod-cache", llb.CacheMountShared)),
			llb.AddMount("/root/.cache/go-build", llb.Scratch(),
				llb.AsPersistentCacheDir("warpgate-go-build-cache", llb.CacheMountShared)),
		)
	}

	state = state.Run(runOpts...).Root()

	return state, nil
}

// applyFileProvisioner applies file copy operations to LLB state.
func (b *BuildKitBuilder) applyFileProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	ctx := context.Background()
	logging.DebugContext(ctx, "[FILE PROV] Starting - source: %s, dest: %s", prov.Source, prov.Destination)

	if prov.Source == "" || prov.Destination == "" {
		logging.DebugContext(ctx, "[FILE PROV] Empty source or dest, skipping")
		return state, nil
	}

	sourcePath, err := b.makeRelativePath(prov.Source)
	if err != nil {
		return state, fmt.Errorf("failed to resolve source path: %w", err)
	}

	logging.DebugContext(ctx, "[FILE PROV] Resolved source path: %s", sourcePath)

	absSource, err := filepath.Abs(filepath.Join(b.contextDir, sourcePath))
	if err != nil {
		return state, fmt.Errorf("failed to resolve absolute source path: %w", err)
	}

	logging.DebugContext(ctx, "[FILE PROV] Context: %s, Source: %s, Abs: %s, Dest: %s",
		b.contextDir, sourcePath, absSource, prov.Destination)

	info, err := os.Stat(absSource)
	if err != nil {
		return state, fmt.Errorf("failed to stat source %s: %w", absSource, err)
	}

	logging.DebugContext(ctx, "[FILE PROV] Source is dir: %v, size: %d", info.IsDir(), info.Size())

	if info.IsDir() {
		// For directories: BuildKit copies the directory INTO the destination
		// So copying "ares" to "/tmp" creates "/tmp/ares"
		logging.DebugContext(ctx, "[FILE PROV] Directory copy: %s -> %s (will create %s/%s)",
			sourcePath, prov.Destination, prov.Destination, filepath.Base(sourcePath))

		state = state.File(
			llb.Copy(
				llb.Local("context"),
				sourcePath,
				prov.Destination,
				&llb.CopyInfo{
					CreateDestPath: true,
				},
			),
		)
	} else {
		state = state.File(
			llb.Copy(
				llb.Local("context"),
				sourcePath,
				prov.Destination,
			),
		)
	}

	if prov.Mode != "" {
		state = state.Run(
			llb.Shlexf("chmod %s %s", prov.Mode, prov.Destination),
		).Root()
	}

	return state, nil
}

// applyScriptProvisioner applies script files to LLB state.
func (b *BuildKitBuilder) applyScriptProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if len(prov.Scripts) == 0 {
		return state, nil
	}

	for _, script := range prov.Scripts {
		scriptPath, err := b.makeRelativePath(script)
		if err != nil {
			return state, fmt.Errorf("failed to resolve script path: %w", err)
		}

		destPath := fmt.Sprintf("/tmp/%s", filepath.Base(script))
		state = state.File(
			llb.Copy(
				llb.Local("context"),
				scriptPath,
				destPath,
			),
		)

		state = state.Run(
			llb.Shlexf("chmod +x %s && %s", destPath, destPath),
		).Root()
	}

	return state, nil
}

// applyPowerShellProvisioner applies PowerShell scripts to LLB state.
func (b *BuildKitBuilder) applyPowerShellProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if len(prov.PSScripts) == 0 {
		return state, nil
	}

	for _, script := range prov.PSScripts {
		scriptPath, err := b.makeRelativePath(script)
		if err != nil {
			return state, fmt.Errorf("failed to resolve PowerShell script path: %w", err)
		}

		destPath := fmt.Sprintf("/tmp/%s", filepath.Base(script))
		state = state.File(
			llb.Copy(
				llb.Local("context"),
				scriptPath,
				destPath,
			),
		)

		state = state.Run(
			llb.Shlexf("pwsh -ExecutionPolicy Bypass -File %s", destPath),
		).Root()
	}

	return state, nil
}

// applyAnsibleProvisioner applies Ansible playbook to LLB state.
func (b *BuildKitBuilder) applyAnsibleProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if prov.PlaybookPath == "" {
		return state, nil
	}

	playbookPath, err := b.makeRelativePath(prov.PlaybookPath)
	if err != nil {
		return state, fmt.Errorf("failed to resolve playbook path: %w", err)
	}

	state = state.File(
		llb.Copy(
			llb.Local("context"),
			playbookPath,
			"/tmp/playbook.yml",
		),
	)

	absPlaybookPath := filepath.Join(b.contextDir, playbookPath)
	collectionRoot := detectCollectionRoot(absPlaybookPath)
	if collectionRoot != "" {
		relCollectionRoot, err := b.makeRelativePath(collectionRoot)
		if err != nil {
			ctx := context.Background()
			logging.WarnContext(ctx, "Failed to resolve collection root, skipping: %v", err)
		} else {
			state = state.File(
				llb.Copy(
					llb.Local("context"),
					relCollectionRoot,
					"/tmp/ansible-collection",
				),
			)
			state = state.Run(
				llb.Shlex("ansible-galaxy collection install /tmp/ansible-collection/ -p /usr/share/ansible/collections"),
			).Root()
		}
	}

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

	cmd := "ansible-playbook /tmp/playbook.yml -i localhost, -c local"
	for key, value := range prov.ExtraVars {
		cmd += fmt.Sprintf(" -e %s=%s", key, value)
	}

	runOpts := []llb.RunOption{
		llb.Shlex(cmd),
		llb.AddMount("/var/cache/apt", llb.Scratch(),
			llb.AsPersistentCacheDir("warpgate-apt-cache", llb.CacheMountShared)),
		llb.AddMount("/var/lib/apt/lists", llb.Scratch(),
			llb.AsPersistentCacheDir("warpgate-apt-lists", llb.CacheMountShared)),
	}

	state = state.Run(runOpts...).Root()

	return state, nil
}

// applyPostChanges applies post-build changes to LLB state.
func (b *BuildKitBuilder) applyPostChanges(state llb.State, postChanges []string) llb.State {
	env := map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	for _, change := range postChanges {
		parts := strings.Fields(change)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "ENV":
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

			expandedValue := b.expandContainerVars(value, env)
			state = state.AddEnv(key, expandedValue)
			env[key] = expandedValue

		case "WORKDIR":
			state = state.Dir(parts[1])
		case "USER":
			state = state.User(parts[1])
		}
	}
	return state
}

// expandContainerVars expands $VAR references in a string using container environment.
func (b *BuildKitBuilder) expandContainerVars(s string, env map[string]string) string {
	result := s
	for key, value := range env {
		result = strings.ReplaceAll(result, "$"+key, value)
	}
	return result
}

// detectCollectionRoot detects if a playbook is from an Ansible collection source directory.
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
	ctx := context.Background()
	exportAttrs := map[string]string{
		"name": imageName,
	}

	if len(labels) > 0 {
		for key, value := range labels {
			// BuildKit expects labels in the format "label:key=value"
			labelKey := fmt.Sprintf("label:%s", key)
			exportAttrs[labelKey] = value
			logging.DebugContext(ctx, "Adding label to image: %s=%s", key, value)
		}
	}

	return exportAttrs
}

// configureCacheOptions configures cache import/export for BuildKit.
func (b *BuildKitBuilder) configureCacheOptions(solveOpt *client.SolveOpt, cfg builder.Config) {
	ctx := context.Background()
	// Determine if caching should be disabled
	// For local templates, disable caching by default to ensure changes are reflected
	// Can be overridden with explicit cache parameters (--cache-from, --cache-to)
	noCache := cfg.NoCache || cfg.IsLocalTemplate

	if !noCache {
		if len(b.cacheFrom) > 0 {
			logging.InfoContext(ctx, "Configuring cache import from %d source(s)", len(b.cacheFrom))
			for _, cacheSource := range b.cacheFrom {
				solveOpt.CacheImports = append(solveOpt.CacheImports, client.CacheOptionsEntry{
					Type:  "registry",
					Attrs: parseCacheAttrs(cacheSource),
				})
			}
		}

		if len(b.cacheTo) > 0 {
			logging.InfoContext(ctx, "Configuring cache export to %d destination(s)", len(b.cacheTo))
			for _, cacheDest := range b.cacheTo {
				solveOpt.CacheExports = append(solveOpt.CacheExports, client.CacheOptionsEntry{
					Type:  "registry",
					Attrs: parseCacheAttrs(cacheDest),
				})
			}
		}
	} else {
		reason := "building from scratch"
		if cfg.IsLocalTemplate {
			reason = "local template detected (changes will be reflected immediately)"
		}
		logging.InfoContext(ctx, "Caching disabled - %s", reason)
	}
}

// getLocalImageDigest retrieves the digest of a local Docker image using the Docker SDK
func (b *BuildKitBuilder) getLocalImageDigest(ctx context.Context, imageName string) string {
	inspect, err := b.dockerClient.ImageInspect(ctx, imageName)
	if err != nil {
		logging.WarnContext(ctx, "Failed to inspect image %s: %v", imageName, err)
		return ""
	}

	if len(inspect.RepoDigests) > 0 {
		repoDigest := inspect.RepoDigests[0]
		if strings.Contains(repoDigest, "@") {
			parts := strings.Split(repoDigest, "@")
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}

	if inspect.ID != "" {
		return inspect.ID
	}

	return ""
}

// loadAndTagImage loads the built Docker image tar into Docker and tags it using Docker SDK
func (b *BuildKitBuilder) loadAndTagImage(ctx context.Context, imageTarPath, imageName string) error {
	logging.InfoContext(ctx, "Loading image into Docker...")
	imageFile, err := os.Open(imageTarPath)
	if err != nil {
		return fmt.Errorf("failed to open image tar: %w", err)
	}
	defer func() {
		if err := imageFile.Close(); err != nil {
			logging.WarnContext(ctx, "Failed to close image file: %v", err)
		}
	}()

	resp, err := b.dockerClient.ImageLoad(ctx, imageFile)
	if err != nil {
		return fmt.Errorf("failed to load image into Docker: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.WarnContext(ctx, "Failed to close response body: %v", err)
		}
	}()

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		logging.WarnContext(ctx, "Failed to read load response: %v", err)
	} else {
		logging.DebugContext(ctx, "Image load response: %s", buf.String())
	}

	logging.InfoContext(ctx, "Image loaded successfully: %s", imageName)
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

// calculateBuildContext finds the common parent directory of all files referenced in the config.
func (b *BuildKitBuilder) calculateBuildContext(cfg builder.Config) (string, error) {
	pv := templates.NewPathValidator()

	paths, err := b.collectProvisionerPaths(cfg.Provisioners, pv)
	if err != nil {
		return "", err
	}

	if len(paths) == 0 {
		return ".", nil
	}

	contextRoots := make([]string, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			contextRoots = append(contextRoots, filepath.Dir(p))
			continue
		}
		if info.IsDir() {
			contextRoots = append(contextRoots, p)
		} else {
			contextRoots = append(contextRoots, filepath.Dir(p))
		}
	}

	commonParent := contextRoots[0]
	for _, p := range contextRoots[1:] {
		commonParent = findCommonParent(commonParent, p)
	}

	ctx := context.Background()
	logging.DebugContext(ctx, "Calculated build context from %d file(s): %s", len(paths), commonParent)
	return commonParent, nil
}

// collectProvisionerPaths extracts all file paths from provisioners.
func (b *BuildKitBuilder) collectProvisionerPaths(provisioners []builder.Provisioner, pv *templates.PathValidator) ([]string, error) {
	var paths []string

	for _, prov := range provisioners {
		provPaths, err := b.getProvisionerPaths(prov, pv)
		if err != nil {
			return nil, err
		}
		paths = append(paths, provPaths...)
	}

	return paths, nil
}

// getProvisionerPaths returns all file paths for a single provisioner.
func (b *BuildKitBuilder) getProvisionerPaths(prov builder.Provisioner, pv *templates.PathValidator) ([]string, error) {
	switch prov.Type {
	case "ansible":
		return b.getAnsiblePaths(prov, pv)
	case "file":
		return b.getFilePaths(prov, pv)
	case "script":
		return expandPathList(prov.Scripts, pv, "script")
	case "powershell":
		return expandPathList(prov.PSScripts, pv, "PowerShell script")
	default:
		return nil, nil
	}
}

// getAnsiblePaths returns paths for ansible provisioner.
func (b *BuildKitBuilder) getAnsiblePaths(prov builder.Provisioner, pv *templates.PathValidator) ([]string, error) {
	var paths []string
	if prov.PlaybookPath != "" {
		expanded, err := pv.ExpandPath(prov.PlaybookPath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand playbook path %s: %w", prov.PlaybookPath, err)
		}
		paths = append(paths, expanded)
	}
	if prov.GalaxyFile != "" {
		expanded, err := pv.ExpandPath(prov.GalaxyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to expand galaxy file path %s: %w", prov.GalaxyFile, err)
		}
		paths = append(paths, expanded)
	}
	return paths, nil
}

// getFilePaths returns paths for file provisioner.
func (b *BuildKitBuilder) getFilePaths(prov builder.Provisioner, pv *templates.PathValidator) ([]string, error) {
	if prov.Source == "" {
		return nil, nil
	}
	expanded, err := pv.ExpandPath(prov.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to expand file source path %s: %w", prov.Source, err)
	}
	return []string{expanded}, nil
}

// expandPathList expands a list of paths using the path validator.
func expandPathList(scripts []string, pv *templates.PathValidator, pathType string) ([]string, error) {
	var paths []string
	for _, script := range scripts {
		expanded, err := pv.ExpandPath(script)
		if err != nil {
			return nil, fmt.Errorf("failed to expand %s path %s: %w", pathType, script, err)
		}
		paths = append(paths, expanded)
	}
	return paths, nil
}

// findCommonParent finds the common parent directory of two paths
func findCommonParent(path1, path2 string) string {
	abs1, err1 := filepath.Abs(path1)
	abs2, err2 := filepath.Abs(path2)
	if err1 != nil || err2 != nil {
		return "/"
	}

	parts1 := strings.Split(filepath.Clean(abs1), string(filepath.Separator))
	parts2 := strings.Split(filepath.Clean(abs2), string(filepath.Separator))

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

	result := filepath.Join(common...)

	if !strings.HasPrefix(result, string(filepath.Separator)) {
		result = string(filepath.Separator) + result
	}

	return result
}

// makeRelativePath converts an absolute path to be relative to the build context
func (b *BuildKitBuilder) makeRelativePath(path string) (string, error) {
	pv := templates.NewPathValidator()
	absPath, err := pv.ExpandPath(path)
	if err != nil {
		return "", fmt.Errorf("failed to expand path: %w", err)
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

// Build creates a container image using BuildKit's Low-Level Build (LLB) primitives
// by converting the Warpgate configuration to LLB, executing the build with BuildKit, and
// loading the resulting image into Docker's image store.
func (b *BuildKitBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	if cfg.IsDockerfileBased() {
		return b.BuildDockerfile(ctx, cfg)
	}

	startTime := time.Now()
	logging.InfoContext(ctx, "Building image: %s (native LLB)", cfg.Name)

	contextDir, err := b.calculateBuildContext(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate build context: %w", err)
	}
	logging.DebugContext(ctx, "Using build context: %s", contextDir)

	b.contextDir = contextDir

	state, err := b.convertToLLB(cfg)
	if err != nil {
		return nil, fmt.Errorf("LLB conversion failed: %w", err)
	}

	def, err := state.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("LLB marshal failed: %w", err)
	}

	logging.DebugContext(ctx, "LLB definition: %d bytes", len(def.Def))

	imageName := fmt.Sprintf("%s:%s", cfg.Name, cfg.Version)
	if cfg.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", cfg.Registry, imageName)
	}

	imageTarPath := filepath.Join(os.TempDir(), fmt.Sprintf("warpgate-image-%d.tar", time.Now().Unix()))
	defer func() {
		if err := os.Remove(imageTarPath); err != nil {
			logging.WarnContext(ctx, "Failed to remove temporary image tar: %v", err)
		}
	}()

	exportAttrs := buildExportAttributes(imageName, cfg.Labels)

	localDirs := map[string]string{
		"context": contextDir,
	}

	solveOpt := client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type:   client.ExporterDocker,
				Output: fixedWriteCloser(imageTarPath),
				Attrs:  exportAttrs,
			},
		},
		LocalDirs: localDirs,
		Session:   createAuthProvider(),
	}

	b.configureCacheOptions(&solveOpt, cfg)

	ch := make(chan *client.SolveStatus)
	done := make(chan struct{})

	go b.displayProgress(ctx, ch, done)

	_, err = b.client.Solve(ctx, def, solveOpt, ch)
	<-done

	if err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	if err := b.loadAndTagImage(ctx, imageTarPath, imageName); err != nil {
		return nil, err
	}

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

// fixedWriteCloser returns a factory function for creating file-based WriteClosers.
// This is used by BuildKit's solve options for exporting build outputs to files.
//
// The returned function ignores the metadata map parameter and creates a new file
// at the specified path. The caller is responsible for closing the returned
// WriteCloser to ensure the file is properly flushed and closed.
//
// Resource Management: The created file handle must be closed by the caller.
// BuildKit typically handles this automatically when the solve operation completes.
func fixedWriteCloser(filepath string) func(map[string]string) (io.WriteCloser, error) {
	return func(m map[string]string) (io.WriteCloser, error) {
		f, err := os.Create(filepath)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
}

// displayProgress consumes BuildKit solve status updates and displays build progress.
// This function runs in a goroutine during the build process.
//
// Concurrency: This function should be run as a goroutine. It reads from the
// status channel until it's closed, then signals completion via the done channel.
// The done channel MUST be closed by this function to signal build completion.
//
// Parameters:
//   - ctx: Context for logging operations
//   - ch: Channel of solve status updates from BuildKit (closed when build completes)
//   - done: Channel to signal when progress display is complete (closed by this function)
func (b *BuildKitBuilder) displayProgress(ctx context.Context, ch <-chan *client.SolveStatus, done chan<- struct{}) {
	defer close(done)

	for status := range ch {
		// codespell:ignore vertexes
		for _, vertex := range status.Vertexes {
			if vertex.Name != "" {
				logging.DebugContext(ctx, "[%s] %s", vertex.Digest.String()[:12], vertex.Name)
			}
		}
		for _, log := range status.Logs {
			logging.PrintContext(ctx, string(log.Data))
		}
	}
}

// extractRegistryFromImageRef extracts the registry hostname from an image reference.
func extractRegistryFromImageRef(imageRef string) string {
	parts := strings.SplitN(imageRef, "@", 2)
	imageWithoutDigest := parts[0]

	slashParts := strings.Split(imageWithoutDigest, "/")

	if len(slashParts) == 1 {
		return "docker.io"
	}

	registryCandidate := slashParts[0]

	if strings.Contains(registryCandidate, ".") || strings.Contains(registryCandidate, ":") || registryCandidate == "localhost" {
		return registryCandidate
	}

	return "docker.io"
}

// Push pushes the built image to a container registry using the Docker SDK.
func (b *BuildKitBuilder) Push(ctx context.Context, imageRef, registry string) (string, error) {
	logging.InfoContext(ctx, "Pushing image: %s", imageRef)

	fullImageRef := imageRef
	if registry != "" && !strings.Contains(imageRef, "/") {
		fullImageRef = fmt.Sprintf("%s/%s", registry, imageRef)
		if err := b.Tag(ctx, imageRef, fullImageRef); err != nil {
			return "", fmt.Errorf("failed to tag image with registry: %w", err)
		}
	}

	registryHostname := extractRegistryFromImageRef(fullImageRef)
	logging.DebugContext(ctx, "Using registry hostname for auth: %s", registryHostname)

	registryAuth, err := ToDockerSDKAuth(ctx, registryHostname)
	if err != nil {
		logging.WarnContext(ctx, "Failed to get registry credentials: %v (attempting push anyway)", err)
		registryAuth = ""
	}

	pushOpts := dockerimage.PushOptions{
		RegistryAuth: registryAuth,
	}

	logging.InfoContext(ctx, "Pushing to: %s", fullImageRef)
	resp, err := b.dockerClient.ImagePush(ctx, fullImageRef, pushOpts)
	if err != nil {
		return "", fmt.Errorf("failed to push %s: %w", fullImageRef, err)
	}
	defer func() {
		if err := resp.Close(); err != nil {
			logging.WarnContext(ctx, "Failed to close response: %v", err)
		}
	}()

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp); err != nil {
		logging.WarnContext(ctx, "Failed to read push response: %v", err)
	} else {
		output := buf.String()
		logging.DebugContext(ctx, "Push response: %s", output)

		if strings.Contains(output, "\"error\"") {
			return "", fmt.Errorf("push failed: %s", output)
		}
	}

	inspect, err := b.dockerClient.ImageInspect(ctx, fullImageRef)
	if err != nil {
		logging.WarnContext(ctx, "Failed to inspect image after push: %v", err)
		return "", nil
	}

	if len(inspect.RepoDigests) > 0 {
		repoDigest := inspect.RepoDigests[0]
		digestParts := strings.Split(repoDigest, "@")
		if len(digestParts) == 2 {
			digest := digestParts[1]
			logging.InfoContext(ctx, "Image digest: %s", digest)
			return digest, nil
		}
	}

	logging.WarnContext(ctx, "No digest found for %s", fullImageRef)
	return "", nil
}

// Tag creates an additional tag for an existing image using the Docker SDK.
func (b *BuildKitBuilder) Tag(ctx context.Context, imageRef, newTag string) error {
	if err := b.dockerClient.ImageTag(ctx, imageRef, newTag); err != nil {
		return fmt.Errorf("docker tag failed: %w", err)
	}

	logging.DebugContext(ctx, "Tagged %s as %s", imageRef, newTag)
	return nil
}

// Remove deletes an image from the local Docker image store using the Docker SDK.
func (b *BuildKitBuilder) Remove(ctx context.Context, imageRef string) error {
	removeOpts := dockerimage.RemoveOptions{
		Force:         false,
		PruneChildren: true,
	}

	_, err := b.dockerClient.ImageRemove(ctx, imageRef, removeOpts)
	if err != nil {
		return fmt.Errorf("docker rmi failed: %w", err)
	}

	logging.DebugContext(ctx, "Removed image: %s", imageRef)
	return nil
}

// CreateAndPushManifest creates and pushes a multi-architecture manifest list to a registry.
func (b *BuildKitBuilder) CreateAndPushManifest(ctx context.Context, manifestName string, entries []manifests.ManifestEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no manifest entries provided")
	}

	logging.InfoContext(ctx, "Creating multi-arch manifest: %s", manifestName)
	logging.InfoContext(ctx, "Manifest %s will include %d architectures:", manifestName, len(entries))

	manifestDescriptors := make([]specs.Descriptor, 0, len(entries))

	for _, entry := range entries {
		logging.InfoContext(ctx, "  - %s/%s (digest: %s)", entry.OS, entry.Architecture, entry.Digest.String())

		if entry.Digest.String() == "" {
			return fmt.Errorf("no digest found for %s/%s", entry.OS, entry.Architecture)
		}

		platform := specs.Platform{
			OS:           entry.OS,
			Architecture: entry.Architecture,
		}
		if entry.Variant != "" {
			platform.Variant = entry.Variant
		}

		inspect, err := b.dockerClient.ImageInspect(ctx, entry.ImageRef)
		var imageSize int64
		if err != nil {
			logging.WarnContext(ctx, "Failed to inspect image %s for size (using 0): %v", entry.ImageRef, err)
			imageSize = 0
		} else {
			imageSize = inspect.Size
		}

		manifestDescriptors = append(manifestDescriptors, specs.Descriptor{
			MediaType: "application/vnd.docker.distribution.manifest.v2+json",
			Digest:    entry.Digest,
			Size:      imageSize,
			Platform:  &platform,
		})
	}

	manifestList := specs.Index{
		Versioned: specsgo.Versioned{
			SchemaVersion: 2,
		},
		MediaType: "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests: manifestDescriptors,
	}

	manifestJSON, err := json.Marshal(manifestList)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest list: %w", err)
	}

	logging.DebugContext(ctx, "Manifest list JSON: %s", string(manifestJSON))
	logging.InfoContext(ctx, "Successfully created multi-arch manifest: %s", manifestName)

	ref, err := name.ParseReference(manifestName)
	if err != nil {
		return fmt.Errorf("failed to parse manifest name: %w", err)
	}

	adds := make([]mutate.IndexAddendum, 0, len(entries))
	for _, entry := range entries {
		imgRef, err := name.ParseReference(entry.ImageRef)
		if err != nil {
			logging.WarnContext(ctx, "Failed to parse image reference %s: %v (skipping)", entry.ImageRef, err)
			continue
		}

		desc, err := remote.Get(imgRef, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
		if err != nil {
			logging.WarnContext(ctx, "Failed to get remote descriptor for %s: %v (skipping)", entry.ImageRef, err)
			continue
		}

		img, err := desc.Image()
		if err != nil {
			logging.WarnContext(ctx, "Failed to convert descriptor to image for %s: %v (skipping)", entry.ImageRef, err)
			continue
		}

		platform := v1.Platform{
			OS:           entry.OS,
			Architecture: entry.Architecture,
		}
		if entry.Variant != "" {
			platform.Variant = entry.Variant
		}

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

	mediaType := types.MediaType(manifestList.MediaType)
	idx := mutate.IndexMediaType(mutate.AppendManifests(
		mutate.IndexMediaType(empty.Index, mediaType),
		adds...,
	), mediaType)

	logging.InfoContext(ctx, "Pushing manifest list to registry: %s", manifestName)
	if err := remote.WriteIndex(ref, idx,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	); err != nil {
		return fmt.Errorf("failed to push manifest list: %w", err)
	}

	logging.InfoContext(ctx, "Successfully pushed multi-arch manifest: %s", manifestName)
	return nil
}

// InspectManifest retrieves information about a manifest list from Docker.
func (b *BuildKitBuilder) InspectManifest(ctx context.Context, manifestName string) ([]manifests.ManifestEntry, error) {
	logging.DebugContext(ctx, "Inspecting manifest: %s", manifestName)

	inspect, err := b.dockerClient.DistributionInspect(ctx, manifestName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to inspect manifest: %w", err)
	}

	logging.DebugContext(ctx, "Manifest descriptor: %+v", inspect.Descriptor)

	var entries []manifests.ManifestEntry

	if inspect.Descriptor.Platform != nil {
		entries = append(entries, manifests.ManifestEntry{
			ImageRef:     manifestName,
			OS:           inspect.Descriptor.Platform.OS,
			Architecture: inspect.Descriptor.Platform.Architecture,
			Variant:      inspect.Descriptor.Platform.Variant,
			Digest:       digest.Digest(inspect.Descriptor.Digest.String()),
		})
	}

	logging.DebugContext(ctx, "Found %d manifest entries", len(entries))
	return entries, nil
}

// BuildDockerfile builds a container image using a Dockerfile with BuildKit's native client.
func (b *BuildKitBuilder) BuildDockerfile(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	startTime := time.Now()
	logging.InfoContext(ctx, "Building image from Dockerfile: %s (native BuildKit)", cfg.Name)

	imageName := fmt.Sprintf("%s:%s", cfg.Name, cfg.Version)
	if cfg.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", cfg.Registry, imageName)
	}

	dockerfileCfg := cfg.Dockerfile
	dockerfilePath := dockerfileCfg.GetDockerfilePath()
	buildContext := dockerfileCfg.GetBuildContext()

	logging.DebugContext(ctx, "Dockerfile: %s, Context: %s", dockerfilePath, buildContext)

	relDockerfilePath, err := filepath.Rel(buildContext, dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate relative Dockerfile path: %w", err)
	}

	frontendAttrs := map[string]string{
		"filename": relDockerfilePath,
	}

	if dockerfileCfg.Target != "" {
		frontendAttrs["target"] = dockerfileCfg.Target
	}

	for key, value := range dockerfileCfg.Args {
		frontendAttrs[fmt.Sprintf("build-arg:%s", key)] = value
	}

	for key, value := range cfg.BuildArgs {
		frontendAttrs[fmt.Sprintf("build-arg:%s", key)] = value
	}

	if len(cfg.Architectures) == 1 {
		frontendAttrs["platform"] = fmt.Sprintf("linux/%s", cfg.Architectures[0])
	}

	if cfg.NoCache {
		frontendAttrs["no-cache"] = ""
	}

	imageTarPath := filepath.Join(os.TempDir(), fmt.Sprintf("warpgate-image-%d.tar", time.Now().Unix()))
	defer func() {
		if err := os.Remove(imageTarPath); err != nil {
			logging.WarnContext(ctx, "Failed to remove temporary image tar: %v", err)
		}
	}()

	exportAttrs := buildExportAttributes(imageName, cfg.Labels)

	solveOpt := client.SolveOpt{
		Frontend:      "dockerfile.v0",
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
		Session: createAuthProvider(),
	}

	b.configureCacheOptions(&solveOpt, cfg)

	ch := make(chan *client.SolveStatus)
	done := make(chan struct{})

	go b.displayProgress(ctx, ch, done)

	_, err = b.client.Solve(ctx, nil, solveOpt, ch)
	<-done

	if err != nil {
		return nil, fmt.Errorf("dockerfile build failed: %w", err)
	}

	if err := b.loadAndTagImage(ctx, imageTarPath, imageName); err != nil {
		return nil, err
	}

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
		return stderrors.Join(errs...)
	}

	return nil
}
