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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/templates"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage warpgate templates",
	Long:  `List, search, inspect, and manage template sources.`,
}

var templatesAddCmd = &cobra.Command{
	Use:   "add [url-or-path] or add [name] [url]",
	Short: "Add a template source",
	Long: `Add a template source (git URL or local directory).

For git URLs:
  - Auto-generate name: warpgate templates add https://github.com/user/my-templates.git
  - Custom name: warpgate templates add my-custom-name https://github.com/user/my-templates.git

For local paths (name is not supported):
  - warpgate templates add /Users/user/my-templates
  - warpgate templates add ~/warpgate-templates
  - warpgate templates add ../templates

Examples:
  warpgate templates add https://github.com/user/my-templates.git
  warpgate templates add custom-name https://github.com/user/my-templates.git
  warpgate templates add /Users/user/my-templates
  warpgate templates add ~/warpgate-templates`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTemplatesAdd,
}

var templatesRemoveCmd = &cobra.Command{
	Use:   "remove [path-or-name]",
	Short: "Remove a template source",
	Long: `Remove a template source by path or repository name.

Examples:
  warpgate templates remove /Users/user/my-templates
  warpgate templates remove official`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplatesRemove,
}

var (
	templatesListFormat string
	templatesListSource string
)

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	Long: `List all available templates from configured sources.

Examples:
  # List all templates
  warpgate templates list

  # List only local templates
  warpgate templates list --source local

  # Output in GitHub Actions matrix format
  warpgate templates list --format gha-matrix

  # Combine filters
  warpgate templates list --source local --format gha-matrix`,
	RunE: runTemplatesList,
}

var templatesSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for templates",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplatesSearch,
}

var templatesInfoCmd = &cobra.Command{
	Use:   "info [template]",
	Short: "Show template information",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplatesInfo,
}

var templatesUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update template cache",
	Long:  `Update the local cache of templates from all configured repositories.`,
	RunE:  runTemplatesUpdate,
}

func init() {
	templatesCmd.AddCommand(templatesAddCmd)
	templatesCmd.AddCommand(templatesRemoveCmd)
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesSearchCmd)
	templatesCmd.AddCommand(templatesInfoCmd)
	templatesCmd.AddCommand(templatesUpdateCmd)

	// Add flags to templates list command
	templatesListCmd.Flags().StringVarP(&templatesListFormat, "format", "f", "table", "Output format (table, json, gha-matrix)")
	templatesListCmd.Flags().StringVarP(&templatesListSource, "source", "s", "all", "Filter by source (all, local, git, or specific repo name)")
}

func runTemplatesAdd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	name, urlOrPath, err := parseTemplatesAddArgs(args)
	if err != nil {
		return err
	}

	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("config not available in context")
	}

	if isGitURL(urlOrPath) {
		return addGitRepository(ctx, cfg, name, urlOrPath)
	}

	return addLocalPath(ctx, cfg, urlOrPath)
}

// parseTemplatesAddArgs parses and validates the arguments for templates add command
func parseTemplatesAddArgs(args []string) (name string, urlOrPath string, err error) {
	if len(args) == 2 {
		// Two args: [name] [url]
		name = args[0]
		urlOrPath = args[1]

		if !isGitURL(urlOrPath) {
			return "", "", fmt.Errorf("when providing a name, the second argument must be a git URL (not a local path)")
		}
	} else {
		// One arg: [url-or-path]
		urlOrPath = args[0]
	}
	return name, urlOrPath, nil
}

// isGitURL checks if a string is a git URL
func isGitURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@")
}

// addGitRepository adds a git repository to the configuration
func addGitRepository(ctx context.Context, cfg *globalconfig.Config, name, url string) error {
	// Initialize repositories map if nil
	if cfg.Templates.Repositories == nil {
		cfg.Templates.Repositories = make(map[string]string)
	}

	// Auto-generate name if not provided
	if name == "" {
		name = extractRepoName(url)
	}

	// Check if already exists
	if existing, ok := cfg.Templates.Repositories[name]; ok {
		if existing == url {
			logging.WarnContext(ctx, "Repository '%s' already exists", name)
			return nil
		}
		return fmt.Errorf("repository name '%s' already exists with different URL: %s", name, existing)
	}

	logging.InfoContext(ctx, "Adding git repository: %s -> %s", name, url)
	cfg.Templates.Repositories[name] = url

	if err := saveConfigValue("templates.repositories", cfg.Templates.Repositories); err != nil {
		return err
	}

	configPath, _ := globalconfig.ConfigFile("config.yaml")
	logging.InfoContext(ctx, "Repository added successfully as '%s' to %s", name, configPath)
	logging.InfoContext(ctx, "Run 'warpgate templates update' to fetch templates")
	return nil
}

