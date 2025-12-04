/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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

package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/common/pkg/manifests"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/opencontainers/go-digest"
	"go.podman.io/image/v5/transports/alltransports"
	imagetypes "go.podman.io/image/v5/types"
	"go.podman.io/storage"
)

// ManifestManager handles multi-arch manifest creation and management
// This replaces the merge-manifests job from GitHub Actions
type ManifestManager struct {
	store         storage.Store
	systemContext *imagetypes.SystemContext
}

// NewManifestManager creates a new manifest manager
func NewManifestManager(store storage.Store, systemContext *imagetypes.SystemContext) *ManifestManager {
	return &ManifestManager{
		store:         store,
		systemContext: systemContext,
	}
}

// ManifestEntry represents a single architecture in a manifest
type ManifestEntry struct {
	ImageRef     string
	Digest       digest.Digest
	Platform     string
	Architecture string
	OS           string
	Variant      string
}

// CreateManifest creates a multi-arch manifest from individual architecture images
// TODO: Implement multi-arch manifest support with updated containers API
func (mm *ManifestManager) CreateManifest(ctx context.Context, manifestName string, entries []ManifestEntry) (manifests.List, error) {
	logging.Info("Creating multi-arch manifest: %s", manifestName)

	if len(entries) == 0 {
		return nil, fmt.Errorf("no manifest entries provided")
	}

	// Create a new manifest list
	manifestList := manifests.Create()

	logging.Info("Manifest %s will include %d architectures:", manifestName, len(entries))

	// TODO: Update to use current manifests API
	// The Add method signature has changed in newer versions
	// For now, return the manifest list without adding entries
	logging.Warn("Multi-arch manifest creation not yet fully implemented")

	return manifestList, nil
}

// PushManifest pushes a manifest list to a registry
// TODO: Implement manifest push with updated containers API
func (mm *ManifestManager) PushManifest(ctx context.Context, manifestBytes string, destination string) error {
	logging.Info("Pushing manifest to %s", destination)

	// Parse the destination reference
	destRefStr := destination
	if !strings.HasPrefix(destination, "docker://") {
		destRefStr = "docker://" + destination
	}

	_, err := alltransports.ParseImageName(destRefStr)
	if err != nil {
		return fmt.Errorf("failed to parse destination: %w", err)
	}

	// TODO: Update to use current manifests API
	// The Push method and related types have changed
	logging.Warn("Manifest push not yet fully implemented")

	return fmt.Errorf("manifest push not yet implemented")
}

// InspectManifest inspects a manifest list
// TODO: Implement manifest inspection with updated containers API
func (mm *ManifestManager) InspectManifest(ctx context.Context, manifestName string) ([]ManifestEntry, error) {
	logging.Debug("Inspecting manifest: %s", manifestName)

	// Parse the manifest reference
	_, err := alltransports.ParseImageName(manifestName)
	if err != nil {
		_, err = alltransports.ParseImageName("containers-storage:" + manifestName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse manifest reference: %w", err)
		}
	}

	// TODO: Update to use current manifests API
	// The LoadFromImage function signature has changed
	logging.Warn("Manifest inspection not yet fully implemented")

	return nil, fmt.Errorf("manifest inspection not yet implemented")
}

// RemoveManifest removes a manifest list
// TODO: Implement manifest removal with updated containers API
func (mm *ManifestManager) RemoveManifest(ctx context.Context, manifestName string) error {
	logging.Debug("Removing manifest: %s", manifestName)

	// TODO: Update to use current manifests API
	logging.Warn("Manifest removal not yet fully implemented")

	return fmt.Errorf("manifest removal not yet implemented")
}
