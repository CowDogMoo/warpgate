package blueprint_test

import (
	"os"
	"path/filepath"
	"testing"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestParseCommandLineFlags(t *testing.T) {
	repoRoot, err := gitutils.RepoRoot()
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}
	tests := []struct {
		name        string
		setup       func(t *testing.T) (*cobra.Command, func())
		expectError bool
	}{
		{
			name: "valid flags",
			setup: func(t *testing.T) (*cobra.Command, func()) {
				blueprintName := "ttpforge"
				blueprintPath := filepath.Join(repoRoot, "blueprints", blueprintName)
				configFilePath := filepath.Join(blueprintPath, "config.yaml")

				// Create command and set flags
				cmd := &cobra.Command{}
				cmd.Flags().String("blueprint", blueprintName, "")
				cmd.Flags().String("provisionPath", repoRoot, "")
				cmd.Flags().String("config", configFilePath, "")

				return cmd, func() {
				}
			},
			expectError: false,
		},
		{
			name: "invalid provision path",
			setup: func(t *testing.T) (*cobra.Command, func()) {
				cmd := &cobra.Command{}
				cmd.Flags().String("provisionPath", "/invalid", "")
				cmd.Flags().String("blueprint", "invalidBlueprint", "")
				return cmd, func() {}
			},
			expectError: true,
		},
		{
			name: "config file does not exist",
			setup: func(t *testing.T) (*cobra.Command, func()) {
				cmd := &cobra.Command{}
				cmd.Flags().String("blueprint", "nonexistentBlueprint", "")
				return cmd, func() {}
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, cleanup := tc.setup(t)
			defer cleanup()

			blueprint := &bp.Blueprint{}
			blueprint.Name = "ttpforge"
			if err := blueprint.Initialize(); err != nil {
				t.Fatalf("Failed to initialize blueprint: %v", err)
			}

			if err := blueprint.SetConfigPath(); err != nil {
				t.Fatalf("Failed to set config path: %v", err)
			}

			err = blueprint.ParseCommandLineFlags(cmd)
			if (err != nil) != tc.expectError {
				t.Errorf("ParseCommandLineFlags() error = %v, expectError %v", err, tc.expectError)
			}

			if !tc.expectError {
				blueprintName, _ := cmd.Flags().GetString("blueprint")

				if blueprint.Name != blueprintName {
					t.Errorf("Expected Name to be %s, got %s", blueprintName, blueprint.Name)
				}

				expectedPath := blueprint.Path
				if blueprint.Path != expectedPath {
					t.Errorf("Expected Path to be %s, got %s", expectedPath, blueprint.Path)
				}
			}
		})
	}
}

func TestSetConfigPath(t *testing.T) {
	repoRoot, err := gitutils.RepoRoot()
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}
	tests := []struct {
		name         string
		blueprintDir string
		blueprint    bp.Blueprint
		setup        func() bp.Blueprint // function to setup any required state
		cleanup      func()              // function to clean up any state after the test
		wantErr      bool
	}{
		{
			name:         "config file does not exist",
			blueprintDir: "/nonexistent",
			setup: func() bp.Blueprint {
				return bp.Blueprint{
					Name: "nonexistent",
					Path: "/nonexistent",
				}
			},
			wantErr: true,
		},
		{
			name:         "config file exists",
			blueprintDir: repoRoot,
			setup: func() bp.Blueprint {
				blueprintPath := filepath.Join(repoRoot, "blueprints", "ttpforge")
				if _, err := os.Stat(repoRoot); os.IsNotExist(err) {
					t.Fatalf("blueprint directory does not exist at %s", blueprintPath)
				}

				return bp.Blueprint{
					Name: "ttpforge",
					Path: blueprintPath,
				}
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blueprint := tc.setup()
			err := blueprint.SetConfigPath()

			if (err != nil) != tc.wantErr {
				t.Errorf("SetConfigPath() error = %v, wantErr %v", err, tc.wantErr)
			}

			if tc.cleanup != nil {
				tc.cleanup()
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *bp.Blueprint
		expectError bool
	}{
		{
			name: "valid blueprint initialization",
			setup: func() *bp.Blueprint {
				return &bp.Blueprint{Name: "ttpforge"}
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blueprint := tc.setup()
			defer func() {
				if blueprint != nil && blueprint.BuildDir != "" {
					os.RemoveAll(blueprint.BuildDir)
				}
			}()

			err := blueprint.Initialize()
			if (err != nil) != tc.expectError {
				t.Errorf("Initialize() error = %v, expectError %v", err, tc.expectError)
			}
		})
	}
}

func TestCreateBuildDir(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *bp.Blueprint
		expectError bool
	}{
		{
			name: "successful build directory creation",
			setup: func() *bp.Blueprint {
				return &bp.Blueprint{Name: "ttpforge"}
			},
			expectError: false,
		},
		{
			name: "nil blueprint",
			setup: func() *bp.Blueprint {
				return nil
			},
			expectError: true,
		},
		{
			name: "empty blueprint name",
			setup: func() *bp.Blueprint {
				return &bp.Blueprint{}
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blueprint := tc.setup()
			err := blueprint.CreateBuildDir()
			if (err != nil) != tc.expectError {
				t.Errorf("CreateBuildDir() error = %v, expectError %v", err, tc.expectError)
			}

			if !tc.expectError {
				buildDir := blueprint.BuildDir
				assert.DirExists(t, buildDir)
				assert.DirExists(t, filepath.Join(buildDir, "blueprints", blueprint.Name))
				assert.DirExists(t, filepath.Join(buildDir, "blueprints", blueprint.Name, "scripts"))
			}
		})
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

	repoRoot, err := gitutils.RepoRoot()
	if err != nil {
		t.Fatalf("failed to get repo root: %v", err)
	}
	err = sys.Cp(repoRoot, tempDir)
	if err != nil {
		t.Fatalf("failed to copy repo: %v", err)
	}

	tests := []struct {
		name           string
		blueprintName  string
		configContent  string
		expectedErr    string
		expectedResult []packer.PackerTemplate
	}{
		{
			name:          "sliver blueprint",
			blueprintName: "sliver",
			expectedErr:   "",
			expectedResult: []packer.PackerTemplate{
				{
					AMI: packer.AMI{
						InstanceType: "t2.micro",
						Region:       "us-west-2",
						SSHUser:      "ubuntu",
					},
					Container: packer.Container{
						ImageHashes: map[string]string{
							"amd64": "hash1",
							"arm64": "hash2",
						},
						Registry: packer.ContainerImageRegistry{
							Server:     "testserver",
							Username:   "testuser",
							Credential: "testtoken",
						},
						Workdir: "/tmp",
					},
					ImageValues: packer.ImageValues{
						Name:    "ubuntu",
						Version: "jammy",
					},
					User: "ubuntu",
				},
			},
		},
		{
			name:          "ttpforge blueprint",
			blueprintName: "ttpforge",
			expectedErr:   "",
			expectedResult: []packer.PackerTemplate{
				{
					Container: packer.Container{
						Registry: packer.ContainerImageRegistry{
							Server:     "testserver",
							Username:   "testuser",
							Credential: "testtoken",
						},
						Workdir: "/tmp",
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
			expectedResult: []packer.PackerTemplate{},
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
				err := blueprint.LoadPackerTemplates()
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				return
			}

			err := blueprint.LoadPackerTemplates()
			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, blueprint.PackerTemplates)
			}
		})
	}
}
