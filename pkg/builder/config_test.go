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

func TestProvisionerEnhancedAnsible(t *testing.T) {
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

func TestPostProcessorManifest(t *testing.T) {
	pp := PostProcessor{
		Type:      "manifest",
		Output:    "manifest.json",
		StripPath: true,
		Only:      []string{"docker.amd64"},
	}

	assert.Equal(t, "manifest", pp.Type)
	assert.Equal(t, "manifest.json", pp.Output)
	assert.True(t, pp.StripPath)
	assert.Contains(t, pp.Only, "docker.amd64")
}

func TestPostProcessorDockerTag(t *testing.T) {
	pp := PostProcessor{
		Type:       "docker-tag",
		Repository: "ghcr.io/org/image",
		Tags:       []string{"latest", "v1.0.0"},
		Force:      true,
		Except:     []string{"amazon-ebs.ubuntu"},
	}

	assert.Equal(t, "docker-tag", pp.Type)
	assert.Equal(t, "ghcr.io/org/image", pp.Repository)
	assert.Contains(t, pp.Tags, "latest")
	assert.Contains(t, pp.Tags, "v1.0.0")
	assert.True(t, pp.Force)
	assert.Contains(t, pp.Except, "amazon-ebs.ubuntu")
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
		PostProcessors: []PostProcessor{
			{
				Type:      "manifest",
				Output:    "manifest.json",
				StripPath: true,
			},
			{
				Type:       "docker-tag",
				Repository: "ghcr.io/test/image",
				Tags:       []string{"latest"},
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
	assert.Len(t, config.PostProcessors, 2)

	// Verify provisioner conditionals
	assert.Contains(t, config.Provisioners[0].Only, "docker.amd64")

	// Verify enhanced Ansible
	assert.Equal(t, "ubuntu", config.Provisioners[1].User)
	assert.Contains(t, config.Provisioners[1].AnsibleEnvVars, "ANSIBLE_REMOTE_TMP=/tmp")

	// Verify post-processors
	assert.Equal(t, "manifest", config.PostProcessors[0].Type)
	assert.Equal(t, "docker-tag", config.PostProcessors[1].Type)
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
		PostProcessors: []PostProcessor{
			{
				Type:   "manifest",
				Output: "manifest.json",
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
	assert.Contains(t, yamlStr, "post_processors:")
	assert.Contains(t, yamlStr, "type: manifest")

	// Unmarshal back
	var unmarshaled Config
	err = yaml.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, config.Name, unmarshaled.Name)
	assert.True(t, unmarshaled.Base.Privileged)
	assert.Contains(t, unmarshaled.Provisioners[0].Only, "docker.amd64")
	assert.Len(t, unmarshaled.PostProcessors, 1)
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

func TestPostProcessorDefaultValues(t *testing.T) {
	pp := PostProcessor{
		Type: "manifest",
	}

	// Default values should be false/empty
	assert.False(t, pp.StripPath)
	assert.False(t, pp.Force)
	assert.False(t, pp.KeepInputArtifact)
	assert.Nil(t, pp.Only)
	assert.Nil(t, pp.Except)
}
