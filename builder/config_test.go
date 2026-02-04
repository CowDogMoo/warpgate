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

package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBaseImageDockerOptions(t *testing.T) {
	base := BaseImage{
		Image:      "ubuntu:22.04",
		Pull:       true,
		Privileged: true,
		Volumes: map[string]string{
			"/sys/fs/cgroup": "/sys/fs/cgroup:rw",
		},
		RunCommand: []string{"-d", "-i", "-t"},
		Changes: []string{
			"USER security",
			"WORKDIR /workspace",
			"ENV PATH=/opt/tools:$PATH",
		},
	}

	assert.Equal(t, "ubuntu:22.04", base.Image)
	assert.True(t, base.Pull)
	assert.True(t, base.Privileged)
	assert.Equal(t, "/sys/fs/cgroup:rw", base.Volumes["/sys/fs/cgroup"])
	assert.Len(t, base.RunCommand, 3)
	assert.Len(t, base.Changes, 3)
}

func TestProvisionerConditionals(t *testing.T) {
	prov := Provisioner{
		Type:   "shell",
		Only:   []string{"docker.amd64", "docker.arm64"},
		Except: []string{"amazon-ebs.ubuntu"},
		Inline: []string{"apt-get update"},
	}

	assert.Equal(t, "shell", prov.Type)
	assert.Contains(t, prov.Only, "docker.amd64")
	assert.Contains(t, prov.Only, "docker.arm64")
	assert.Contains(t, prov.Except, "amazon-ebs.ubuntu")
}

func TestProvisionerAnsible(t *testing.T) {
	prov := Provisioner{
		Type:            "ansible",
		User:            "ubuntu",
		PlaybookPath:    "/playbook.yml",
		GalaxyFile:      "/requirements.yml",
		InventoryFile:   "inventory.yml",
		AnsibleEnvVars:  []string{"ANSIBLE_REMOTE_TMP=/tmp"},
		CollectionsPath: "/path/to/collections",
		UseProxy:        true,
		ExtraVars: map[string]string{
			"ansible_python_interpreter": "/usr/bin/python3",
		},
	}

	assert.Equal(t, "ansible", prov.Type)
	assert.Equal(t, "ubuntu", prov.User)
	assert.Equal(t, "inventory.yml", prov.InventoryFile)
	assert.Contains(t, prov.AnsibleEnvVars, "ANSIBLE_REMOTE_TMP=/tmp")
	assert.Equal(t, "/path/to/collections", prov.CollectionsPath)
	assert.True(t, prov.UseProxy)
	assert.Equal(t, "/usr/bin/python3", prov.ExtraVars["ansible_python_interpreter"])
}

func TestConfigWithNewFeatures(t *testing.T) {
	config := Config{
		Metadata: Metadata{
			Name:        "test-template",
			Version:     "1.0.0",
			Description: "Test template",
			Author:      "Test Author",
			License:     "MIT",
		},
		Name:    "test-template",
		Version: "latest",
		Base: BaseImage{
			Image:      "ubuntu:22.04",
			Pull:       true,
			Privileged: true,
			Volumes: map[string]string{
				"/sys/fs/cgroup": "/sys/fs/cgroup:rw",
			},
			Changes: []string{"USER security"},
		},
		Provisioners: []Provisioner{
			{
				Type:   "shell",
				Only:   []string{"docker.amd64"},
				Inline: []string{"apt-get update"},
			},
			{
				Type:           "ansible",
				User:           "ubuntu",
				PlaybookPath:   "/playbook.yml",
				AnsibleEnvVars: []string{"ANSIBLE_REMOTE_TMP=/tmp"},
			},
		},
		Targets: []Target{
			{
				Type:      "container",
				Platforms: []string{"linux/amd64", "linux/arm64"},
			},
		},
	}

	// Verify structure
	assert.Equal(t, "test-template", config.Name)
	assert.True(t, config.Base.Privileged)
	assert.Len(t, config.Provisioners, 2)

	// Verify provisioner conditionals
	assert.Contains(t, config.Provisioners[0].Only, "docker.amd64")

	// Verify Ansible provisioner
	assert.Equal(t, "ubuntu", config.Provisioners[1].User)
	assert.Contains(t, config.Provisioners[1].AnsibleEnvVars, "ANSIBLE_REMOTE_TMP=/tmp")
}

