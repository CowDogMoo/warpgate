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
	"reflect"
	"testing"
)

func TestImageRef(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		tag  string
		want string
	}{
		{
			name: "no registry leaves the name bare",
			cfg:  Config{Name: "mealie"},
			tag:  "latest",
			want: "mealie:latest",
		},
		{
			name: "registry is prefixed whole",
			cfg:  Config{Name: "mealie", Registry: "ghcr.io/cowdogmoo"},
			tag:  "v3.24.0",
			want: "ghcr.io/cowdogmoo/mealie:v3.24.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ImageRef(tt.cfg, tt.tag); got != tt.want {
				t.Errorf("ImageRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrimaryImageRef(t *testing.T) {
	cfg := Config{Name: "mealie", Version: "v3.24.0", Registry: "ghcr.io/cowdogmoo"}

	if got, want := PrimaryImageRef(cfg), "ghcr.io/cowdogmoo/mealie:v3.24.0"; got != want {
		t.Errorf("PrimaryImageRef() = %q, want %q", got, want)
	}
}

func TestAdditionalTagRefs(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want []string
	}{
		{
			name: "no targets",
			cfg:  Config{Name: "mealie", Version: "latest"},
		},
		{
			name: "container target without tags",
			cfg: Config{
				Name: "mealie", Version: "latest",
				Targets: []Target{{Type: "container"}},
			},
		},
		{
			name: "a tag equal to the version is already carried",
			cfg: Config{
				Name: "mealie", Version: "latest",
				Targets: []Target{{Type: "container", Tags: []string{"latest"}}},
			},
		},
		{
			name: "extra tags become references in declaration order",
			cfg: Config{
				Name: "mealie", Version: "latest", Registry: "ghcr.io/cowdogmoo",
				Targets: []Target{{Type: "container", Tags: []string{"latest", "v3.24.0", "stable"}}},
			},
			want: []string{"ghcr.io/cowdogmoo/mealie:v3.24.0", "ghcr.io/cowdogmoo/mealie:stable"},
		},
		{
			name: "repeated and empty tags are dropped",
			cfg: Config{
				Name: "mealie", Version: "latest",
				Targets: []Target{{Type: "container", Tags: []string{"stable", "", "stable"}}},
			},
			want: []string{"mealie:stable"},
		},
		{
			name: "non-container target tags are not image tags",
			cfg: Config{
				Name: "mealie", Version: "latest",
				Targets: []Target{{Type: "ami", Tags: []string{"v3.24.0"}}},
			},
		},
		{
			name: "tags from every container target are collected",
			cfg: Config{
				Name: "mealie", Version: "latest",
				Targets: []Target{
					{Type: "container", Tags: []string{"stable"}},
					{Type: "ami", Tags: []string{"ignored"}},
					{Type: "container", Tags: []string{"edge", "stable"}},
				},
			},
			want: []string{"mealie:stable", "mealie:edge"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AdditionalTagRefs(tt.cfg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AdditionalTagRefs() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAdditionalTagRefsDedupesRequestedTags checks the two tag sources merge into
// one list of distinct references: a requested tag repeating the version, an
// empty value, a repeat of itself, or a tag the target already declares must not
// produce a second reference to the same image.
func TestAdditionalTagRefsDedupesRequestedTags(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Name:      "mealie",
		Version:   "v9.9.9",
		Registry:  "ghcr.io/cowdogmoo",
		ExtraTags: []string{"stable", "v9.9.9", "", "v9.9", "v9.9"},
		Targets:   []Target{{Type: "container", Tags: []string{"stable", "latest"}}},
	}

	// Requested tags come first, then the declared ones the request did not
	// already cover.
	want := []string{
		"ghcr.io/cowdogmoo/mealie:stable",
		"ghcr.io/cowdogmoo/mealie:v9.9",
		"ghcr.io/cowdogmoo/mealie:latest",
	}
	if got := AdditionalTagRefs(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("AdditionalTagRefs() = %v, want %v", got, want)
	}
}

// TestAdditionalTagRefsSkipsReleaseTagsForComponents checks a per-architecture
// component build carries neither the declared tags nor the requested ones.
func TestAdditionalTagRefsSkipsReleaseTagsForComponents(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Name:            "mealie",
		Version:         "amd64",
		Registry:        "ghcr.io/cowdogmoo",
		ExtraTags:       []string{"v9.9.9"},
		Targets:         []Target{{Type: "container", Tags: []string{"stable"}}},
		SkipReleaseTags: true,
	}

	if got := AdditionalTagRefs(cfg); got != nil {
		t.Errorf("AdditionalTagRefs() = %v, want none for a component build", got)
	}
}
