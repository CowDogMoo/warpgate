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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
)

func TestCompareSemanticVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   *string
		v2   *string
		want int // positive if v1 > v2, negative if v1 < v2, 0 if equal
	}{
		{
			name: "both nil",
			v1:   nil,
			v2:   nil,
			want: 0,
		},
		{
			name: "v1 nil",
			v1:   nil,
			v2:   aws.String("1.0.0"),
			want: -1,
		},
		{
			name: "v2 nil",
			v1:   aws.String("1.0.0"),
			v2:   nil,
			want: 1,
		},
		{
			name: "equal versions",
			v1:   aws.String("1.0.0"),
			v2:   aws.String("1.0.0"),
			want: 0,
		},
		{
			name: "v1 greater major",
			v1:   aws.String("2.0.0"),
			v2:   aws.String("1.0.0"),
			want: 1,
		},
		{
			name: "v1 lesser major",
			v1:   aws.String("1.0.0"),
			v2:   aws.String("2.0.0"),
			want: -1,
		},
		{
			name: "v1 greater minor",
			v1:   aws.String("1.2.0"),
			v2:   aws.String("1.1.0"),
			want: 1,
		},
		{
			name: "v1 greater patch",
			v1:   aws.String("1.0.2"),
			v2:   aws.String("1.0.1"),
			want: 1,
		},
		{
			name: "different lengths",
			v1:   aws.String("1.0"),
			v2:   aws.String("1.0.0"),
			want: 0,
		},
		{
			name: "different lengths with difference",
			v1:   aws.String("1.0"),
			v2:   aws.String("1.0.1"),
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSemanticVersions(tt.v1, tt.v2)
			switch {
			case tt.want > 0 && got <= 0:
				t.Errorf("CompareSemanticVersions() = %v, want positive", got)
			case tt.want < 0 && got >= 0:
				t.Errorf("CompareSemanticVersions() = %v, want negative", got)
			case tt.want == 0 && got != 0:
				t.Errorf("CompareSemanticVersions() = %v, want 0", got)
			}
		})
	}
}

