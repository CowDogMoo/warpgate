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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"

	// CRITICAL: This enables docker-container:// protocol
	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// BuildKitBuilder implements container image building using Docker BuildKit
type BuildKitBuilder struct {
	client        *client.Client
	builderName   string
	containerName string
	contextDir    string   // Build context directory (calculated intelligently)
	cacheFrom     []string // External cache sources
	cacheTo       []string // External cache destinations
}

// NewBuildKitBuilder creates a new BuildKit builder instance
func NewBuildKitBuilder(ctx context.Context) (*BuildKitBuilder, error) {
	// Detect active buildx builder
	builderName, containerName, err := detectBuildxBuilder(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active buildx builder found: %w", err)
	}

	// Connect using docker-container:// protocol
	addr := fmt.Sprintf("docker-container://%s", containerName)
	c, err := client.New(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to BuildKit: %w", err)
	}

	// Verify connection
	info, err := c.Info(ctx)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("BuildKit connection failed: %w", err)
	}

	logging.Info("BuildKit client connected: %s (version %s)",
		containerName, info.BuildkitVersion.Version)

	return &BuildKitBuilder{
		client:        c,
		builderName:   builderName,
		containerName: containerName,
		cacheFrom:     []string{},
		cacheTo:       []string{},
	}, nil
}

// SetCacheOptions sets external cache sources and destinations
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

// detectBuildxBuilder detects the active buildx builder
func detectBuildxBuilder(ctx context.Context) (string, string, error) {
	// Check if docker command exists
	if _, err := exec.LookPath("docker"); err != nil {
		return "", "", fmt.Errorf("docker command not found: %w\n\nPlease install Docker Desktop or Docker Engine", err)
	}

	// Parse `docker buildx ls` output
	cmd := exec.CommandContext(ctx, "docker", "buildx", "ls")
	output, err := cmd.Output()
	if err != nil {
		// Check if it's an exit error to provide better context
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "Cannot connect") || strings.Contains(stderr, "connection refused") {
				return "", "", fmt.Errorf("cannot connect to Docker daemon: %w\n\nPlease ensure Docker is running", err)
			}
			return "", "", fmt.Errorf("docker buildx command failed: %w\nOutput: %s", err, stderr)
		}
		return "", "", fmt.Errorf("failed to execute docker buildx ls: %w", err)
	}

	// Find active builder (marked with *) and extract the running node's container name
	lines := strings.Split(string(output), "\n")
	var activeBuilder string

	for i, line := range lines {
		// Skip empty lines
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		// Look for the active builder (has * suffix)
		if strings.HasSuffix(fields[0], "*") {
			activeBuilder = strings.TrimSuffix(fields[0], "*")

			// Now look at the next line(s) for running nodes
			for j := i + 1; j < len(lines); j++ {
				nodeLine := lines[j]
				// Node lines start with whitespace and \_
				if !strings.HasPrefix(strings.TrimSpace(nodeLine), "\\_") &&
					!strings.HasPrefix(strings.TrimSpace(nodeLine), "|") {
					break // End of this builder's nodes
				}

				if strings.Contains(nodeLine, "running") {
					// Extract node name (after the \_ prefix)
					nodeFields := strings.Fields(nodeLine)
					if len(nodeFields) >= 2 {
						// Container name format: buildx_buildkit_<builder>0
						containerName := fmt.Sprintf("buildx_buildkit_%s0", activeBuilder)
						return activeBuilder, containerName, nil
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf("no running buildx builder found")
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

// loadAndTagImage loads the built OCI tar into Docker and tags it
func loadAndTagImage(ctx context.Context, ociTarPath, imageName string) error {
	// Load OCI tar into Docker
	logging.Info("Loading image into Docker...")
	loadCmd := exec.CommandContext(ctx, "docker", "load", "-i", ociTarPath)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("failed to load image into Docker: %w", err)
	}

	// Tag the image with proper name
	logging.Info("Tagging image as %s", imageName)
	tagCmd := exec.CommandContext(ctx, "docker", "tag", imageName, imageName)
	if err := tagCmd.Run(); err != nil {
		logging.Warn("Failed to tag image (may already be tagged): %v", err)
	}

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

// Build creates a container image using BuildKit LLB
func (b *BuildKitBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
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

	// Step 5: Export to OCI tar file and load into Docker
	ociTarPath := filepath.Join(os.TempDir(), fmt.Sprintf("warpgate-image-%d.tar", time.Now().Unix()))
	defer func() {
		if err := os.Remove(ociTarPath); err != nil {
			logging.Warn("Failed to remove temporary OCI tar: %v", err)
		}
	}()

	// Build export attributes
	exportAttrs := buildExportAttributes(imageName, cfg.Labels)

	solveOpt := client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type:   client.ExporterOCI,
				Output: fixedWriteCloser(ociTarPath),
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
	if err := loadAndTagImage(ctx, ociTarPath, imageName); err != nil {
		return nil, err
	}

	duration := time.Since(startTime)

	return &builder.BuildResult{
		ImageRef: imageName,
		Platform: getPlatformString(cfg),
		Duration: duration.String(),
		Notes:    []string{"Built with native BuildKit LLB", "Image loaded to Docker"},
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

// CreateAndPushManifest creates and pushes a multi-arch manifest using docker buildx imagetools
func (b *BuildKitBuilder) CreateAndPushManifest(ctx context.Context, manifestName string, entries []builder.ManifestEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no manifest entries provided")
	}

	logging.Info("Creating multi-arch manifest: %s", manifestName)
	logging.Info("Manifest %s will include %d architectures:", manifestName, len(entries))

	// Build the list of image references for the manifest
	imageRefs := make([]string, 0, len(entries))
	for _, entry := range entries {
		logging.Info("  - %s/%s (ref: %s)", entry.OS, entry.Architecture, entry.ImageRef)
		imageRefs = append(imageRefs, entry.ImageRef)
	}

	// Build the docker buildx imagetools create command
	// Format: docker buildx imagetools create --tag <manifest> <image1> <image2> ...
	args := []string{"buildx", "imagetools", "create", "--tag", manifestName}
	args = append(args, imageRefs...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logging.Info("Creating manifest with: docker %s", strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	logging.Info("Successfully created and pushed multi-arch manifest: %s", manifestName)
	return nil
}

// InspectManifest inspects a manifest using docker buildx imagetools inspect
func (b *BuildKitBuilder) InspectManifest(ctx context.Context, manifestName string) ([]builder.ManifestEntry, error) {
	logging.Debug("Inspecting manifest: %s", manifestName)

	cmd := exec.CommandContext(ctx, "docker", "buildx", "imagetools", "inspect", manifestName, "--raw")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect manifest: %w", err)
	}

	logging.Debug("Manifest inspection output: %s", string(output))
	// Note: This is a simplified version. Full implementation would parse the JSON output.
	// For now, we just verify the manifest exists.
	return nil, nil
}

// Close cleans up any resources
func (b *BuildKitBuilder) Close() error {
	if b.client != nil {
		return b.client.Close()
	}
	return nil
}
