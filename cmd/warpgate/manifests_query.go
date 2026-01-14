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

package main

import (
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/spf13/cobra"
)

// runManifestsInspect inspects a manifest from the registry
func runManifestsInspect(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Use first tag
	tag := "latest"
	if len(manifestsInspectOpts.tags) > 0 {
		tag = manifestsInspectOpts.tags[0]
	}

	manifestRef := manifests.BuildManifestReference(manifestsSharedOpts.registry, manifestsSharedOpts.namespace, manifestsInspectOpts.name, tag)
	logging.InfoContext(ctx, "Inspecting manifest: %s", manifestRef)

	// Inspect the manifest
	manifestInfo, err := manifests.InspectManifest(ctx, manifests.InspectOptions{
		Registry:  manifestsSharedOpts.registry,
		Namespace: manifestsSharedOpts.namespace,
		ImageName: manifestsInspectOpts.name,
		Tag:       tag,
		AuthFile:  manifestsSharedOpts.authFile,
	})
	if err != nil {
		return fmt.Errorf("failed to inspect manifest: %w", err)
	}

	// Display manifest information
	displayManifestInfo(manifestInfo)

	return nil
}

// runManifestsList lists available manifest tags
func runManifestsList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	imageRef := manifests.BuildManifestReference(manifestsSharedOpts.registry, manifestsSharedOpts.namespace, manifestsListOpts.name, "")
	logging.InfoContext(ctx, "Listing tags for: %s", strings.TrimSuffix(imageRef, ":"))

	// List tags
	tags, err := manifests.ListTags(ctx, manifests.ListOptions{
		Registry:  manifestsSharedOpts.registry,
		Namespace: manifestsSharedOpts.namespace,
		ImageName: manifestsListOpts.name,
		AuthFile:  manifestsSharedOpts.authFile,
	})
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tags) == 0 {
		logging.InfoContext(ctx, "No tags found")
		return nil
	}

	logging.InfoContext(ctx, "Found %d tag(s):", len(tags))
	for _, tag := range tags {
		fmt.Printf("  - %s\n", tag)
	}

	return nil
}

// displayManifestInfo displays detailed manifest information
func displayManifestInfo(info *manifests.ManifestInfo) {
	fmt.Println("\n=== Manifest Information ===")
	fmt.Printf("Name:         %s\n", info.Name)
	fmt.Printf("Tag:          %s\n", info.Tag)
	fmt.Printf("Digest:       %s\n", info.Digest)

	isMultiArch := len(info.Architectures) > 1
	if isMultiArch {
		fmt.Printf("Media Type:   %s (multi-architecture manifest)\n", info.MediaType)
	} else {
		fmt.Printf("Media Type:   %s (single-architecture manifest)\n", info.MediaType)
	}

	fmt.Printf("Size:         %d bytes\n", info.Size)

	if len(info.Annotations) > 0 {
		fmt.Println("\nAnnotations:")
		for k, v := range info.Annotations {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	if isMultiArch {
		fmt.Printf("\n=== Architectures (%d) ===\n", len(info.Architectures))
		for i, arch := range info.Architectures {
			fmt.Printf("\n[%d] %s/%s", i+1, arch.OS, arch.Architecture)
			if arch.Variant != "" {
				fmt.Printf("/%s", arch.Variant)
			}
			fmt.Println()
			fmt.Printf("    Manifest Digest: %s\n", arch.Digest)
			fmt.Printf("    Size:            %d bytes\n", arch.Size)
			if arch.MediaType != "" {
				fmt.Printf("    Media Type:      %s\n", arch.MediaType)
			}
		}
	} else {
		fmt.Println("\n=== Platform ===")
		arch := info.Architectures[0]
		fmt.Printf("\nOS/Architecture: %s/%s", arch.OS, arch.Architecture)
		if arch.Variant != "" {
			fmt.Printf("/%s", arch.Variant)
		}
		fmt.Println()
		fmt.Printf("Config Digest:   %s\n", arch.Digest)
		fmt.Printf("Config Size:     %d bytes\n", arch.Size)
		if arch.MediaType != "" {
			fmt.Printf("Config Media:    %s\n", arch.MediaType)
		}
	}
	fmt.Println()
}
