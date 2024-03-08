package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	warpgate "github.com/cowdogmoo/warpgate/cmd"
	"github.com/spf13/cobra"
)

func TestRunImageBuilder(t *testing.T) {
	tests := []struct {
		name          string
		provisionPath string
		blueprint     string
		expectErr     bool
		setup         func(t *testing.T) (provisionPath, blueprintDir string, cleanupFunc func())
	}{
		{
			name:      "Valid Inputs",
			blueprint: "validBlueprint",
			expectErr: false,
			setup: func(t *testing.T) (string, string, func()) {
				provisionDir, err := os.MkdirTemp("", "provision")
				if err != nil {
					t.Errorf("failed to create temp provision dir: %v", err)
					cobra.CheckErr(err)
				}

				blueprintDir, err := os.MkdirTemp("", "blueprint")
				if err != nil {
					t.Errorf("failed to create temp blueprint dir: %v", err)
					cobra.CheckErr(err)
				}

				// Create mock config.yaml file based on the repo's structure
				configContent := `---
blueprint:
  name: validBlueprint
packer_templates:
  - name: valid-packer-template.pkr.hcl
    base:
      name: ubuntu
      version: "20.04"
    tag:
      name: valid-tag
      version: "1.0"
container:
  workdir: /app
  entrypoint: "/entrypoint.sh"
  user: appuser
  registry:
    server: registry.example.com
    username: user
    credential: secret`
				configFilePath := filepath.Join(blueprintDir, "config.yaml")
				t.Log(configFilePath)
				err = os.WriteFile(configFilePath, []byte(configContent), 0644)
				if err != nil {
					t.Errorf("failed to create mock config file: %v", err)
					cobra.CheckErr(err)
				}

				return provisionDir, blueprintDir, func() {
					os.RemoveAll(provisionDir)
					os.RemoveAll(blueprintDir)
				}
			},
		},
		{
			name:          "Invalid Provision Path",
			provisionPath: "invalidPath",
			blueprint:     "validBlueprint",
			expectErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var blueprintDir string
			var cleanup func()
			if tc.setup != nil {
				_, blueprintDir, cleanup = tc.setup(t)
				defer cleanup()
			}

			// Convert blueprintDir to an absolute path
			absBlueprintDir, err := filepath.Abs(blueprintDir)
			if err != nil {
				t.Fatalf("failed to get absolute path for blueprint dir: %v", err)
			}

			// Debug: Log the absolute path being used
			t.Logf("Using blueprint config path: %s", absBlueprintDir)

			cmd := &cobra.Command{}
			cmd.Flags().String("provisionPath", tc.provisionPath, "")
			cmd.Flags().String("blueprint", absBlueprintDir, "") // Use absolute path

			// Set Viper's configuration path to the absolute blueprint directory
			if err := warpgate.SetBlueprintConfigPath(absBlueprintDir); err != nil {
				t.Fatalf("failed to set blueprint config path: %v", err)
			}

			err = warpgate.RunImageBuilder(cmd, nil)
			if (err != nil) != tc.expectErr {
				t.Errorf("runImageBuilder() error = %v, expectErr %v", err, tc.expectErr)
			}
		})
	}
}