func TestConfigYAMLMarshaling(t *testing.T) {
	config := Config{
		Name:    "test",
		Version: "1.0.0",
		Base: BaseImage{
			Image:      "ubuntu:22.04",
			Privileged: true,
			Changes:    []string{"USER test"},
		},
		Provisioners: []Provisioner{
			{
				Type:   "shell",
				Only:   []string{"docker.amd64"},
				Inline: []string{"echo test"},
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&config)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify YAML contains expected fields
	yamlStr := string(data)
	assert.Contains(t, yamlStr, "privileged: true")
	assert.Contains(t, yamlStr, "only:")
	assert.Contains(t, yamlStr, "docker.amd64")

	// Unmarshal back
	var unmarshaled Config
	err = yaml.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, config.Name, unmarshaled.Name)
	assert.True(t, unmarshaled.Base.Privileged)
	assert.Contains(t, unmarshaled.Provisioners[0].Only, "docker.amd64")
}

func TestBaseImageDefaultValues(t *testing.T) {
	base := BaseImage{
		Image: "ubuntu:22.04",
	}

	// Default values should be false/empty
	assert.False(t, base.Pull)
	assert.False(t, base.Privileged)
	assert.Nil(t, base.Volumes)
	assert.Nil(t, base.RunCommand)
	assert.Nil(t, base.Changes)
}

func TestProvisionerDefaultValues(t *testing.T) {
	prov := Provisioner{
		Type: "shell",
	}

	// Default values should be empty
	assert.Nil(t, prov.Only)
	assert.Nil(t, prov.Except)
	assert.Empty(t, prov.User)
	assert.Nil(t, prov.AnsibleEnvVars)
	assert.False(t, prov.UseProxy)
}

func TestDockerfileConfig(t *testing.T) {
	tests := []struct {
		name            string
		config          DockerfileConfig
		expectedPath    string
		expectedContext string
		description     string
	}{
		{
			name:            "defaults when empty",
			config:          DockerfileConfig{},
			expectedPath:    "Dockerfile",
			expectedContext: ".",
			description:     "Should use default values when fields are empty",
		},
		{
			name: "custom values",
			config: DockerfileConfig{
				Path:    "build/Dockerfile.custom",
				Context: "src",
			},
			expectedPath:    "build/Dockerfile.custom",
			expectedContext: "src",
			description:     "Should use custom values when provided",
		},
		{
			name: "with build args and target",
			config: DockerfileConfig{
				Path:    "Dockerfile",
				Context: ".",
				Args: map[string]string{
					"VERSION": "1.0.0",
					"ARCH":    "amd64",
				},
				Target: "production",
			},
			expectedPath:    "Dockerfile",
			expectedContext: ".",
			description:     "Should handle build args and target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPath, tt.config.GetDockerfilePath(), tt.description)
			assert.Equal(t, tt.expectedContext, tt.config.GetBuildContext(), tt.description)
		})
	}
}

func TestIsDockerfileBased(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name: "with dockerfile config",
			config: Config{
				Dockerfile: &DockerfileConfig{
					Path: "Dockerfile",
				},
			},
			expected: true,
		},
		{
			name: "without dockerfile config",
			config: Config{
				Base: BaseImage{
					Image: "ubuntu:22.04",
				},
			},
			expected: false,
		},
		{
			name:     "nil dockerfile config",
			config:   Config{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.IsDockerfileBased())
		})
	}
}

