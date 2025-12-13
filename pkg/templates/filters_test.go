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

func TestBySource(t *testing.T) {
	filter := NewFilter()

	templates := []TemplateInfo{
		{
			Name:       "template1",
			Repository: "local:/path/to/templates",
		},
		{
			Name:       "template2",
			Repository: "official",
		},
		{
			Name:       "template3",
			Repository: "local:/another/path",
		},
		{
			Name:       "template4",
			Repository: "custom-repo",
		},
	}

	tests := []struct {
		name   string
		source string
		want   int
	}{
		{
			name:   "filter by 'all' returns all templates",
			source: "all",
			want:   4,
		},
		{
			name:   "filter by empty string returns all templates",
			source: "",
			want:   4,
		},
		{
			name:   "filter by 'local' returns local templates",
			source: "local",
			want:   2,
		},
		{
			name:   "filter by 'git' returns non-local templates",
			source: "git",
			want:   2,
		},
		{
			name:   "filter by specific repo name",
			source: "official",
			want:   1,
		},
		{
			name:   "filter by non-existent repo",
			source: "nonexistent",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.BySource(templates, tt.source)
			if len(got) != tt.want {
				t.Errorf("BySource() got %d templates, want %d", len(got), tt.want)
			}
		})
	}
}

func TestBySource_LocalMatching(t *testing.T) {
	filter := NewFilter()

	templates := []TemplateInfo{
		{
			Name:       "local-template",
			Repository: "local:/path/to/templates",
		},
		{
			Name:       "git-template",
			Repository: "https://git.example.com/jdoe/templates.git",
		},
	}

	got := filter.BySource(templates, "local")

	if len(got) != 1 {
		t.Errorf("BySource('local') got %d templates, want 1", len(got))
	}

	if len(got) > 0 && got[0].Name != "local-template" {
		t.Errorf("BySource('local') got template name %s, want 'local-template'", got[0].Name)
	}
}

func TestByName(t *testing.T) {
	filter := NewFilter()

	templates := []TemplateInfo{
		{Name: "attack-box"},
		{Name: "sliver"},
		{Name: "atomic-red-team"},
		{Name: "custom-attack"},
	}

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "partial match case insensitive",
			query: "attack",
			want:  []string{"attack-box", "custom-attack"},
		},
		{
			name:  "exact match",
			query: "sliver",
			want:  []string{"sliver"},
		},
		{
			name:  "empty query returns all",
			query: "",
			want:  []string{"attack-box", "sliver", "atomic-red-team", "custom-attack"},
		},
		{
			name:  "no matches",
			query: "nonexistent",
			want:  []string{},
		},
		{
			name:  "case insensitive search",
			query: "SLIVER",
			want:  []string{"sliver"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.ByName(templates, tt.query)
			if len(got) != len(tt.want) {
				t.Errorf("ByName() got %d templates, want %d", len(got), len(tt.want))
				return
			}
			for i, wantName := range tt.want {
				if got[i].Name != wantName {
					t.Errorf("ByName() template[%d] = %s, want %s", i, got[i].Name, wantName)
				}
			}
		})
	}
}

func TestByTag(t *testing.T) {
	filter := NewFilter()

	templates := []TemplateInfo{
		{
			Name: "template1",
			Tags: []string{"security", "offensive"},
		},
		{
			Name: "template2",
			Tags: []string{"security", "defensive"},
		},
		{
			Name: "template3",
			Tags: []string{"dev", "testing"},
		},
		{
			Name: "template4",
			Tags: []string{},
		},
	}

	tests := []struct {
		name string
		tag  string
		want []string
	}{
		{
			name: "filter by security tag",
			tag:  "security",
			want: []string{"template1", "template2"},
		},
		{
			name: "filter by offensive tag",
			tag:  "offensive",
			want: []string{"template1"},
		},
		{
			name: "filter by dev tag",
			tag:  "dev",
			want: []string{"template3"},
		},
		{
			name: "empty tag returns all",
			tag:  "",
			want: []string{"template1", "template2", "template3", "template4"},
		},
		{
			name: "no matching tag",
			tag:  "nonexistent",
			want: []string{},
		},
		{
			name: "case insensitive tag match",
			tag:  "SECURITY",
			want: []string{"template1", "template2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.ByTag(templates, tt.tag)
			if len(got) != len(tt.want) {
				t.Errorf("ByTag() got %d templates, want %d", len(got), len(tt.want))
				return
			}
			for i, wantName := range tt.want {
				if got[i].Name != wantName {
					t.Errorf("ByTag() template[%d] = %s, want %s", i, got[i].Name, wantName)
				}
			}
		})
	}
}

func TestByTag_NoTags(t *testing.T) {
	filter := NewFilter()

	templates := []TemplateInfo{
		{
			Name: "template1",
			Tags: []string{},
		},
		{
			Name: "template2",
			Tags: nil,
		},
	}

	got := filter.ByTag(templates, "security")

	if len(got) != 0 {
		t.Errorf("ByTag() with templates without tags got %d templates, want 0", len(got))
	}
}

func TestFilterChaining(t *testing.T) {
	filter := NewFilter()

	templates := []TemplateInfo{
		{
			Name:       "attack-box",
			Repository: "local:/path/to/templates",
			Tags:       []string{"security", "offensive"},
		},
		{
			Name:       "sliver",
			Repository: "official",
			Tags:       []string{"security", "c2"},
		},
		{
			Name:       "custom-attack",
			Repository: "local:/another/path",
			Tags:       []string{"custom"},
		},
	}

	// Chain filters: local source -> security tag -> name contains "attack"
	result := filter.BySource(templates, "local")
	result = filter.ByTag(result, "security")
	result = filter.ByName(result, "attack")

	if len(result) != 1 {
		t.Errorf("Filter chain got %d templates, want 1", len(result))
	}

	if len(result) > 0 && result[0].Name != "attack-box" {
		t.Errorf("Filter chain got template %s, want 'attack-box'", result[0].Name)
	}
}

func TestBySource_SpecificRepoWithColon(t *testing.T) {
	filter := NewFilter()

	templates := []TemplateInfo{
		{
			Name:       "template1",
			Repository: "local:myrepo",
		},
		{
			Name:       "template2",
			Repository: "local:otherrepo",
		},
	}

	// Test filtering by specific local repo name
	got := filter.BySource(templates, "myrepo")

	if len(got) != 1 {
		t.Errorf("BySource('myrepo') got %d templates, want 1", len(got))
	}

	if len(got) > 0 && got[0].Name != "template1" {
		t.Errorf("BySource('myrepo') got template %s, want 'template1'", got[0].Name)
	}
}
