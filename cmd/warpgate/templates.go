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
	"github.com/spf13/cobra"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage warpgate templates",
	Long:  `List, search, inspect, and manage template sources.`,
	Args:  cobra.NoArgs,
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
	templatesListQuiet  bool
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
	Args: cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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
	templatesListCmd.Flags().BoolVarP(&templatesListQuiet, "quiet", "q", false, "Suppress informational output")

	// Register shell completions for templates list command
	registerTemplatesListCompletions(templatesListCmd)
}

// registerTemplatesListCompletions registers dynamic shell completion functions for templates list command flags.
func registerTemplatesListCompletions(cmd *cobra.Command) {
	// Format completion
	_ = cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"table\tHuman-readable table format (default)",
			"json\tJSON format for programmatic use",
			"gha-matrix\tGitHub Actions matrix format",
		}, cobra.ShellCompDirectiveNoFileComp
	})

	// Source completion
	_ = cmd.RegisterFlagCompletionFunc("source", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"all\tAll configured sources (default)",
			"local\tLocal directory sources only",
			"git\tGit repository sources only",
		}, cobra.ShellCompDirectiveNoFileComp
	})
}
