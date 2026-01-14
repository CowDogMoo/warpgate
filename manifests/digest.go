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
	"path/filepath"
	"strings"
	"time"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/opencontainers/go-digest"
)

// DigestFile represents a parsed digest file
type DigestFile struct {
	Path         string
	ImageName    string
	Architecture string
	Digest       digest.Digest
	ModTime      time.Time
}

// DiscoveryOptions contains options for discovering digest files
type DiscoveryOptions struct {
	ImageName string
	Directory string
}

// CreationOptions contains options for creating manifests
type CreationOptions struct {
	Registry    string
	Namespace   string
	ImageName   string
	Tag         string
	Annotations map[string]string // OCI annotations
	Labels      map[string]string // OCI labels
}

// ManifestEntry represents a single architecture image in a multi-arch manifest
type ManifestEntry struct {
	ImageRef     string
	Digest       digest.Digest
	Platform     string
	Architecture string
	OS           string
	Variant      string
}

// DiscoverDigestFiles discovers and parses digest files in the specified directory
func DiscoverDigestFiles(ctx context.Context, opts DiscoveryOptions) ([]DigestFile, error) {
	logging.InfoContext(ctx, "Discovering digest files in %s", opts.Directory)

	// Pattern: digest-{IMAGE_NAME}-{ARCHITECTURE}.txt
	pattern := fmt.Sprintf("digest-%s-*.txt", opts.ImageName)
	matches, err := filepath.Glob(filepath.Join(opts.Directory, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to glob digest files: %w", err)
	}

	if len(matches) == 0 {
		return nil, nil
	}

	digestFiles := make([]DigestFile, 0, len(matches))
	for _, path := range matches {
		df, err := ParseDigestFile(path)
		if err != nil {
			logging.WarnContext(ctx, "Skipping invalid digest file %s: %v", path, err)
			continue
		}
		digestFiles = append(digestFiles, df)
	}

	return digestFiles, nil
}

// ParseDigestFile parses a single digest file
func ParseDigestFile(path string) (DigestFile, error) {
	// Extract architecture from filename
	// Expected format: digest-{IMAGE_NAME}-{ARCHITECTURE}.txt
	// Known architectures: amd64, arm64, arm-v7, arm-v6, ppc64le, s390x, 386, riscv64
	basename := filepath.Base(path)

	// Remove prefix and suffix
	if !strings.HasPrefix(basename, "digest-") || !strings.HasSuffix(basename, ".txt") {
		return DigestFile{}, fmt.Errorf("invalid filename format: %s (expected digest-*-*.txt)", basename)
	}

	// Remove "digest-" prefix and ".txt" suffix
	nameArch := strings.TrimPrefix(basename, "digest-")
	nameArch = strings.TrimSuffix(nameArch, ".txt")

	// Known architecture patterns (including variants)
	archPatterns := []string{
		"amd64", "arm64", "arm-v7", "arm-v6", "arm-v8",
		"ppc64le", "s390x", "386", "riscv64",
	}

	var imageName, architecture string
	found := false

	// Try to match known architecture patterns from the end
	for _, arch := range archPatterns {
		if strings.HasSuffix(nameArch, "-"+arch) {
			imageName = strings.TrimSuffix(nameArch, "-"+arch)
			architecture = arch
			found = true
			break
		}
	}

	if !found {
		return DigestFile{}, fmt.Errorf("invalid filename format: %s (unknown architecture)", basename)
	}

	// Read digest content
	content, err := os.ReadFile(path)
	if err != nil {
		return DigestFile{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse digest (should be sha256:...)
	digestStr := strings.TrimSpace(string(content))
	imageDigest, err := digest.Parse(digestStr)
	if err != nil {
		return DigestFile{}, fmt.Errorf("invalid digest format: %w", err)
	}

	// Get file modification time
	fileInfo, err := os.Stat(path)
	if err != nil {
		return DigestFile{}, fmt.Errorf("failed to stat file: %w", err)
	}

	return DigestFile{
		Path:         path,
		ImageName:    imageName,
		Architecture: architecture,
		Digest:       imageDigest,
		ModTime:      fileInfo.ModTime(),
	}, nil
}

// SaveDigestToFile saves an image digest to a file
func SaveDigestToFile(ctx context.Context, imageName, arch, digestStr, dir string) error {
	if digestStr == "" {
		return fmt.Errorf("empty digest provided")
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, config.DirPermReadWriteExec); err != nil {
		return fmt.Errorf("failed to create digest directory: %w", err)
	}

	// Create filename: digest-{image}-{arch}.txt
	filename := fmt.Sprintf("digest-%s-%s.txt", imageName, arch)
	filepath := filepath.Join(dir, filename)

	// Write digest to file
	if err := os.WriteFile(filepath, []byte(digestStr), 0644); err != nil {
		return fmt.Errorf("failed to write digest file: %w", err)
	}

	logging.InfoContext(ctx, "Saved digest to %s", filepath)
	return nil
}
