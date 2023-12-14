//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"

	// mage utility functions
	"github.com/magefile/mage/sh"
)

type compileParams struct {
	GOOS   string
	GOARCH string
}

var repoRoot string

func init() {
	var err error
	repoRoot, err = git.RepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get repo root: %v", err)
		cobra.CheckErr(err)
	}
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
// If "true", compiles all supported releases for warpgate.
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
// release=true mage compile # Compiles all supported releases for warpgate
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

	cwd, err := changeToRepoRoot()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	doCompile := func(release bool) error {
		var p compileParams
		p.populateFromEnv() // Populate the GOOS and GOARCH parameters

		var args []string

		if release {
			fmt.Println("Compiling all supported releases for warpgate with goreleaser")
			args = []string{"release", "--snapshot", "--clean", "--skip", "validate"}
		} else {
			fmt.Printf("Compiling the warpgate binary for %s/%s, please wait.\n", p.GOOS, p.GOARCH)
			args = []string{"build", "--snapshot", "--clean", "--skip", "validate", "--single-target"}
		}

		if err := sh.RunV("goreleaser", args...); err != nil {
			return fmt.Errorf("goreleaser failed to execute: %v", err)
		}
		return nil
	}

	return doCompile(isRelease)
}

func changeToRepoRoot() (originalCwd string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}

	if cwd != repoRoot {
		if err := os.Chdir(repoRoot); err != nil {
			return "", fmt.Errorf("failed to change directory to repo root: %v", err)
		}
	}

	return cwd, nil
}

// RunTests executes all unit tests.
//
// Example usage:
//
// ```go
// mage runtests
// ```
//
// **Returns:**
//
// error: An error if any issue occurs while running the tests.
func RunTests() error {
	fmt.Println("Running unit tests.")
	if _, err := sys.RunCommand(filepath.Join(".hooks", "run-go-tests.sh"), "all"); err != nil {
		return fmt.Errorf("failed to run unit tests: %v", err)
	}
	return nil
}
