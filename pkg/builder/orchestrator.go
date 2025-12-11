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

package builder

import (
	"context"
	"fmt"

	"github.com/cowdogmoo/warpgate/pkg/logging"
	"golang.org/x/sync/errgroup"
)

// DefaultMaxConcurrency is the default number of parallel builds.
const DefaultMaxConcurrency = 2

// BuildOrchestrator coordinates multi-architecture builds with controlled concurrency.
type BuildOrchestrator struct {
	maxConcurrency int
}

// NewBuildOrchestrator creates a new build orchestrator with the specified concurrency limit.
func NewBuildOrchestrator(maxConcurrency int) *BuildOrchestrator {
	if maxConcurrency <= 0 {
		maxConcurrency = DefaultMaxConcurrency
	}

	return &BuildOrchestrator{
		maxConcurrency: maxConcurrency,
	}
}

// BuildRequest represents a single-architecture build operation within a multi-arch build.
type BuildRequest struct {
	Config       Config
	Architecture string // Target CPU architecture (e.g., "amd64", "arm64")
	Platform     string // Full platform specification (e.g., "linux/amd64", "linux/arm64/v8")
	Tag          string // Image tag for this specific build
}

// BuildMultiArch builds images for multiple architectures in parallel with controlled concurrency.
func (bo *BuildOrchestrator) BuildMultiArch(ctx context.Context, requests []BuildRequest, builder ContainerBuilder) ([]BuildResult, error) {
	logging.Info("Starting multi-arch build for %d architectures", len(requests))

	// Use errgroup with concurrency limit
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(bo.maxConcurrency)

	results := make([]BuildResult, len(requests))

	for i, req := range requests {
		i, req := i, req // Capture loop variables
		g.Go(func() error {
			logging.Info("Building %s for %s", req.Config.Name, req.Architecture)

			// Modify config for specific architecture
			configCopy := req.Config
			configCopy.Base.Platform = req.Platform

			// Build the image
			result, err := builder.Build(ctx, configCopy)
			if err != nil {
				logging.Error("Failed to build %s for %s: %v", req.Config.Name, req.Architecture, err)
				return fmt.Errorf("build %s (%s): %w", req.Config.Name, req.Architecture, err)
			}

			// Tag with architecture-specific tag
			archTag := fmt.Sprintf("%s-%s", req.Tag, req.Architecture)
			if err := builder.Tag(ctx, result.ImageRef, archTag); err != nil {
				logging.Error("Failed to tag image: %v", err)
				return fmt.Errorf("tag %s for %s: %w", archTag, req.Architecture, err)
			}

			result.ImageRef = archTag
			results[i] = *result
			logging.Info("Successfully built %s for %s: %s", req.Config.Name, req.Architecture, archTag)
			return nil
		})
	}

	// Wait for all builds to complete
	if err := g.Wait(); err != nil {
		return results, fmt.Errorf("multi-arch build failed: %w", err)
	}

	logging.Info("Successfully completed all %d architecture builds", len(requests))
	return results, nil
}

// PushMultiArch pushes all architecture-specific images to a registry in parallel.
func (bo *BuildOrchestrator) PushMultiArch(ctx context.Context, results []BuildResult, registry string, builder ContainerBuilder) error {
	logging.Info("Pushing %d architecture images to %s", len(results), registry)

	// Use errgroup for concurrent pushes (no concurrency limit needed for push operations)
	g, ctx := errgroup.WithContext(ctx)

	for i := range results {
		i := i // Capture loop index
		result := results[i]
		g.Go(func() error {
			registryRef := fmt.Sprintf("%s/%s", registry, result.ImageRef)
			logging.Info("Pushing %s", registryRef)

			digest, err := builder.Push(ctx, result.ImageRef, registry)
			if err != nil {
				logging.Error("Failed to push %s: %v", registryRef, err)
				return fmt.Errorf("push %s: %w", registryRef, err)
			}

			// Update the digest in the result if we got one from the push
			if digest != "" {
				results[i].Digest = digest
				logging.Info("Successfully pushed %s with digest %s", registryRef, digest)
			} else {
				logging.Info("Successfully pushed %s", registryRef)
			}

			return nil
		})
	}

	// Wait for all pushes to complete
	if err := g.Wait(); err != nil {
		return fmt.Errorf("multi-arch push failed: %w", err)
	}

	return nil
}
