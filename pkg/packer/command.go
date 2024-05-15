package packer

import (
	"fmt"
	"strings"

	"github.com/l50/goutils/v2/sys"
)

type PackerCommandRunner interface {
	RunBuild(args []string, dir string) error
	RunInit(args []string, dir string) error
	RunValidate(args []string, dir string) error
	RunVersion() (string, error)
}

func (p *BlueprintPacker) runCommand(subCmd string, args []string, dir string, outputHandler func(string)) error {
	cmd := sys.Cmd{
		CmdString:     "packer",
		Args:          append([]string{subCmd}, args...),
		Dir:           dir,
		OutputHandler: outputHandler,
	}

	if _, err := cmd.RunCmd(); err != nil {
		return fmt.Errorf("error running %s command: %v", subCmd, err)
	}

	if subCmd == "build" {
		p.ParseImageHashes("")
	}

	return nil
}

// RunBuild runs the build command with the provided arguments.
//
// **Parameters:**
// - args: The arguments for the build command.
//
// **Returns:**
// - error: An error if the build command fails.
func (p *BlueprintPacker) RunBuild(args []string, dir string) error {
	if dir == "" {
		dir = "."
	}

	outputHandler := func(s string) {
		fmt.Println(s)
		p.ParseImageHashes(s)
	}

	return p.runCommand("build", args, dir, outputHandler)
}

// RunInit runs the init command with the provided arguments.
func (p *BlueprintPacker) RunInit(args []string, dir string) error {
	// if dir is present, use it as the working directory
	// otherwise default to the current working directory
	if dir == "" {
		dir = "."
	}

	outputHandler := func(s string) {
		fmt.Println(s)
	}

	return p.runCommand("init", args, dir, outputHandler)
}

// RunValidate runs the validate command with the provided arguments.
func (p *BlueprintPacker) RunValidate(args []string, dir string) error {
	// if dir is present, use it as the working directory
	// otherwise default to the current working directory
	if dir == "" {
		dir = "."
	}

	outputHandler := func(s string) {
		fmt.Println(s)
	}

	return p.runCommand("validate", args, dir, outputHandler)
}

// RunVersion runs the version command and returns the Packer version.
func (p *BlueprintPacker) RunVersion() (string, error) {
	var versionOutput strings.Builder
	outputHandler := func(s string) {
		versionOutput.WriteString(s)
	}

	err := p.runCommand("version", []string{}, "", outputHandler)
	if err != nil {
		return "", err
	}

	return versionOutput.String(), nil
}
