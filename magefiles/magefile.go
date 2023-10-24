//go:build mage

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fatih/color"
	goutils "github.com/l50/goutils"
	mageutils "github.com/l50/goutils/v2/dev/mage"
	"github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"

	// mage utility functions
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func init() {
	os.Setenv("GO111MODULE", "on")
}

// InstallDeps Installs go dependencies
func InstallDeps() error {
	fmt.Println(color.YellowString("Running go mod tidy on magefiles and repo root."))
	cwd := sys.Gwd()
	if err := sys.Cd("magefiles"); err != nil {
		return fmt.Errorf("failed to cd into magefiles directory: %v", err)
	}

	if err := mageutils.Tidy(); err != nil {
		return fmt.Errorf("failed to install dependencies: %v", err)
	}

	if err := sys.Cd(cwd); err != nil {
		return fmt.Errorf("failed to cd back into repo root: %v", err)
	}

	if err := mageutils.Tidy(); err != nil {
		return fmt.Errorf("failed to install dependencies: %v", err)
	}

	fmt.Println(color.YellowString("Installing dependencies."))
	if err := goutils.InstallGoPCDeps(); err != nil {
		return fmt.Errorf(color.RedString(
			"failed to install pre-commit dependencies: %v", err))
	}

	if err := goutils.InstallVSCodeModules(); err != nil {
		return fmt.Errorf(color.RedString(
			"failed to install vscode-go modules: %v", err))
	}

	return nil
}

// InstallPreCommitHooks Installs pre-commit hooks locally
func InstallPreCommitHooks() error {
	mg.Deps(InstallDeps)

	fmt.Println(color.YellowString("Installing pre-commit hooks."))
	if err := goutils.InstallPCHooks(); err != nil {
		return err
	}

	return nil
}

// RunPreCommit runs all pre-commit hooks locally
func RunPreCommit() error {
	mg.Deps(InstallDeps)

	fmt.Println(color.YellowString("Updating pre-commit hooks."))
	if err := goutils.UpdatePCHooks(); err != nil {
		return err
	}

	fmt.Println(color.YellowString(
		"Clearing the pre-commit cache to ensure we have a fresh start."))
	if err := goutils.ClearPCCache(); err != nil {
		return err
	}

	fmt.Println(color.YellowString("Running all pre-commit hooks locally."))
	if err := goutils.RunPCHooks(); err != nil {
		return err
	}

	return nil
}

// UpdateMirror updates pkg.go.dev with the release associated with the input tag
func UpdateMirror(tag string) error {
	var err error
	fmt.Println(color.YellowString("Updating pkg.go.dev with the new tag %s.", tag))

	err = sh.RunV("curl", "--silent", fmt.Sprintf(
		"https://sum.golang.org/lookup/github.com/l50/goproject@%s",
		tag))
	if err != nil {
		return fmt.Errorf(color.RedString("failed to update proxy.golang.org: %w", err))
	}

	err = sh.RunV("curl", "--silent", fmt.Sprintf(
		"https://proxy.golang.org/github.com/l50/goproject/@v/%s.info",
		tag))
	if err != nil {
		return fmt.Errorf(color.RedString("failed to update pkg.go.dev: %w", err))
	}

	return nil
}

type compileParams struct {
	GOOS   string
	GOARCH string
}

func (p *compileParams) populateFromEnv() {
	if p.GOOS == "" {
		p.GOOS = os.Getenv("GOOS")
		if p.GOOS == "" {
			p.GOOS = runtime.GOOS
		}
	}

	if p.GOARCH == "" {
		p.GOARCH = os.Getenv("GOARCH")
		if p.GOARCH == "" {
			p.GOARCH = runtime.GOARCH
		}
	}
}

// Compile compiles the Go project using goreleaser. The behavior is
// controlled by the 'release' environment variable. If the GOOS and
// GOARCH environment variables are not set, the function defaults
// to the current system's OS and architecture.
//
// **Environment Variables:**
//
// release: Determines the compilation mode.
//
// If "true", compiles all supported releases for TTPForge.
// If "false", compiles only the binary for the specified OS
// and architecture (based on GOOS and GOARCH) or the current
// system's default if the vars aren't set.
//
// GOOS: Target operating system for compilation. Defaults to the
// current system's OS if not set.
//
// GOARCH: Target architecture for compilation. Defaults to the
// current system's architecture if not set.
//
// Example usage:
//
// ```go
// release=true mage compile # Compiles all supported releases for TTPForge
// GOOS=darwin GOARCH=arm64 mage compile false # Compiles the binary for darwin/arm64
// GOOS=linux GOARCH=amd64 mage compile false # Compiles the binary for linux/amd64
// ```
//
// **Returns:**
//
// error: An error if any issue occurs during compilation.
func Compile() error {
	// Check for the presence of the 'release' environment variable
	release, ok := os.LookupEnv("release")
	if !ok {
		return fmt.Errorf("'release' environment variable not set. It should be 'true' or 'false'. Example: release=true mage Compile")
	}

	isRelease := false
	if release == "true" {
		isRelease = true
	} else if release != "false" {
		return fmt.Errorf("invalid value for 'release' environment variable. It should be 'true' or 'false'")
	}

	if !sys.CmdExists("goreleaser") {
		return fmt.Errorf("goreleaser is not installed, please run mage installdeps")
	}

	cwd, err := git.RepoRoot()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	doCompile := func(release bool) error {
		var p compileParams
		p.populateFromEnv() // Populate the GOOS and GOARCH parameters

		var args []string

		if release {
			fmt.Println("Compiling all supported releases for TTPForge with goreleaser")
			args = []string{"release", "--snapshot", "--clean", "--skip", "validate"}
		} else {
			fmt.Printf("Compiling the TTPForge binary for %s/%s, please wait.\n", p.GOOS, p.GOARCH)
			args = []string{"build", "--snapshot", "--clean", "--skip", "validate", "--single-target"}
		}

		if err := sh.RunV("goreleaser", args...); err != nil {
			return fmt.Errorf("goreleaser failed to execute: %v", err)
		}
		return nil
	}

	return doCompile(isRelease)
}
