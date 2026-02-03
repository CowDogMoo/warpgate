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

package templates

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// VersionManager handles template version validation and compatibility checking
type VersionManager struct {
	warpgateVersion *semver.Version
}

// NewVersionManager returns a [VersionManager] that resolves template
// compatibility against the given semantic version string.
func NewVersionManager(warpgateVersion string) (*VersionManager, error) {
	ver, err := semver.NewVersion(warpgateVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid warpgate version: %w", err)
	}

	return &VersionManager{
		warpgateVersion: ver,
	}, nil
}

// ParseVersion parses a semantic version string
func (vm *VersionManager) ParseVersion(version string) (*semver.Version, error) {
	// Handle empty version
	if version == "" || version == "latest" {
		return nil, nil // nil means "latest"
	}

	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	ver, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %w", err)
	}

	return ver, nil
}

// ValidateConstraint checks if a version satisfies a constraint
func (vm *VersionManager) ValidateConstraint(version, constraint string) (bool, error) {
	// Parse the version
	ver, err := vm.ParseVersion(version)
	if err != nil {
		return false, err
	}

	// If version is nil (latest), it always satisfies
	if ver == nil {
		return true, nil
	}

	// Parse the constraint
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, fmt.Errorf("invalid constraint: %w", err)
	}

	return c.Check(ver), nil
}

// CheckCompatibility checks if a template is compatible with warpgate version
func (vm *VersionManager) CheckCompatibility(templateVersion, requiredWarpgateVersion string) (bool, []string, error) {
	warnings := []string{}

	// If no required version specified, assume compatible
	if requiredWarpgateVersion == "" {
		return true, warnings, nil
	}

	// Parse constraint
	constraint, err := semver.NewConstraint(requiredWarpgateVersion)
	if err != nil {
		return false, warnings, fmt.Errorf("invalid warpgate version constraint: %w", err)
	}

	// Check if current warpgate version satisfies the constraint
	compatible := constraint.Check(vm.warpgateVersion)

	if !compatible {
		warnings = append(warnings, fmt.Sprintf(
			"Template requires warpgate %s, but current version is %s",
			requiredWarpgateVersion, vm.warpgateVersion.String(),
		))
	}

	return compatible, warnings, nil
}

// CompareVersions compares two semantic versions and returns -1 if v1 < v2,
// 0 if v1 == v2, or 1 if v1 > v2.
func (vm *VersionManager) CompareVersions(v1, v2 string) (int, error) {
	ver1, err := vm.ParseVersion(v1)
	if err != nil {
		return 0, err
	}

	ver2, err := vm.ParseVersion(v2)
	if err != nil {
		return 0, err
	}

	// Handle nil (latest) versions
	if ver1 == nil && ver2 == nil {
		return 0, nil
	}
	if ver1 == nil {
		return 1, nil // latest is always greater
	}
	if ver2 == nil {
		return -1, nil
	}

	return ver1.Compare(ver2), nil
}

// IsBreakingChange checks if a version change is a breaking change (major version bump)
func (vm *VersionManager) IsBreakingChange(oldVersion, newVersion string) (bool, error) {
	old, err := vm.ParseVersion(oldVersion)
	if err != nil {
		return false, err
	}

	new, err := vm.ParseVersion(newVersion)
	if err != nil {
		return false, err
	}

	// If either is nil, can't determine
	if old == nil || new == nil {
		return false, nil
	}

	// Breaking change if major version increased
	return new.Major() > old.Major(), nil
}

// GetLatestVersion returns the latest version from a list of versions
func (vm *VersionManager) GetLatestVersion(ctx context.Context, versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions provided")
	}

	var latest *semver.Version
	latestStr := ""

	for _, v := range versions {
		ver, err := vm.ParseVersion(v)
		if err != nil {
			logging.WarnContext(ctx, "Skipping invalid version: %s - %v", v, err)
			continue
		}

		// Skip nil (latest) versions in comparison
		if ver == nil {
			continue
		}

		if latest == nil || ver.GreaterThan(latest) {
			latest = ver
			latestStr = v
		}
	}

	if latestStr == "" {
		return "latest", nil
	}

	return latestStr, nil
}

// ValidateVersionRange checks if a version is within a range
func (vm *VersionManager) ValidateVersionRange(version, minVersion, maxVersion string) (bool, error) {
	ver, err := vm.ParseVersion(version)
	if err != nil {
		return false, err
	}

	// nil (latest) always satisfies range
	if ver == nil {
		return true, nil
	}

	// Check minimum version
	if minVersion != "" {
		min, err := vm.ParseVersion(minVersion)
		if err != nil {
			return false, fmt.Errorf("invalid minimum version: %w", err)
		}
		if min != nil && ver.LessThan(min) {
			return false, nil
		}
	}

	// Check maximum version
	if maxVersion != "" {
		max, err := vm.ParseVersion(maxVersion)
		if err != nil {
			return false, fmt.Errorf("invalid maximum version: %w", err)
		}
		if max != nil && ver.GreaterThan(max) {
			return false, nil
		}
	}

	return true, nil
}