// addLocalPath adds a local template directory to the configuration
func addLocalPath(ctx context.Context, cfg *globalconfig.Config, path string) error {
	// Expand home directory if needed
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	// Make path absolute if relative
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		path = absPath
	}

	// Validate path
	if err := validateLocalPath(path); err != nil {
		return err
	}

	logging.InfoContext(ctx, "Adding local template directory: %s", path)

	// Check if already exists
	for _, existingPath := range cfg.Templates.LocalPaths {
		if existingPath == path {
			logging.WarnContext(ctx, "Path already exists in local_paths")
			return nil
		}
	}

	// Add to local_paths
	cfg.Templates.LocalPaths = append(cfg.Templates.LocalPaths, path)

	if err := saveConfigValue("templates.local_paths", cfg.Templates.LocalPaths); err != nil {
		return err
	}

	configPath, _ := globalconfig.ConfigFile("config.yaml")
	logging.InfoContext(ctx, "Template directory added successfully to %s", configPath)
	return nil
}

// validateLocalPath checks if a path exists and is a directory
func validateLocalPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}

// saveConfigValue saves a configuration value to the config file
func saveConfigValue(key string, value interface{}) error {
	configPath, err := globalconfig.ConfigFile("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	v.Set(key, value)

	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// extractRepoName extracts a repository name from a git URL
func extractRepoName(gitURL string) string {
	// Remove .git suffix if present
	name := strings.TrimSuffix(gitURL, ".git")

	// Extract the last part of the path
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}

	// For git@github.com:user/repo format
	if strings.Contains(name, ":") {
		parts := strings.Split(name, ":")
		if len(parts) > 1 {
			subparts := strings.Split(parts[1], "/")
			if len(subparts) > 0 {
				name = subparts[len(subparts)-1]
			}
		}
	}

	// Clean up the name
	name = strings.TrimSpace(name)
	if name == "" {
		name = "templates"
	}

	return name
}

func runTemplatesRemove(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pathOrName := args[0]

	logging.InfoContext(ctx, "Removing template source: %s", pathOrName)

	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("config not available in context")
	}

	// Normalize path for comparison
	normalizedPath := normalizePathForComparison(pathOrName)

	// Remove from local paths and repositories
	removedFromPaths, removedFromRepos := removeTemplateSource(cfg, pathOrName, normalizedPath)

	if !removedFromPaths && !removedFromRepos {
		return fmt.Errorf("template source not found: %s", pathOrName)
	}

	// Save both updated values
	if err := saveTemplatesConfig(cfg); err != nil {
		return err
	}

	// Log results
	configPath, _ := globalconfig.ConfigFile("config.yaml")
	if removedFromPaths {
		logging.InfoContext(ctx, "Removed from local_paths in %s", configPath)
	}
	if removedFromRepos {
		logging.InfoContext(ctx, "Removed from repositories in %s", configPath)
	}
	return nil
}

// normalizePathForComparison normalizes a path for comparison
func normalizePathForComparison(pathOrName string) string {
	path := pathOrName

	// Expand home directory if needed
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Make path absolute if relative (and it looks like a path)
	if !filepath.IsAbs(path) && (strings.Contains(path, "/") || strings.HasPrefix(path, ".")) {
		if absPath, err := filepath.Abs(path); err == nil {
			path = absPath
		}
	}

	return path
}

