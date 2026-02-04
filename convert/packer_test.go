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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
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

	converter, err := NewPackerConverter(opts)

	require.NoError(t, err)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			templateDir := filepath.Join(tmpDir, tt.templateDir)
			err := os.MkdirAll(templateDir, 0755)
			require.NoError(t, err)

			// Create README if needed
			if tt.createFile {
				readmePath := filepath.Join(templateDir, "README.md")
				err := os.WriteFile(readmePath, []byte(tt.readmeData), 0644)
				require.NoError(t, err)
			}

			converter, err := NewPackerConverter(PackerConverterOptions{
				TemplateDir: templateDir,
			})
			require.NoError(t, err)

			result := converter.extractDescription(context.Background())
			assert.Equal(t, tt.expected, result)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter, err := NewPackerConverter(PackerConverterOptions{
				IncludeAMI: tt.includeAMI,
			})
			require.NoError(t, err)

			// Create HCL parser for buildTargets
			hclParser := NewHCLParser()
			result := converter.buildTargets(hclParser, "", nil, false)
			assert.Len(t, result, tt.expectedLen)

			// Verify container target
			assert.Equal(t, "container", result[0].Type)
			assert.Contains(t, result[0].Platforms, "linux/amd64")
			assert.Contains(t, result[0].Platforms, "linux/arm64")

			// Verify AMI target if included
			if tt.includeAMI {
				assert.Equal(t, "ami", result[1].Type)
				// Region now comes from config (defaults to empty)
				assert.Equal(t, "t3.micro", result[1].InstanceType)
				assert.Equal(t, 50, result[1].VolumeSize)
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
}

variable "provision_repo_path" {
  type    = string
  description = "Path to provision repo"
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
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test Author",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
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
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		BaseImage:   "debian:bullseye",
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify base image was overridden
	assert.Equal(t, "debian:bullseye:latest", config.Base.Image)
}

func TestConvertWithProvisionerConditionals(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)
	require.NoError(t, err)

	// Create variables.pkr.hcl
	varsContent := `variable "base_image" {
  type    = string
  default = "ubuntu"
}`
	err = os.WriteFile(filepath.Join(tmpDir, "variables.pkr.hcl"), []byte(varsContent), 0644)
	require.NoError(t, err)

	// Create docker.pkr.hcl with only/except conditionals
	dockerContent := `build {
  provisioner "shell" {
    only = ["docker.amd64", "docker.arm64"]
    inline = ["apt-get update"]
  }

  provisioner "shell" {
    except = ["amazon-ebs.ubuntu"]
    inline = ["echo 'docker only'"]
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Run conversion
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify provisioners with conditionals
	assert.Len(t, config.Provisioners, 2)

	// First provisioner with "only"
	assert.Equal(t, "shell", config.Provisioners[0].Type)
	assert.Len(t, config.Provisioners[0].Only, 2)
	assert.Contains(t, config.Provisioners[0].Only, "docker.amd64")
	assert.Contains(t, config.Provisioners[0].Only, "docker.arm64")

	// Second provisioner with "except"
	assert.Equal(t, "shell", config.Provisioners[1].Type)
	assert.Len(t, config.Provisioners[1].Except, 1)
	assert.Contains(t, config.Provisioners[1].Except, "amazon-ebs.ubuntu")
}

func TestConvertWithAnsible(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)
	require.NoError(t, err)

	// Create variables.pkr.hcl
	varsContent := `variable "base_image" {
  type    = string
  default = "ubuntu"
}

