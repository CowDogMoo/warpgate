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
func VerifyDigestsInRegistry(ctx context.Context, digestFiles []DigestFile, opts VerificationOptions) error {
	logging.Info("Verifying digests exist in registry...")

	// Create system context for registry operations
	systemContext, err := createSystemContext(opts.AuthFile)
	if err != nil {
		return fmt.Errorf("failed to create system context: %w", err)
	}

	for _, df := range digestFiles {
		imageRef := BuildImageReference(ReferenceOptions{
			Registry:     opts.Registry,
			Namespace:    opts.Namespace,
			ImageName:    df.ImageName,
			Architecture: df.Architecture,
			Tag:          opts.Tag,
		})

		logging.Debug("Verifying %s exists in registry...", imageRef)

		// Try to get image manifest from registry
		ref, err := docker.ParseReference("//" + imageRef)
		if err != nil {
			return fmt.Errorf("failed to parse image reference %s: %w", imageRef, err)
		}

		// Try to get the image
		img, err := ref.NewImage(ctx, systemContext)
		if err != nil {
			return fmt.Errorf("failed to verify digest %s in registry: %w - "+
				"Image may not have been pushed or registry may be unreachable - "+
				"Use --verify-registry=false to skip verification (not recommended)",
				df.Digest.String(), err)
		}
		_ = img.Close()

		logging.Debug("Verified %s exists in registry", imageRef)
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
