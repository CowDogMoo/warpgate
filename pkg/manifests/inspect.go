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

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"

	"github.com/cowdogmoo/warpgate/v3/pkg/logging"
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

// ArchitectureInfo field meanings depend on whether this is from a single-arch or multi-arch manifest:
//
// Multi-architecture manifest (OCI Index/Docker Manifest List):
//   - Digest: The digest of the platform-specific manifest (not the config blob)
//   - Size: The size of the platform-specific manifest
//   - MediaType: The media type of the platform-specific manifest
//   - OS/Architecture/Variant: Platform information from the index descriptor
//
// Single-architecture manifest:
//   - Digest: The digest of the config blob (not the manifest itself)
//   - Size: The size of the config blob
//   - MediaType: The media type of the config blob (e.g., application/vnd.docker.container.image.v1+json)
//   - OS/Architecture/Variant: Platform information from the config file
type ArchitectureInfo struct {
	OS           string
	Architecture string
	Variant      string
	Digest       string // Platform manifest digest (multi-arch) or config blob digest (single-arch)
	Size         int64
	MediaType    string
}

// InspectManifest inspects a manifest from the registry using go-containerregistry
func InspectManifest(ctx context.Context, opts InspectOptions) (*ManifestInfo, error) {
	logging.Debug("Inspecting manifest: %s/%s:%s", opts.Registry, opts.ImageName, opts.Tag)

	// Build image reference
	imageRef := BuildManifestReference(opts.Registry, opts.Namespace, opts.ImageName, opts.Tag)

	// Parse reference using go-containerregistry
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	// Get descriptor (manifest metadata)
	descriptor, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	// Calculate digest
	manifestDigest := digest.Digest(descriptor.Digest.String())

	info := &ManifestInfo{
		Name:        opts.ImageName,
		Tag:         opts.Tag,
		Digest:      manifestDigest.String(),
		MediaType:   string(descriptor.MediaType),
		Size:        descriptor.Size,
		Annotations: make(map[string]string),
	}

	// Parse manifest based on media type
	if isMultiArchMediaType(descriptor.MediaType) {
		// Multi-arch manifest (OCI Index or Docker Manifest List)
		if err := parseMultiArchManifestFromDescriptor(descriptor, info); err != nil {
			return nil, fmt.Errorf("failed to parse multi-arch manifest: %w", err)
		}
	} else {
		// Single-arch manifest
		if err := parseSingleArchManifestFromDescriptor(descriptor, info); err != nil {
			return nil, fmt.Errorf("failed to parse single-arch manifest: %w", err)
		}
	}

	return info, nil
}

// isMultiArchMediaType checks if the media type represents a multi-arch manifest
func isMultiArchMediaType(mediaType types.MediaType) bool {
	return mediaType == types.OCIManifestSchema1 ||
		mediaType == types.DockerManifestList
}

// parseMultiArchManifestFromDescriptor parses a multi-architecture manifest from a descriptor
func parseMultiArchManifestFromDescriptor(descriptor *remote.Descriptor, info *ManifestInfo) error {
	// Try to get as index (multi-arch)
	index, err := descriptor.ImageIndex()
	if err != nil {
		return fmt.Errorf("failed to get image index: %w", err)
	}

	// Get index manifest
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return fmt.Errorf("failed to get index manifest: %w", err)
	}

	// Extract annotations
	if indexManifest.Annotations != nil {
		info.Annotations = indexManifest.Annotations
	}

	// Extract architectures from the v1.IndexManifest
	for _, desc := range indexManifest.Manifests {
		archInfo := ArchitectureInfo{
			Digest:    desc.Digest.String(),
			Size:      desc.Size,
			MediaType: string(desc.MediaType),
		}

		// Extract platform information from the v1.Descriptor
		if desc.Platform != nil {
			archInfo.OS = desc.Platform.OS
			archInfo.Architecture = desc.Platform.Architecture
			archInfo.Variant = desc.Platform.Variant
		}

		info.Architectures = append(info.Architectures, archInfo)
	}

	return nil
}

// parseSingleArchManifestFromDescriptor parses a single-architecture manifest from a descriptor
func parseSingleArchManifestFromDescriptor(descriptor *remote.Descriptor, info *ManifestInfo) error {
	// Try to get as image (single-arch)
	img, err := descriptor.Image()
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	// Get manifest
	manifest, err := img.Manifest()
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	// Get config file for platform info
	configFile, err := img.ConfigFile()
	if err != nil {
		logging.Warn("Failed to get config file for manifest %s: %v. Architecture info will be incomplete.",
			manifest.Config.Digest.String()[:12], err)
		// Continue without platform info - we'll use "unknown" values
		configFile = nil
	}

	// Single architecture manifest
	archInfo := ArchitectureInfo{
		Digest:       manifest.Config.Digest.String(),
		Size:         manifest.Config.Size,
		MediaType:    string(manifest.Config.MediaType),
		OS:           "unknown",
		Architecture: "unknown",
	}

	// Extract platform info from config if available
	if configFile != nil {
		archInfo.OS = configFile.OS
		archInfo.Architecture = configFile.Architecture
		archInfo.Variant = configFile.Variant
	}

	info.Architectures = []ArchitectureInfo{archInfo}

	if manifest.Annotations != nil {
		info.Annotations = manifest.Annotations
	}

	return nil
}

// ListTags lists available tags for an image in the registry using go-containerregistry
func ListTags(ctx context.Context, opts ListOptions) ([]string, error) {
	logging.Debug("Listing tags for: %s/%s", opts.Registry, opts.ImageName)

	// Build repository reference
	imageRef := BuildManifestReference(opts.Registry, opts.Namespace, opts.ImageName, "")
	// Remove trailing colon if no tag was specified
	imageRef = fmt.Sprintf("%s:%s", imageRef[:len(imageRef)-1], "latest")

	// Parse as repository
	repo, err := name.NewRepository(imageRef[:len(imageRef)-7]) // Remove ":latest"
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	// List tags using go-containerregistry
	tags, err := remote.List(repo, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return tags, nil
}
