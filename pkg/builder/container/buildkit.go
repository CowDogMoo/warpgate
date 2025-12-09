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

	// Find running builder
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "running") {
			// Parse builder name (first field)
			fields := strings.Fields(line)
			if len(fields) > 0 {
				builderName := strings.TrimSuffix(fields[0], "*")
				// Container name format: buildx_buildkit_<builder>0
				containerName := fmt.Sprintf("buildx_buildkit_%s0", builderName)
				return builderName, containerName, nil
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
	for _, change := range postChanges {
		parts := strings.Fields(change)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "ENV":
			// ENV KEY=value or ENV KEY value
			if strings.Contains(parts[1], "=") {
				kv := strings.SplitN(parts[1], "=", 2)
				state = state.AddEnv(kv[0], kv[1])
			} else if len(parts) >= 3 {
				state = state.AddEnv(parts[1], parts[2])
			}
		case "WORKDIR":
			state = state.Dir(parts[1])
		case "USER":
			state = state.User(parts[1])
			// ENTRYPOINT and CMD handled via image metadata
		}
	}
	return state
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

	// Step 5: Build solve options
	solveOpt := client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type: client.ExporterImage,
				Attrs: map[string]string{
					"name": imageName,
					"push": "false",
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

	duration := time.Since(startTime)

	return &builder.BuildResult{
		ImageRef: imageName,
		Platform: fmt.Sprintf("linux/%s", cfg.Architectures[0]),
		Duration: duration.String(),
		Notes:    []string{"Built with native BuildKit LLB"},
	}, nil
}

// displayProgress displays build progress
func (b *BuildKitBuilder) displayProgress(ch <-chan *client.SolveStatus, done chan<- struct{}) {
	defer close(done)

	for status := range ch {
		for _, vertex := range status.Vertices {
			if vertex.Name != "" {
				logging.Info("[%s] %s", vertex.Digest.String()[:12], vertex.Name)
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