variable "provision_repo_path" {
  type = string
  description = "Path to provision repo"
}`
	err = os.WriteFile(filepath.Join(tmpDir, "variables.pkr.hcl"), []byte(varsContent), 0644)
	require.NoError(t, err)

	// Create docker.pkr.hcl with Ansible provisioner
	dockerContent := `build {
  provisioner "ansible" {
    user                     = "ubuntu"
    playbook_file            = "${var.provision_repo_path}/playbook.yml"
    galaxy_file              = "${var.provision_repo_path}/requirements.yml"
    inventory_file           = "inventory.yml"
    ansible_collections_path = "/path/to/collections"
    use_proxy                = true
    ansible_env_vars = [
      "ANSIBLE_ROLES_PATH=/path/to/roles",
      "PACKER_BUILD_NAME={{ build_name }}"
    ]
    extra_arguments = [
      "-e", "ansible_python_interpreter=/usr/bin/python3",
      "-e", "custom_var=value"
    ]
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Run conversion
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify Ansible fields
	assert.Len(t, config.Provisioners, 1)
	ansible := config.Provisioners[0]

	assert.Equal(t, "ansible", ansible.Type)
	assert.Equal(t, "ubuntu", ansible.User)
	assert.Equal(t, "${PROVISION_REPO_PATH}/playbook.yml", ansible.PlaybookPath)
	assert.Equal(t, "${PROVISION_REPO_PATH}/requirements.yml", ansible.GalaxyFile)
	assert.Equal(t, "inventory.yml", ansible.InventoryFile)
	assert.Equal(t, "/path/to/collections", ansible.CollectionsPath)
	assert.True(t, ansible.UseProxy)

	// Verify ansible_env_vars
	assert.Len(t, ansible.AnsibleEnvVars, 2)
	assert.Contains(t, ansible.AnsibleEnvVars, "ANSIBLE_ROLES_PATH=/path/to/roles")
	assert.Contains(t, ansible.AnsibleEnvVars, "PACKER_BUILD_NAME={{ build_name }}")

	// Verify extra_vars parsed from extra_arguments
	assert.NotEmpty(t, ansible.ExtraVars)
	assert.Equal(t, "/usr/bin/python3", ansible.ExtraVars["ansible_python_interpreter"])
	assert.Equal(t, "value", ansible.ExtraVars["custom_var"])
}

func TestConvertWithMultipleFeatures(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md
	readmeContent := `# Full Featured Template

Template with all supported features.`
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
}

variable "container_user" {
  type    = string
  default = "security"
}`
	err = os.WriteFile(filepath.Join(tmpDir, "variables.pkr.hcl"), []byte(varsContent), 0644)
	require.NoError(t, err)

	// Create docker.pkr.hcl with multiple features
	dockerContent := `build {
  provisioner "shell" {
    only = ["docker.amd64", "docker.arm64"]
    inline = [
      "apt-get update",
      "useradd ${var.container_user}"
    ]
  }

  provisioner "ansible" {
    only = ["docker.amd64", "docker.arm64"]
    user = "${var.container_user}"
    playbook_file = "/playbook.yml"
    ansible_env_vars = [
      "ANSIBLE_REMOTE_TMP=/tmp/ansible"
    ]
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Run conversion
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test Author",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify we have all components
	assert.Len(t, config.Provisioners, 2)

	// Verify shell provisioner with conditionals
	shell := config.Provisioners[0]
	assert.Equal(t, "shell", shell.Type)
	assert.Contains(t, shell.Only, "docker.amd64")
	assert.Contains(t, shell.Only, "docker.arm64")
	assert.Contains(t, shell.Inline, "apt-get update")

	// Verify ansible provisioner with user and conditionals
	ansible := config.Provisioners[1]
	assert.Equal(t, "ansible", ansible.Type)
	assert.Equal(t, "security", ansible.User) // Variable is evaluated to its default value
	assert.Contains(t, ansible.Only, "docker.amd64")
	assert.Contains(t, ansible.AnsibleEnvVars, "ANSIBLE_REMOTE_TMP=/tmp/ansible")
}

func TestConvertHCLProvisioners(t *testing.T) {
	converter, err := NewPackerConverter(PackerConverterOptions{})
	require.NoError(t, err)

	tests := []struct {
		name     string
		builds   []PackerBuild
		expected int
		validate func(t *testing.T, provs []builder.Provisioner)
	}{
		{
			name: "shell provisioner with conditionals",
			builds: []PackerBuild{
				{
					Provisioners: []PackerProvisioner{
						{
							Type:   "shell",
							Only:   []string{"docker.amd64", "docker.arm64"},
							Except: []string{"amazon-ebs.ubuntu"},
							Inline: []string{"echo test"},
						},
					},
				},
			},
			expected: 1,
			validate: func(t *testing.T, provs []builder.Provisioner) {
				assert.Equal(t, "shell", provs[0].Type)
				assert.Contains(t, provs[0].Only, "docker.amd64")
				assert.Contains(t, provs[0].Only, "docker.arm64")
				assert.Contains(t, provs[0].Except, "amazon-ebs.ubuntu")
				assert.Contains(t, provs[0].Inline, "echo test")
			},
		},
		{
			name: "ansible provisioner with fields",
			builds: []PackerBuild{
				{
					Provisioners: []PackerProvisioner{
						{
							Type:            "ansible",
							User:            "ubuntu",
							PlaybookFile:    "/playbook.yml",
							GalaxyFile:      "/requirements.yml",
							InventoryFile:   "inventory.yml",
							CollectionsPath: "/collections",
							UseProxy:        true,
							AnsibleEnvVars:  []string{"ANSIBLE_REMOTE_TMP=/tmp"},
							ExtraArguments:  []string{"-e", "var=value"},
						},
					},
				},
			},
			expected: 1,
			validate: func(t *testing.T, provs []builder.Provisioner) {
				assert.Equal(t, "ansible", provs[0].Type)
				assert.Equal(t, "ubuntu", provs[0].User)
				assert.Equal(t, "/playbook.yml", provs[0].PlaybookPath)
				assert.Equal(t, "/requirements.yml", provs[0].GalaxyFile)
				assert.Equal(t, "inventory.yml", provs[0].InventoryFile)
				assert.Equal(t, "/collections", provs[0].CollectionsPath)
				assert.True(t, provs[0].UseProxy)
				assert.Contains(t, provs[0].AnsibleEnvVars, "ANSIBLE_REMOTE_TMP=/tmp")
				assert.Equal(t, "value", provs[0].ExtraVars["var"])
			},
		},
		{
			name: "shell provisioner with script",
			builds: []PackerBuild{
				{
					Provisioners: []PackerProvisioner{
						{
							Type:   "shell",
							Script: "/path/to/script.sh",
						},
					},
				},
			},
			expected: 1,
			validate: func(t *testing.T, provs []builder.Provisioner) {
				assert.Equal(t, "shell", provs[0].Type)
				assert.Len(t, provs[0].Scripts, 1)
				assert.Contains(t, provs[0].Scripts, "/path/to/script.sh")
			},
		},
		{
			name: "shell provisioner with scripts array",
			builds: []PackerBuild{
				{
					Provisioners: []PackerProvisioner{
						{
							Type:    "shell",
							Scripts: []string{"/script1.sh", "/script2.sh"},
						},
					},
				},
			},
			expected: 1,
			validate: func(t *testing.T, provs []builder.Provisioner) {
				assert.Equal(t, "shell", provs[0].Type)
				assert.Len(t, provs[0].Scripts, 2)
				assert.Contains(t, provs[0].Scripts, "/script1.sh")
				assert.Contains(t, provs[0].Scripts, "/script2.sh")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.convertHCLProvisioners(tt.builds)
			assert.Len(t, result, tt.expected)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestParseAnsibleExtraArgs(t *testing.T) {
	converter, err := NewPackerConverter(PackerConverterOptions{})
	require.NoError(t, err)

	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: map[string]string{},
		},
		{
			name: "single -e flag",
			args: []string{"-e", "var=value"},
			expected: map[string]string{
				"var": "value",
			},
		},
		{
			name: "multiple -e flags",
			args: []string{"-e", "var1=value1", "-e", "var2=value2"},
			expected: map[string]string{
				"var1": "value1",
				"var2": "value2",
			},
		},
		{
			name: "--extra-vars long form",
			args: []string{"--extra-vars", "var=value"},
			expected: map[string]string{
				"var": "value",
			},
		},
		{
			name: "mixed with other flags",
			args: []string{"-vvv", "-e", "var=value", "--connection", "packer"},
			expected: map[string]string{
				"var": "value",
			},
		},
		{
			name: "value with equals sign",
			args: []string{"-e", "var=key=value"},
			expected: map[string]string{
				"var": "key=value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.parseAnsibleExtraArgs(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertWithDockerSourceConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test Docker Source"), 0644)
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

	// Create docker.pkr.hcl with source block containing Docker-specific configuration
	dockerContent := `source "docker" "amd64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/amd64"
  privileged = true
  pull       = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "ENTRYPOINT [\"/bin/bash\"]",
    "USER sliver",
    "WORKDIR /home/sliver",
    "ENV PATH=/opt/sliver:$PATH"
  ]

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

build {
  sources = ["source.docker.amd64"]

  provisioner "shell" {
    inline = ["apt-get update"]
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Run conversion
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify Docker-specific configuration was captured
	assert.True(t, config.Base.Privileged, "Privileged flag should be set")
	assert.True(t, config.Base.Pull, "Pull flag should be set")

	// Verify volumes
	require.NotNil(t, config.Base.Volumes)
	assert.Equal(t, "/sys/fs/cgroup:rw", config.Base.Volumes["/sys/fs/cgroup"])

	// Verify run_command
	require.NotNil(t, config.Base.RunCommand)
	assert.Len(t, config.Base.RunCommand, 5)
	assert.Contains(t, config.Base.RunCommand, "-d")
	assert.Contains(t, config.Base.RunCommand, "--cgroupns=host")

	// Verify changes (Dockerfile instructions)
	require.NotNil(t, config.Base.Changes)
	assert.Len(t, config.Base.Changes, 4)
	assert.Contains(t, config.Base.Changes, "ENTRYPOINT [\"/bin/bash\"]")
	assert.Contains(t, config.Base.Changes, "USER sliver")
	assert.Contains(t, config.Base.Changes, "WORKDIR /home/sliver")
	assert.Contains(t, config.Base.Changes, "ENV PATH=/opt/sliver:$PATH")
}

func TestConvertWithPostProcessors(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test Post-Processors"), 0644)
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

	// Create docker.pkr.hcl with post-processors
	dockerContent := `build {
  provisioner "shell" {
    inline = ["apt-get update"]
  }

  post-processor "docker-tag" {
    repository = "ghcr.io/myorg/myimage"
    tags       = ["latest", "v1.0.0", "dev"]
  }

  post-processor "docker-push" {}
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Run conversion
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify provisioners
	assert.Len(t, config.Provisioners, 1)
	assert.Equal(t, "shell", config.Provisioners[0].Type)

	// Verify target has post-processor info
	require.Len(t, config.Targets, 1)
	containerTarget := config.Targets[0]

	assert.Equal(t, "container", containerTarget.Type)
	assert.Equal(t, "ghcr.io/myorg/myimage", containerTarget.Registry)
	assert.Len(t, containerTarget.Tags, 3)
	assert.Contains(t, containerTarget.Tags, "latest")
	assert.Contains(t, containerTarget.Tags, "v1.0.0")
	assert.Contains(t, containerTarget.Tags, "dev")
	assert.True(t, containerTarget.Push)
}

func TestConvertWithNestedPostProcessors(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test Nested"), 0644)
	require.NoError(t, err)

	// Create docker.pkr.hcl with nested post-processors block
	dockerContent := `build {
  post-processors {
    post-processor "docker-tag" {
      repository = "docker.io/org/app"
      tags       = ["stable"]
    }
    post-processor "docker-push" {}
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Run conversion
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify target has post-processor info from nested block
	require.Len(t, config.Targets, 1)
	containerTarget := config.Targets[0]

	assert.Equal(t, "docker.io/org/app", containerTarget.Registry)
	assert.Contains(t, containerTarget.Tags, "stable")
	assert.True(t, containerTarget.Push)
}

func TestProvisionersMatchDirect(t *testing.T) {
	converter, err := NewPackerConverter(PackerConverterOptions{})
	require.NoError(t, err)

	tests := []struct {
		name     string
		p1       []builder.Provisioner
		p2       []builder.Provisioner
		expected bool
	}{
		{
			name:     "empty lists match",
			p1:       []builder.Provisioner{},
			p2:       []builder.Provisioner{},
			expected: true,
		},
		{
			name:     "both nil match",
			p1:       nil,
			p2:       nil,
			expected: true,
		},
		{
			name:     "same shell types match",
			p1:       []builder.Provisioner{{Type: "shell"}},
			p2:       []builder.Provisioner{{Type: "shell"}},
			expected: true,
		},
		{
			name:     "same ansible types and playbook match",
			p1:       []builder.Provisioner{{Type: "ansible", PlaybookPath: "/playbook.yml"}},
			p2:       []builder.Provisioner{{Type: "ansible", PlaybookPath: "/playbook.yml"}},
			expected: true,
		},
		{
			name:     "different lengths do not match",
			p1:       []builder.Provisioner{{Type: "shell"}},
			p2:       []builder.Provisioner{{Type: "shell"}, {Type: "ansible"}},
			expected: false,
		},
		{
			name:     "different types do not match",
			p1:       []builder.Provisioner{{Type: "shell"}},
			p2:       []builder.Provisioner{{Type: "ansible"}},
			expected: false,
		},
		{
			name:     "different ansible playbook paths do not match",
			p1:       []builder.Provisioner{{Type: "ansible", PlaybookPath: "/path1.yml"}},
			p2:       []builder.Provisioner{{Type: "ansible", PlaybookPath: "/path2.yml"}},
			expected: false,
		},
		{
			name: "multiple provisioners match",
			p1: []builder.Provisioner{
				{Type: "shell"},
				{Type: "ansible", PlaybookPath: "/playbook.yml"},
			},
			p2: []builder.Provisioner{
				{Type: "shell"},
				{Type: "ansible", PlaybookPath: "/playbook.yml"},
			},
			expected: true,
		},
		{
			name:     "non-ansible types only check type",
			p1:       []builder.Provisioner{{Type: "file", PlaybookPath: "/path1"}},
			p2:       []builder.Provisioner{{Type: "file", PlaybookPath: "/path2"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.provisionersMatch(tt.p1, tt.p2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertWithAMIProvisioners(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README.md
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test AMI"), 0644)
	require.NoError(t, err)

	// Create ami.pkr.hcl (no docker.pkr.hcl so AMI provisioners become primary)
	amiContent := `build {
  provisioner "shell" {
    inline = ["yum update -y"]
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "ami.pkr.hcl"), []byte(amiContent), 0644)
	require.NoError(t, err)

	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// When no docker provisioners exist, AMI provisioners should be used
	assert.Len(t, config.Provisioners, 1)
	assert.Equal(t, "shell", config.Provisioners[0].Type)
	assert.Contains(t, config.Provisioners[0].Inline, "yum update -y")
}

func TestConvertWithDifferentDockerAndAMIProvisioners(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)
	require.NoError(t, err)

	// Create docker.pkr.hcl
	dockerContent := `build {
  provisioner "shell" {
    inline = ["apt-get update"]
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "docker.pkr.hcl"), []byte(dockerContent), 0644)
	require.NoError(t, err)

	// Create ami.pkr.hcl with different provisioners
	amiContent := `build {
  provisioner "shell" {
    inline = ["yum update -y"]
  }
  provisioner "ansible" {
    playbook_file = "/playbook.yml"
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "ami.pkr.hcl"), []byte(amiContent), 0644)
	require.NoError(t, err)

	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
		License:     "MIT",
		Version:     "1.0.0",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Docker provisioners should be used (primary), even though AMI differs
	assert.Len(t, config.Provisioners, 1)
	assert.Equal(t, "shell", config.Provisioners[0].Type)
}

func TestExtractDockerTargetConfigWithIgnoredPostProcessors(t *testing.T) {
	converter, err := NewPackerConverter(PackerConverterOptions{})
	require.NoError(t, err)

	builds := []PackerBuild{
		{
			PostProcessors: []PackerPostProcessor{
				{Type: "manifest"},
				{Type: "compress"},
				{Type: "checksum"},
			},
		},
	}

	registry, tags, push := converter.extractDockerTargetConfig(context.Background(), builds)
	assert.Empty(t, registry)
	assert.Nil(t, tags)
	assert.False(t, push)
}

func TestConvertWithDefaultVersionAndLicense(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)
	require.NoError(t, err)

	// Convert with no version/license specified - should use config defaults
	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
		Author:      "Test",
	})
	require.NoError(t, err)

	config, err := converter.Convert(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config)

	// Version and license should come from global config defaults
	assert.NotEmpty(t, config.Metadata.Version)
	assert.NotEmpty(t, config.Metadata.License)
}

func TestExtractDescription_ReadmeWithDashItems(t *testing.T) {
	tmpDir := t.TempDir()

	readmeContent := `# My Template

- Feature 1
- Feature 2

Actual description after list items.`
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readmeContent), 0644)
	require.NoError(t, err)

	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
	})
	require.NoError(t, err)

	result := converter.extractDescription(context.Background())
	// Should skip the dash lines and get the actual description
	assert.Equal(t, "Actual description after list items.", result)
}

