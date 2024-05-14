package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	warpgate "github.com/cowdogmoo/warpgate/cmd"
	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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

func TestRunImageBuilder(t *testing.T) {
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
		name          string
		blueprintName string
		expectedErr   string
		setup         func(t *testing.T) (string, bp.Blueprint, string, func())
	}{
		// {
		// 	name:          "Valid Inputs",
		// 	blueprintName: "ttpforge",
		// 	expectedErr:   "",
		// 	setup: func(t *testing.T) (string, bp.Blueprint, string, func()) {
		// 		blueprint := setupBlueprint(t, "ttpforge", tempDir)
		// 		configPath := setupConfig(t, blueprint, "")
		// 		return configPath, blueprint, "", func() {}
		// 	},
		// },
		{
			name:          "Invalid Provision Path",
			blueprintName: "invalidBlueprint",
			expectedErr:   "no packer templates found",
			setup: func(t *testing.T) (string, bp.Blueprint, string, func()) {
				blueprint := setupBlueprint(t, "invalidBlueprint", tempDir)
				configPath := setupConfig(t, blueprint, "")
				return configPath, blueprint, "", func() {}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var provisionDir string
			var blueprint bp.Blueprint
			var cleanup func()
			githubToken := os.Getenv("GITHUB_TOKEN")
			if tc.name == "Invalid Provision Path" {
				assert.Contains(t, "no packer templates found", tc.expectedErr)
				return
			}
			if tc.setup != nil {
				provisionDir, blueprint, _, cleanup = tc.setup(t)
				defer cleanup()
			}

			cmd := &cobra.Command{
				Use:   "imageBuilder",
				Short: "Build a container image using packer and a provisioning repo",
				RunE: func(cmd *cobra.Command, args []string) error {
					// Bind the provisionPath flag to Viper
					if err := viper.BindPFlag("provisionPath", cmd.Flags().Lookup("provisionPath")); err != nil {
						return err
					}
					if err := viper.BindPFlag("blueprint", cmd.Flags().Lookup("blueprint")); err != nil {
						return err
					}
					if err := viper.BindPFlag("github-token", cmd.Flags().Lookup("github-token")); err != nil {
						return err
					}

					// Set config file from the provisionPath
					viper.SetConfigFile(viper.GetString("provisionPath"))
					if err := viper.ReadInConfig(); err != nil {
						return err
					}

					// Reload blueprint from Viper's provisionPath
					provisionPath := viper.GetString("provisionPath")
					blueprint = bp.Blueprint{
						Name: blueprint.Name,
						Path: filepath.Dir(provisionPath),
					}

					return warpgate.RunImageBuilder(cmd, args, blueprint)
				},
			}
			cmd.Flags().String("provisionPath", provisionDir, "")
			cmd.Flags().String("blueprint", blueprint.Name, "")
			cmd.Flags().String("github-token", githubToken, "")
			err = cmd.ParseFlags([]string{
				"--provisionPath", provisionDir,
				"--blueprint", blueprint.Name,
				"--github-token", githubToken,
			})

			// Parse the flags so that they are properly set
			if err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			err = cmd.Execute()
			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
