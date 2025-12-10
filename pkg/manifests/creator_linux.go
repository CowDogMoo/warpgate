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

package manifests

import (
	"context"
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder/buildah"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"go.podman.io/storage/pkg/unshare"
)

// CreateAndPushManifest creates and pushes a multi-arch manifest from digest files
func CreateAndPushManifest(ctx context.Context, digestFiles []DigestFile, opts CreationOptions) error {
	logging.Info("Creating multi-arch manifest with %d architecture(s)", len(digestFiles))

	// Ensure we're in the right namespace for container operations
	unshare.MaybeReexecUsingUserNamespace(false)

	// Get container builder configuration
	builderCfg := buildah.GetDefaultConfig()

	// Create a container builder to access manifest manager
	bldr, err := buildah.NewBuildahBuilder(builderCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container builder: %w", err)
	}
	defer func() {
		if err := bldr.Close(); err != nil {
			logging.Error("Failed to close builder: %v", err)
		}
	}()

	// Get manifest manager
	manifestMgr := bldr.GetManifestManager()

	// Create manifest entries
	entries := make([]ManifestEntry, 0, len(digestFiles))
	for _, df := range digestFiles {
		// Build the full image reference for this architecture
		imageRef := BuildImageReference(ReferenceOptions{
			Registry:     opts.Registry,
			Namespace:    opts.Namespace,
			ImageName:    df.ImageName,
			Architecture: df.Architecture,
			Tag:          opts.Tag,
		})

		// Parse platform info
		os := "linux"
		arch := df.Architecture
		variant := ""

		// Handle arm/v7, arm/v6, etc.
		if strings.Contains(arch, "/") {
			parts := strings.Split(arch, "/")
			if len(parts) >= 2 {
				arch = parts[0]
				variant = parts[1]
			}
		}

		platform := fmt.Sprintf("%s/%s", os, df.Architecture)

		entry := ManifestEntry{
			ImageRef:     imageRef,
			Digest:       df.Digest,
			Platform:     platform,
			Architecture: arch,
			OS:           os,
			Variant:      variant,
		}

		entries = append(entries, entry)
		logging.Debug("Added manifest entry: %s (%s)", entry.Platform, entry.Digest.String())
	}

	// Create the manifest
	manifestName := fmt.Sprintf("%s:%s", opts.ImageName, opts.Tag)
	manifestList, err := manifestMgr.CreateManifest(ctx, manifestName, entries)
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// Set manifest-level annotations (passing nil for manifest-level)
	if len(opts.Annotations) > 0 {
		logging.Debug("Adding %d annotation(s) to manifest", len(opts.Annotations))
		if err := manifestList.SetAnnotations(nil, opts.Annotations); err != nil {
			return fmt.Errorf("failed to set manifest annotations: %w", err)
		}
	}

	// Labels are typically added as annotations with specific keys
	if len(opts.Labels) > 0 {
		logging.Debug("Adding %d label(s) to manifest", len(opts.Labels))
		labelAnnotations := make(map[string]string)
		for k, v := range opts.Labels {
			labelAnnotations["org.opencontainers.image."+k] = v
		}
		if err := manifestList.SetAnnotations(nil, labelAnnotations); err != nil {
			return fmt.Errorf("failed to set manifest labels: %w", err)
		}
	}

	// Push the manifest to the registry
	destination := BuildManifestReference(opts.Registry, opts.Namespace, opts.ImageName, opts.Tag)
	if err := manifestMgr.PushManifest(ctx, manifestList, destination); err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	return nil
}
