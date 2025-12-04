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
	"testing"
)

func TestNewVersionManager(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
	}{
		{
			name:        "valid version",
			version:     "1.0.0",
			expectError: false,
		},
		{
			name:        "valid version with v prefix",
			version:     "v2.1.3",
			expectError: false,
		},
		{
			name:        "invalid version",
			version:     "invalid",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := NewVersionManager(tc.version)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if vm == nil {
					t.Error("Version manager is nil")
				}
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	tests := []struct {
		name        string
		version     string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "valid version",
			version:     "1.2.3",
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "version with v prefix",
			version:     "v1.2.3",
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "latest version",
			version:     "latest",
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "empty version",
			version:     "",
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "invalid version",
			version:     "invalid",
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ver, err := vm.ParseVersion(tc.version)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tc.expectNil && ver != nil {
					t.Error("Expected nil version but got non-nil")
				}
				if !tc.expectNil && ver == nil {
					t.Error("Expected non-nil version but got nil")
				}
			}
		})
	}
}

func TestValidateConstraint(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	tests := []struct {
		name        string
		version     string
		constraint  string
		expected    bool
		expectError bool
	}{
		{
			name:        "exact version match",
			version:     "1.0.0",
			constraint:  "1.0.0",
			expected:    true,
			expectError: false,
		},
		{
			name:        "range match",
			version:     "1.5.0",
			constraint:  ">=1.0.0, <2.0.0",
			expected:    true,
			expectError: false,
		},
		{
			name:        "range no match",
			version:     "2.0.0",
			constraint:  ">=1.0.0, <2.0.0",
			expected:    false,
			expectError: false,
		},
		{
			name:        "latest always matches",
			version:     "latest",
			constraint:  ">=1.0.0",
			expected:    true,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := vm.ValidateConstraint(tc.version, tc.constraint)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("Expected %v, got %v", tc.expected, result)
				}
			}
		})
	}
}

func TestCheckCompatibility(t *testing.T) {
	vm, err := NewVersionManager("1.5.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	tests := []struct {
		name                    string
		templateVersion         string
		requiredWarpgateVersion string
		expectedCompatible      bool
		expectedWarnings        int
	}{
		{
			name:                    "compatible version",
			templateVersion:         "1.0.0",
			requiredWarpgateVersion: ">=1.0.0",
			expectedCompatible:      true,
			expectedWarnings:        0,
		},
		{
			name:                    "incompatible version",
			templateVersion:         "1.0.0",
			requiredWarpgateVersion: ">=2.0.0",
			expectedCompatible:      false,
			expectedWarnings:        1,
		},
		{
			name:                    "no required version",
			templateVersion:         "1.0.0",
			requiredWarpgateVersion: "",
			expectedCompatible:      true,
			expectedWarnings:        0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compatible, warnings, err := vm.CheckCompatibility(tc.templateVersion, tc.requiredWarpgateVersion)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if compatible != tc.expectedCompatible {
				t.Errorf("Expected compatible=%v, got %v", tc.expectedCompatible, compatible)
			}
			if len(warnings) != tc.expectedWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tc.expectedWarnings, len(warnings), warnings)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	tests := []struct {
		name        string
		v1          string
		v2          string
		expected    int
		expectError bool
	}{
		{
			name:        "v1 < v2",
			v1:          "1.0.0",
			v2:          "2.0.0",
			expected:    -1,
			expectError: false,
		},
		{
			name:        "v1 == v2",
			v1:          "1.0.0",
			v2:          "1.0.0",
			expected:    0,
			expectError: false,
		},
		{
			name:        "v1 > v2",
			v1:          "2.0.0",
			v2:          "1.0.0",
			expected:    1,
			expectError: false,
		},
		{
			name:        "both latest",
			v1:          "latest",
			v2:          "latest",
			expected:    0,
			expectError: false,
		},
		{
			name:        "v1 latest, v2 version",
			v1:          "latest",
			v2:          "1.0.0",
			expected:    1,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := vm.CompareVersions(tc.v1, tc.v2)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("Expected %d, got %d", tc.expected, result)
				}
			}
		})
	}
}

func TestIsBreakingChange(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	tests := []struct {
		name        string
		oldVersion  string
		newVersion  string
		expected    bool
		expectError bool
	}{
		{
			name:        "major version increase",
			oldVersion:  "1.0.0",
			newVersion:  "2.0.0",
			expected:    true,
			expectError: false,
		},
		{
			name:        "minor version increase",
			oldVersion:  "1.0.0",
			newVersion:  "1.1.0",
			expected:    false,
			expectError: false,
		},
		{
			name:        "patch version increase",
			oldVersion:  "1.0.0",
			newVersion:  "1.0.1",
			expected:    false,
			expectError: false,
		},
		{
			name:        "same version",
			oldVersion:  "1.0.0",
			newVersion:  "1.0.0",
			expected:    false,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := vm.IsBreakingChange(tc.oldVersion, tc.newVersion)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("Expected %v, got %v", tc.expected, result)
				}
			}
		})
	}
}

func TestGetLatestVersion(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	tests := []struct {
		name        string
		versions    []string
		expected    string
		expectError bool
	}{
		{
			name:        "single version",
			versions:    []string{"1.0.0"},
			expected:    "1.0.0",
			expectError: false,
		},
		{
			name:        "multiple versions",
			versions:    []string{"1.0.0", "2.0.0", "1.5.0"},
			expected:    "2.0.0",
			expectError: false,
		},
		{
			name:        "versions with v prefix",
			versions:    []string{"v1.0.0", "v2.0.0", "v1.5.0"},
			expected:    "v2.0.0",
			expectError: false,
		},
		{
			name:        "no versions",
			versions:    []string{},
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := vm.GetLatestVersion(tc.versions)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("Expected %s, got %s", tc.expected, result)
				}
			}
		})
	}
}

func TestValidateVersionRange(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	tests := []struct {
		name        string
		version     string
		minVersion  string
		maxVersion  string
		expected    bool
		expectError bool
	}{
		{
			name:        "within range",
			version:     "1.5.0",
			minVersion:  "1.0.0",
			maxVersion:  "2.0.0",
			expected:    true,
			expectError: false,
		},
		{
			name:        "below minimum",
			version:     "0.5.0",
			minVersion:  "1.0.0",
			maxVersion:  "2.0.0",
			expected:    false,
			expectError: false,
		},
		{
			name:        "above maximum",
			version:     "2.5.0",
			minVersion:  "1.0.0",
			maxVersion:  "2.0.0",
			expected:    false,
			expectError: false,
		},
		{
			name:        "latest always in range",
			version:     "latest",
			minVersion:  "1.0.0",
			maxVersion:  "2.0.0",
			expected:    true,
			expectError: false,
		},
		{
			name:        "no max version",
			version:     "3.0.0",
			minVersion:  "1.0.0",
			maxVersion:  "",
			expected:    true,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := vm.ValidateVersionRange(tc.version, tc.minVersion, tc.maxVersion)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("Expected %v, got %v", tc.expected, result)
				}
			}
		})
	}
}
