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
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/cowdogmoo/warpgate/pkg/templates"
	"github.com/spf13/cobra"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage warpgate templates",
	Long:  `List, search, and inspect available templates.`,
}

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	Long:  `List all available templates from the official registry.`,
	RunE:  runTemplatesList,
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

var templatesRepoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage template repositories",
	Long:  `Add, remove, or list template repositories.`,
}

var templatesRepoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured repositories",
	RunE:  runTemplatesRepoList,
}

var templatesRepoAddCmd = &cobra.Command{
	Use:   "add [name] [git-url]",
	Short: "Add a template repository",
	Args:  cobra.ExactArgs(2),
	RunE:  runTemplatesRepoAdd,
}

var templatesRepoRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a template repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplatesRepoRemove,
}

func init() {
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesSearchCmd)
	templatesCmd.AddCommand(templatesInfoCmd)
	templatesCmd.AddCommand(templatesUpdateCmd)
	templatesCmd.AddCommand(templatesRepoCmd)

	templatesRepoCmd.AddCommand(templatesRepoListCmd)
	templatesRepoCmd.AddCommand(templatesRepoAddCmd)
	templatesRepoCmd.AddCommand(templatesRepoRemoveCmd)
}

func runTemplatesList(cmd *cobra.Command, args []string) error {
	logging.Info("Fetching available templates...")

	// Create template loader
	loader, err := templates.NewTemplateLoader()
	if err != nil {
		return fmt.Errorf("failed to create template loader: %w", err)
	}

	// List all templates
	templateList, err := loader.List()
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	if len(templateList) == 0 {
		logging.Info("No templates found in registry")
		return nil
	}

	// Display templates in a formatted table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION\tAUTHOR")
	fmt.Fprintln(w, "----\t-------\t-----------\t------")

	for _, tmpl := range templateList {
		version := tmpl.Version
		if version == "" {
			version = "latest"
		}
		author := tmpl.Author
		if author == "" {
			author = "unknown"
		}
		description := tmpl.Description
		if len(description) > 60 {
			description = description[:57] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tmpl.Name, version, description, author)
	}

	w.Flush()
	fmt.Printf("\nTotal templates: %d\n", len(templateList))
	return nil
}

func runTemplatesSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	logging.Info("Searching for templates matching: %s", query)

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
		logging.Info("No templates found matching query: %s", query)
		return nil
	}

	// Display search results
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tREPOSITORY\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t----------\t-----------")

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

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tmpl.Name, version, repo, description)
	}

	w.Flush()
	fmt.Printf("\nFound %d template(s) matching query: %s\n", len(results), query)
	return nil
}

func runTemplatesInfo(cmd *cobra.Command, args []string) error {
	templateName := args[0]
	logging.Info("Fetching information for template: %s", templateName)

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
	logging.Info("Updating template cache...")

	// Create template registry
	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	// Update all caches
	if err := registry.UpdateAllCaches(); err != nil {
		return fmt.Errorf("failed to update template cache: %w", err)
	}

	logging.Info("Template cache updated successfully")
	return nil
}

func runTemplatesRepoList(cmd *cobra.Command, args []string) error {
	// Create template registry
	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	repos := registry.GetRepositories()
	if len(repos) == 0 {
		logging.Info("No repositories configured")
		return nil
	}

	// Display repositories
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tGIT URL")
	fmt.Fprintln(w, "----\t-------")

	for name, url := range repos {
		fmt.Fprintf(w, "%s\t%s\n", name, url)
	}

	w.Flush()
	fmt.Printf("\nTotal repositories: %d\n", len(repos))
	return nil
}

func runTemplatesRepoAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	gitURL := args[1]

	logging.Info("Adding repository: %s -> %s", name, gitURL)

	// Create template registry
	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	// Add repository
	registry.AddRepository(name, gitURL)

	// Save configuration
	if err := registry.SaveRepositories(); err != nil {
		return fmt.Errorf("failed to save repository configuration: %w", err)
	}

	logging.Info("Repository added successfully")
	logging.Info("Run 'warpgate templates update' to fetch templates from the new repository")
	return nil
}

func runTemplatesRepoRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	logging.Info("Removing repository: %s", name)

	// Create template registry
	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	// Remove repository
	registry.RemoveRepository(name)

	// Save configuration
	if err := registry.SaveRepositories(); err != nil {
		return fmt.Errorf("failed to save repository configuration: %w", err)
	}

	logging.Info("Repository removed successfully")
	return nil
}
