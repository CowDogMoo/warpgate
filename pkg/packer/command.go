package packer

import (
	"fmt"
	"strings"

	"github.com/l50/goutils/v2/sys"
)

// PackerCommandRunner defines the interface for running Packer commands.
//
// **Methods:**
//
// RunBuild: Runs the Packer build command.
// RunInit: Runs the Packer init command.
// RunValidate: Runs the Packer validate command.
// RunVersion: Runs the Packer version command.
type PackerCommandRunner interface {
	RunBuild(args []string, dir string) error
	RunInit(args []string, dir string) error
	RunValidate(args []string, dir string) error
	RunVersion() (string, error)
}

// runCommand executes a Packer command with the specified sub-command,
// arguments, and working directory. It uses the provided output handler
// to process the command output.
//
// **Parameters:**
//
// subCmd: The Packer sub-command to run (e.g., "build").
// args: A slice of strings representing the command arguments.
// dir: The directory in which to run the command.
// outputHandler: A function to handle the command output.
//
// **Returns:**
//
// error: An error if the command fails.
func (p *BlueprintPacker) runCommand(
	subCmd string, args []string, dir string,
	outputHandler func(string)) error {

	cmd := sys.Cmd{
		CmdString:     "packer",
		Args:          append([]string{subCmd}, args...),
		Dir:           dir,
		OutputHandler: outputHandler,
	}

	fmt.Printf(
		"executing command: %s %v in directory: %s",
		cmd.CmdString, cmd.Args, cmd.Dir)

	if _, err := cmd.RunCmd(); err != nil {
		return fmt.Errorf("error running %s command: %v", subCmd, err)
	}

	if subCmd == "build" && p.Container.Registry.Server != "" {
		p.ParseImageHashes("")
	}

	return nil
}

// RunBuild runs the Packer build command with the provided arguments.
//
// **Parameters:**
//
// args: A slice of strings representing the build command arguments.
// dir: The directory in which to run the command. If empty, the current
// directory is used.
//
// **Returns:**
//
// error: An error if the build command fails.
func (p *BlueprintPacker) RunBuild(args []string, dir string) error {
	if dir == "" {
		dir = "."
	}

	fmt.Printf("Running Packer build command from the %s directory...", dir)
	outputHandler := func(s string) {
		fmt.Println(s)
		if p.Container.Registry.Server != "" {
			p.ParseImageHashes(s)
		}
	}

	if len(args) > 0 && args[0] == "build" {
		args = args[1:]
	}

	return p.runCommand("build", args, dir, outputHandler)
}

// RunInit runs the Packer init command with the provided arguments.
//
// **Parameters:**
//
// args: A slice of strings representing the init command arguments.
// dir: The directory in which to run the command. If empty, the current
// directory is used.
//
// **Returns:**
//
// error: An error if the init command fails.
func (p *BlueprintPacker) RunInit(args []string, dir string) error {
	if dir == "" {
		dir = "."
	}

	outputHandler := func(s string) {
		fmt.Println(s)
	}

	return p.runCommand("init", args, dir, outputHandler)
}

// RunValidate runs the Packer validate command with the provided arguments.
//
// **Parameters:**
//
// args: A slice of strings representing the validate command arguments.
// dir: The directory in which to run the command. If empty, the current
// directory is used.
//
// **Returns:**
//
// error: An error if the validate command fails.
func (p *BlueprintPacker) RunValidate(args []string, dir string) error {
	if dir == "" {
		dir = "."
	}

	outputHandler := func(s string) {
		fmt.Println(s)
	}

	return p.runCommand("validate", args, dir, outputHandler)
}

// RunVersion runs the Packer version command and returns the Packer version.
//
// **Returns:**
//
// string: The version of Packer.
// error: An error if the version command fails.
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
