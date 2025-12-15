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

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cowdogmoo/warpgate/v3/pkg/builder"
	"github.com/cowdogmoo/warpgate/v3/pkg/logging"
	"github.com/cowdogmoo/warpgate/v3/pkg/templates"
)

// OutputFormatter formats command output for display.
type OutputFormatter struct {
	format string // text, json, table
}

// NewOutputFormatter creates a new output formatter with the specified format.
func NewOutputFormatter(format string) *OutputFormatter {
	return &OutputFormatter{
		format: format,
	}
}

// DisplayBuildResult displays build results to the user.
func (f *OutputFormatter) DisplayBuildResult(ctx context.Context, result *builder.BuildResult) {
	logging.InfoContext(ctx, "Build completed successfully!")

	if result.ImageRef != "" {
		logging.InfoContext(ctx, "Image: %s", result.ImageRef)
	}

	if result.AMIID != "" {
		logging.InfoContext(ctx, "AMI ID: %s", result.AMIID)
		logging.InfoContext(ctx, "Region: %s", result.Region)
	}

	logging.InfoContext(ctx, "Duration: %s", result.Duration)

	for _, note := range result.Notes {
		logging.InfoContext(ctx, "Note: %s", note)
	}
}

// DisplayBuildResults displays multiple build results (multi-arch builds).
func (f *OutputFormatter) DisplayBuildResults(ctx context.Context, results []builder.BuildResult) {
	for i, result := range results {
		if i > 0 {
			fmt.Println() // Blank line between results
		}
		f.DisplayBuildResult(ctx, &result)
	}
}

// DisplayTemplateList displays a list of templates in the specified format.
func (f *OutputFormatter) DisplayTemplateList(templateList []templates.TemplateInfo) error {
	switch f.format {
	case "json":
		return f.displayTemplatesJSON(templateList)
	case "gha-matrix":
		return f.displayTemplatesGHAMatrix(templateList)
	case "table", "text", "":
		return f.displayTemplatesTable(templateList)
	default:
		return fmt.Errorf("unknown format: %s (supported: table, json, gha-matrix)", f.format)
	}
}

// displayTemplatesTable outputs templates in a formatted table.
func (f *OutputFormatter) displayTemplatesTable(templateList []templates.TemplateInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tVERSION\tSOURCE\tDESCRIPTION\tAUTHOR"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintln(w, "----\t-------\t------\t-----------\t------"); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	for _, tmpl := range templateList {
		version := tmpl.Version
		if version == "" {
			version = "latest"
		}
		author := tmpl.Author
		if author == "" {
			author = "unknown"
		}
		source := tmpl.Repository
		if source == "" {
			source = "unknown"
		}
		description := tmpl.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", tmpl.Name, version, source, description, author); err != nil {
			return fmt.Errorf("failed to write template row: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}
	fmt.Printf("\nTotal templates: %d\n", len(templateList))
	return nil
}

// displayTemplatesJSON outputs templates as JSON.
func (f *OutputFormatter) displayTemplatesJSON(templateList []templates.TemplateInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(templateList)
}

// GHATemplateEntry represents a template entry in GitHub Actions matrix format.
type GHATemplateEntry struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Vars        string `json:"vars,omitempty"`
}

// GHAMatrix represents the GitHub Actions matrix structure.
type GHAMatrix struct {
	Template []GHATemplateEntry `json:"template"`
}

// displayTemplatesGHAMatrix outputs templates in GitHub Actions matrix format.
func (f *OutputFormatter) displayTemplatesGHAMatrix(templateList []templates.TemplateInfo) error {
	matrix := GHAMatrix{
		Template: make([]GHATemplateEntry, 0, len(templateList)),
	}

	for _, tmpl := range templateList {
		entry := GHATemplateEntry{
			Name:        tmpl.Name,
			Namespace:   extractNamespace(tmpl.Repository),
			Version:     tmpl.Version,
			Description: tmpl.Description,
		}

		// Add common variables that templates might need
		if needsProvisionRepo(tmpl.Name) {
			entry.Vars = "PROVISION_REPO_PATH=$HOME/ansible-collection-arsenal"
		}

		matrix.Template = append(matrix.Template, entry)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(matrix)
}

// extractNamespace extracts the namespace from a repository source.
func extractNamespace(repository string) string {
	// For local sources, extract the repository name
	if strings.HasPrefix(repository, "local:") {
		parts := strings.Split(repository, ":")
		if len(parts) > 1 {
			return parts[1]
		}
	}

	// For git repositories, default namespace
	return "cowdogmoo"
}

// needsProvisionRepo checks if a template needs ansible provisioning repo.
func needsProvisionRepo(templateName string) bool {
	provisionTemplates := map[string]bool{
		"attack-box":      true,
		"atomic-red-team": true,
		"sliver":          true,
		"ttpforge":        true,
	}
	return provisionTemplates[templateName]
}

// DisplayTemplateSearchResults displays search results in table format.
func (f *OutputFormatter) DisplayTemplateSearchResults(results []templates.TemplateInfo, query string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tVERSION\tREPOSITORY\tDESCRIPTION"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintln(w, "----\t-------\t----------\t-----------"); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	for _, tmpl := range results {
		version := tmpl.Version
		if version == "" {
			version = "latest"
		}
		repo := tmpl.Repository
		if repo == "" {
			repo = "official"
		}
		description := tmpl.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tmpl.Name, version, repo, description); err != nil {
			return fmt.Errorf("failed to write template row: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}
	fmt.Printf("\nFound %d template(s) matching query: %s\n", len(results), query)
	return nil
}
