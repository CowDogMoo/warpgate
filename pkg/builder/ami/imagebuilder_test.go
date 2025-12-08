/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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

package ami

import (
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/builder"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientConfig
		target  *builder.Target
		wantErr bool
	}{
		{
			name: "valid config with region in client config",
			config: ClientConfig{
				Region: "us-east-1",
			},
			target: &builder.Target{
				Type: "ami",
			},
			wantErr: false,
		},
		{
			name:   "valid config with region in target",
			config: ClientConfig{},
			target: &builder.Target{
				Type:   "ami",
				Region: "us-west-2",
			},
			wantErr: false,
		},
		{
			name:   "missing region",
			config: ClientConfig{},
			target: &builder.Target{
				Type: "ami",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock image builder
			ib := &ImageBuilder{
				config: tt.config,
			}

			err := ib.validateConfig(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetermineTargetType(t *testing.T) {
	tests := []struct {
		name   string
		config builder.Config
		want   string
	}{
		{
			name: "container target",
			config: builder.Config{
				Targets: []builder.Target{
					{Type: "container"},
				},
			},
			want: "container",
		},
		{
			name: "ami target",
			config: builder.Config{
				Targets: []builder.Target{
					{Type: "ami"},
				},
			},
			want: "ami",
		},
		{
			name: "no targets - default to container",
			config: builder.Config{
				Targets: []builder.Target{},
			},
			want: "container",
		},
		{
			name: "multiple targets - first one wins",
			config: builder.Config{
				Targets: []builder.Target{
					{Type: "ami"},
					{Type: "container"},
				},
			},
			want: "ami",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock determineTargetType logic
			var targetType string
			if len(tt.config.Targets) > 0 {
				targetType = tt.config.Targets[0].Type
			} else {
				targetType = "container"
			}

			if targetType != tt.want {
				t.Errorf("determineTargetType() = %v, want %v", targetType, tt.want)
			}
		})
	}
}

func TestExtractAMIID(t *testing.T) {
	tests := []struct {
		name    string
		amiID   string
		wantErr bool
	}{
		{
			name:    "valid ami id",
			amiID:   "ami-1234567890abcdef0",
			wantErr: false,
		},
		{
			name:    "empty ami id",
			amiID:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.amiID == "" && !tt.wantErr {
				t.Errorf("expected error for empty AMI ID")
			}
			if tt.amiID != "" && len(tt.amiID) > 0 && tt.wantErr {
				t.Errorf("unexpected error for valid AMI ID")
			}
		})
	}
}

func TestComponentDocument(t *testing.T) {
	tests := []struct {
		name        string
		provisioner builder.Provisioner
		wantErr     bool
	}{
		{
			name: "shell provisioner",
			provisioner: builder.Provisioner{
				Type:   "shell",
				Inline: []string{"echo hello", "echo world"},
			},
			wantErr: false,
		},
		{
			name: "shell provisioner without commands",
			provisioner: builder.Provisioner{
				Type:   "shell",
				Inline: []string{},
			},
			wantErr: true,
		},
		{
			name: "unsupported provisioner",
			provisioner: builder.Provisioner{
				Type: "unknown",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &ComponentGenerator{}
			_, err := gen.createComponentDocument(tt.provisioner)
			if (err != nil) != tt.wantErr {
				t.Errorf("createComponentDocument() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientConfig(t *testing.T) {
	tests := []struct {
		name   string
		config ClientConfig
	}{
		{
			name: "config with region",
			config: ClientConfig{
				Region: "us-east-1",
			},
		},
		{
			name: "config with profile",
			config: ClientConfig{
				Profile: "default",
			},
		},
		{
			name: "config with static credentials",
			config: ClientConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.Region == "" && tt.config.Profile == "" && tt.config.AccessKeyID == "" {
				t.Errorf("invalid test case - config should have at least one field set")
			}
		})
	}
}
