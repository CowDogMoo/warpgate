//go:build mage
// +build mage

package main

import (
	"fmt"
	"path/filepath"

	"github.com/l50/goutils/v2/docs"
	"github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/afero"
)

// GeneratePackageDocs creates documentation for the various packages
// in the project.
//
// Example usage:
//
// ```go
// mage generatepackagedocs
// ```
//
// **Returns:**
//
// error: An error if any issue occurs during documentation generation.
func GeneratePackageDocs() error {
	fs := afero.NewOsFs()

	repoRoot, err := git.RepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get repo root: %v", err)
	}
	sys.Cd(repoRoot)

	repo := docs.Repo{
		Owner: "cowdogmoo",
		Name:  "warpgate",
	}

	templatePath := filepath.Join(repoRoot, "magefiles", "tmpl", "README.md.tmpl")
	// Set the packages to exclude (optional)
	excludedPkgs := []string{"main"}
	if err := docs.CreatePackageDocs(fs, repo, templatePath, excludedPkgs...); err != nil {
		return fmt.Errorf("failed to create package docs: %v", err)
	}

	return nil
}
