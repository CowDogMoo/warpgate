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

package buildah

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/common/pkg/manifests"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	warpgatemanifests "github.com/cowdogmoo/warpgate/pkg/manifests"
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

// CreateManifest creates a multi-arch manifest from individual architecture images
func (mm *ManifestManager) CreateManifest(ctx context.Context, manifestName string, entries []warpgatemanifests.ManifestEntry) (manifests.List, error) {
	logging.Info("Creating multi-arch manifest: %s", manifestName)

	if len(entries) == 0 {
		return nil, fmt.Errorf("no manifest entries provided")
	}

	// Create a new manifest list
	manifestList := manifests.Create()

	logging.Info("Manifest %s will include %d architectures:", manifestName, len(entries))

	// Add each architecture image to the manifest
	for _, entry := range entries {
		logging.Info("  - %s/%s (digest: %s)", entry.OS, entry.Architecture, entry.Digest.String())

		// Get the image from the store to determine its size and type
		img, err := mm.store.Image(entry.Digest.String())
		if err != nil {
			return nil, fmt.Errorf("failed to get image for digest %s: %w", entry.Digest, err)
		}

		// Get the manifest to determine its size
		manifestBytes, err := mm.store.ImageBigData(img.ID, "manifest")
		if err != nil {
			return nil, fmt.Errorf("failed to get manifest data: %w", err)
		}

		// Determine manifest type (default to OCI)
		manifestType := "application/vnd.oci.image.manifest.v1+json"

		// Add this architecture to the manifest list
		err = manifestList.AddInstance(
			entry.Digest,              // manifestDigest
			int64(len(manifestBytes)), // manifestSize
			manifestType,              // manifestType
			entry.OS,                  // os
			entry.Architecture,        // architecture
			"",                        // osVersion
			nil,                       // osFeatures
			entry.Variant,             // variant
			nil,                       // features
			nil,                       // annotations
		)
		if err != nil {
			return nil, fmt.Errorf("failed to add instance to manifest: %w", err)
		}
	}

	logging.Info("Successfully created multi-arch manifest with %d architectures", len(entries))
	return manifestList, nil
}

// PushManifest pushes a manifest list to a registry
func (mm *ManifestManager) PushManifest(ctx context.Context, manifestList manifests.List, destination string) error {
	logging.Info("Pushing manifest to %s", destination)

	// Parse the destination reference
	destRefStr := destination
	if !strings.HasPrefix(destination, "docker://") {
		destRefStr = "docker://" + destination
	}

	destRef, err := alltransports.ParseImageName(destRefStr)
	if err != nil {
		return fmt.Errorf("failed to parse destination: %w", err)
	}

	// Serialize the manifest list to OCI format
	manifestBytes, err := manifestList.Serialize("application/vnd.oci.image.index.v1+json")
	if err != nil {
		// Fallback to Docker format if OCI fails
		manifestBytes, err = manifestList.Serialize("application/vnd.docker.distribution.manifest.list.v2+json")
		if err != nil {
			return fmt.Errorf("failed to serialize manifest: %w", err)
		}
	}

	// Create a temporary image in the store with the manifest list
	tempImageName := fmt.Sprintf("localhost/temp-manifest-%s", strings.ReplaceAll(destination, "/", "-"))

	// Store the manifest list in the image store
	img, err := mm.store.CreateImage(tempImageName, nil, "", "", nil)
	if err != nil {
		return fmt.Errorf("failed to create temporary manifest image: %w", err)
	}
	defer func() {
		// Clean up temporary image
		if _, err := mm.store.DeleteImage(img.ID, true); err != nil {
			logging.Warn("Failed to delete temporary manifest image: %v", err)
		}
	}()

	// Set the manifest list data
	err = mm.store.SetImageBigData(img.ID, "manifest", manifestBytes, nil)
	if err != nil {
		return fmt.Errorf("failed to set manifest data: %w", err)
	}

	// Push the manifest using buildah's push functionality
	pushOpts := buildah.PushOptions{
		Store:         mm.store,
		SystemContext: mm.systemContext,
	}

	_, digest, err := buildah.Push(ctx, img.ID, destRef, pushOpts)
	if err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	logging.Info("Successfully pushed manifest list to %s", destination)
	logging.Info("Manifest digest: %s", digest.String())

	return nil
}

// InspectManifest inspects a manifest list
func (mm *ManifestManager) InspectManifest(ctx context.Context, manifestName string) ([]ManifestEntry, error) {
	logging.Debug("Inspecting manifest: %s", manifestName)

	// Get the image from storage
	img, err := mm.store.Image(manifestName)
	if err != nil {
		return nil, fmt.Errorf("failed to find manifest image: %w", err)
	}

	// Get the manifest data
	manifestBytes, err := mm.store.ImageBigData(img.ID, "manifest")
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest data: %w", err)
	}

	// Parse the manifest list
	manifestList, err := manifests.FromBlob(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest list: %w", err)
	}

	// Get all instances from the manifest
	instances := manifestList.Instances()
	entries := make([]ManifestEntry, 0, len(instances))

	for _, instanceDigest := range instances {
		os, _ := manifestList.OS(instanceDigest)
		arch, _ := manifestList.Architecture(instanceDigest)
		variant, _ := manifestList.Variant(instanceDigest)

		entry := ManifestEntry{
			Digest:       instanceDigest,
			OS:           os,
			Architecture: arch,
			Variant:      variant,
			Platform:     fmt.Sprintf("%s/%s", os, arch),
		}

		if variant != "" {
			entry.Platform = fmt.Sprintf("%s/%s/%s", os, arch, variant)
		}

		entries = append(entries, entry)
		logging.Debug("  - %s (digest: %s)", entry.Platform, entry.Digest.String())
	}

	logging.Info("Manifest %s contains %d architecture(s)", manifestName, len(entries))
	return entries, nil
}

// RemoveManifest removes a manifest list from local storage
func (mm *ManifestManager) RemoveManifest(ctx context.Context, manifestName string) error {
	logging.Debug("Removing manifest: %s", manifestName)

	// Get the image from storage
	img, err := mm.store.Image(manifestName)
	if err != nil {
		return fmt.Errorf("failed to find manifest image: %w", err)
	}

	// Delete the manifest image
	if _, err := mm.store.DeleteImage(img.ID, true); err != nil {
		return fmt.Errorf("failed to remove manifest: %w", err)
	}

	logging.Info("Successfully removed manifest: %s", manifestName)
	return nil
}
