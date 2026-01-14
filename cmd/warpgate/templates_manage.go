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

	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/templates"
	"github.com/spf13/cobra"
)

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

	// Use the templates manager
	manager := templates.NewManager(cfg)
	validator := templates.NewPathValidator()

	if validator.IsGitURL(urlOrPath) {
		return manager.AddGitRepository(ctx, name, urlOrPath)
	}

	return manager.AddLocalPath(ctx, urlOrPath)
}

// parseTemplatesAddArgs parses and validates the arguments for templates add command
func parseTemplatesAddArgs(args []string) (name string, urlOrPath string, err error) {
	validator := templates.NewPathValidator()

	if len(args) == 2 {
		// Two args: [name] [url]
		name = args[0]
		urlOrPath = args[1]

		if !validator.IsGitURL(urlOrPath) {
			return "", "", fmt.Errorf("when providing a name, the second argument must be a git URL (not a local path)")
		}
	} else {
		// One arg: [url-or-path]
		urlOrPath = args[0]
	}
	return name, urlOrPath, nil
}

func runTemplatesRemove(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pathOrName := args[0]

	logging.InfoContext(ctx, "Removing template source: %s", pathOrName)

	cfg := configFromContext(cmd)
	if cfg == nil {
		return fmt.Errorf("config not available in context")
	}

	// Use the templates manager
	manager := templates.NewManager(cfg)
	return manager.RemoveSource(ctx, pathOrName)
}

func runTemplatesUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logging.InfoContext(ctx, "Updating template cache...")

	// Create template registry
	registry, err := templates.NewTemplateRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	// Update all caches
	if err := registry.UpdateAllCaches(ctx); err != nil {
		return fmt.Errorf("failed to update template cache: %w", err)
	}

	logging.InfoContext(ctx, "Template cache updated successfully")
	return nil
}
