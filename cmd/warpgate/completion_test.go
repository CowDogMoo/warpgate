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
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCommandStructure(t *testing.T) {
	t.Parallel()

	if completionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("completionCmd.Use = %q, unexpected", completionCmd.Use)
	}

	// Verify valid args (cobra may sort these alphabetically)
	expectedArgs := map[string]bool{"bash": true, "zsh": true, "fish": true, "powershell": true}
	if len(completionCmd.ValidArgs) != len(expectedArgs) {
		t.Fatalf("ValidArgs length = %d, want %d", len(completionCmd.ValidArgs), len(expectedArgs))
	}
	for _, arg := range completionCmd.ValidArgs {
		if !expectedArgs[arg] {
			t.Errorf("unexpected ValidArg: %q", arg)
		}
	}
}

func TestCompletionCommandArgsValidation(t *testing.T) {
	t.Parallel()

	// ExactArgs(1) and OnlyValidArgs should reject 0 args
	err := cobra.ExactArgs(1)(completionCmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}

	// ExactArgs(1) should accept 1 arg
	err = cobra.ExactArgs(1)(completionCmd, []string{"bash"})
	if err != nil {
		t.Errorf("expected no error for 1 arg, got: %v", err)
	}

	// ExactArgs(1) should reject 2 args
	err = cobra.ExactArgs(1)(completionCmd, []string{"bash", "zsh"})
	if err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestCompletionCommandDisableFlags(t *testing.T) {
	t.Parallel()

	if !completionCmd.DisableFlagsInUseLine {
		t.Error("DisableFlagsInUseLine should be true")
	}
}