func TestSortComponentVersions(t *testing.T) {
	tests := []struct {
		name     string
		versions []types.ComponentVersion
		want     []string // expected version order (descending)
	}{
		{
			name:     "empty list",
			versions: []types.ComponentVersion{},
			want:     []string{},
		},
		{
			name: "single version",
			versions: []types.ComponentVersion{
				{Version: aws.String("1.0.0")},
			},
			want: []string{"1.0.0"},
		},
		{
			name: "already sorted",
			versions: []types.ComponentVersion{
				{Version: aws.String("2.0.0")},
				{Version: aws.String("1.0.0")},
			},
			want: []string{"2.0.0", "1.0.0"},
		},
		{
			name: "reverse order",
			versions: []types.ComponentVersion{
				{Version: aws.String("1.0.0")},
				{Version: aws.String("2.0.0")},
			},
			want: []string{"2.0.0", "1.0.0"},
		},
		{
			name: "multiple versions",
			versions: []types.ComponentVersion{
				{Version: aws.String("1.2.0")},
				{Version: aws.String("2.0.0")},
				{Version: aws.String("1.0.5")},
				{Version: aws.String("1.0.0")},
			},
			want: []string{"2.0.0", "1.2.0", "1.0.5", "1.0.0"},
		},
		{
			name: "with nil version",
			versions: []types.ComponentVersion{
				{Version: aws.String("2.0.0")},
				{Version: nil},
				{Version: aws.String("1.0.0")},
			},
			want: []string{"2.0.0", "1.0.0", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted := SortComponentVersions(tt.versions)

			if len(sorted) != len(tt.want) {
				t.Errorf("SortComponentVersions() returned %d items, want %d", len(sorted), len(tt.want))
				return
			}

			for i, v := range sorted {
				got := ""
				if v.Version != nil {
					got = *v.Version
				}
				if got != tt.want[i] {
					t.Errorf("SortComponentVersions()[%d] = %v, want %v", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestParseSemanticVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{
			name:      "empty version",
			version:   "",
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
		},
		{
			name:      "full version",
			version:   "1.2.3",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "major only",
			version:   "5",
			wantMajor: 5,
			wantMinor: 0,
			wantPatch: 0,
		},
		{
			name:      "major.minor only",
			version:   "3.4",
			wantMajor: 3,
			wantMinor: 4,
			wantPatch: 0,
		},
		{
			name:      "double digits",
			version:   "10.20.30",
			wantMajor: 10,
			wantMinor: 20,
			wantPatch: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch := ParseSemanticVersion(tt.version)
			if major != tt.wantMajor {
				t.Errorf("ParseSemanticVersion() major = %v, want %v", major, tt.wantMajor)
			}
			if minor != tt.wantMinor {
				t.Errorf("ParseSemanticVersion() minor = %v, want %v", minor, tt.wantMinor)
			}
			if patch != tt.wantPatch {
				t.Errorf("ParseSemanticVersion() patch = %v, want %v", patch, tt.wantPatch)
			}
		})
	}
}

func TestFormatSemanticVersion(t *testing.T) {
	tests := []struct {
		name  string
		major int
		minor int
		patch int
		want  string
	}{
		{
			name:  "zero version",
			major: 0,
			minor: 0,
			patch: 0,
			want:  "0.0.0",
		},
		{
			name:  "typical version",
			major: 1,
			minor: 2,
			patch: 3,
			want:  "1.2.3",
		},
		{
			name:  "double digits",
			major: 10,
			minor: 20,
			patch: 30,
			want:  "10.20.30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSemanticVersion(tt.major, tt.minor, tt.patch)
			if got != tt.want {
				t.Errorf("FormatSemanticVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIncrementPatchVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "increment patch",
			version: "1.0.0",
			want:    "1.0.1",
		},
		{
			name:    "increment high patch",
			version: "1.0.9",
			want:    "1.0.10",
		},
		{
			name:    "empty version",
			version: "",
			want:    "0.0.1",
		},
		{
			name:    "major.minor only",
			version: "2.5",
			want:    "2.5.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IncrementPatchVersion(tt.version)
			if got != tt.want {
				t.Errorf("IncrementPatchVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSemanticVersion_MalformedInput(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{
			name:      "non-numeric major",
			version:   "abc.1.2",
			wantMajor: 0,
			wantMinor: 1,
			wantPatch: 2,
		},
		{
			name:      "non-numeric minor",
			version:   "1.xyz.2",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 2,
		},
		{
			name:      "non-numeric patch",
			version:   "1.2.abc",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 0,
		},
		{
			name:      "extra components",
			version:   "1.2.3.4.5",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "spaces in version",
			version:   " 1 . 2 . 3 ",
			wantMajor: 1, // Sscanf handles leading spaces
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "leading zeros",
			version:   "01.02.03",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "mixed content",
			version:   "1a.2b.3c",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "negative-like (dash prefix)",
			version:   "-1.-2.-3",
			wantMajor: -1, // Sscanf parses negative numbers
			wantMinor: -2,
			wantPatch: -3,
		},
		{
			name:      "very large numbers",
			version:   "999999.888888.777777",
			wantMajor: 999999,
			wantMinor: 888888,
			wantPatch: 777777,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch := ParseSemanticVersion(tt.version)
			if major != tt.wantMajor {
				t.Errorf("ParseSemanticVersion() major = %v, want %v", major, tt.wantMajor)
			}
			if minor != tt.wantMinor {
				t.Errorf("ParseSemanticVersion() minor = %v, want %v", minor, tt.wantMinor)
			}
			if patch != tt.wantPatch {
				t.Errorf("ParseSemanticVersion() patch = %v, want %v", patch, tt.wantPatch)
			}
		})
	}
}

func TestSortComponentVersions_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		versions []types.ComponentVersion
		want     []string
	}{
		{
			name: "all nil versions",
			versions: []types.ComponentVersion{
				{Version: nil},
				{Version: nil},
				{Version: nil},
			},
			want: []string{"", "", ""},
		},
		{
			name: "same versions",
			versions: []types.ComponentVersion{
				{Version: aws.String("1.0.0")},
				{Version: aws.String("1.0.0")},
				{Version: aws.String("1.0.0")},
			},
			want: []string{"1.0.0", "1.0.0", "1.0.0"},
		},
		{
			name: "mixed nil and values",
			versions: []types.ComponentVersion{
				{Version: nil},
				{Version: aws.String("1.0.0")},
				{Version: nil},
				{Version: aws.String("2.0.0")},
			},
			want: []string{"2.0.0", "1.0.0", "", ""},
		},
		{
			name: "versions with different component counts",
			versions: []types.ComponentVersion{
				{Version: aws.String("1")},
				{Version: aws.String("1.0")},
				{Version: aws.String("1.0.0")},
				{Version: aws.String("1.0.0.0")},
			},
			// All these are semantically equal (1.0.0.0 = 1), so order is preserved (stable sort)
			want: []string{"1", "1.0", "1.0.0", "1.0.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted := SortComponentVersions(tt.versions)

			if len(sorted) != len(tt.want) {
				t.Errorf("SortComponentVersions() returned %d items, want %d", len(sorted), len(tt.want))
				return
			}

			for i, v := range sorted {
				got := ""
				if v.Version != nil {
					got = *v.Version
				}
				if got != tt.want[i] {
					t.Errorf("SortComponentVersions()[%d] = %v, want %v", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestCompareSemanticVersions_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		v1   *string
		v2   *string
		want int
	}{
		{
			name: "empty strings",
			v1:   aws.String(""),
			v2:   aws.String(""),
			want: 0,
		},
		{
			name: "one empty string",
			v1:   aws.String("1.0.0"),
			v2:   aws.String(""),
			want: 1,
		},
		{
			name: "very different lengths",
			v1:   aws.String("1.0.0.0.0.0"),
			v2:   aws.String("1"),
			want: 0,
		},
		{
			name: "large version numbers",
			v1:   aws.String("100.200.300"),
			v2:   aws.String("1.2.3"),
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSemanticVersions(tt.v1, tt.v2)
			switch {
			case tt.want > 0 && got <= 0:
				t.Errorf("CompareSemanticVersions() = %v, want positive", got)
			case tt.want < 0 && got >= 0:
				t.Errorf("CompareSemanticVersions() = %v, want negative", got)
			case tt.want == 0 && got != 0:
				t.Errorf("CompareSemanticVersions() = %v, want 0", got)
			}
		})
	}
}

func TestNormalizeSemanticVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "empty string",
			version: "",
			want:    "1.0.0",
		},
		{
			name:    "latest lowercase",
			version: "latest",
			want:    "1.0.0",
		},
		{
			name:    "LATEST uppercase",
			version: "LATEST",
			want:    "1.0.0",
		},
		{
			name:    "Latest mixed case",
			version: "Latest",
			want:    "1.0.0",
		},
		{
			name:    "valid version",
			version: "1.2.3",
			want:    "1.2.3",
		},
		{
			name:    "version with leading zeros",
			version: "01.02.03",
			want:    "1.2.3",
		},
		{
			name:    "major only",
			version: "5",
			want:    "5.0.0",
		},
		{
			name:    "major.minor only",
			version: "2.3",
			want:    "2.3.0",
		},
		{
			name:    "zero version",
			version: "0.0.0",
			want:    "0.0.0",
		},
		{
			name:    "invalid string",
			version: "invalid",
			want:    "1.0.0",
		},
		{
			name:    "version with extra components",
			version: "1.2.3.4",
			want:    "1.2.3",
		},
		{
			name:    "version with v prefix",
			version: "v1.2.3",
			want:    "0.2.3", // v prefix makes major parse as 0, but minor/patch parse correctly
		},
		{
			name:    "whitespace only",
			version: "   ",
			want:    "1.0.0", // Parses to 0.0.0 but not valid "0.0.0" format
		},
		{
			name:    "double digit version",
			version: "10.20.30",
			want:    "10.20.30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSemanticVersion(tt.version)
			if got != tt.want {
				t.Errorf("NormalizeSemanticVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