// removeTemplateSource removes a template source from configuration
func removeTemplateSource(cfg *globalconfig.Config, pathOrName, normalizedPath string) (removedFromPaths, removedFromRepos bool) {
	// Try to remove from local_paths
	newLocalPaths := []string{}
	for _, existingPath := range cfg.Templates.LocalPaths {
		if existingPath != normalizedPath && existingPath != pathOrName {
			newLocalPaths = append(newLocalPaths, existingPath)
		} else {
			removedFromPaths = true
		}
	}
	cfg.Templates.LocalPaths = newLocalPaths

	// Try to remove from repositories by name
	if _, exists := cfg.Templates.Repositories[pathOrName]; exists {
		delete(cfg.Templates.Repositories, pathOrName)
		removedFromRepos = true
	}

	return removedFromPaths, removedFromRepos
}

// saveTemplatesConfig saves template configuration to the config file
func saveTemplatesConfig(cfg *globalconfig.Config) error {
	configPath, err := globalconfig.ConfigFile("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	v.Set("templates.local_paths", cfg.Templates.LocalPaths)
	v.Set("templates.repositories", cfg.Templates.Repositories)

	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func runTemplatesList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logging.InfoContext(ctx, "Fetching available templates...")

	// Create template registry
	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	// List all templates from all sources
	templateList, err := registry.List("all")
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	// Apply source filter if specified
	if templatesListSource != "all" {
		templateList = filterTemplatesBySource(templateList, templatesListSource)
	}

	if len(templateList) == 0 {
		logging.InfoContext(ctx, "No templates found. Configure template repositories or local paths in ~/.warpgate/config.yaml")
		return nil
	}

	// Output in requested format
	switch templatesListFormat {
	case "json":
		return outputTemplatesJSON(templateList)
	case "gha-matrix":
		return outputTemplatesGHAMatrix(templateList)
	case "table":
		return outputTemplatesTable(templateList)
	default:
		return fmt.Errorf("unknown format: %s (supported: table, json, gha-matrix)", templatesListFormat)
	}
}

// filterTemplatesBySource filters templates by source type or name
func filterTemplatesBySource(templateList []templates.TemplateInfo, source string) []templates.TemplateInfo {
	var filtered []templates.TemplateInfo

	for _, tmpl := range templateList {
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

// outputTemplatesTable outputs templates in a formatted table
func outputTemplatesTable(templateList []templates.TemplateInfo) error {
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

// outputTemplatesJSON outputs templates as JSON
func outputTemplatesJSON(templateList []templates.TemplateInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(templateList)
}

// GHATemplateEntry represents a template entry in GitHub Actions matrix format
type GHATemplateEntry struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Vars        string `json:"vars,omitempty"`
}

// GHAMatrix represents the GitHub Actions matrix structure
type GHAMatrix struct {
	Template []GHATemplateEntry `json:"template"`
}

// outputTemplatesGHAMatrix outputs templates in GitHub Actions matrix format
func outputTemplatesGHAMatrix(templateList []templates.TemplateInfo) error {
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
		// Users can extend this with --var flags in the build command
		if needsProvisionRepo(tmpl.Name) {
			entry.Vars = "PROVISION_REPO_PATH=$HOME/ansible-collection-arsenal"
		}

		matrix.Template = append(matrix.Template, entry)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(matrix)
}

// extractNamespace extracts the namespace from a repository source
func extractNamespace(repository string) string {
	// For local sources, extract the repository name
	if strings.HasPrefix(repository, "local:") {
		parts := strings.Split(repository, ":")
		if len(parts) > 1 {
			// Return the base name without "local:" prefix
			return parts[1]
		}
	}

	// For git repositories, use the repository name
	// Default to "cowdogmoo" as that's the common namespace
	return "cowdogmoo"
}

// needsProvisionRepo checks if a template needs ansible provisioning repo
func needsProvisionRepo(templateName string) bool {
	// Templates that typically use ansible provisioning
	provisionTemplates := map[string]bool{
		"attack-box":      true,
		"atomic-red-team": true,
		"sliver":          true,
		"ttpforge":        true,
	}

	return provisionTemplates[templateName]
}

func runTemplatesSearch(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	query := args[0]
	logging.InfoContext(ctx, "Searching for templates matching: %s", query)

	// Create template registry
	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	// Search for templates
	results, err := registry.Search(query)
	if err != nil {
		return fmt.Errorf("failed to search templates: %w", err)
	}

	if len(results) == 0 {
		logging.InfoContext(ctx, "No templates found matching query: %s", query)
		return nil
	}

	// Display search results
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

func runTemplatesInfo(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	templateName := args[0]
	logging.InfoContext(ctx, "Fetching information for template: %s", templateName)

	// Create template loader
	loader, err := templates.NewTemplateLoader()
	if err != nil {
		return fmt.Errorf("failed to create template loader: %w", err)
	}

	// Load the template configuration
	cfg, err := loader.LoadTemplate(templateName)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	displayTemplateInfo(templateName, cfg)
	return nil
}

func displayTemplateInfo(templateName string, cfg *builder.Config) {
	fmt.Printf("\nTemplate: %s\n", templateName)
	fmt.Println("=" + string(make([]byte, len(templateName)+10)))

	displayMetadata(cfg)
	displayBuildConfig(cfg)
	displayTargets(cfg)
	displayProvisioners(cfg)
	displayEnvironmentVars(cfg)

	fmt.Println()
}

func displayMetadata(cfg *builder.Config) {
	if cfg.Metadata.Description != "" {
		fmt.Printf("\nDescription:\n  %s\n", cfg.Metadata.Description)
	}
	if cfg.Metadata.Version != "" {
		fmt.Printf("\nVersion: %s\n", cfg.Metadata.Version)
	}
	if cfg.Metadata.Author != "" {
		fmt.Printf("Author: %s\n", cfg.Metadata.Author)
	}
	if len(cfg.Metadata.Tags) > 0 {
		fmt.Printf("Tags: %v\n", cfg.Metadata.Tags)
	}
}

func displayBuildConfig(cfg *builder.Config) {
	fmt.Printf("\nBuild Configuration:\n")
	fmt.Printf("  Name: %s\n", cfg.Name)
	if cfg.Base.Image != "" {
		fmt.Printf("  Base Image: %s\n", cfg.Base.Image)
	}
}

func displayTargets(cfg *builder.Config) {
	if len(cfg.Targets) == 0 {
		return
	}

	fmt.Printf("\nTargets:\n")
	for _, target := range cfg.Targets {
		fmt.Printf("  - Type: %s\n", target.Type)
		if target.Registry != "" {
			fmt.Printf("    Registry: %s\n", target.Registry)
		}
		if len(target.Tags) > 0 {
			fmt.Printf("    Tags: %v\n", target.Tags)
		}
		if len(target.Platforms) > 0 {
			fmt.Printf("    Platforms: %v\n", target.Platforms)
		}
	}
}

func displayProvisioners(cfg *builder.Config) {
	if len(cfg.Provisioners) == 0 {
		return
	}

	fmt.Printf("\nProvisioners (%d):\n", len(cfg.Provisioners))
	for i, prov := range cfg.Provisioners {
		fmt.Printf("  %d. Type: %s\n", i+1, prov.Type)
		displayProvisionerDetails(prov)
	}
}

func displayProvisionerDetails(prov builder.Provisioner) {
	switch prov.Type {
	case "shell":
		if len(prov.Inline) > 0 {
			fmt.Printf("     Commands: %d inline\n", len(prov.Inline))
		}
	case "script":
		if len(prov.Scripts) > 0 {
			fmt.Printf("     Scripts: %v\n", prov.Scripts)
		}
	case "ansible":
		if prov.PlaybookPath != "" {
			fmt.Printf("     Playbook: %s\n", prov.PlaybookPath)
		}
	}
}

func displayEnvironmentVars(cfg *builder.Config) {
	if len(cfg.Base.Env) == 0 {
		return
	}

	fmt.Printf("\nEnvironment Variables:\n")
	for key, value := range cfg.Base.Env {
		fmt.Printf("  %s: %v\n", key, value)
	}
}

func runTemplatesUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logging.InfoContext(ctx, "Updating template cache...")

	// Create template registry
	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	// Update all caches
	if err := registry.UpdateAllCaches(); err != nil {
		return fmt.Errorf("failed to update template cache: %w", err)
	}

	logging.InfoContext(ctx, "Template cache updated successfully")
	return nil
}
