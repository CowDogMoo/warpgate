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
	}, nil
}

// detectBuildxBuilder detects the active buildx builder
func detectBuildxBuilder(ctx context.Context) (string, string, error) {
	// Parse `docker buildx ls` output
	cmd := exec.CommandContext(ctx, "docker", "buildx", "ls")
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
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

// convertToLLB converts a Warpgate config to BuildKit LLB
func (b *BuildKitBuilder) convertToLLB(cfg builder.Config) (llb.State, error) {
	// Start with base image
	platform := specs.Platform{
		OS:           "linux",
		Architecture: cfg.Architectures[0], // TODO: handle multiple
	}

	state := llb.Image(cfg.Base.Image, llb.Platform(platform))

	// Apply base environment variables
	for key, value := range cfg.Base.Env {
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

	// Execute as shell command
	state = state.Run(
		llb.Shlex(fmt.Sprintf("sh -c '%s'", combinedCmd)),
	).Root()

	return state, nil
}

// applyFileProvisioner applies file copy operations to LLB state
func (b *BuildKitBuilder) applyFileProvisioner(state llb.State, prov builder.Provisioner) (llb.State, error) {
	if prov.Source == "" || prov.Destination == "" {
		return state, nil
	}

	sourcePath := os.ExpandEnv(prov.Source)

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

	playbookPath := os.ExpandEnv(prov.PlaybookPath)

	// Copy playbook
	state = state.File(
		llb.Copy(
			llb.Local("context"),
			playbookPath,
			"/tmp/playbook.yml",
		),
	)

	// Check for collection
	collectionRoot := detectCollectionRoot(playbookPath)
	if collectionRoot != "" {
		// Copy entire collection
		state = state.File(
			llb.Copy(
				llb.Local("context"),
				collectionRoot,
				"/tmp/ansible-collection",
			),
		)
		// Install collection
		state = state.Run(
			llb.Shlex("ansible-galaxy collection install /tmp/ansible-collection/ -p /usr/share/ansible/collections"),
		).Root()
	}

	// Copy galaxy requirements if specified
	if prov.GalaxyFile != "" {
		galaxyPath := os.ExpandEnv(prov.GalaxyFile)
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

	// Run playbook
	state = state.Run(llb.Shlex(cmd)).Root()

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

// Build creates a container image using BuildKit LLB
func (b *BuildKitBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	startTime := time.Now()
	logging.Info("Building image: %s (native LLB)", cfg.Name)

	// Step 1: Convert to LLB
	state, err := b.convertToLLB(cfg)
	if err != nil {
		return nil, fmt.Errorf("LLB conversion failed: %w", err)
	}

	// Step 2: Marshal LLB definition
	def, err := state.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("LLB marshal failed: %w", err)
	}

	logging.Debug("LLB definition: %d bytes", len(def.Def))

	// Step 3: Prepare image name
	imageName := fmt.Sprintf("%s:%s", cfg.Name, cfg.Version)
	if cfg.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", cfg.Registry, imageName)
	}

	// Step 4: Determine context directory (use current directory)
	contextDir := "."

	// Step 5: Export to OCI tar file and load into Docker
	ociTarPath := filepath.Join(os.TempDir(), fmt.Sprintf("warpgate-image-%d.tar", time.Now().Unix()))
	defer func() {
		if err := os.Remove(ociTarPath); err != nil {
			logging.Warn("Failed to remove temporary OCI tar: %v", err)
		}
	}()

	solveOpt := client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type:   client.ExporterOCI,
				Output: fixedWriteCloser(ociTarPath),
				Attrs: map[string]string{
					"name": imageName,
				},
			},
		},
		LocalDirs: map[string]string{
			"context": contextDir,
		},
	}

	// Step 6: Execute with progress streaming
	ch := make(chan *client.SolveStatus)
	done := make(chan struct{})

	go b.displayProgress(ch, done)

	_, err = b.client.Solve(ctx, def, solveOpt, ch)
	<-done

	if err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	// Step 7: Load OCI tar into Docker
	logging.Info("Loading image into Docker...")
	loadCmd := exec.CommandContext(ctx, "docker", "load", "-i", ociTarPath)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	if err := loadCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to load image into Docker: %w", err)
	}

	// Step 8: Tag the image with proper name
	logging.Info("Tagging image as %s", imageName)
	tagCmd := exec.CommandContext(ctx, "docker", "tag", imageName, imageName)
	if err := tagCmd.Run(); err != nil {
		logging.Warn("Failed to tag image (may already be tagged): %v", err)
	}

	duration := time.Since(startTime)

	return &builder.BuildResult{
		ImageRef: imageName,
		Platform: fmt.Sprintf("linux/%s", cfg.Architectures[0]),
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

// Close cleans up any resources
func (b *BuildKitBuilder) Close() error {
	if b.client != nil {
		return b.client.Close()
	}
	return nil
}
