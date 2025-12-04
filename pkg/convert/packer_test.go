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

package convert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPackerConverter(t *testing.T) {
	opts := PackerConverterOptions{
		TemplateDir: "/path/to/template",
		Author:      "Test Author",
		License:     "MIT",
		Version:     "1.0.0",
	}

	converter := NewPackerConverter(opts)

	assert.NotNil(t, converter)
	assert.Equal(t, "/path/to/template", converter.options.TemplateDir)
	assert.Equal(t, "Test Author", converter.options.Author)
	assert.Equal(t, "MIT", converter.options.License)
	assert.Equal(t, "1.0.0", converter.options.Version)
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name        string
		readmeData  string
		expected    string
		createFile  bool
		templateDir string
	}{
		{
			name: "extract description from README",
			readmeData: `# My Template

This is a test template for security testing.

Some more content here.`,
			expected:    "This is a test template for security testing.",
			createFile:  true,
			templateDir: "test-template",
		},
		{
			name:        "no README file",
			readmeData:  "",
			expected:    "test-template security tooling image",
			createFile:  false,
			templateDir: "test-template",
		},
		{
			name: "empty README",
			readmeData: `# My Template

`,
			expected:    "test-template security tooling image",
			createFile:  true,
			templateDir: "test-template",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			templateDir := filepath.Join(tmpDir, tc.templateDir)
			err := os.MkdirAll(templateDir, 0755)
			require.NoError(t, err)

			// Create README if needed
			if tc.createFile {
				readmePath := filepath.Join(templateDir, "README.md")
				err := os.WriteFile(readmePath, []byte(tc.readmeData), 0644)
				require.NoError(t, err)
			}

			converter := NewPackerConverter(PackerConverterOptions{
				TemplateDir: templateDir,
			})

			result := converter.extractDescription()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractBaseImage(t *testing.T) {
	tests := []struct {
		name            string
		variablesData   string
		expectedImage   string
		expectedVersion string
		createFile      bool
	}{
		{
			name: "extract base image from variables.pkr.hcl",
			variablesData: `variable "base_image" {
  type    = string
  default = "ubuntu"
}

variable "base_image_version" {
  type    = string
  default = "22.04"
}`,
			expectedImage:   "ubuntu",
			expectedVersion: "22.04",
			createFile:      true,
		},
		{
			name:            "no variables file",
			variablesData:   "",
			expectedImage:   "ubuntu",
			expectedVersion: "latest",
			createFile:      false,
		},
		{
			name: "custom base image",
			variablesData: `variable "base_image" {
  type    = string
  default = "debian"
}

variable "base_image_version" {
  type    = string
  default = "bullseye"
}`,
			expectedImage:   "debian",
			expectedVersion: "bullseye",
			createFile:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Create variables file if needed
			if tc.createFile {
				varsPath := filepath.Join(tmpDir, "variables.pkr.hcl")
				err := os.WriteFile(varsPath, []byte(tc.variablesData), 0644)
				require.NoError(t, err)
			}

			converter := NewPackerConverter(PackerConverterOptions{
				TemplateDir: tmpDir,
			})

			image, version := converter.extractBaseImage()
			assert.Equal(t, tc.expectedImage, image)
			assert.Equal(t, tc.expectedVersion, version)
		})
	}
}

func TestReplaceVarReferences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replace provision_repo_path variable",
			input:    "${var.provision_repo_path}/ansible/playbook.yml",
			expected: "${PROVISION_REPO_PATH}/ansible/playbook.yml",
		},
		{
			name:     "replace multiple variables",
			input:    "${var.provision_repo_path}/ansible/${var.template_name}.yml",
			expected: "${PROVISION_REPO_PATH}/ansible/${TEMPLATE_NAME}.yml",
		},
		{
			name:     "no variables to replace",
			input:    "/path/to/playbook.yml",
			expected: "/path/to/playbook.yml",
		},
		{
			name:     "mixed case variable",
			input:    "${var.Template_Name}",
			expected: "${TEMPLATE_NAME}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			converter := NewPackerConverter(PackerConverterOptions{})
			result := converter.replaceVarReferences(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseShellProvisioner(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []string
		isNil    bool
	}{
		{
			name: "parse inline commands",
			body: `  inline = [
    "apt-get update",
    "apt-get install -y curl"
  ]`,
			expected: []string{"apt-get update", "apt-get install -y curl"},
			isNil:    false,
		},
		{
			name: "parse single command",
			body: `  inline = [
    "echo hello"
  ]`,
			expected: []string{"echo hello"},
			isNil:    false,
		},
		{
			name:     "no inline commands",
			body:     `  script = "install.sh"`,
			expected: nil,
			isNil:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			converter := NewPackerConverter(PackerConverterOptions{})
			result := converter.parseShellProvisioner(tc.body)

			if tc.isNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, "shell", result.Type)
				assert.Equal(t, tc.expected, result.Inline)
			}
		})
	}
}

func TestParseAnsibleProvisioner(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		expectedPlaybook string
		expectedGalaxy   string
		isNil            bool
	}{
		{
			name: "parse ansible provisioner with galaxy",
			body: `  playbook_file = "${var.provision_repo_path}/ansible/playbook.yml"
  galaxy_file = "${var.provision_repo_path}/ansible/requirements.yml"`,
			expectedPlaybook: "${PROVISION_REPO_PATH}/ansible/playbook.yml",
			expectedGalaxy:   "${PROVISION_REPO_PATH}/ansible/requirements.yml",
			isNil:            false,
		},
		{
			name:             "parse ansible provisioner without galaxy",
			body:             `  playbook_file = "/path/to/playbook.yml"`,
			expectedPlaybook: "/path/to/playbook.yml",
			expectedGalaxy:   "",
			isNil:            false,
		},
		{
			name:  "no playbook file",
			body:  `  some_other_field = "value"`,
			isNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			converter := NewPackerConverter(PackerConverterOptions{})
			result := converter.parseAnsibleProvisioner(tc.body)

			if tc.isNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, "ansible", result.Type)
				assert.Equal(t, tc.expectedPlaybook, result.PlaybookPath)
				assert.Equal(t, tc.expectedGalaxy, result.GalaxyFile)
			}
		})
	}
}

