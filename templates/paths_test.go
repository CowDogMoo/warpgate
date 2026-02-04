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

package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsGitURL(t *testing.T) {
	pv := NewPathValidator()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "https URL",
			input: "https://git.example.com/jdoe/repo.git",
			want:  true,
		},
		{
			name:  "http URL",
			input: "http://git.example.com/jdoe/repo.git",
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
			if got := pv.IsGitURL(tt.input); got != tt.want {
				t.Errorf("IsGitURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	pv := NewPathValidator()
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "home directory path",
			input:   "~/templates",
			want:    filepath.Join(home, "templates"),
			wantErr: false,
		},
		{
			name:    "relative path with dot",
			input:   "./templates",
			want:    filepath.Join(cwd, "templates"),
			wantErr: false,
		},
		{
			name:    "relative path with parent",
			input:   "../templates",
			want:    filepath.Join(filepath.Dir(cwd), "templates"),
			wantErr: false,
		},
		{
			name:    "absolute path",
			input:   "/absolute/path/templates",
			want:    "/absolute/path/templates",
			wantErr: false,
		},
		{
			name:    "repository name without path indicators",
			input:   "official",
			want:    "official",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pv.NormalizePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateLocalPath(t *testing.T) {
	pv := NewPathValidator()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a temporary file
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid directory",
			path:    tmpDir,
			wantErr: false,
		},
		{
			name:    "file instead of directory",
			path:    tmpFile,
			wantErr: true,
			errMsg:  "not a directory",
		},
		{
			name:    "non-existent path",
			path:    filepath.Join(tmpDir, "nonexistent"),
			wantErr: true,
			errMsg:  "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidateLocalPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLocalPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateLocalPath() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	pv := NewPathValidator()
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "home directory path",
			input:   "~/templates",
			want:    filepath.Join(home, "templates"),
			wantErr: false,
		},
		{
			name:    "relative path",
			input:   "templates",
			want:    filepath.Join(cwd, "templates"),
			wantErr: false,
		},
		{
			name:    "absolute path unchanged",
			input:   "/absolute/path",
			want:    "/absolute/path",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pv.ExpandPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExpandPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name   string
		gitURL string
		want   string
	}{
		{
			name:   "https URL with .git",
			gitURL: "https://git.example.com/jdoe/my-templates.git",
			want:   "my-templates",
		},
		{
			name:   "https URL without .git",
			gitURL: "https://git.example.com/jdoe/my-templates",
			want:   "my-templates",
		},
		{
			name:   "git SSH URL",
			gitURL: "git@github.com:user/my-repo.git",
			want:   "my-repo",
		},
		{
			name:   "complex URL with multiple slashes",
			gitURL: "https://gitlab.com/group/subgroup/my-project.git",
			want:   "my-project",
		},
		{
			name:   "URL with trailing slash",
			gitURL: "https://git.example.com/jdoe/repo.git/",
			want:   "templates",
		},
		{
			name:   "empty URL",
			gitURL: "",
			want:   "templates",
		},
		{
			name:   "URL with only domain",
			gitURL: "https://github.com",
			want:   "github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractRepoName(tt.gitURL); got != tt.want {
				t.Errorf("ExtractRepoName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGitURL_SSHURLs(t *testing.T) {
	t.Parallel()

	pv := NewPathValidator()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "git@ with github",
			input: "git@github.com:user/repo.git",
			want:  true,
		},
		{
			name:  "git@ with gitlab",
			input: "git@gitlab.com:group/project.git",
			want:  true,
		},
		{
			name:  "git@ with custom host",
			input: "git@git.mycompany.com:team/repo.git",
			want:  true,
		},
		{
			name:  "git@ with port-like path",
			input: "git@host.com:2222/user/repo.git",
			want:  true,
		},
		{
			name:  "not git@ prefix",
			input: "notgit@github.com:user/repo.git",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pv.IsGitURL(tt.input); got != tt.want {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPath_EnvVars(t *testing.T) {
	// Set a test environment variable
	t.Setenv("WARPGATE_TEST_VAR", "/custom/path")

	result, err := ExpandPath("${WARPGATE_TEST_VAR}/templates")
	if err != nil {
		t.Fatalf("ExpandPath() error = %v", err)
	}

	if result != "/custom/path/templates" {
		t.Errorf("ExpandPath() = %q, want %q", result, "/custom/path/templates")
	}
}

func TestExpandPath_TildeOnly(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home dir")
	}

	result, err := ExpandPath("~")
	if err != nil {
		t.Fatalf("ExpandPath(~) error = %v", err)
	}

	if result != home {
		t.Errorf("ExpandPath(~) = %q, want %q", result, home)
	}
}

func TestNormalizePath_RepositoryName(t *testing.T) {
	t.Parallel()

	pv := NewPathValidator()

	// A simple name without path indicators should be returned as-is
	result, err := pv.NormalizePath("official")
	if err != nil {
		t.Fatalf("NormalizePath() error = %v", err)
	}
	if result != "official" {
		t.Errorf("NormalizePath() = %q, want %q", result, "official")
	}
}

func TestValidateLocalPath_NonExistent(t *testing.T) {
	t.Parallel()

	pv := NewPathValidator()

	err := pv.ValidateLocalPath("/definitely/not/a/real/path/xyz")
	if err == nil {
		t.Fatal("ValidateLocalPath() expected error for non-existent path, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("ValidateLocalPath() error = %v, want error containing 'does not exist'", err)
	}
}

func TestValidateLocalPath_FileInsteadOfDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	pv := NewPathValidator()

	err := pv.ValidateLocalPath(tmpFile)
	if err == nil {
		t.Fatal("ValidateLocalPath() expected error for file instead of dir, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("ValidateLocalPath() error = %v, want error containing 'not a directory'", err)
	}
}

func TestMustExpandPath(t *testing.T) {
	t.Parallel()

	// MustExpandPath with a valid path should not panic
	home, _ := os.UserHomeDir()
	result := MustExpandPath("~/test")
	expected := filepath.Join(home, "test")
	if result != expected {
		t.Errorf("MustExpandPath(~/test) = %q, want %q", result, expected)
	}
}

func TestFileExistsAndDirExists(t *testing.T) {
	t.Parallel()

	pv := NewPathValidator()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// FileExists should work for both files and dirs
	if !pv.FileExists(tmpFile) {
		t.Error("FileExists() returned false for existing file")
	}
	if !pv.FileExists(tmpDir) {
		t.Error("FileExists() returned false for existing directory")
	}
	if pv.FileExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("FileExists() returned true for non-existent path")
	}

	// DirExists should only work for dirs
	if !pv.DirExists(tmpDir) {
		t.Error("DirExists() returned false for existing directory")
	}
	if pv.DirExists(tmpFile) {
		t.Error("DirExists() returned true for file")
	}
	if pv.DirExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("DirExists() returned true for non-existent path")
	}
}

func TestExtractRepoName_GitSSHFormat(t *testing.T) {
	tests := []struct {
		name   string
		gitURL string
		want   string
	}{
		{
			name:   "git SSH with single repo",
			gitURL: "git@github.com:user/repo.git",
			want:   "repo",
		},
		{
			name:   "git SSH with nested path",
			gitURL: "git@gitlab.com:group/subgroup/project.git",
			want:   "project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractRepoName(tt.gitURL); got != tt.want {
				t.Errorf("ExtractRepoName() = %v, want %v", got, tt.want)
			}
		})
	}
}
