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

	"github.com/cowdogmoo/warpgate/v3/errors"
	"github.com/cowdogmoo/warpgate/v3/logging"
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

			configCopy := req.Config
			configCopy.Base.Platform = req.Platform

			// Use architecture as tag to prevent local image overwrites
			configCopy.Version = req.Architecture

			result, err := builder.Build(ctx, configCopy)
			if err != nil {
				logging.Error("Failed to build %s for %s: %v", req.Config.Name, req.Architecture, err)
				return errors.Wrap(fmt.Sprintf("build %s", req.Config.Name), req.Architecture, err)
			}

			results[i] = *result
			logging.Info("Successfully built %s for %s: %s", req.Config.Name, req.Architecture, result.ImageRef)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, errors.Wrap("complete multi-arch build", "", err)
	}

	logging.Info("Successfully completed all %d architecture builds", len(requests))
	return results, nil
}

// PushMultiArch pushes all architecture-specific images to a registry in parallel.
func (bo *BuildOrchestrator) PushMultiArch(ctx context.Context, results []BuildResult, registry string, builder ContainerBuilder) error {
	logging.Info("Pushing %d architecture images to %s", len(results), registry)

	g, ctx := errgroup.WithContext(ctx)

	for i := range results {
		i := i // Capture loop index
		result := results[i]
		g.Go(func() error {
			logging.Info("Pushing %s to %s", result.ImageRef, registry)

			digest, err := builder.Push(ctx, result.ImageRef, registry)
			if err != nil {
				logging.Error("Failed to push %s: %v", result.ImageRef, err)
				return errors.Wrap("push image", result.ImageRef, err)
			}

			// Update the digest in the result if we got one from the push
			if digest != "" {
				results[i].Digest = digest
				logging.Info("Successfully pushed %s with digest %s", result.ImageRef, digest)
			} else {
				logging.Info("Successfully pushed %s", result.ImageRef)
			}

			return nil
		})
	}

	// Wait for all pushes to complete
	if err := g.Wait(); err != nil {
		return errors.Wrap("complete multi-arch push", "", err)
	}

	return nil
}
