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
	"fmt"
	"time"

	"github.com/cowdogmoo/warpgate/v3/pkg/logging"
	"github.com/opencontainers/go-digest"
)

// ValidationOptions contains options for validating digest files
type ValidationOptions struct {
	ImageName string
	MaxAge    time.Duration
}

// FilterOptions contains options for filtering digest files by architecture
type FilterOptions struct {
	RequiredArchitectures []string
	BestEffort            bool
}

// ValidateDigestFiles validates digest files
func ValidateDigestFiles(digestFiles []DigestFile, opts ValidationOptions) error {
	now := time.Now()

	for _, df := range digestFiles {
		// Validate image name matches
		if df.ImageName != opts.ImageName {
			return fmt.Errorf("digest file %s has incorrect image name: expected %s, got %s",
				df.Path, opts.ImageName, df.ImageName)
		}

		// Validate digest format
		if df.Digest.Algorithm() != digest.SHA256 {
			return fmt.Errorf("digest file %s uses unsupported algorithm: %s (expected sha256)",
				df.Path, df.Digest.Algorithm())
		}

		// Validate age if max-age is set
		if opts.MaxAge > 0 {
			age := now.Sub(df.ModTime)
			if age > opts.MaxAge {
				return fmt.Errorf("digest file %s is too old: %s (max age: %s)",
					df.Path, age, opts.MaxAge)
			}
		}

		logging.Debug("Validated digest file: %s (%s, age: %s)",
			df.Path, df.Architecture, now.Sub(df.ModTime))
	}

	return nil
}

// FilterArchitectures filters digest files based on architecture requirements
func FilterArchitectures(digestFiles []DigestFile, opts FilterOptions) ([]DigestFile, error) {
	// If no requirements, return all (auto-detect mode)
	if len(opts.RequiredArchitectures) == 0 {
		architectures := make([]string, 0, len(digestFiles))
		for _, df := range digestFiles {
			architectures = append(architectures, df.Architecture)
		}
		logging.Info("Auto-detected %d architecture(s): %v", len(architectures), architectures)
		return digestFiles, nil
	}

	// Create map of required architectures
	required := make(map[string]bool)
	for _, arch := range opts.RequiredArchitectures {
		required[arch] = true
	}

	// Filter digest files
	filtered := make([]DigestFile, 0, len(digestFiles))
	for _, df := range digestFiles {
		if required[df.Architecture] {
			filtered = append(filtered, df)
			delete(required, df.Architecture)
		}
	}

	// Check if any required architectures are missing
	if len(required) > 0 {
		missing := make([]string, 0, len(required))
		for arch := range required {
			missing = append(missing, arch)
		}

		if opts.BestEffort {
			logging.Warn("Some required architectures are missing: %v", missing)
			logging.Warn("Continuing in best-effort mode with %d available architecture(s)", len(filtered))
		} else {
			return nil, fmt.Errorf("missing required architectures: %v (use --best-effort to create partial manifest)", missing)
		}
	}

	return filtered, nil
}
