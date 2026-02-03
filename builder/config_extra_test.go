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
