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

// Tests for generic utility functions

func TestSortBy(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  []int
	}{
		{
			name:  "sort ascending",
			input: []int{3, 1, 4, 1, 5, 9},
			want:  []int{1, 1, 3, 4, 5, 9},
		},
		{
			name:  "already sorted",
			input: []int{1, 2, 3},
			want:  []int{1, 2, 3},
		},
		{
			name:  "empty slice",
			input: []int{},
			want:  []int{},
		},
		{
			name:  "single element",
			input: []int{42},
			want:  []int{42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]int, len(tt.input))
			copy(input, tt.input)
			got := SortBy(input, func(a, b int) bool { return a < b })
			if len(got) != len(tt.want) {
				t.Errorf("SortBy() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SortBy()[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      []int
	}{
		{
			name:      "filter even numbers",
			input:     []int{1, 2, 3, 4, 5, 6},
			predicate: func(n int) bool { return n%2 == 0 },
			want:      []int{2, 4, 6},
		},
		{
			name:      "filter greater than 3",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(n int) bool { return n > 3 },
			want:      []int{4, 5},
		},
		{
			name:      "filter none match",
			input:     []int{1, 2, 3},
			predicate: func(n int) bool { return n > 10 },
			want:      []int{},
		},
		{
			name:      "filter empty slice",
			input:     []int{},
			predicate: func(n int) bool { return true },
			want:      []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(tt.input, tt.predicate)
			if len(got) != len(tt.want) {
				t.Errorf("Filter() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Filter()[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMap(t *testing.T) {
	t.Run("map int to string", func(t *testing.T) {
		input := []int{1, 2, 3}
		got := Map(input, func(n int) string { return string(rune('A' + n - 1)) })
		want := []string{"A", "B", "C"}
		if len(got) != len(want) {
			t.Errorf("Map() length = %d, want %d", len(got), len(want))
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("Map()[%d] = %s, want %s", i, got[i], want[i])
			}
		}
	})

	t.Run("map double", func(t *testing.T) {
		input := []int{1, 2, 3}
		got := Map(input, func(n int) int { return n * 2 })
		want := []int{2, 4, 6}
		if len(got) != len(want) {
			t.Errorf("Map() length = %d, want %d", len(got), len(want))
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("Map()[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	})

	t.Run("map empty slice", func(t *testing.T) {
		input := []int{}
		got := Map(input, func(n int) int { return n * 2 })
		if len(got) != 0 {
			t.Errorf("Map() of empty slice should return empty slice, got %v", got)
		}
	})
}

func TestContains(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      bool
	}{
		{
			name:      "contains element",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(n int) bool { return n == 3 },
			want:      true,
		},
		{
			name:      "does not contain element",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(n int) bool { return n == 10 },
			want:      false,
		},
		{
			name:      "empty slice",
			input:     []int{},
			predicate: func(n int) bool { return true },
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Contains(tt.input, tt.predicate)
			if got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirst(t *testing.T) {
	t.Run("find first even", func(t *testing.T) {
		input := []int{1, 3, 4, 5, 6}
		got, found := First(input, func(n int) bool { return n%2 == 0 })
		if !found {
			t.Errorf("First() found = false, want true")
			return
		}
		if got != 4 {
			t.Errorf("First() = %d, want 4", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		input := []int{1, 3, 5}
		_, found := First(input, func(n int) bool { return n%2 == 0 })
		if found {
			t.Errorf("First() found = true, want false")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []int{}
		_, found := First(input, func(n int) bool { return true })
		if found {
			t.Errorf("First() on empty slice found = true, want false")
		}
	})
}

func TestUnique(t *testing.T) {
	t.Run("remove duplicates", func(t *testing.T) {
		input := []int{1, 2, 2, 3, 3, 3, 4}
		got := Unique(input, func(n int) int { return n })
		want := []int{1, 2, 3, 4}
		if len(got) != len(want) {
			t.Errorf("Unique() length = %d, want %d", len(got), len(want))
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("Unique()[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		input := []int{1, 2, 3}
		got := Unique(input, func(n int) int { return n })
		if len(got) != 3 {
			t.Errorf("Unique() length = %d, want 3", len(got))
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []int{}
		got := Unique(input, func(n int) int { return n })
		if len(got) != 0 {
			t.Errorf("Unique() of empty slice should return empty slice")
		}
	})
}

func TestGroupBy(t *testing.T) {
	t.Run("group by modulo", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5, 6}
		got := GroupBy(input, func(n int) int { return n % 2 })

		if len(got[0]) != 3 {
			t.Errorf("GroupBy() evens count = %d, want 3", len(got[0]))
		}
		if len(got[1]) != 3 {
			t.Errorf("GroupBy() odds count = %d, want 3", len(got[1]))
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []int{}
		got := GroupBy(input, func(n int) int { return n })
		if len(got) != 0 {
			t.Errorf("GroupBy() of empty slice should return empty map")
		}
	})
}