func TestExtractDescription_ReadmeWithMultipleHeaders(t *testing.T) {
	tmpDir := t.TempDir()

	readmeContent := `# Main Header

## Sub Header

Description under subheader.`
	err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readmeContent), 0644)
	require.NoError(t, err)

	converter, err := NewPackerConverter(PackerConverterOptions{
		TemplateDir: tmpDir,
	})
	require.NoError(t, err)

	result := converter.extractDescription(context.Background())
	// Should skip headers (lines starting with #) after the first one
	assert.Equal(t, "Description under subheader.", result)
}

func TestParseAnsibleExtraArgsEdgeCases(t *testing.T) {
	converter, err := NewPackerConverter(PackerConverterOptions{})
	require.NoError(t, err)

	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name:     "trailing -e without value",
			args:     []string{"-e"},
			expected: map[string]string{},
		},
		{
			name:     "trailing --extra-vars without value",
			args:     []string{"--extra-vars"},
			expected: map[string]string{},
		},
		{
			name:     "-e with value missing equals",
			args:     []string{"-e", "noequalshere"},
			expected: map[string]string{},
		},
		{
			name:     "nil args",
			args:     nil,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.parseAnsibleExtraArgs(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractDockerTargetConfig(t *testing.T) {
	converter, err := NewPackerConverter(PackerConverterOptions{})
	require.NoError(t, err)

	tests := []struct {
		name             string
		builds           []PackerBuild
		expectedRegistry string
		expectedTags     []string
		expectedPush     bool
	}{
		{
			name:             "empty builds",
			builds:           []PackerBuild{},
			expectedRegistry: "",
			expectedTags:     nil,
			expectedPush:     false,
		},
		{
			name: "docker-tag only",
			builds: []PackerBuild{
				{
					PostProcessors: []PackerPostProcessor{
						{
							Type:       "docker-tag",
							Repository: "ghcr.io/test/image",
							Tags:       []string{"latest"},
						},
					},
				},
			},
			expectedRegistry: "ghcr.io/test/image",
			expectedTags:     []string{"latest"},
			expectedPush:     false,
		},
		{
			name: "docker-push only",
			builds: []PackerBuild{
				{
					PostProcessors: []PackerPostProcessor{
						{Type: "docker-push"},
					},
				},
			},
			expectedRegistry: "",
			expectedTags:     nil,
			expectedPush:     true,
		},
		{
			name: "both docker-tag and docker-push",
			builds: []PackerBuild{
				{
					PostProcessors: []PackerPostProcessor{
						{
							Type:       "docker-tag",
							Repository: "quay.io/org/app",
							Tags:       []string{"v1.0.0", "latest"},
						},
						{Type: "docker-push"},
					},
				},
			},
			expectedRegistry: "quay.io/org/app",
			expectedTags:     []string{"v1.0.0", "latest"},
			expectedPush:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, tags, push := converter.extractDockerTargetConfig(context.Background(), tt.builds)
			assert.Equal(t, tt.expectedRegistry, registry)
			assert.Equal(t, tt.expectedTags, tags)
			assert.Equal(t, tt.expectedPush, push)
		})
	}
}
