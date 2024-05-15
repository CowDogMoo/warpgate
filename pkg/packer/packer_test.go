package packer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
)

var (
	repoRoot string
)

func init() {
	var err error
	repoRoot, err = gitutils.RepoRoot()
	if err != nil {
		panic(err)
	}
}

func setupBlueprint(t *testing.T, blueprintName string, tempDir string) bp.Blueprint {
	blueprintPath := filepath.Join(tempDir, "blueprints", blueprintName)
	if _, err := os.Stat(blueprintPath); os.IsNotExist(err) {
		t.Fatalf("blueprint directory does not exist at %s", blueprintPath)
	}

	return bp.Blueprint{
		Name: blueprintName,
		Path: blueprintPath,
	}
}

func setupConfig(t *testing.T, blueprint bp.Blueprint, configContent string) string {
	configPath := filepath.Join(blueprint.Path, "config.yaml")
	if configContent != "" {
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}
	}
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	return configPath
}

func TestLoadPackerTemplates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repo_copy")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = sys.Cp(repoRoot, tempDir)
	if err != nil {
		t.Fatalf("failed to copy repo: %v", err)
	}

	tests := []struct {
		name           string
		blueprintName  string
		configContent  string
		expectedErr    string
		expectedResult []packer.BlueprintPacker
	}{
		{
			name:          "sliver blueprint",
			blueprintName: "sliver",
			expectedErr:   "",
			expectedResult: []packer.BlueprintPacker{
				{
					Base: packer.BlueprintBase{
						Name:    "ubuntu",
						Version: "jammy",
					},
					Tag: packer.BlueprintTag{
						Name:    "l50/sliver",
						Version: "latest",
					},
					User: "sliver",
				},
			},
		},
		{
			name:          "ttpforge blueprint",
			blueprintName: "ttpforge",
			expectedErr:   "",
			expectedResult: []packer.BlueprintPacker{
				{
					Base: packer.BlueprintBase{
						Name:    "ubuntu",
						Version: "jammy",
					},
					Tag: packer.BlueprintTag{
						Name:    "l50/ttpforge",
						Version: "latest",
					},
					User: "ubuntu",
				},
			},
		},
		{
			name:           "invalid config content",
			blueprintName:  "sliver",
			configContent:  `packer_templates: "not_a_list"`,
			expectedErr:    "failed to unmarshal packer templates",
			expectedResult: nil,
		},
		{
			name:           "empty packer templates",
			blueprintName:  "sliver",
			configContent:  "packer_templates: []",
			expectedErr:    "no packer templates found",
			expectedResult: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blueprint := setupBlueprint(t, tc.blueprintName, tempDir)
			if tc.name == "invalid blueprint name" {
				assert.Contains(t, "blueprint directory does not exist", tc.expectedErr)
				return
			}

			if tc.name == "missing config file" {
				configPath := filepath.Join(blueprint.Path, "config.yaml")
				err := os.Remove(configPath)
				if err != nil && !os.IsNotExist(err) {
					t.Fatalf("failed to remove config file: %v", err)
				}
			}

			if tc.configContent != "" {
				setupConfig(t, blueprint, tc.configContent)
			} else {
				setupConfig(t, blueprint, "")
			}

			if tc.expectedErr != "" && (tc.name == "missing config file" || tc.name == "invalid config content" || tc.name == "empty packer templates") {
				result, err := packer.LoadPackerTemplates()
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				assert.Nil(t, result)
				return
			}

			result, err := packer.LoadPackerTemplates()
			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestParseImageHashes(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedHashes map[string]string
	}{
		{
			name:           "valid output with hashes",
			output:         "Imported Docker image: sha256: abcd1234",
			expectedHashes: map[string]string{"docker": "abcd1234"},
		},
		{
			name:           "invalid output without hashes",
			output:         "No Docker image imported",
			expectedHashes: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blueprintPacker := &packer.BlueprintPacker{}
			blueprintPacker.ParseImageHashes(tc.output)
			assert.Equal(t, tc.expectedHashes, blueprintPacker.ImageHashes)
		})
	}
}
