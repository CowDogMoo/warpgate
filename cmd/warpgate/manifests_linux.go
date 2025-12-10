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
	"github.com/cowdogmoo/warpgate/pkg/builder/buildkit"
	"github.com/cowdogmoo/warpgate/pkg/manifests"
)

// manifestBuilder is an interface for builders that support manifest operations
type manifestBuilder interface {
	builder.ContainerBuilder
	CreateAndPushManifest(ctx context.Context, manifestName string, entries []manifests.ManifestEntry) error
}

// createBuilderForManifests creates a BuildKit builder for manifest operations on Linux
func createBuilderForManifests(ctx context.Context) (manifestBuilder, error) {
	// Create the BuildKit builder
	return buildkit.NewBuildKitBuilder(ctx)
}

// createManifestWithBuilder creates and pushes a manifest using the builder
func createManifestWithBuilder(ctx context.Context, bldr manifestBuilder, manifestName string, entries []manifests.ManifestEntry) error {
	// Use the builder's CreateAndPushManifest method
	if err := bldr.CreateAndPushManifest(ctx, manifestName, entries); err != nil {
		return fmt.Errorf("failed to create and push manifest: %w", err)
	}
	return nil
}
