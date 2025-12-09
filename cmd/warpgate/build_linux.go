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

package main

import (
	"context"
	"fmt"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/builder/buildah"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// autoSelectBuilderFunc automatically selects the best builder for Linux
func autoSelectBuilderFunc(ctx context.Context) (builder.ContainerBuilder, error) {
	logging.Info("Auto-selecting builder for Linux platform")

	// Try Buildah first (native Linux solution)
	bldr, err := newBuildahBuilderFunc(ctx)
	if err == nil {
		logging.Info("Auto-selected: Buildah")
		return bldr, nil
	}

	logging.Warn("Buildah not available (%v), falling back to BuildKit", err)

	// Fall back to BuildKit
	bldr, err = newBuildKitBuilderFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("no available builders: buildah failed, buildkit failed: %w", err)
	}

	logging.Info("Auto-selected: BuildKit")
	return bldr, nil
}

// newBuildahBuilderFunc creates a new Buildah builder (Linux only)
func newBuildahBuilderFunc(_ context.Context) (builder.ContainerBuilder, error) {
	logging.Info("Creating Buildah builder")
	builderCfg := buildah.GetDefaultConfig()
	return buildah.NewBuildahBuilder(builderCfg)
}

// createAndPushManifestPlatformSpecific handles platform-specific manifest creation (Buildah on Linux)
func createAndPushManifestPlatformSpecific(ctx context.Context, manifestName, destination string, entries []builder.ManifestEntry, bldr builder.ContainerBuilder) error {
	// Check if this is a Buildah builder
	if bb, ok := bldr.(*buildah.BuildahBuilder); ok {
		// Buildah uses ManifestManager
		manifestMgr := bb.GetManifestManager()

		// Create the manifest
		manifestList, err := manifestMgr.CreateManifest(ctx, manifestName, entries)
		if err != nil {
			return fmt.Errorf("failed to create manifest: %w", err)
		}

		// Push the manifest to the registry
		if err := manifestMgr.PushManifest(ctx, manifestList, destination); err != nil {
			return fmt.Errorf("failed to push manifest: %w", err)
		}

		logging.InfoContext(ctx, "Successfully created and pushed multi-arch manifest to %s", destination)
		return nil
	}

	logging.WarnContext(ctx, "Multi-arch manifest creation not supported for this builder type - skipping")
	return nil
}
