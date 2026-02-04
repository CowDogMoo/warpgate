/*
Copyright (c) 2025 Jayson Grace <jayson.e.grace@gmail.com>

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
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %q, want %q", versionCmd.Use, "version")
	}
	if versionCmd.Args == nil {
		t.Error("versionCmd should have args validation")
	}
}

func TestVersionCommandOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "warpgate"}
	cmd.AddCommand(versionCmd)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("version command returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "warpgate version") {
		t.Errorf("output should contain 'warpgate version', got: %q", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("output should contain 'commit:', got: %q", output)
	}
	if !strings.Contains(output, "built:") {
		t.Errorf("output should contain 'built:', got: %q", output)
	}
}

func TestVersionVariables(t *testing.T) {
	t.Parallel()

	// These are set at compile time or by debug.ReadBuildInfo
	// In test context, they should have default values
	if version == "" {
		t.Error("version should not be empty")
	}
	if commit == "" {
		t.Error("commit should not be empty")
	}
	if date == "" {
		t.Error("date should not be empty")
	}
}

func TestVersionCommand_OutputFields(t *testing.T) {
	buf := new(bytes.Buffer)
	versionCmd.SetOut(buf)
	versionCmd.SetErr(buf)
	defer func() {
		versionCmd.SetOut(nil)
		versionCmd.SetErr(nil)
	}()

	versionCmd.Run(versionCmd, []string{})
	output := buf.String()
	if !strings.Contains(output, "warpgate version") {
		t.Errorf("expected version output, got: %s", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("expected commit info in output, got: %s", output)
	}
	if !strings.Contains(output, "built:") {
		t.Errorf("expected built info in output, got: %s", output)
	}
}

func TestVersionCmd_Run(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "version"}
	buf := &strings.Builder{}
	cmd.SetOut(buf)

	// Call the same Run function that versionCmd uses
	cmd.Run = versionCmd.Run
	cmd.Run(cmd, []string{})

	output := buf.String()
	if !strings.Contains(output, "warpgate version") {
		t.Errorf("version output missing 'warpgate version': %q", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("version output missing 'commit:': %q", output)
	}
	if !strings.Contains(output, "built:") {
		t.Errorf("version output missing 'built:': %q", output)
	}
}
