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
	"fmt"

	"github.com/cowdogmoo/warpgate/v3/pkg/logging"
	"github.com/cowdogmoo/warpgate/v3/pkg/templates"
	"github.com/spf13/cobra"
)

var (
	syntaxOnly bool
	_          context.Context // Force context import to be recognized
)

var validateCmd = &cobra.Command{
	Use:   "validate [config]",
	Short: "Validate a template configuration",
	Long: `Validate a template configuration.

By default, performs full validation including checking file existence.
Use --syntax-only to validate only the structure and syntax without checking
that referenced files (playbooks, scripts, etc.) exist on disk.`,
	Args: cobra.ExactArgs(1),
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().BoolVar(&syntaxOnly, "syntax-only", false, "Only validate syntax and structure, skip file existence checks")
}

func runValidate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	configPath := args[0]

	if syntaxOnly {
		logging.InfoContext(ctx, "Validating configuration (syntax only): %s", configPath)
	} else {
		logging.InfoContext(ctx, "Validating configuration: %s", configPath)
	}

	// Load configuration
	loader := templates.NewLoader()
	cfg, err := loader.LoadFromFileWithVars(configPath, nil)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	validator := templates.NewValidator()
	if err := validator.ValidateWithOptions(cfg, templates.ValidationOptions{
		SyntaxOnly: syntaxOnly,
	}); err != nil {
		logging.ErrorContext(ctx, "Validation failed: %v", err)
		return fmt.Errorf("validation failed: %w", err)
	}

	logging.InfoContext(ctx, "Configuration is valid!")
	return nil
}
