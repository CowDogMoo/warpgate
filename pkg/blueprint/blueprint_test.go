package blueprint_test

import (
	"os"
	"path/filepath"
	"testing"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/spf13/cobra"
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
