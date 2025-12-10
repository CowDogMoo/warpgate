/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package cli

import (
	"testing"
)

func TestValidateBuildOptions(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		opts    BuildCLIOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options with config file",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
			},
			wantErr: false,
		},
		{
			name: "valid options with template",
			opts: BuildCLIOptions{
				Template: "attack-box",
			},
			wantErr: false,
		},
		{
			name: "invalid builder type",
			opts: BuildCLIOptions{
				ConfigFile:  "warpgate.yaml",
				BuilderType: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid builder type",
		},
		{
			name: "invalid label format",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Labels:     []string{"invalidlabel"},
			},
			wantErr: true,
			errMsg:  "invalid label format",
		},
		{
			name: "valid labels",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Labels:     []string{"version=1.0", "maintainer=team"},
			},
			wantErr: false,
		},
		{
			name: "invalid build-arg format",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				BuildArgs:  []string{"invalidarg"},
			},
			wantErr: true,
			errMsg:  "invalid build-arg format",
		},
		{
			name: "valid build-args",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				BuildArgs:  []string{"GO_VERSION=1.21", "BUILD_DATE=2024-01-01"},
			},
			wantErr: false,
		},
		{
			name: "invalid variable format",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Variables:  []string{"invalidvar"},
			},
			wantErr: true,
			errMsg:  "invalid variable format",
		},
		{
			name: "valid variables",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Variables:  []string{"VAR1=value1", "VAR2=value2"},
			},
			wantErr: false,
		},
		{
			name: "push without registry",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Push:       true,
			},
			wantErr: true,
			errMsg:  "--push requires --registry",
		},
		{
			name: "push with registry - valid",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Push:       true,
				Registry:   "ghcr.io/myorg",
			},
			wantErr: false,
		},
		{
			name: "save-digests without push",
			opts: BuildCLIOptions{
				ConfigFile:  "warpgate.yaml",
				SaveDigests: true,
			},
			wantErr: true,
			errMsg:  "--save-digests requires --push",
		},
		{
			name: "save-digests with push - valid",
			opts: BuildCLIOptions{
				ConfigFile:  "warpgate.yaml",
				Push:        true,
				Registry:    "ghcr.io/myorg",
				SaveDigests: true,
			},
			wantErr: false,
		},
		{
			name: "multiple input sources - template and config",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Template:   "attack-box",
			},
			wantErr: true,
			errMsg:  "only one of",
		},
		{
			name: "multiple input sources - template and git",
			opts: BuildCLIOptions{
				Template: "attack-box",
				FromGit:  "https://github.com/user/templates.git",
			},
			wantErr: true,
			errMsg:  "only one of",
		},
		{
			name: "multiple input sources - all three",
			opts: BuildCLIOptions{
				ConfigFile: "warpgate.yaml",
				Template:   "attack-box",
				FromGit:    "https://github.com/user/templates.git",
			},
			wantErr: true,
			errMsg:  "only one of",
		},
		{
			name: "valid builder type - buildkit",
			opts: BuildCLIOptions{
				ConfigFile:  "warpgate.yaml",
				BuilderType: "buildkit",
			},
			wantErr: false,
		},
		{
			name: "valid builder type - auto",
			opts: BuildCLIOptions{
				ConfigFile:  "warpgate.yaml",
				BuilderType: "auto",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateBuildOptions(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBuildOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || len(err.Error()) == 0 {
					t.Errorf("ValidateBuildOptions() expected error message containing %q, got nil error", tt.errMsg)
				}
			}
		})
	}
}

func TestValidateTemplateAddOptions(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		nameParm  string
		urlOrPath string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid git URL without name",
			nameParm:  "",
			urlOrPath: "https://github.com/user/templates.git",
			wantErr:   false,
		},
		{
			name:      "valid git URL with name",
			nameParm:  "my-templates",
			urlOrPath: "https://github.com/user/templates.git",
			wantErr:   false,
		},
		{
			name:      "local path without name",
			nameParm:  "",
			urlOrPath: "/path/to/templates",
			wantErr:   false,
		},
		{
			name:      "local path with name - invalid",
			nameParm:  "my-templates",
			urlOrPath: "/path/to/templates",
			wantErr:   true,
			errMsg:    "when providing a name",
		},
		{
			name:      "empty URL or path",
			nameParm:  "",
			urlOrPath: "",
			wantErr:   true,
			errMsg:    "URL or path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTemplateAddOptions(tt.nameParm, tt.urlOrPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTemplateAddOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || len(err.Error()) == 0 {
					t.Errorf("ValidateTemplateAddOptions() expected error message containing %q, got nil error", tt.errMsg)
				}
			}
		})
	}
}

func TestValidateConfigSetOptions(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config key",
			key:     "log.level",
			value:   "debug",
			wantErr: false,
		},
		{
			name:    "valid nested key",
			key:     "templates.local_paths",
			value:   "/path/to/templates",
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			value:   "value",
			wantErr: true,
			errMsg:  "key is required",
		},
		{
			name:    "empty value",
			key:     "log.level",
			value:   "",
			wantErr: true,
			errMsg:  "value is required",
		},
		{
			name:    "key starts with dot",
			key:     ".log.level",
			value:   "debug",
			wantErr: true,
			errMsg:  "invalid config key format",
		},
		{
			name:    "key ends with dot",
			key:     "log.level.",
			value:   "debug",
			wantErr: true,
			errMsg:  "invalid config key format",
		},
		{
			name:    "consecutive dots in key",
			key:     "log..level",
			value:   "debug",
			wantErr: true,
			errMsg:  "invalid config key format",
		},
		{
			name:    "simple key without dots",
			key:     "level",
			value:   "debug",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateConfigSetOptions(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfigSetOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || len(err.Error()) == 0 {
					t.Errorf("ValidateConfigSetOptions() expected error message containing %q, got nil error", tt.errMsg)
				}
			}
		})
	}
}

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "https URL",
			input: "https://github.com/user/repo.git",
			want:  true,
		},
		{
			name:  "http URL",
			input: "http://github.com/user/repo.git",
			want:  true,
		},
		{
			name:  "git SSH URL",
			input: "git@github.com:user/repo.git",
			want:  true,
		},
		{
			name:  "local path",
			input: "/path/to/templates",
			want:  false,
		},
		{
			name:  "relative path",
			input: "../templates",
			want:  false,
		},
		{
			name:  "home path",
			input: "~/templates",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitURL(tt.input); got != tt.want {
				t.Errorf("IsGitURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidConfigKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid simple key",
			input: "key",
			want:  true,
		},
		{
			name:  "valid nested key",
			input: "log.level",
			want:  true,
		},
		{
			name:  "valid deeply nested key",
			input: "templates.local_paths.default",
			want:  true,
		},
		{
			name:  "empty key",
			input: "",
			want:  false,
		},
		{
			name:  "starts with dot",
			input: ".key",
			want:  false,
		},
		{
			name:  "ends with dot",
			input: "key.",
			want:  false,
		},
		{
			name:  "consecutive dots",
			input: "key..value",
			want:  false,
		},
		{
			name:  "only dots",
			input: "...",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidConfigKey(tt.input); got != tt.want {
				t.Errorf("isValidConfigKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
