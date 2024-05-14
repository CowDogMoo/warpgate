package blueprint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Blueprint represents the configuration of a blueprint for image building.
//
// **Attributes:**
//
// Name: Name of the blueprint.
// ProvisioningRepo: Path to the repository containing provisioning logic.
type Blueprint struct {
	Name             string `mapstructure:"name"`
	Path             string `mapstructure:"path"`
	ProvisioningRepo string `mapstructure:"provisioning_repo"`
}

// Data holds a blueprint and its associated Packer templates and container
// configuration.
//
// **Attributes:**
//
// Blueprint: The blueprint configuration.
// PackerTemplates: A slice of Packer templates associated with the blueprint.
// Container: The container configuration for the blueprint.
type Data struct {
	Blueprint       Blueprint
	PackerTemplates []packer.BlueprintPacker
	Container       packer.BlueprintContainer
}

// ParseCommandLineFlags parses command line flags for a Blueprint.
//
// **Parameters:**
//
// cmd: A Cobra command object containing flags and arguments for the command.
//
// **Returns:**
//
// error: An error if any issue occurs while parsing the command line flags.
func (b *Blueprint) ParseCommandLineFlags(cmd *cobra.Command) error {
	var err error
	b.ProvisioningRepo, err = cmd.Flags().GetString("provisionPath")
	if err != nil {
		return fmt.Errorf("failed to get provisionPath: %v", err)
	}

	if strings.Contains(b.ProvisioningRepo, "~") {
		b.ProvisioningRepo = sys.ExpandHomeDir(b.ProvisioningRepo)
	}

	b.Name, err = cmd.Flags().GetString("blueprint")
	if err != nil {
		return fmt.Errorf("failed to retrieve blueprint: %v", err)
	}

	repoRoot, err := gitutils.RepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get repo root: %v", err)
	}

	b.Path = filepath.Join(repoRoot, "blueprints", b.Name)
	if _, err := os.Stat(filepath.Join(b.Path, "config.yaml")); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist at %s", filepath.Join(b.Path, "config.yaml"))
	}

	return b.SetConfigPath()
}

// SetConfigPath sets the configuration path for a Blueprint.
//
// **Returns:**
//
// error: An error if the configuration path cannot be set.
func (b *Blueprint) SetConfigPath() error {
	bpConfig := filepath.Join(b.Path, "config.yaml")

	// Ensure the target blueprint config exists
	if _, err := os.Stat(bpConfig); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config file does not exist at %s", bpConfig)
		}
		return fmt.Errorf("error accessing config file at %s: %v", bpConfig, err)
	}

	viper.SetConfigFile(bpConfig)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	return nil
}
