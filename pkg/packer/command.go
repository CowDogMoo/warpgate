package packer

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

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
	RunBuild(args []string, dir string) ([]ImageHash, string, error)
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
// string: The captured command output.
// error: An error if the command fails.
func (p *PackerTemplates) runCommand(
	subCmd string, args []string, dir string,
	outputHandler func(string)) (string, error) {

	var outputBuffer bytes.Buffer
	var mu sync.Mutex // Add a mutex to synchronize access to the buffer

	cmd := sys.Cmd{
		CmdString: "packer",
		Args:      append([]string{subCmd}, args...),
		Dir:       dir,
		OutputHandler: func(s string) {
			outputHandler(s)
			mu.Lock()                          // Lock the mutex before writing to the buffer
			outputBuffer.WriteString(s + "\n") // Capture output
			mu.Unlock()                        // Unlock the mutex after writing to the buffer
		},
	}

	fmt.Printf(
		"Executing command: %s %v in directory: %s\n",
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
// []ImageHash: A slice of image hashes parsed from the build output.
// string: The AMI ID parsed from the build output.
// error: An error if the build command fails.
func (p *PackerTemplates) RunBuild(args []string, dir string) ([]ImageHash, string, error) {
	if dir == "" {
		dir = "."
	}

	fmt.Printf("Running Packer build command from the %s directory...\n", dir)

	var outputBuffer bytes.Buffer
	var mu sync.Mutex // Add a mutex to synchronize access to the buffer

	outputHandler := func(s string) {
		fmt.Println(s)
		mu.Lock() // Lock the mutex before writing to the buffer
		outputBuffer.WriteString(s)
		mu.Unlock() // Unlock the mutex after writing to the buffer
	}

	// If args start with "build", remove it
	if len(args) > 0 && args[0] == "build" {
		args = args[1:]
	}

	output, err := p.runCommand("build", args, dir, outputHandler) // Capture output
	if err != nil {
		return nil, "", err
	}

	// Check if this is a Docker build by analyzing the command or output
	isDockerBuild := false
	for _, arg := range args {
		if strings.Contains(arg, "-only=*docker.*") {
			isDockerBuild = true
			break
		}
	}

	// If Docker registry is configured or it's a Docker build, parse image hashes
	if p.Container.ImageRegistry.Server != "" || isDockerBuild {
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
func (p *PackerTemplates) RunInit(args []string, dir string) error {
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
func (p *PackerTemplates) RunValidate(args []string, dir string) error {
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
func (p *PackerTemplates) RunVersion() (string, error) {
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
