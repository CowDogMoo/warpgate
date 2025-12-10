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
	"os"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/builder/buildah"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// autoSelectBuilderFunc automatically selects the best builder for Linux
func autoSelectBuilderFunc(ctx context.Context) (builder.ContainerBuilder, error) {
	logging.Info("Auto-selecting builder for Linux platform")

	// Check if we're in a rootless VFS environment - prefer BuildKit in this case
	// Buildah has known issues with VFS driver in rootless mode due to chown errors
	// See: https://github.com/containers/buildah/issues/5744
	cfg, err := globalconfig.Load()
	if err == nil && cfg.Storage.Driver == "vfs" && os.Geteuid() != 0 {
		logging.Info("Detected rootless VFS environment, preferring BuildKit over Buildah")
		bldr, err := newBuildKitBuilderFunc(ctx)
		if err == nil {
			logging.Info("Auto-selected: BuildKit (rootless VFS)")
			return bldr, nil
		}
		logging.Warn("BuildKit not available (%v), trying Buildah despite rootless VFS", err)
	}

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
