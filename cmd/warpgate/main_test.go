package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestExecute(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	originalCfgFile := cfgFile
	cfgFile = ""
	t.Cleanup(func() {
		cfgFile = originalCfgFile
	})

	tests := []struct {
		name            string
		args            []string
		wantErr         bool
		wantErrContains string
		wantContains    string
	}{
		{
			name:         "help output",
			args:         []string{"--help"},
			wantContains: "Warpgate",
		},
		{
			name:            "unknown flag",
			args:            []string{"--unknown"},
			wantErr:         true,
			wantErrContains: "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs(tt.args)
			t.Cleanup(func() {
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(os.Stderr)
				rootCmd.SetArgs(nil)
			})

			err := Execute()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error %q missing %q", err.Error(), tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.wantContains != "" && !strings.Contains(buf.String(), tt.wantContains) {
				t.Errorf("output missing %q", tt.wantContains)
			}
		})
	}
}
