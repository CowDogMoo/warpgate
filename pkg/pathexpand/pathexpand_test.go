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

package pathexpand

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "Tilde only",
			input:    "~",
			expected: home,
			wantErr:  false,
		},
		{
			name:     "Tilde with path",
			input:    "~/projects",
			expected: filepath.Join(home, "projects"),
			wantErr:  false,
		},
		{
			name:     "Tilde with deep path",
			input:    "~/cowdogmoo/ansible-collection-workstation",
			expected: filepath.Join(home, "cowdogmoo/ansible-collection-workstation"),
			wantErr:  false,
		},
		{
			name:     "Absolute path unchanged",
			input:    "/opt/shared/templates",
			expected: "/opt/shared/templates",
			wantErr:  false,
		},
		{
			name:     "Relative path unchanged",
			input:    "./templates",
			expected: "./templates",
			wantErr:  false,
		},
		{
			name:     "Environment variable expansion",
			input:    "${HOME}/work",
			expected: filepath.Join(home, "work"),
			wantErr:  false,
		},
		{
			name:     "Empty string unchanged",
			input:    "",
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ExpandPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMustExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Tilde with path",
			input:    "~/projects",
			expected: filepath.Join(home, "projects"),
		},
		{
			name:     "Absolute path unchanged",
			input:    "/opt/templates",
			expected: "/opt/templates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MustExpandPath(tt.input)
			if result != tt.expected {
				t.Errorf("MustExpandPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}
