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

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sync/errgroup"

	"github.com/cowdogmoo/warpgate/v3/logging"
)

// VerificationOptions contains options for registry verification
type VerificationOptions struct {
	Registry      string
	Namespace     string
	Tag           string
	AuthFile      string
	MaxConcurrent int // Maximum concurrent verifications (default: 5)
}

// VerifyDigestsInRegistry verifies that digests exist in the registry.
func VerifyDigestsInRegistry(ctx context.Context, digestFiles []DigestFile, opts VerificationOptions) error {
	logging.Info("Verifying %d digest(s) exist in registry...", len(digestFiles))

	// Limit concurrency to avoid overwhelming the registry
	maxConcurrent := opts.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5 // Default
	}
	if maxConcurrent > 20 {
		logging.Warn("Limiting concurrency to 20 (requested: %d) to avoid overwhelming the registry", maxConcurrent)
		maxConcurrent = 20
	}
	logging.Debug("Using concurrency limit of %d for digest verification", maxConcurrent)

	// Use errgroup with concurrency limit for better error handling and consistency
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	// Verify each digest in parallel
	for _, df := range digestFiles {
		df := df // Capture loop variable
		g.Go(func() error {
			// Check context before starting work
			if err := ctx.Err(); err != nil {
				return err
			}
			// Build digest-based reference
			// Format: registry/namespace/image@sha256:digest
			imageRef := BuildManifestReference(opts.Registry, opts.Namespace, df.ImageName, "")
			imageRef = strings.TrimSuffix(imageRef, ":") // Remove trailing colon if no tag
			imageRef = fmt.Sprintf("%s@%s", imageRef, df.Digest.String())

			logging.Debug("Verifying %s exists in registry...", imageRef)

			// Parse reference using go-containerregistry
			ref, err := name.ParseReference(imageRef)
			if err != nil {
				return fmt.Errorf("failed to parse image reference %s: %w", imageRef, err)
			}

			// Try to get the image descriptor from registry
			// This verifies the image exists without pulling layers
			_, err = remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
			if err != nil {
				return fmt.Errorf("failed to verify digest %s in registry: %w - "+
					"Image may not have been pushed or registry may be unreachable - "+
					"Use --verify-registry=false to skip verification (not recommended)",
					df.Digest.String(), err)
			}

			logging.Debug("Verified %s exists in registry", imageRef)
			return nil
		})
	}

	// Wait for all verifications to complete
	// If any verification fails, the context is canceled and all remaining verifications stop
	if err := g.Wait(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	logging.Info("All digests verified in registry")
	return nil
}

// CheckManifestExists checks if a manifest already exists with the same content
func CheckManifestExists(ctx context.Context, digestFiles []DigestFile) (bool, error) {
	logging.Debug("Checking if manifest already exists...")

	// For idempotency checking, we would need to:
	// 1. Pull the existing manifest from the registry
	// 2. Compare its architecture list and digests with what we're about to create
	// 3. Return true if they match

	// This is a simplified implementation - in a production system,
	// you would actually fetch and compare the manifest
	// For now, we'll return false to always create the manifest
	return false, nil
}

// HealthCheckRegistry performs a health check on the registry using go-containerregistry
func HealthCheckRegistry(ctx context.Context, opts VerificationOptions) error {
	logging.Info("Performing registry health check for %s...", opts.Registry)

	// Try to access a well-known public image to verify registry connectivity
	testRef := opts.Registry + "/library/hello-world:latest"
	if opts.Registry == "ghcr.io" {
		// GitHub Container Registry doesn't have library namespace
		testRef = opts.Registry + "/hello-world/hello-world:latest"
	}

	logging.Debug("Testing registry connectivity with %s", testRef)

	ref, err := name.ParseReference(testRef)
	if err != nil {
		return fmt.Errorf("registry health check failed - unable to parse test reference: %w", err)
	}

	// Try to get the image descriptor (this will test auth and connectivity)
	_, err = remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		// Don't fail on health check, just warn
		logging.Warn("Registry health check warning: %v", err)
		logging.Info("Registry may require authentication or test image not available")
		logging.Info("Proceeding with manifest creation...")
		return nil
	}

	logging.Info("Registry health check passed")
	return nil
}
