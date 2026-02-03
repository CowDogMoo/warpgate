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

package main

import (
	"strings"
	"testing"
)

func TestParseTemplatesAddArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantName    string
		wantURL     string
		wantErr     bool
		errContains string
	}{
		{
			name:     "one arg - git URL",
			args:     []string{"https://github.com/user/templates.git"},
			wantName: "",
			wantURL:  "https://github.com/user/templates.git",
		},
		{
			name:     "one arg - local path",
			args:     []string{"/home/user/templates"},
			wantName: "",
			wantURL:  "/home/user/templates",
		},
		{
			name:     "two args - named git URL",
			args:     []string{"my-templates", "https://github.com/user/templates.git"},
			wantName: "my-templates",
			wantURL:  "https://github.com/user/templates.git",
		},
		{
			name:        "two args - local path is invalid",
			args:        []string{"my-name", "/local/path"},
			wantErr:     true,
			errContains: "must be a git URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			name, urlOrPath, err := parseTemplatesAddArgs(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if urlOrPath != tt.wantURL {
				t.Errorf("urlOrPath = %q, want %q", urlOrPath, tt.wantURL)
			}
		})
	}
}
