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
	"github.com/cowdogmoo/warpgate/v3/templates"
	"github.com/spf13/cobra"
)

var (
	initFromTemplate string
	initOutputDir    string
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new template",
	Long: `Initialize a new warpgate template with scaffolding.

This command creates a new template directory with the basic structure:
  - warpgate.yaml: Main template configuration
  - README.md: Template documentation
  - scripts/: Directory for provisioning scripts

Use --from to fork an existing template as a starting point.`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initFromTemplate, "from", "f", "", "Fork from existing template")
	initCmd.Flags().StringVarP(&initOutputDir, "output", "o", ".", "Output directory for template")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	templateName := args[0]

	// Use the scaffolder service
	scaffolder := templates.NewScaffolder()

	// If forking from existing template
	if initFromTemplate != "" {
		return scaffolder.Fork(ctx, initFromTemplate, templateName, initOutputDir)
	}

	// Create new template from scratch
	return scaffolder.Create(ctx, templateName, initOutputDir)
}
