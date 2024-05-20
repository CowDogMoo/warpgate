package packer

import (
	"bytes"
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
func (p *PackerTemplate) runCommand(
	subCmd string, args []string, dir string,
	outputHandler func(string)) (string, error) {

	var outputBuffer bytes.Buffer
	cmd := sys.Cmd{
		CmdString: "packer",
		Args:      append([]string{subCmd}, args...),
		Dir:       dir,
		OutputHandler: func(s string) {
			outputHandler(s)
			outputBuffer.WriteString(s + "\n") // Capture output
		},
	}

	fmt.Printf(
		"executing command: %s %v in directory: %s\n",
		cmd.CmdString, cmd.Args, cmd.Dir)

	if _, err := cmd.RunCmd(); err != nil {
		return "", fmt.Errorf("error running %s command: %v", subCmd, err)
	}

	return outputBuffer.String(), nil
}

// RunBuild runs the Packer build command and captures the output to parse image
// hashes and AMI details.
//
// **Parameters:**
//
// args: A slice of strings containing the arguments to pass to the Packer build command.
// dir: The directory to run the Packer build command in.
//
// **Returns:**
//
// map[string]string: A map of image hashes parsed from the build output.
// string: The AMI ID parsed from the build output.
// error: An error if the build command fails.
func (p *PackerTemplate) RunBuild(args []string, dir string) (map[string]string, string, error) {
	if dir == "" {
		dir = "."
	}

	fmt.Printf("Running Packer build command from the %s directory...\n", dir)

	var outputBuffer bytes.Buffer
	outputHandler := func(s string) {
		fmt.Println(s)
		outputBuffer.WriteString(s)
	}

	if len(args) > 0 && args[0] == "build" {
		args = args[1:]
	}

	output, err := p.runCommand("build", args, dir, outputHandler) // Capture output
	if err != nil {
		return nil, "", err
	}

	if p.Container.ImageRegistry.Server != "" {
		imageHashes := p.ParseImageHashes(output)
		amiID := p.ParseAMIDetails(output)
		return imageHashes, amiID, nil
	}

	return nil, "", nil
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
func (p *PackerTemplate) RunInit(args []string, dir string) error {
	if dir == "" {
		dir = "."
	}

	outputHandler := func(s string) {
		fmt.Println(s)
	}

	_, err := p.runCommand("init", args, dir, outputHandler)
	if err != nil {
		return err
	}

	return nil
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
func (p *PackerTemplate) RunValidate(args []string, dir string) error {
	if dir == "" {
		dir = "."
	}

	outputHandler := func(s string) {
		fmt.Println(s)
	}

	_, err := p.runCommand("validate", args, dir, outputHandler)
	if err != nil {
		return err
	}

	return nil
}

// RunVersion runs the Packer version command and returns the Packer version.
//
// **Returns:**
//
// string: The version of Packer.
// error: An error if the version command fails.
func (p *PackerTemplate) RunVersion() (string, error) {
	var versionOutput strings.Builder
	outputHandler := func(s string) {
		versionOutput.WriteString(s)
	}

	_, err := p.runCommand("version", []string{}, "", outputHandler)
	if err != nil {
		return "", err
	}

	return versionOutput.String(), nil
}
