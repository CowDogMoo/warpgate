/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

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

package ami

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
)

// SortComponentVersions sorts component versions by semantic version in descending order.
// The most recent version will be first in the returned slice.
func SortComponentVersions(versions []types.ComponentVersion) []types.ComponentVersion {
	sorted := make([]types.ComponentVersion, len(versions))
	copy(sorted, versions)

	// Simple bubble sort for semantic versions (usually small number of versions)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if CompareSemanticVersions(sorted[j].Version, sorted[j+1].Version) < 0 {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}

// CompareSemanticVersions compares two semantic versions.
// Returns: positive if v1 > v2, negative if v1 < v2, 0 if equal.
// Handles nil pointers gracefully.
func CompareSemanticVersions(v1, v2 *string) int {
	if v1 == nil && v2 == nil {
		return 0
	}
	if v1 == nil {
		return -1
	}
	if v2 == nil {
		return 1
	}

	parts1 := strings.Split(*v1, ".")
	parts2 := strings.Split(*v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			_, _ = fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			_, _ = fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 != n2 {
			return n1 - n2
		}
	}

	return 0
}

// ParseSemanticVersion parses a semantic version string into its major, minor, and patch components.
// Returns (0, 0, 0) if the version string is empty or invalid.
func ParseSemanticVersion(version string) (major, minor, patch int) {
	if version == "" {
		return 0, 0, 0
	}

	parts := strings.Split(version, ".")
	if len(parts) >= 1 {
		_, _ = fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) >= 2 {
		_, _ = fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) >= 3 {
		_, _ = fmt.Sscanf(parts[2], "%d", &patch)
	}

	return major, minor, patch
}

// FormatSemanticVersion formats major, minor, patch components into a semantic version string.
func FormatSemanticVersion(major, minor, patch int) string {
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

// IncrementPatchVersion increments the patch component of a semantic version.
func IncrementPatchVersion(version string) string {
	major, minor, patch := ParseSemanticVersion(version)
	return FormatSemanticVersion(major, minor, patch+1)
}
