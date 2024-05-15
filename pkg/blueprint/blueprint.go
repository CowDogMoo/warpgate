package blueprint

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	"github.com/otiai10/copy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	BlueprintKey       = "blueprint"
	ContainerKey       = "container"
	PackerTemplatesKey = "packer_templates"
)

// Blueprint represents the configuration of a blueprint for image building.
//
// **Attributes:**
//
// Name: Name of the blueprint.
// Path: Path to the blueprint configuration.
// ProvisioningRepo: Path to the repository containing provisioning logic.
// BuildDir: Path to the temporary build directory.
type Blueprint struct {
	Name             string `mapstructure:"name"`
	Path             string `mapstructure:"path"`
	ProvisioningRepo string `mapstructure:"provisioning_repo"`
	BuildDir         string `mapstructure:"build_dir"`
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

	// Expand input provisioning repo path to absolute path
	b.ProvisioningRepo = sys.ExpandHomeDir(b.ProvisioningRepo)

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

// Initialize initializes the blueprint by setting up the necessary packer templates.
//
// **Returns:**
//
// error: An error if the initialization fails.
func (b *Blueprint) Initialize() error {
	if b.BuildDir == "" {
		if err := b.CreateBuildDir(); err != nil {
			return fmt.Errorf("failed to create build directory: %v", err)
		}
	}

	if b.Path == "" || b.Name == "" {
		return fmt.Errorf("blueprint path or name is not set")
	}

	packerDir := filepath.Join(b.Path, "packer_templates")

	// Ensure the packer templates directory exists
	if _, err := os.Stat(packerDir); os.IsNotExist(err) {
		return fmt.Errorf("expected packer_templates directory not found in %s", b.Path)
	} else if err != nil {
		return fmt.Errorf("error checking packer_templates directory: %v", err)
	}

	// Run packer init in the packer templates directory
	cmd := sys.Cmd{
		CmdString:     "packer",
		Args:          []string{"init", "."},
		Dir:           packerDir,
		OutputHandler: func(s string) { fmt.Println(s) },
	}

	if _, err := cmd.RunCmd(); err != nil {
		return fmt.Errorf("failed to initialize blueprint with packer init: %v", err)
	}

	return nil
}

// CreateBuildDir creates a temporary build directory and copies the repo into it.
//
// **Returns:**
//
// error: An error if the build directory creation or repo copy fails.
// CreateBuildDir creates a temporary build directory and copies the repo into it.
//
// **Returns:**
//
// error: An error if the build directory creation or repo copy fails.
func (b *Blueprint) CreateBuildDir() error {
	if b == nil {
		return fmt.Errorf("blueprint is nil")
	}

	if b.Name == "" {
		return fmt.Errorf("blueprint name is empty")
	}

	buildDir := filepath.Join(os.TempDir(), "builds", b.Name)
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			return fmt.Errorf("failed to create build directory %s: %v", buildDir, err)
		}
	}

	repoRoot, err := gitutils.RepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get repo root: %v", err)
	}

	// Copy the warpgate repo to the build directory, excluding .git
	opt := copy.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			if srcinfo.IsDir() && srcinfo.Name() == ".git" {
				return true, nil
			}
			return false, nil
		},
	}
	if err := copy.Copy(repoRoot, buildDir, opt); err != nil {
		return fmt.Errorf("failed to copy repo to build directory: %v", err)
	}

	fmt.Printf("Successfully copied repo from %s to build directory %s", repoRoot, buildDir)

	b.BuildDir = buildDir
	b.Path = filepath.Join(buildDir, "blueprints", b.Name)

	return nil
}
