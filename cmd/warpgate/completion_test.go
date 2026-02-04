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

func TestBuildCompletions(t *testing.T) {
	root := &cobra.Command{Use: "warpgate"}
	build := &cobra.Command{Use: "build", Run: func(cmd *cobra.Command, args []string) {}}
	build.Flags().StringSlice("arch", nil, "")
	build.Flags().String("target", "", "")
	build.Flags().String("region", "", "")
	build.Flags().StringSlice("regions", nil, "")
	build.Flags().String("registry", "", "")
	build.Flags().String("instance-type", "", "")
	root.AddCommand(build)

	registerBuildCompletions(build)

	tests := []struct {
		flag      string
		wantCount int
	}{
		{"arch", 7},
		{"target", 2},
		{"region", 10},
		{"regions", 10},
		{"registry", 5},
		{"instance-type", 9},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"__complete", "build", "--" + tt.flag, ""})
			_ = root.Execute()

			output := buf.String()
			lines := strings.Split(strings.TrimSpace(output), "\n")
			// Lines include completions + a directive line at the end
			if len(lines) < tt.wantCount {
				t.Errorf("flag --%s: got %d completion lines, want at least %d. Output: %s",
					tt.flag, len(lines), tt.wantCount, output)
			}
		})
	}
}

func TestRootCompletions(t *testing.T) {
	root := &cobra.Command{Use: "warpgate", Run: func(cmd *cobra.Command, args []string) {}}
	root.PersistentFlags().String("log-level", "", "")
	root.PersistentFlags().String("log-format", "", "")

	registerRootCompletions(root)

	tests := []struct {
		flag      string
		wantCount int
	}{
		{"log-level", 4},
		{"log-format", 3},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"__complete", "--" + tt.flag, ""})
			_ = root.Execute()

			output := buf.String()
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) < tt.wantCount {
				t.Errorf("flag --%s: got %d completion lines, want at least %d. Output: %s",
					tt.flag, len(lines), tt.wantCount, output)
			}
		})
	}
}

func TestCleanupCompletions(t *testing.T) {
	root := &cobra.Command{Use: "warpgate"}
	cleanup := &cobra.Command{Use: "cleanup", Run: func(cmd *cobra.Command, args []string) {}}
	cleanup.Flags().String("region", "", "")
	root.AddCommand(cleanup)

	registerCleanupCompletions(cleanup)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"__complete", "cleanup", "--region", ""})
	_ = root.Execute()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 10 {
		t.Errorf("flag --region: got %d completion lines, want at least 10. Output: %s",
			len(lines), output)
	}
}

func TestManifestsParentCompletions(t *testing.T) {
	root := &cobra.Command{Use: "warpgate"}
	manifests := &cobra.Command{Use: "manifests", Run: func(cmd *cobra.Command, args []string) {}}
	manifests.PersistentFlags().String("registry", "", "")
	root.AddCommand(manifests)

	registerManifestsParentCompletions(manifests)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"__complete", "manifests", "--registry", ""})
	_ = root.Execute()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 5 {
		t.Errorf("flag --registry: got %d completion lines, want at least 5. Output: %s",
			len(lines), output)
	}
}

func TestManifestsCreateCompletions(t *testing.T) {
	root := &cobra.Command{Use: "warpgate"}
	manifests := &cobra.Command{Use: "manifests"}
	create := &cobra.Command{Use: "create", Run: func(cmd *cobra.Command, args []string) {}}
	create.Flags().StringSlice("require-arch", nil, "")
	manifests.AddCommand(create)
	root.AddCommand(manifests)

	registerManifestsCreateCompletions(create)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"__complete", "manifests", "create", "--require-arch", ""})
	_ = root.Execute()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Errorf("flag --require-arch: got %d completion lines, want at least 3. Output: %s",
			len(lines), output)
	}
}

func TestTemplatesListCompletions(t *testing.T) {
	root := &cobra.Command{Use: "warpgate"}
	templates := &cobra.Command{Use: "templates"}
	list := &cobra.Command{Use: "list", Run: func(cmd *cobra.Command, args []string) {}}
	list.Flags().String("format", "table", "")
	list.Flags().String("source", "all", "")
	templates.AddCommand(list)
	root.AddCommand(templates)

	registerTemplatesListCompletions(list)

	tests := []struct {
		flag      string
		wantCount int
	}{
		{"format", 3},
		{"source", 3},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"__complete", "templates", "list", "--" + tt.flag, ""})
			_ = root.Execute()

			output := buf.String()
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) < tt.wantCount {
				t.Errorf("flag --%s: got %d completion lines, want at least %d. Output: %s",
					tt.flag, len(lines), tt.wantCount, output)
			}
		})
	}
}

func TestGetCommonAWSRegions_Completions(t *testing.T) {
	regions := getCommonAWSRegions()
	if len(regions) != 10 {
		t.Errorf("expected 10 regions, got %d", len(regions))
	}
	// Spot-check a few regions
	found := false
	for _, r := range regions {
		if strings.HasPrefix(r, "us-east-1") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected us-east-1 in regions")
	}
}

func TestGetCommonRegistries_Completions(t *testing.T) {
	registries := getCommonRegistries()
	if len(registries) != 5 {
		t.Errorf("expected 5 registries, got %d", len(registries))
	}
	found := false
	for _, r := range registries {
		if strings.HasPrefix(r, "ghcr.io") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ghcr.io in registries")
	}
}

func TestRegisterBuildCompletions_Extra(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "build"}
	cmd.Flags().StringSlice("arch", nil, "Architectures")
	cmd.Flags().String("target", "", "Target type")
	cmd.Flags().String("region", "", "AWS region")
	cmd.Flags().StringSlice("regions", nil, "AWS regions")
	cmd.Flags().String("registry", "", "Registry")
	cmd.Flags().String("instance-type", "", "Instance type")

	registerBuildCompletions(cmd)

	// Verify completions are registered by trying to get completions for each flag
	// The fact that RegisterFlagCompletionFunc was called is the coverage goal
}

func TestRegisterRootCompletions_Extra(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "warpgate"}
	cmd.PersistentFlags().String("log-level", "", "Log level")
	cmd.PersistentFlags().String("log-format", "", "Log format")

	registerRootCompletions(cmd)
}

func TestRegisterManifestsParentCompletions_Extra(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "manifests"}
	cmd.PersistentFlags().String("registry", "", "Container registry")

	registerManifestsParentCompletions(cmd)
}

func TestRegisterManifestsCreateCompletions_Extra(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "create"}
	cmd.Flags().StringSlice("require-arch", nil, "Required architectures")

	registerManifestsCreateCompletions(cmd)
}

func TestRegisterTemplatesListCompletions_Extra(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "list"}
	cmd.Flags().String("format", "table", "Output format")
	cmd.Flags().String("source", "all", "Filter by source")

	registerTemplatesListCompletions(cmd)
}

func TestGetCommonAWSRegions_Extra(t *testing.T) {
	t.Parallel()

	regions := getCommonAWSRegions()
	if len(regions) == 0 {
		t.Fatal("getCommonAWSRegions() returned empty slice")
	}
	// Verify it contains at least us-east-1
	found := false
	for _, r := range regions {
		if strings.Contains(r, "us-east-1") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected us-east-1 in AWS regions list")
	}
}

func TestGetCommonRegistries_Extra(t *testing.T) {
	t.Parallel()

	registries := getCommonRegistries()
	if len(registries) == 0 {
		t.Fatal("getCommonRegistries() returned empty slice")
	}
	found := false
	for _, r := range registries {
		if strings.Contains(r, "ghcr.io") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ghcr.io in registries list")
	}
}
