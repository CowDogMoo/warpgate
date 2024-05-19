package packer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cowdogmoo/warpgate/pkg/packer"
)

func setupPackerTemplate(t *testing.T, tempDir string) {
	templateContent := `
variable "base_image" {
  type = string
  default = "ubuntu"
}

source "null" "example" {
  communicator = "none"
}

build {
  sources = ["source.null.example"]
}
`
	templateDir := filepath.Join(tempDir, "packer_templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("failed to create packer templates directory: %v", err)
	}

	templatePath := filepath.Join(tempDir, "template.pkr.hcl")
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("failed to write packer template file: %v", err)
	}
}

func TestRunBuild(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "packer_test")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	setupPackerTemplate(t, tempDir)

	tests := []struct {
		name          string
		args          []string
		dir           string
		expectedError bool
	}{
		{
			name:          "valid packer build command",
			args:          []string{"."},
			dir:           tempDir,
			expectedError: false,
		},
		{
			name:          "invalid packer build command",
			args:          []string{"invalid_command"},
			dir:           tempDir,
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packerTemplate := packer.PackerTemplate{}
			_, _, err := packerTemplate.RunBuild(tc.args, tc.dir)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunInit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "packer_test")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	setupPackerTemplate(t, tempDir)

	tests := []struct {
		name          string
		args          []string
		dir           string
		expectedError bool
	}{
		{
			name:          "valid packer init command",
			args:          []string{"."},
			dir:           tempDir,
			expectedError: false,
		},
		{
			name:          "invalid packer init command",
			args:          []string{"invalid_command"},
			dir:           tempDir,
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packerTemplate := packer.PackerTemplate{}
			err := packerTemplate.RunInit(tc.args, tc.dir)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunValidate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "packer_test")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	setupPackerTemplate(t, tempDir)

	tests := []struct {
		name          string
		args          []string
		dir           string
		expectedError bool
	}{
		{
			name:          "valid packer validate command",
			args:          []string{"."},
			dir:           tempDir,
			expectedError: false,
		},
		{
			name:          "invalid packer validate command",
			args:          []string{"invalid_command"},
			dir:           tempDir,
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packerTemplate := packer.PackerTemplate{}
			err := packerTemplate.RunValidate(tc.args, tc.dir)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	tests := []struct {
		name          string
		expectedError bool
	}{
		{
			name:          "valid packer version command",
			expectedError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packerTemplate := packer.PackerTemplate{}
			version, err := packerTemplate.RunVersion()
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, version)
			}
		})
	}
}
