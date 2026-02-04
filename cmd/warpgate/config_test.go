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

func TestConfigCommandStructure(t *testing.T) {
	t.Parallel()

	if configCmd.Use != "config" {
		t.Errorf("configCmd.Use = %q, want %q", configCmd.Use, "config")
	}

	expectedSubcmds := []string{"init", "show", "path", "set", "get"}
	subCmds := configCmd.Commands()
	subCmdNames := make(map[string]bool)
	for _, c := range subCmds {
		subCmdNames[c.Name()] = true
	}

	for _, name := range expectedSubcmds {
		if !subCmdNames[name] {
			t.Errorf("missing config subcommand: %s", name)
		}
	}
}

func TestConfigInitCommandFlags(t *testing.T) {
	t.Parallel()

	f := configInitCmd.Flags().Lookup("force")
	if f == nil {
		t.Fatal("missing --force flag on config init")
	}
	if f.Shorthand != "f" {
		t.Errorf("--force shorthand = %q, want %q", f.Shorthand, "f")
	}
	if f.DefValue != "false" {
		t.Errorf("--force default = %q, want %q", f.DefValue, "false")
	}
}

func TestConfigShowNoArgs(t *testing.T) {
	t.Parallel()

	err := cobra.NoArgs(configShowCmd, []string{})
	if err != nil {
		t.Errorf("expected no error for 0 args, got: %v", err)
	}
}

func TestConfigPathNoArgs(t *testing.T) {
	t.Parallel()

	err := cobra.NoArgs(configPathCmd, []string{})
	if err != nil {
		t.Errorf("expected no error for 0 args, got: %v", err)
	}
}

func TestConfigSetExactArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"zero args", []string{}, true},
		{"one arg", []string{"key"}, true},
		{"two args", []string{"key", "value"}, false},
		{"three args", []string{"key", "value", "extra"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := cobra.ExactArgs(2)(configSetCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("cobra.ExactArgs(2)(%v) error = %v, wantErr = %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestConfigGetExactArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"zero args", []string{}, true},
		{"one arg", []string{"key"}, false},
		{"two args", []string{"key", "extra"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := cobra.ExactArgs(1)(configGetCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("cobra.ExactArgs(1)(%v) error = %v, wantErr = %v", tt.args, err, tt.wantErr)
			}
		})
	}
}
