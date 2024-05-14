package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	warpgate "github.com/cowdogmoo/warpgate/cmd"
	"github.com/cowdogmoo/warpgate/pkg/blueprint"
	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

func TestRunImageBuilder(t *testing.T) {
	tests := []struct {
		name          string
		provisionPath string
		blueprint     bp.Blueprint
		expectErr     bool
		setup         func(t *testing.T) (string, bp.Blueprint, string, func())
	}{
		{
			name: "Valid Inputs",
			setup: func(t *testing.T) (string, bp.Blueprint, string, func()) {
				blueprintName := "ttpforge"
				blueprintPath := filepath.Join(repoRoot, "blueprints", blueprintName)
				configFilePath := filepath.Join(blueprintPath, "config.yaml")

				// Ensure the config file exists
				if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
					t.Fatalf("config file does not exist at %s", configFilePath)
				}

				blueprint := bp.Blueprint{
					Name:             blueprintName,
					Path:             blueprintPath,
					ProvisioningRepo: repoRoot,
				}

				return repoRoot, blueprint, "", func() {}
			},
			expectErr: false,
		},
		{
			name: "Invalid Provision Path",
			setup: func(t *testing.T) (string, bp.Blueprint, string, func()) {
				provisionDir := "/invalid"
				blueprint := bp.Blueprint{Name: "invalidBlueprint"}
				return provisionDir, blueprint, "", func() {}
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var provisionDir string
			var blueprint blueprint.Blueprint
			var githubToken string
			var cleanup func()
			if tc.setup != nil {
				provisionDir, blueprint, githubToken, cleanup = tc.setup(t)
				defer cleanup()
			}

			cmd := &cobra.Command{}
			cmd.Flags().String("provisionPath", provisionDir, "")
			cmd.Flags().String("blueprint", blueprint.Name, "")
			cmd.Flags().String("github-token", githubToken, "")

			if tc.provisionPath != "" {
				if err := cmd.Flags().Set("provisionPath", tc.provisionPath); err != nil {
					t.Fatalf("failed to set provisionPath: %v", err)
				}
			}

			viper.SetConfigFile(filepath.Join(blueprint.Path, "config.yaml"))
			if err := viper.ReadInConfig(); err != nil && !tc.expectErr {
				t.Fatalf("failed to read config file: %v", err)
			}

			err := warpgate.RunImageBuilder(cmd, nil, blueprint)
			if (err != nil) != tc.expectErr {
				t.Errorf("runImageBuilder() error = %v, expectErr %v", err, tc.expectErr)
			}
		})
	}
}