func TestExtractAnsibleExtraVars(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected map[string]string
	}{
		{
			name: "extract shell executable",
			body: `  extra_arguments = [
    "-e", "ansible_shell_executable=${var.shell}"
  ]`,
			expected: map[string]string{
				"ansible_shell_executable": "/bin/bash",
			},
		},
		{
			name:     "no extra vars",
			body:     `  playbook_file = "test.yml"`,
			expected: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			converter := NewPackerConverter(PackerConverterOptions{})
			result := converter.extractAnsibleExtraVars(tc.body)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseProvisionersFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name: "parse multiple provisioners",
			content: `build {
  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y curl"
    ]
  }

  provisioner "ansible" {
    playbook_file = "/path/to/playbook.yml"
  }
}`,
			expected: 2,
		},
		{
			name: "parse nested braces",
			content: `build {
  provisioner "shell" {
    inline = [
      "if [ -f /test ]; then { echo 'found'; }; fi"
    ]
  }
}`,
			expected: 1,
		},
		{
			name:     "no provisioners",
			content:  `build { }`,
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			converter := NewPackerConverter(PackerConverterOptions{})
			result := converter.parseProvisionersFromContent(tc.content)
			assert.Len(t, result, tc.expected)
		})
	}
}

func TestBuildTargets(t *testing.T) {
	tests := []struct {
		name        string
		includeAMI  bool
		expectedLen int
	}{
		{
			name:        "container target only",
			includeAMI:  false,
			expectedLen: 1,
		},
		{
			name:        "container and AMI targets",
			includeAMI:  true,
			expectedLen: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			converter := NewPackerConverter(PackerConverterOptions{
				IncludeAMI: tc.includeAMI,
			})
			result := converter.buildTargets()
			assert.Len(t, result, tc.expectedLen)

			// Verify container target
			assert.Equal(t, "container", result[0].Type)
			assert.Contains(t, result[0].Platforms, "linux/amd64")
			assert.Contains(t, result[0].Platforms, "linux/arm64")

			// Verify AMI target if included
			if tc.includeAMI {
				assert.Equal(t, "ami", result[1].Type)
				assert.Equal(t, "us-east-1", result[1].Region)
			}
		})
	}
}

func TestConvertEndToEnd(t *testing.T) {
	// Create temporary directory with complete Packer template structure
	tmpDir := t.TempDir()

	// Create README.md
	readmeContent := `# Test Template

This is a test template for unit testing.

More content here.`
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readmeContent), 0644)
	require.NoError(t, err)

	// Create variables.pkr.hcl
	varsContent := `variable "base_image" {
  type    = string
  default = "ubuntu"
}

variable "base_image_version" {
  type    = string
  default = "22.04"
}`
	err = os.WriteFile(filepath.Join(tmpDir, "variables.pkr.hcl"), []byte(varsContent), 0644)
	require.NoError(t, err)

	// Create docker.pkr.hcl
	dockerContent := `build {
  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y curl"
    ]
  }

  provisioner "ansible" {
    playbook_file = "${var.provision_repo_path}/ansible/playbook.yml"
    galaxy_file = "${var.provision_repo_path}/ansible/requirements.yml"
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Run conversion
	converter := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test Author",
		License:     "MIT",
		Version:     "1.0.0",
	})

	config, err := converter.Convert()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify metadata
	assert.Equal(t, filepath.Base(tmpDir), config.Metadata.Name)
	assert.Equal(t, "1.0.0", config.Metadata.Version)
	assert.Equal(t, "This is a test template for unit testing.", config.Metadata.Description)
	assert.Equal(t, "Test Author", config.Metadata.Author)
	assert.Equal(t, "MIT", config.Metadata.License)

	// Verify base image
	assert.Equal(t, "ubuntu:22.04", config.Base.Image)
	assert.True(t, config.Base.Pull)

	// Verify provisioners
	assert.Len(t, config.Provisioners, 2)
	assert.Equal(t, "shell", config.Provisioners[0].Type)
	assert.Equal(t, "ansible", config.Provisioners[1].Type)

	// Verify shell provisioner
	assert.Contains(t, config.Provisioners[0].Inline, "apt-get update")
	assert.Contains(t, config.Provisioners[0].Inline, "apt-get install -y curl")

	// Verify ansible provisioner
	assert.Equal(t, "${PROVISION_REPO_PATH}/ansible/playbook.yml", config.Provisioners[1].PlaybookPath)
	assert.Equal(t, "${PROVISION_REPO_PATH}/ansible/requirements.yml", config.Provisioners[1].GalaxyFile)

	// Verify targets
	assert.Len(t, config.Targets, 1)
	assert.Equal(t, "container", config.Targets[0].Type)
}

func TestConvertWithBaseImageOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal structure
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)
	require.NoError(t, err)

	// Run conversion with base image override
	converter := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		BaseImage:   "debian:bullseye",
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})

	config, err := converter.Convert()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify base image was overridden
	assert.Equal(t, "debian:bullseye:latest", config.Base.Image)
}
