//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"

	"github.com/cowdogmoo/warpgate/pkg/magehelpers"
	"github.com/fatih/color"
	"github.com/l50/goutils/v2/dev/lint"
	mageutils "github.com/l50/goutils/v2/dev/mage"
	"github.com/l50/goutils/v2/sys"
	"github.com/magefile/mage/sh"
)

func init() {
	os.Setenv("GO111MODULE", "on")
}

// InstallDeps installs the Go dependencies necessary for developing
// on the project.
//
// Example usage:
//
// ```go
// mage installdeps
// ```
//
// **Returns:**
//
// error: An error if any issue occurs while trying to
// install the dependencies.
func InstallDeps() error {
	fmt.Println(color.YellowString("Installing dependencies."))
	if err := lint.InstallGoPCDeps(); err != nil {
		return fmt.Errorf("failed to install pre-commit dependencies: %v", err)
	}

	if err := mageutils.InstallVSCodeModules(); err != nil {
		return fmt.Errorf(color.RedString(
			"failed to install vscode-go modules: %v", err))
	}

	return nil
}

// RunPreCommit updates, clears, and executes all pre-commit hooks
// locally. The function follows a three-step process:
//
// First, it updates the pre-commit hooks.
// Next, it clears the pre-commit cache to ensure a clean environment.
// Lastly, it executes all pre-commit hooks locally.
//
// Example usage:
//
// ```go
// mage runprecommit
// ```
//
// **Returns:**
//
// error: An error if any issue occurs at any of the three stages
// of the process.
func RunPreCommit() error {
	if !sys.CmdExists("pre-commit") {
		return fmt.Errorf("pre-commit is not installed, please install it " +
			"with the following command: `python3 -m pip install pre-commit`")
	}

	fmt.Println(color.YellowString("Updating pre-commit hooks."))
	if err := lint.UpdatePCHooks(); err != nil {
		return err
	}

	fmt.Println(color.YellowString("Clearing the pre-commit cache to ensure we have a fresh start."))
	if err := lint.ClearPCCache(); err != nil {
		return err
	}

	fmt.Println(color.YellowString("Running all pre-commit hooks locally."))
	if err := lint.RunPCHooks(); err != nil {
		return err
	}

	return nil
}

// UpdateMirror updates pkg.go.dev with the release associated with the
// input tag
//
// Example usage:
//
// ```go
// mage updatemirror v2.0.1
// ```
//
// **Parameters:**
//
// tag: the tag to update pkg.go.dev with
//
// **Returns:**
//
// error: An error if any issue occurs while updating pkg.go.dev
func UpdateMirror(tag string) error {
	var err error
	fmt.Printf("Updating pkg.go.dev with the new tag %s.", tag)

	err = sh.RunV("curl", "--silent", fmt.Sprintf(
		"https://sum.golang.org/lookup/github.com/l50/goutils/v2@%s",
		tag))
	if err != nil {
		return fmt.Errorf("failed to update proxy.golang.org: %w", err)
	}

	err = sh.RunV("curl", "--silent", fmt.Sprintf(
		"https://proxy.golang.org/github.com/l50/goutils/v2/@v/%s.info",
		tag))
	if err != nil {
		return fmt.Errorf("failed to update pkg.go.dev: %w", err)
	}

	return nil
}

// Compile compiles the warpgate binary.
func Compile() error {
	fmt.Println("Compiling the warpgate binary, please wait.")
	if err := magehelpers.Compile(); err != nil {
		return fmt.Errorf("failed to compile warpgate: %v", err)
	}

	return nil
}

// RunTests executes all unit tests.
func RunTests() error {
	if err := magehelpers.RunTests(); err != nil {
		return fmt.Errorf("failed to run unit tests: %v", err)
	}

	return nil
}

// GeneratePackageDocs creates documentation for the various packages
// in the project.
func GeneratePackageDocs() error {
	if err := magehelpers.GeneratePackageDocs(); err != nil {
		return fmt.Errorf("failed to generate package documentation: %v", err)
	}

	return nil
}

// DeleteReleaseAndTag deletes a GitHub release and its corresponding tag.
//
// Example usage:
//
// ```go
// mage deletereleaseandtag v1.0.5
// ```
//
// **Parameters:**
//
// tag: the tag of the release to delete
//
// **Returns:**
//
// error: An error if any issue occurs while deleting the release or tag
func DeleteReleaseAndTag(tag string) error {
	fmt.Println(color.YellowString("Deleting GitHub release and tag:", tag))

	// Delete the GitHub release
	err := sh.RunV("gh", "release", "delete", tag, "--yes")
	if err != nil {
		return fmt.Errorf("failed to delete GitHub release: %v", err)
	}

	// Delete the GitHub tag
	err = sh.RunV("git", "tag", "-d", tag)
	if err != nil {
		return fmt.Errorf("failed to delete local tag: %v", err)
	}

	err = sh.RunV("git", "push", "origin", "--delete", tag)
	if err != nil {
		return fmt.Errorf("failed to delete remote tag: %v", err)
	}

	fmt.Println(color.GreenString("Successfully deleted GitHub release and tag:", tag))
	return nil
}

// CreateRelease creates a new GitHub release and updates the CHANGELOG.
//
// Example usage:
//
// ```go
// mage createrelease v1.0.6
// ```
//
// **Parameters:**
//
// nextVersion: the version for the new release
//
// **Returns:**
//
// error: An error if any issue occurs while creating the release
func CreateRelease(nextVersion string) error {
	fmt.Println(color.YellowString("Creating new GitHub release:", nextVersion))

	// Create the changelog
	err := sh.RunV("gh", "changelog", "new", "--next-version", nextVersion)
	if err != nil {
		return fmt.Errorf("failed to create changelog: %v", err)
	}

	// Create the GitHub release
	err = sh.RunV("gh", "release", "create", nextVersion, "-F", "CHANGELOG.md")
	if err != nil {
		return fmt.Errorf("failed to create GitHub release: %v", err)
	}

	fmt.Println(color.GreenString("Successfully created GitHub release:", nextVersion))
	return nil
}

// FullReleaseProcess handles deleting the old release and tag, and creating a new release.
//
// Example usage:
//
// ```go
// mage fullreleaseprocess v1.0.5 v1.0.6
// ```
//
// **Parameters:**
//
// oldTag: the old tag to delete
// newTag: the new tag for the release
//
// **Returns:**
//
// error: An error if any issue occurs during the process
func FullReleaseProcess(oldTag, newTag string) error {
	if err := DeleteReleaseAndTag(oldTag); err != nil {
		return err
	}

	if err := CreateRelease(newTag); err != nil {
		return err
	}

	return nil
}
