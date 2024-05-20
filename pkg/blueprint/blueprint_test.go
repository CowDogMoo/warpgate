package blueprint_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func setupConfig(t *testing.T, blueprint *bp.Blueprint, configContent string) string {
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

func setupBlueprint(t *testing.T, blueprintName string, tempDir string) *bp.Blueprint {
	blueprintPath := filepath.Join(tempDir, "blueprints", blueprintName)
	if _, err := os.Stat(blueprintPath); os.IsNotExist(err) {
		t.Fatalf("blueprint directory does not exist at %s", blueprintPath)
	}

	return &bp.Blueprint{
		Name: blueprintName,
		Path: blueprintPath,
	}
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
						ImageRegistry: packer.ContainerImageRegistry{
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
						ImageRegistry: packer.ContainerImageRegistry{
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

func TestBuildPackerImages(t *testing.T) {
	// Set up blueprint and config for testing
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

	blueprint := setupBlueprint(t, "sliver", tempDir)
	setupConfig(t, blueprint, `
packer_templates:
  - ami:
      instance_type: "t2.micro"
      region: "us-west-2"
      ssh_user: "ubuntu"
    container:
      image_hashes:
        amd64: "hash1"
        arm64: "hash2"
      image_registry:
        server: "testserver"
        username: "testuser"
        credential: "testtoken"
      workdir: "/tmp"
    image_values:
      name: "ubuntu"
      version: "jammy"
    user: "ubuntu"
`)

	t.Run("BuildPackerImages", func(t *testing.T) {
		hashes, err := blueprint.BuildPackerImages()
		assert.NoError(t, err)
		assert.NotNil(t, hashes)
	})
}

func TestBuildPackerImage(t *testing.T) {
	// Set up blueprint and config for testing
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

	blueprint := setupBlueprint(t, "sliver", tempDir)
	setupConfig(t, blueprint, `
packer_templates:
  - ami:
      instance_type: "t2.micro"
      region: "us-west-2"
      ssh_user: "ubuntu"
    container:
      image_hashes:
        amd64: "hash1"
        arm64: "hash2"
      image_registry:
        server: "testserver"
        username: "testuser"
        credential: "testtoken"
      workdir: "/tmp"
    image_values:
      name: "ubuntu"
      version: "jammy"
    user: "ubuntu"
`)

	// Test BuildPackerImage
	t.Run("BuildPackerImages", func(t *testing.T) {
		hashes, err := blueprint.BuildPackerImages()
		assert.NoError(t, err)
		assert.NotNil(t, hashes)
	})
}

// MockCmdRunner is a function that runs a command using the sys.Cmd struct.
// This will be used to replace the actual command runner with a mock function.
var MockCmdRunner func(c sys.Cmd) (string, error)

// RunCommandWithMock runs the command using the mock function if set,
// otherwise it uses the real implementation.
func RunCommandWithMock(c sys.Cmd) (string, error) {
	if MockCmdRunner != nil {
		return MockCmdRunner(c)
	}
	return c.RunCmd()
}

// MockSysCmd is a mock of sys.Cmd
type MockSysCmd struct {
	mock.Mock
	CmdString     string
	Args          []string
	Dir           string
	Timeout       time.Duration
	OutputHandler func(string)
}

func (m *MockSysCmd) RunCmd() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// MockPackerTemplate is a mock of packer.PackerTemplate
type MockPackerTemplate struct {
	mock.Mock
}

func (m *MockPackerTemplate) RunInit(args []string, dir string) error {
	calledArgs := m.Called(args, dir)
	return calledArgs.Error(0)
}

func (m *MockPackerTemplate) RunBuild(args []string, dir string) (map[string]string, string, error) {
	calledArgs := m.Called(args, dir)
	return calledArgs.Get(0).(map[string]string), calledArgs.String(1), calledArgs.Error(2)
}

func TestBuildImageAttempt(t *testing.T) {
	tests := []struct {
		name           string
		blueprintName  string
		configContent  string
		expectedErr    string
		expectedResult map[string]string
	}{
		{
			name:          "valid blueprint",
			blueprintName: "sliver",
			configContent: `
packer_templates:
  - ami:
      instance_type: "t2.micro"
      region: "us-west-2"
      ssh_user: "ubuntu"
    container:
      image_hashes:
        amd64: "hash1"
        arm64: "hash2"
      image_registry:
        server: "testserver"
        username: "testuser"
        credential: "testtoken"
      workdir: "/tmp"
    image_values:
      name: "ubuntu"
      version: "jammy"
    user: "ubuntu"
`,
			expectedErr:    "",
			expectedResult: map[string]string{"amd64": "hash1", "arm64": "hash2"},
		},
		{
			name:          "no packer templates",
			blueprintName: "sliver",
			configContent: ``,
			expectedErr:   "no packer templates found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			blueprint := setupBlueprint(t, tc.blueprintName, tempDir)
			setupConfig(t, blueprint, tc.configContent)

			mockCmd := new(MockSysCmd)
			mockCmd.On("RunCmd").Return("mocked output", nil)

			mockPackerTemplate := new(mocks.PackerTemplate)
			mockPackerTemplate.On("RunInit", mock.Anything, mock.Anything).Return(nil)
			mockPackerTemplate.On("RunBuild", mock.Anything, mock.Anything).Return(tc.expectedResult, "", nil)

			// Replace actual implementations with mocks
			originalPackerTemplates := blueprint.PackerTemplates
			blueprint.PackerTemplates = []packer.PackerTemplate{mockPackerTemplate}

			MockCmdRunner = func(c sys.Cmd) (string, error) {
				if c.CmdString == "ls" {
					return mockCmd.RunCmd()
				}
				return "", nil
			}

			defer func() {
				blueprint.PackerTemplates = originalPackerTemplates
				MockCmdRunner = nil
			}()

			err = blueprint.LoadPackerTemplates()
			if err != nil && tc.expectedErr == "" {
				t.Fatalf("unexpected error loading packer templates: %v", err)
			}

			hashes, err := blueprint.BuildImageAttempt(1)
			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, hashes)
				assert.Equal(t, tc.expectedResult, hashes)
			}
		})
	}
}

func TestValidatePackerTemplate(t *testing.T) {
	tests := []struct {
		name          string
		blueprint     *bp.Blueprint
		expectedError string
	}{
		{
			name: "valid template",
			blueprint: &bp.Blueprint{
				Name:             "test-blueprint",
				Path:             "test-path",
				ProvisioningRepo: "test-repo",
				PackerTemplates: []packer.PackerTemplate{
					{
						ImageValues: packer.ImageValues{Name: "test-image", Version: "1.0"},
						User:        "test-user",
						Container:   packer.Container{Workdir: "test-workdir"},
					},
				},
			},
		},
		{
			name: "missing fields",
			blueprint: &bp.Blueprint{
				Name: "test-blueprint",
				Path: "test-path",
				PackerTemplates: []packer.PackerTemplate{
					{
						ImageValues: packer.ImageValues{Name: "", Version: ""},
						User:        "",
					},
				},
			},
			expectedError: "packer template 'test-blueprint' has uninitialized fields: ImageValues.Name, ImageValues.Version, User, Blueprint.ProvisioningRepo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.blueprint.ValidatePackerTemplate()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
