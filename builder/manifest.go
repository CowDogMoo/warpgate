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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BuildManifest represents the JSON output of a build operation.
// It contains all build results, metadata, and timing information
// that can be used by CI/CD pipelines for deployment and tracking.
type BuildManifest struct {
	// Template is the name of the template that was built
	Template string `json:"template"`

	// Version is the version of the template
	Version string `json:"version"`

	// Timestamp is when the build completed
	Timestamp time.Time `json:"timestamp"`

	// Duration is the total build time in human-readable format
	Duration string `json:"duration"`

	// Builds contains the results for each architecture/target built
	Builds []ManifestBuild `json:"builds"`

	// Manifest contains the multi-arch manifest reference if created
	Manifest *ManifestRef `json:"manifest,omitempty"`

	// Pushed indicates whether builds were pushed to a registry
	Pushed bool `json:"pushed"`

	// WarpgateVersion is the version of warpgate that created this manifest
	WarpgateVersion string `json:"warpgate_version"`
}

// ManifestBuild represents a single build result within the manifest
type ManifestBuild struct {
	// Type is the build target type: "container" or "ami"
	Type string `json:"type"`

	// Platform is the target platform (e.g., "linux/amd64")
	Platform string `json:"platform,omitempty"`

	// Architecture is the target architecture (e.g., "amd64", "arm64")
	Architecture string `json:"architecture,omitempty"`

	// ImageRef is the full image reference for container builds
	ImageRef string `json:"image_ref,omitempty"`

	// Digest is the SHA256 digest for container builds
	Digest string `json:"digest,omitempty"`

	// AMIID is the AMI ID for AWS AMI builds
	AMIID string `json:"ami_id,omitempty"`

	// Region is the AWS region for AMI builds
	Region string `json:"region,omitempty"`

	// Duration is the build time for this specific build
	Duration string `json:"duration,omitempty"`

	// Notes contains any warnings or additional information
	Notes []string `json:"notes,omitempty"`
}

// ManifestRef contains a reference to a multi-architecture manifest
type ManifestRef struct {
	// Ref is the manifest reference (e.g., "ghcr.io/org/image:latest")
	Ref string `json:"ref"`

	// Digest is the manifest digest
	Digest string `json:"digest,omitempty"`
}

// NewBuildManifest creates a new BuildManifest from build results
func NewBuildManifest(config *Config, results []BuildResult, duration time.Duration) *BuildManifest {
	manifest := &BuildManifest{
		Template:        config.Metadata.Name,
		Version:         config.Metadata.Version,
		Timestamp:       time.Now().UTC(),
		Duration:        duration.Round(time.Millisecond).String(),
		WarpgateVersion: Version,
		Builds:          make([]ManifestBuild, 0, len(results)),
	}

	for _, result := range results {
		build := ManifestBuild{
			Platform:     result.Platform,
			Architecture: result.Architecture,
			ImageRef:     result.ImageRef,
			Digest:       result.Digest,
			AMIID:        result.AMIID,
			Region:       result.Region,
			Duration:     result.Duration,
			Notes:        result.Notes,
		}

		// Determine build type from result fields
		if result.AMIID != "" {
			build.Type = "ami"
		} else {
			build.Type = "container"
		}

		manifest.Builds = append(manifest.Builds, build)
	}

	return manifest
}

// WriteManifest writes the build manifest to a JSON file
func WriteManifest(path string, manifest *BuildManifest) error {
	// Validate inputs
	if path == "" {
		return fmt.Errorf("manifest path cannot be empty")
	}
	if manifest == nil {
		return fmt.Errorf("manifest cannot be nil")
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create manifest directory: %w", err)
		}
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write file with restrictive permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

// ReadManifest reads a build manifest from a JSON file
func ReadManifest(path string) (*BuildManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest BuildManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}