func TestArchOverride(t *testing.T) {
	tests := []struct {
		name        string
		override    ArchOverride
		description string
	}{
		{
			name: "dockerfile override",
			override: ArchOverride{
				Dockerfile: &DockerfileConfig{
					Path:    "Dockerfile.arm64",
					Context: ".",
				},
			},
			description: "Should support Dockerfile override for specific architecture",
		},
		{
			name: "base override",
			override: ArchOverride{
				Base: &BaseImage{
					Image: "ubuntu:22.04-arm64",
				},
			},
			description: "Should support base image override for specific architecture",
		},
		{
			name: "provisioners override with append",
			override: ArchOverride{
				Provisioners: []Provisioner{
					{
						Type:   "shell",
						Inline: []string{"echo arm64-specific"},
					},
				},
				AppendProvisioners: true,
			},
			description: "Should support provisioners override with append flag",
		},
		{
			name: "provisioners override without append",
			override: ArchOverride{
				Provisioners: []Provisioner{
					{
						Type:   "shell",
						Inline: []string{"echo replace all"},
					},
				},
				AppendProvisioners: false,
			},
			description: "Should support provisioners override with replace behavior",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.override.Dockerfile != nil {
				assert.NotNil(t, tt.override.Dockerfile, tt.description)
			}
			if tt.override.Base != nil {
				assert.NotNil(t, tt.override.Base, tt.description)
			}
			if len(tt.override.Provisioners) > 0 {
				assert.NotEmpty(t, tt.override.Provisioners, tt.description)
			}
		})
	}
}

func TestConfigWithDockerfileBased(t *testing.T) {
	config := Config{
		Metadata: Metadata{
			Name:        "dockerfile-template",
			Version:     "1.0.0",
			Description: "Dockerfile-based template",
			Author:      "Test Author",
			License:     "MIT",
		},
		Name:    "test-app",
		Version: "latest",
		Dockerfile: &DockerfileConfig{
			Path:    "Dockerfile",
			Context: ".",
			Args: map[string]string{
				"VERSION": "1.0.0",
			},
			Target: "production",
		},
		Targets: []Target{
			{
				Type:      "container",
				Platforms: []string{"linux/amd64", "linux/arm64"},
			},
		},
	}

	assert.True(t, config.IsDockerfileBased())
	assert.NotNil(t, config.Dockerfile)
	assert.Equal(t, "Dockerfile", config.Dockerfile.Path)
	assert.Equal(t, "production", config.Dockerfile.Target)
	assert.Equal(t, "1.0.0", config.Dockerfile.Args["VERSION"])
}

func TestConfigWithArchOverrides(t *testing.T) {
	config := Config{
		Name:    "multi-arch-app",
		Version: "1.0.0",
		Base: BaseImage{
			Image: "ubuntu:22.04",
		},
		ArchOverrides: map[string]ArchOverride{
			"arm64": {
				Dockerfile: &DockerfileConfig{
					Path:    "Dockerfile.arm64",
					Context: ".",
				},
			},
			"amd64": {
				Base: &BaseImage{
					Image: "ubuntu:22.04-amd64-optimized",
				},
				Provisioners: []Provisioner{
					{
						Type:   "shell",
						Inline: []string{"echo amd64-specific setup"},
					},
				},
				AppendProvisioners: true,
			},
		},
		Targets: []Target{
			{
				Type:      "container",
				Platforms: []string{"linux/amd64", "linux/arm64"},
			},
		},
	}

	assert.NotNil(t, config.ArchOverrides)
	assert.Len(t, config.ArchOverrides, 2)

	// Check arm64 override
	arm64Override := config.ArchOverrides["arm64"]
	assert.NotNil(t, arm64Override.Dockerfile)
	assert.Equal(t, "Dockerfile.arm64", arm64Override.Dockerfile.Path)

	// Check amd64 override
	amd64Override := config.ArchOverrides["amd64"]
	assert.NotNil(t, amd64Override.Base)
	assert.Equal(t, "ubuntu:22.04-amd64-optimized", amd64Override.Base.Image)
	assert.True(t, amd64Override.AppendProvisioners)
	assert.Len(t, amd64Override.Provisioners, 1)
}

