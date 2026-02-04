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
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCommand_Bash(t *testing.T) {
	buf := new(bytes.Buffer)
	root := &cobra.Command{Use: "warpgate"}
	root.AddCommand(completionCmd)
	root.SetOut(buf)

	root.SetArgs([]string{"completion", "bash"})
	err := root.Execute()
	if err != nil {
		t.Errorf("completion bash command returned error: %v", err)
	}
}

func TestCompletionCommand_Zsh(t *testing.T) {
	buf := new(bytes.Buffer)
	root := &cobra.Command{Use: "warpgate"}
	root.AddCommand(completionCmd)
	root.SetOut(buf)

	root.SetArgs([]string{"completion", "zsh"})
	err := root.Execute()
	if err != nil {
		t.Errorf("completion zsh command returned error: %v", err)
	}
}

func TestCompletionCommand_Fish(t *testing.T) {
	buf := new(bytes.Buffer)
	root := &cobra.Command{Use: "warpgate"}
	root.AddCommand(completionCmd)
	root.SetOut(buf)

	root.SetArgs([]string{"completion", "fish"})
	err := root.Execute()
	if err != nil {
		t.Errorf("completion fish command returned error: %v", err)
	}
}

func TestCompletionCommand_Powershell(t *testing.T) {
	buf := new(bytes.Buffer)
	root := &cobra.Command{Use: "warpgate"}
	root.AddCommand(completionCmd)
	root.SetOut(buf)

	root.SetArgs([]string{"completion", "powershell"})
	err := root.Execute()
	if err != nil {
		t.Errorf("completion powershell command returned error: %v", err)
	}
}

func TestCompletionCommand_InvalidArg(t *testing.T) {
	buf := new(bytes.Buffer)
	root := &cobra.Command{Use: "warpgate"}
	root.AddCommand(completionCmd)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "invalid"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid completion arg")
	}
}
