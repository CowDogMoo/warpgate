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
	"os"
	"sync"

	"github.com/cowdogmoo/warpgate/pkg/logging"
	"go.podman.io/image/v5/docker"
	imagetypes "go.podman.io/image/v5/types"
)

// VerificationOptions contains options for registry verification
type VerificationOptions struct {
	Registry  string
	Namespace string
	Tag       string
	AuthFile  string
}

// VerifyDigestsInRegistry verifies that digests exist in the registry
// Performs verification in parallel for improved performance
func VerifyDigestsInRegistry(ctx context.Context, digestFiles []DigestFile, opts VerificationOptions) error {
	logging.Info("Verifying %d digest(s) exist in registry...", len(digestFiles))

	// Create system context for registry operations
	systemContext, err := createSystemContext(opts.AuthFile)
	if err != nil {
		return fmt.Errorf("failed to create system context: %w", err)
	}

	// Create channels for parallel verification
	type verificationResult struct {
		digestFile DigestFile
		imageRef   string
		err        error
	}

	results := make(chan verificationResult, len(digestFiles))
	var wg sync.WaitGroup

	// Limit concurrency to avoid overwhelming the registry
	maxConcurrent := 5
	semaphore := make(chan struct{}, maxConcurrent)

	// Verify each digest in parallel
	for _, df := range digestFiles {
		wg.Add(1)
		go func(digestFile DigestFile) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			imageRef := BuildImageReference(ReferenceOptions{
				Registry:     opts.Registry,
				Namespace:    opts.Namespace,
				ImageName:    digestFile.ImageName,
				Architecture: digestFile.Architecture,
				Tag:          opts.Tag,
			})

			logging.Debug("Verifying %s exists in registry...", imageRef)

			// Try to get image manifest from registry
			ref, err := docker.ParseReference("//" + imageRef)
			if err != nil {
				results <- verificationResult{
					digestFile: digestFile,
					imageRef:   imageRef,
					err:        fmt.Errorf("failed to parse image reference %s: %w", imageRef, err),
				}
				return
			}

			// Try to get the image
			img, err := ref.NewImage(ctx, systemContext)
			if err != nil {
				results <- verificationResult{
					digestFile: digestFile,
					imageRef:   imageRef,
					err: fmt.Errorf("failed to verify digest %s in registry: %w - "+
						"Image may not have been pushed or registry may be unreachable - "+
						"Use --verify-registry=false to skip verification (not recommended)",
						digestFile.Digest.String(), err),
				}
				return
			}
			_ = img.Close()

			logging.Debug("Verified %s exists in registry", imageRef)

			results <- verificationResult{
				digestFile: digestFile,
				imageRef:   imageRef,
				err:        nil,
			}
		}(df)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and check for errors
	var verificationErrors []string
	for result := range results {
		if result.err != nil {
			verificationErrors = append(verificationErrors, result.err.Error())
		}
	}

	if len(verificationErrors) > 0 {
		return fmt.Errorf("verification failed for %d digest(s):\n%s",
			len(verificationErrors), joinErrors(verificationErrors))
	}

	logging.Info("All digests verified in registry")
	return nil
}

// joinErrors joins multiple error messages with newlines
func joinErrors(errors []string) string {
	result := ""
	for i, err := range errors {
		if i > 0 {
			result += "\n"
		}
		result += fmt.Sprintf("  %d. %s", i+1, err)
	}
	return result
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

// HealthCheckRegistry performs a health check on the registry
func HealthCheckRegistry(ctx context.Context, opts VerificationOptions) error {
	logging.Info("Performing registry health check for %s...", opts.Registry)

	systemContext, err := createSystemContext(opts.AuthFile)
	if err != nil {
		return fmt.Errorf("failed to create system context: %w", err)
	}

	// Try to access a well-known public image to verify registry connectivity
	// Using a lightweight test reference
	testRef := opts.Registry + "/library/hello-world:latest"
	if opts.Registry == "ghcr.io" {
		// GitHub Container Registry doesn't have library namespace
		testRef = opts.Registry + "/hello-world/hello-world:latest"
	}

	logging.Debug("Testing registry connectivity with %s", testRef)

	ref, err := docker.ParseReference("//" + testRef)
	if err != nil {
		return fmt.Errorf("registry health check failed - unable to parse test reference: %w", err)
	}

	// Try to get the image (this will test auth and connectivity)
	img, err := ref.NewImage(ctx, systemContext)
	if err != nil {
		// Check if it's an auth error vs connectivity error
		logging.Warn("Registry health check warning: %v", err)
		logging.Info("Registry may require authentication or test image not available")
		logging.Info("Proceeding with manifest creation...")
		return nil // Don't fail on health check, just warn
	}
	_ = img.Close()

	logging.Info("Registry health check passed")
	return nil
}

// createSystemContext creates a system context for registry operations
func createSystemContext(authFile string) (*imagetypes.SystemContext, error) {
	systemContext := &imagetypes.SystemContext{}

	// Set authentication file if specified
	if authFile != "" {
		systemContext.AuthFilePath = authFile
	}

	// Check for registry authentication environment variables
	if username := os.Getenv("REGISTRY_USERNAME"); username != "" {
		if password := os.Getenv("REGISTRY_PASSWORD"); password != "" {
			systemContext.DockerAuthConfig = &imagetypes.DockerAuthConfig{
				Username: username,
				Password: password,
			}
		}
	}

	return systemContext, nil
}
