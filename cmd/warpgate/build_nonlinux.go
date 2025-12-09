//go:build !linux

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
	"runtime"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// autoSelectBuilderFunc automatically selects the best builder for non-Linux platforms
func autoSelectBuilderFunc(ctx context.Context) (builder.ContainerBuilder, error) {
	logging.Info("Auto-selecting builder for %s platform", runtime.GOOS)
	logging.Info("Auto-selected: BuildKit (only option for non-Linux)")
	return newBuildKitBuilderFunc(ctx)
}

// newBuildahBuilderFunc returns an error on non-Linux platforms
func newBuildahBuilderFunc(_ context.Context) (builder.ContainerBuilder, error) {
	return nil, fmt.Errorf("buildah builder is only supported on Linux (current platform: %s)", runtime.GOOS)
}

// createAndPushManifestPlatformSpecific handles platform-specific manifest creation (not supported on non-Linux)
func createAndPushManifestPlatformSpecific(ctx context.Context, manifestName, destination string, entries []builder.ManifestEntry, bldr builder.ContainerBuilder) error {
	logging.WarnContext(ctx, "Multi-arch manifest creation with Buildah is only supported on Linux - skipping")
	return nil
}
