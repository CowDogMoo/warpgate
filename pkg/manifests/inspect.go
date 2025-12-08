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
	"encoding/json"
	"fmt"

	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/manifest"
)

// InspectOptions contains options for inspecting manifests
type InspectOptions struct {
	Registry  string
	Namespace string
	ImageName string
	Tag       string
	AuthFile  string
}

// ListOptions contains options for listing tags
type ListOptions struct {
	Registry  string
	Namespace string
	ImageName string
	AuthFile  string
}

// ManifestInfo contains detailed information about a manifest
type ManifestInfo struct {
	Name          string
	Tag           string
	Digest        string
	MediaType     string
	Size          int64
	Annotations   map[string]string
	Architectures []ArchitectureInfo
}

// ArchitectureInfo contains information about a specific architecture in a manifest
type ArchitectureInfo struct {
	OS           string
	Architecture string
	Variant      string
	Digest       string
	Size         int64
	MediaType    string
}

// InspectManifest inspects a manifest from the registry
func InspectManifest(ctx context.Context, opts InspectOptions) (*ManifestInfo, error) {
	logging.Debug("Inspecting manifest: %s/%s:%s", opts.Registry, opts.ImageName, opts.Tag)

	// Build image reference
	imageRef := BuildManifestReference(opts.Registry, opts.Namespace, opts.ImageName, opts.Tag)

	// Create system context
	systemContext, err := createSystemContext(opts.AuthFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create system context: %w", err)
	}

	// Parse reference
	ref, err := docker.ParseReference("//" + imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	// Get image source
	src, err := ref.NewImageSource(ctx, systemContext)
	if err != nil {
		return nil, fmt.Errorf("failed to create image source: %w", err)
	}
	defer src.Close()

	// Get manifest
	manifestBytes, manifestType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	// Calculate digest
	manifestDigest := digest.FromBytes(manifestBytes)

	info := &ManifestInfo{
		Name:        opts.ImageName,
		Tag:         opts.Tag,
		Digest:      manifestDigest.String(),
		MediaType:   manifestType,
		Size:        int64(len(manifestBytes)),
		Annotations: make(map[string]string),
	}

	// Parse manifest based on type
	if manifest.MIMETypeIsMultiImage(manifestType) {
		// Multi-arch manifest (OCI Index or Docker Manifest List)
		if err := parseMultiArchManifest(manifestBytes, manifestType, info); err != nil {
			return nil, fmt.Errorf("failed to parse multi-arch manifest: %w", err)
		}
	} else {
		// Single-arch manifest
		if err := parseSingleArchManifest(manifestBytes, manifestType, info); err != nil {
			return nil, fmt.Errorf("failed to parse single-arch manifest: %w", err)
		}
	}

	return info, nil
}

// parseMultiArchManifest parses a multi-architecture manifest (OCI Index or Docker Manifest List)
func parseMultiArchManifest(manifestBytes []byte, manifestType string, info *ManifestInfo) error {
	// Try parsing as OCI Index first
	var ociIndex v1.Index
	if err := json.Unmarshal(manifestBytes, &ociIndex); err == nil {
		// Extract annotations
		if ociIndex.Annotations != nil {
			info.Annotations = ociIndex.Annotations
		}

		// Extract architectures
		for _, desc := range ociIndex.Manifests {
			archInfo := ArchitectureInfo{
				Digest:    desc.Digest.String(),
				Size:      desc.Size,
				MediaType: desc.MediaType,
			}

			if desc.Platform != nil {
				archInfo.OS = desc.Platform.OS
				archInfo.Architecture = desc.Platform.Architecture
				archInfo.Variant = desc.Platform.Variant
			}

			info.Architectures = append(info.Architectures, archInfo)
		}

		return nil
	}

	// Try parsing as Docker Manifest List
	var dockerList manifest.Schema2List
	if err := json.Unmarshal(manifestBytes, &dockerList); err == nil {
		// Extract architectures from Docker manifest list
		for _, mfst := range dockerList.Manifests {
			archInfo := ArchitectureInfo{
				Digest:       mfst.Digest.String(),
				Size:         mfst.Size,
				MediaType:    mfst.MediaType,
				OS:           mfst.Platform.OS,
				Architecture: mfst.Platform.Architecture,
				Variant:      mfst.Platform.Variant,
			}
			info.Architectures = append(info.Architectures, archInfo)
		}

		return nil
	}

	return fmt.Errorf("failed to parse manifest as OCI Index or Docker Manifest List")
}

// parseSingleArchManifest parses a single-architecture manifest
func parseSingleArchManifest(manifestBytes []byte, manifestType string, info *ManifestInfo) error {
	// For single-arch, we can try to extract platform info from the config
	var ociManifest v1.Manifest
	if err := json.Unmarshal(manifestBytes, &ociManifest); err == nil {
		// Single architecture manifest
		archInfo := ArchitectureInfo{
			Digest:    ociManifest.Config.Digest.String(),
			Size:      ociManifest.Config.Size,
			MediaType: ociManifest.Config.MediaType,
			// Platform info would need to be fetched from config blob
			OS:           "unknown",
			Architecture: "unknown",
		}
		info.Architectures = []ArchitectureInfo{archInfo}

		if ociManifest.Annotations != nil {
			info.Annotations = ociManifest.Annotations
		}

		return nil
	}

	return fmt.Errorf("failed to parse single-arch manifest")
}

// ListTags lists available tags for an image in the registry
func ListTags(ctx context.Context, opts ListOptions) ([]string, error) {
	logging.Debug("Listing tags for: %s/%s", opts.Registry, opts.ImageName)

	// For now, return a simplified implementation
	// Full implementation would require registry API v2 calls
	// This is a placeholder that can be enhanced later

	return nil, fmt.Errorf("tag listing not yet fully implemented - requires registry API v2 support")
}