func TestUnmarshalYAMLDecodeError(t *testing.T) {
	t.Parallel()

	// Provide YAML where a field has the wrong type to trigger a decode error
	yamlContent := `
name: 123
version: true
base: "not-a-map"
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	assert.Error(t, err, "expected decode error for invalid field types")
}

func TestDeprecatedPostProcessorsField(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config without post_processors",
			yamlContent: `
name: test
version: "1.0.0"
base:
  image: ubuntu:22.04
`,
			expectError: false,
		},
		{
			name: "config with deprecated post_processors field",
			yamlContent: `
name: test
version: "1.0.0"
base:
  image: ubuntu:22.04
post_processors:
  - type: docker-tag
    repository: ghcr.io/test/image
`,
			expectError: true,
			errorMsg:    "'post_processors' field is deprecated",
		},
		{
			name: "config with empty post_processors field",
			yamlContent: `
name: test
version: "1.0.0"
base:
  image: ubuntu:22.04
post_processors: []
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			err := yaml.Unmarshal([]byte(tt.yamlContent), &config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigWithSources(t *testing.T) {
	t.Parallel()

	yamlContent := `
name: test-image
version: "1.0.0"
base:
  image: ubuntu:22.04
sources:
  - name: my-tools
    git:
      repository: https://github.com/org/tools.git
      ref: main
      depth: 1
      auth:
        token: "${GITHUB_TOKEN}"
        username: oauth2
  - name: configs
    git:
      repository: https://github.com/org/configs.git
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Sources, 2)

	assert.Equal(t, "my-tools", cfg.Sources[0].Name)
	require.NotNil(t, cfg.Sources[0].Git)
	assert.Equal(t, "https://github.com/org/tools.git", cfg.Sources[0].Git.Repository)
	assert.Equal(t, "main", cfg.Sources[0].Git.Ref)
	assert.Equal(t, 1, cfg.Sources[0].Git.Depth)
	require.NotNil(t, cfg.Sources[0].Git.Auth)
	assert.Equal(t, "${GITHUB_TOKEN}", cfg.Sources[0].Git.Auth.Token)
	assert.Equal(t, "oauth2", cfg.Sources[0].Git.Auth.Username)

	assert.Equal(t, "configs", cfg.Sources[1].Name)
	assert.Nil(t, cfg.Sources[1].Git.Auth)
}

func TestConfigWithPostChanges(t *testing.T) {
	t.Parallel()

	yamlContent := `
name: test-image
version: "1.0.0"
base:
  image: ubuntu:22.04
post_changes:
  - USER nonroot
  - WORKDIR /app
  - ENTRYPOINT ["/bin/app"]
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	require.NoError(t, err)
	assert.Len(t, cfg.PostChanges, 3)
	assert.Equal(t, "USER nonroot", cfg.PostChanges[0])
	assert.Equal(t, "WORKDIR /app", cfg.PostChanges[1])
	assert.Equal(t, "ENTRYPOINT [\"/bin/app\"]", cfg.PostChanges[2])
}

func TestConfigMetadataChangelog(t *testing.T) {
	t.Parallel()

	yamlContent := `
metadata:
  name: test
  version: "2.0.0"
  changelog:
    - version: "2.0.0"
      date: "2025-01-01"
      changes:
        - "Added multi-arch support"
        - "Fixed build caching"
    - version: "1.0.0"
      date: "2024-06-01"
      changes:
        - "Initial release"
  extra:
    maintainer: team@example.com
    homepage: https://example.com
name: test-image
version: "1.0.0"
base:
  image: ubuntu:22.04
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Metadata.Changelog, 2)
	assert.Equal(t, "2.0.0", cfg.Metadata.Changelog[0].Version)
	assert.Equal(t, "2025-01-01", cfg.Metadata.Changelog[0].Date)
	assert.Len(t, cfg.Metadata.Changelog[0].Changes, 2)
	assert.Equal(t, "team@example.com", cfg.Metadata.Extra["maintainer"])
}

func TestConfigWithAMITarget(t *testing.T) {
	t.Parallel()

	yamlContent := `
name: test-ami
version: "1.0.0"
base:
  image: ami-12345678
targets:
  - type: ami
    region: us-east-1
    instance_type: t3.medium
    ami_name: my-custom-ami
    volume_size: 30
    instance_profile_name: MyInstanceProfile
    subnet_id: subnet-abc123
    security_group_ids:
      - sg-123
      - sg-456
    ami_tags:
      Environment: production
      Team: platform
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Targets, 1)

	target := cfg.Targets[0]
	assert.Equal(t, "ami", target.Type)
	assert.Equal(t, "us-east-1", target.Region)
	assert.Equal(t, "t3.medium", target.InstanceType)
	assert.Equal(t, "my-custom-ami", target.AMIName)
	assert.Equal(t, 30, target.VolumeSize)
	assert.Equal(t, "MyInstanceProfile", target.InstanceProfileName)
	assert.Equal(t, "subnet-abc123", target.SubnetID)
	assert.Len(t, target.SecurityGroupIDs, 2)
	assert.Equal(t, "production", target.AMITags["Environment"])
}

func TestConfigWithFastLaunch(t *testing.T) {
	t.Parallel()

	yamlContent := `
name: windows-ami
version: "1.0.0"
base:
  image: ami-windows-2022
targets:
  - type: ami
    region: us-east-1
    fast_launch_enabled: true
    fast_launch_max_parallel_launches: 10
    fast_launch_target_resource_count: 8
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Targets, 1)

	target := cfg.Targets[0]
	assert.True(t, target.FastLaunchEnabled)
	assert.Equal(t, 10, target.FastLaunchMaxParallelLaunches)
	assert.Equal(t, 8, target.FastLaunchTargetResourceCount)
}

func TestBuildResultFields(t *testing.T) {
	t.Parallel()

	t.Run("container result", func(t *testing.T) {
		t.Parallel()
		result := BuildResult{
			ImageRef:     "myimage:latest",
			Digest:       "sha256:abc123",
			Architecture: "amd64",
			Platform:     "linux/amd64",
			Duration:     "2m30s",
			Notes:        []string{"using cache", "pushed to registry"},
		}

		assert.Equal(t, "myimage:latest", result.ImageRef)
		assert.Equal(t, "sha256:abc123", result.Digest)
		assert.Equal(t, "amd64", result.Architecture)
		assert.Equal(t, "linux/amd64", result.Platform)
		assert.Len(t, result.Notes, 2)
	})

	t.Run("AMI result", func(t *testing.T) {
		t.Parallel()
		result := BuildResult{
			AMIID:    "ami-12345678",
			Region:   "us-east-1",
			Duration: "15m",
		}

		assert.Equal(t, "ami-12345678", result.AMIID)
		assert.Equal(t, "us-east-1", result.Region)
		assert.Empty(t, result.ImageRef)
	})
}

func TestProvisionerTypes(t *testing.T) {
	t.Parallel()

	yamlContent := `
name: test
version: "1.0.0"
base:
  image: ubuntu:22.04
provisioners:
  - type: shell
    inline:
      - apt-get update
    environment:
      DEBIAN_FRONTEND: noninteractive
    working_dir: /tmp
    user: root
  - type: file
    source: ./config.json
    destination: /etc/app/config.json
    mode: "0644"
  - type: powershell
    ps_scripts:
      - ./setup.ps1
    execution_policy: Bypass
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Provisioners, 3)

	// Shell provisioner
	shell := cfg.Provisioners[0]
	assert.Equal(t, "shell", shell.Type)
	assert.Equal(t, "noninteractive", shell.Environment["DEBIAN_FRONTEND"])
	assert.Equal(t, "/tmp", shell.WorkingDir)
	assert.Equal(t, "root", shell.User)

	// File provisioner
	file := cfg.Provisioners[1]
	assert.Equal(t, "file", file.Type)
	assert.Equal(t, "./config.json", file.Source)
	assert.Equal(t, "/etc/app/config.json", file.Destination)
	assert.Equal(t, "0644", file.Mode)

	// PowerShell provisioner
	ps := cfg.Provisioners[2]
	assert.Equal(t, "powershell", ps.Type)
	assert.Len(t, ps.PSScripts, 1)
	assert.Equal(t, "Bypass", ps.ExecutionPolicy)
}
