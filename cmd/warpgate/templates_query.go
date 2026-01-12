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
	"strings"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/cli"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/templates"
	"github.com/spf13/cobra"
)

func runTemplatesList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Suppress logging for quiet mode or structured output formats
	shouldSuppressLog := templatesListQuiet || templatesListFormat == "json" || templatesListFormat == "gha-matrix"

	if shouldSuppressLog {
		logging.SetQuiet(true)
		defer logging.SetQuiet(false)
	}

	if !shouldSuppressLog {
		logging.InfoContext(ctx, "Fetching available templates...")
	}

	registry, err := templates.NewTemplateRegistry()
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	var templateList []templates.TemplateInfo

	// Use local-only listing when --source local to avoid git operations
	if templatesListSource == "local" {
		templateList, err = registry.ListLocal()
		if err != nil {
			return fmt.Errorf("failed to list local templates: %w", err)
		}
	} else {
		// List all templates from all sources
		templateList, err = registry.List("all")
		if err != nil {
			return fmt.Errorf("failed to list templates: %w", err)
		}

		// Apply source filter if specified and not "all"
		if templatesListSource != "all" {
			filter := templates.NewFilter()
			templateList = filter.BySource(templateList, templatesListSource)
		}
	}

	// Output empty JSON array for structured formats when no templates found
	if len(templateList) == 0 {
		switch templatesListFormat {
		case "gha-matrix":
			fmt.Println("{\"template\": []}")
			return nil
		case "json":
			fmt.Println("[]")
			return nil
		}

		if !shouldSuppressLog {
			logging.InfoContext(ctx, "No templates found. Configure template repositories or local paths in ~/.warpgate/config.yaml")
		}
		return nil
	}

	formatter := cli.NewOutputFormatter(templatesListFormat)
	return formatter.DisplayTemplateList(templateList)
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

	// Use output formatter
	formatter := cli.NewOutputFormatter("text")
	return formatter.DisplayTemplateSearchResults(results, query)
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
	cfg, err := loader.LoadTemplateWithVars(templateName, nil)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	displayTemplateInfo(templateName, cfg)
	return nil
}

func displayTemplateInfo(templateName string, cfg *builder.Config) {
	fmt.Printf("\nTemplate: %s\n", templateName)
	fmt.Println(strings.Repeat("=", len("Template: ")+len(templateName)))

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
	case "powershell":
		if len(prov.PSScripts) > 0 {
			fmt.Printf("     PSScripts: %v\n", prov.PSScripts)
		}
	case "file":
		if prov.Source != "" {
			fmt.Printf("     Source: %s -> %s\n", prov.Source, prov.Destination)
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
