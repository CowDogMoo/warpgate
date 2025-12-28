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
	"testing"
)

func TestParsePlatform(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PlatformInfo
	}{
		{
			name:  "full platform with variant",
			input: "linux/arm64/v8",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
			},
		},
		{
			name:  "platform without variant",
			input: "linux/amd64",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "amd64",
				Variant:      "",
			},
		},
		{
			name:  "architecture only",
			input: "amd64",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "amd64",
				Variant:      "",
			},
		},
		{
			name:  "architecture with variant (no OS)",
			input: "arm/v7",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "arm",
				Variant:      "v7",
			},
		},
		{
			name:  "windows platform",
			input: "windows/amd64",
			expected: PlatformInfo{
				OS:           "windows",
				Architecture: "amd64",
				Variant:      "",
			},
		},
		{
			name:  "darwin platform",
			input: "darwin/arm64",
			expected: PlatformInfo{
				OS:           "darwin",
				Architecture: "arm64",
				Variant:      "",
			},
		},
		{
			name:  "arm architecture only",
			input: "arm",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "arm",
				Variant:      "",
			},
		},
		{
			name:  "arm64 architecture only",
			input: "arm64",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "",
			},
		},
		{
			name:  "386 architecture",
			input: "linux/386",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "386",
				Variant:      "",
			},
		},
		{
			name:  "ppc64le architecture",
			input: "linux/ppc64le",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "ppc64le",
				Variant:      "",
			},
		},
		{
			name:  "s390x architecture",
			input: "linux/s390x",
			expected: PlatformInfo{
				OS:           "linux",
				Architecture: "s390x",
				Variant:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePlatform(tt.input)
			if result.OS != tt.expected.OS {
				t.Errorf("OS: got %q, want %q", result.OS, tt.expected.OS)
			}
			if result.Architecture != tt.expected.Architecture {
				t.Errorf("Architecture: got %q, want %q", result.Architecture, tt.expected.Architecture)
			}
			if result.Variant != tt.expected.Variant {
				t.Errorf("Variant: got %q, want %q", result.Variant, tt.expected.Variant)
			}
		})
	}
}

func TestFormatPlatform(t *testing.T) {
	tests := []struct {
		name     string
		input    PlatformInfo
		expected string
	}{
		{
			name: "with variant",
			input: PlatformInfo{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "v8",
			},
			expected: "linux/arm64/v8",
		},
		{
			name: "without variant",
			input: PlatformInfo{
				OS:           "linux",
				Architecture: "amd64",
				Variant:      "",
			},
			expected: "linux/amd64",
		},
		{
			name: "windows platform",
			input: PlatformInfo{
				OS:           "windows",
				Architecture: "amd64",
				Variant:      "",
			},
			expected: "windows/amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPlatform(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIsLikelyOS(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"linux", true},
		{"windows", true},
		{"darwin", true},
		{"freebsd", true},
		{"Linux", true}, // Case insensitive
		{"WINDOWS", true},
		{"amd64", false},
		{"arm64", false},
		{"arm", false},
		{"386", false},
		{"ppc64le", false},
		{"s390x", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isLikelyOS(tt.input)
			if result != tt.expected {
				t.Errorf("isLikelyOS(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
