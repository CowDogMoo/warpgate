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

import "strings"

// Filter provides template filtering capabilities.
type Filter struct{}

// NewFilter creates a new template filter.
func NewFilter() *Filter {
	return &Filter{}
}

// BySource filters templates by source type or name.
// Supported source types: "local", "git", "all", or a specific repository name.
func (f *Filter) BySource(templates []TemplateInfo, source string) []TemplateInfo {
	if source == "all" || source == "" {
		return templates
	}

	var filtered []TemplateInfo
	for _, tmpl := range templates {
		repoSource := tmpl.Repository

		switch source {
		case "local":
			// Match templates from local paths
			if strings.HasPrefix(repoSource, "local:") {
				filtered = append(filtered, tmpl)
			}
		case "git":
			// Match templates from git repositories (not local)
			if !strings.HasPrefix(repoSource, "local:") {
				filtered = append(filtered, tmpl)
			}
		default:
			// Match specific repository name
			if repoSource == source || strings.HasSuffix(repoSource, ":"+source) {
				filtered = append(filtered, tmpl)
			}
		}
	}

	return filtered
}

// ByName filters templates by name (case-insensitive partial match).
func (f *Filter) ByName(templates []TemplateInfo, query string) []TemplateInfo {
	if query == "" {
		return templates
	}

	query = strings.ToLower(query)
	var filtered []TemplateInfo

	for _, tmpl := range templates {
		if strings.Contains(strings.ToLower(tmpl.Name), query) {
			filtered = append(filtered, tmpl)
		}
	}

	return filtered
}

// ByTag filters templates by tag.
func (f *Filter) ByTag(templates []TemplateInfo, tag string) []TemplateInfo {
	if tag == "" {
		return templates
	}

	tag = strings.ToLower(tag)
	var filtered []TemplateInfo

	for _, tmpl := range templates {
		for _, t := range tmpl.Tags {
			if strings.ToLower(t) == tag {
				filtered = append(filtered, tmpl)
				break
			}
		}
	}

	return filtered
}
