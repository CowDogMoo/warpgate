package blueprint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	gitutils "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/sys"
	recursiveCp "github.com/otiai10/copy"
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
// BuildDir: Path to the temporary build directory.
// PackerTemplates: A slice of Packer templates for building images.
// Path: Path to the blueprint configuration.
// ProvisioningRepo: Path to the repository containing provisioning logic.
// Tag: Tag configuration for the image built by Packer.
type Blueprint struct {
	Name             string                  `mapstructure:"name"`
	BuildDir         string                  `mapstructure:"build_dir"`
	PackerTemplates  []packer.PackerTemplate `mapstructure:"packer_templates"`
	Path             string                  `mapstructure:"path"`
	ProvisioningRepo string                  `mapstructure:"provisioning_repo"`
	Tag              Tag                     `mapstructure:"tag"`
}

// Tag represents the tag configuration for the image built by Packer.
//
// **Attributes:**
//
// Name: Name of the tag.
// Version: Version of the tag.
type Tag struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
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
		return err
	}

	// Expand input provisioning repo path to absolute path
	if strings.Contains(b.ProvisioningRepo, "~") {
		b.ProvisioningRepo = sys.ExpandHomeDir(b.ProvisioningRepo)
	}

	// Ensure the provisioning repo exists
	if _, err := os.Stat(b.ProvisioningRepo); os.IsNotExist(err) {
		return fmt.Errorf("provisioning repo does not exist: %s", b.ProvisioningRepo)
	} else if err != nil {
		return fmt.Errorf("error checking provisioning repo: %v", err)
	}

	b.Name, err = cmd.Flags().GetString("blueprint")
	if err != nil {
		return err
	}

	return nil
}

func configFileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil || os.IsNotExist(err) {
		return false, errors.New("config file does not exist")
	}

	return true, nil
}

// SetConfigPath sets the configuration path for a Blueprint.
//
// **Returns:**
//
// error: An error if the configuration path cannot be set.
func (b *Blueprint) SetConfigPath() error {
	bpConfig := filepath.Join(b.Path, "config.yaml")
	// Ensure the target blueprint config exists
	if _, err := configFileExists(bpConfig); err != nil {
		return err
	}

	viper.SetConfigFile(bpConfig)
	return viper.ReadInConfig()
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
func (b *Blueprint) CreateBuildDir() error {
	if b == nil {
		return fmt.Errorf("blueprint is nil")
	}

	if b.Name == "" {
		return fmt.Errorf("blueprint name is empty")
	}

	buildDir := filepath.Join(os.TempDir(), "builds", "warpgate")
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
	opt := recursiveCp.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			if srcinfo.IsDir() && srcinfo.Name() == ".git" {
				return true, nil
			}
			return false, nil
		},
	}
	if err := recursiveCp.Copy(repoRoot, buildDir, opt); err != nil {
		return fmt.Errorf("failed to copy repo to build directory: %v", err)
	}

	fmt.Printf("Successfully copied repo from %s to build directory %s\n", repoRoot, buildDir)

	b.BuildDir = buildDir
	b.Path = filepath.Join(buildDir, "blueprints", b.Name)

	return nil
}

// LoadPackerTemplates loads Packer templates from the blueprint.
//
// **Returns:**
//
// error: An error if any issue occurs while loading the Packer templates.
func (b *Blueprint) LoadPackerTemplates(githubToken string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no config file used by viper")
	}
	fmt.Printf("Config file used by viper: %s\n", configFile)

	if err := viper.UnmarshalKey("packer_templates", &b.PackerTemplates); err != nil {
		return fmt.Errorf("failed to unmarshal packer templates: %v", err)
	}

	if len(b.PackerTemplates) == 0 {
		return fmt.Errorf("no packer templates found")
	}

	for i := range b.PackerTemplates {
		if err := b.unmarshalTemplate(i, githubToken); err != nil {
			return err
		}
	}

	return nil
}

func (b *Blueprint) unmarshalTemplate(i int, githubToken string) error {
	// Unmarshal AMI if present
	amiKey := fmt.Sprintf("packer_templates.%d.ami", i)
	if viper.IsSet(amiKey) {
		if err := viper.UnmarshalKey(amiKey, &b.PackerTemplates[i].AMI); err != nil {
			return fmt.Errorf("failed to unmarshal AMI settings for template %d: %v", i, err)
		}
	}

	// Unmarshal Container if present
	containerKey := fmt.Sprintf("packer_templates.%d.container", i)
	if viper.IsSet(containerKey) {
		b.PackerTemplates[i].Container.ImageRegistry.Credential = githubToken

		if err := viper.UnmarshalKey(containerKey, &b.PackerTemplates[i].Container); err != nil {
			return fmt.Errorf("failed to unmarshal container settings for template %d: %v", i, err)
		}
	}

	return nil
}

// BuildPackerImages builds packer images concurrently.
//
// **Returns:**
//
// map[string]string: A map of image hashes generated by the build process.
// error: An error if any issue occurs during the build process.
func (b *Blueprint) BuildPackerImages() (map[string]string, error) {
	errChan := make(chan error, len(b.PackerTemplates))
	imageHashesChan := make(chan map[string]string, len(b.PackerTemplates))
	var wg sync.WaitGroup

	for _, pTmpl := range b.PackerTemplates {
		if err := b.ValidatePackerTemplate(); err != nil {
			return nil, err
		}

		wg.Add(1)
		go func(blueprint Blueprint, pTmpl packer.PackerTemplate) {
			defer wg.Done()

			fmt.Printf("Building %s packer template as part of the %s blueprint", blueprint.PackerTemplates[0].ImageValues.Name, blueprint.Name)
			blueprint.PackerTemplates[0] = pTmpl
			hashes, err := b.buildPackerImage()
			if err != nil {
				errChan <- fmt.Errorf("error building %s: %v", pTmpl.ImageValues.Name, err)
				return
			}
			imageHashesChan <- hashes
		}(*b, pTmpl)
	}

	go func() {
		wg.Wait()
		close(imageHashesChan)
		close(errChan)
	}()

	var errOccurred bool
	for err := range errChan {
		if err != nil {
			errOccurred = true
		}
	}

	imageHashes := make(map[string]string)
	for hashes := range imageHashesChan {
		for k, v := range hashes {
			imageHashes[k] = v
		}
	}

	if errOccurred {
		return nil, fmt.Errorf("errors occurred while building packer images")
	}

	fmt.Printf("All packer templates in %s blueprint built successfully!\n", b.Name)
	return imageHashes, nil
}

// PreparePackerArgs prepares the arguments for the packer build command.
//
// **Returns:**
//
// []string: A slice of arguments for the packer build command.
func (b *Blueprint) PreparePackerArgs() []string {
	pTmpl := b.PackerTemplates[0]
	args := []string{
		"-var", fmt.Sprintf("base_image=%s", pTmpl.ImageValues.Name),
		"-var", fmt.Sprintf("base_image_version=%s", pTmpl.ImageValues.Version),
		"-var", fmt.Sprintf("blueprint_name=%s", b.Name),
		"-var", fmt.Sprintf("user=%s", pTmpl.User),
		"-var", fmt.Sprintf("provision_repo_path=%s", b.ProvisioningRepo),
	}

	args = append(args, b.appendAMIArgs(pTmpl)...)
	args = append(args, b.appendContainerArgs(pTmpl)...)

	args = append(args, ".")
	fmt.Printf("Packer Parameters: %s\n", hideSensitiveArgs(args))
	return args
}

func (b *Blueprint) appendAMIArgs(pTmpl packer.PackerTemplate) []string {
	var args []string
	if amiConfig := pTmpl.AMI; amiConfig.InstanceType != "" {
		fmt.Printf("AMI Config: %v\n", amiConfig)
		args = append(args, "-var", fmt.Sprintf("instance_type=%s", amiConfig.InstanceType))
		args = append(args, "-var", fmt.Sprintf("ami_region=%s", amiConfig.Region))
		if amiConfig.SSHUser != "" {
			args = append(args, "-var", fmt.Sprintf("ssh_username=%s", amiConfig.SSHUser))
		}
	}
	return args
}

func (b *Blueprint) appendContainerArgs(pTmpl packer.PackerTemplate) []string {
	var args []string
	if pTmpl.Container.Workdir != "" {
		args = append(args, "-var", fmt.Sprintf("workdir=%s", pTmpl.Container.Workdir))
	}
	if pTmpl.Container.Entrypoint != "" {
		args = append(args, "-var", fmt.Sprintf("entrypoint=%s", pTmpl.Container.Entrypoint))
	}
	return args
}

// buildPackerImage builds a single packer image.
//
// **Parameters:**
//
// blueprint: A Blueprint instance.
//
// **Returns:**
//
// map[string]string: A map of image hashes generated by the build process.
// error: An error if any issue occurs during the build process.
func (b *Blueprint) buildPackerImage() (map[string]string, error) {
	if err := b.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize blueprint: %v", err)
	}

	const maxRetries = 3
	var lastError error
	imageHashes := make(map[string]string)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		hashes, err := b.BuildImageAttempt(attempt)
		if err != nil {
			lastError = err
			continue // retry
		}

		fmt.Printf("Successfully built container image from the %s packer template\n", b.Name)
		for _, hash := range hashes {
			imageHashes[hash.Arch] = hash.Hash
		}
		return imageHashes, nil // success
	}

	// If the loop completes, it means all attempts failed
	return nil, fmt.Errorf("all attempts failed to build container image from %s packer template: %v", b.Name, lastError)
}

// ValidatePackerTemplate validates the Packer template for the blueprint.
//
// **Returns:**
//
// error: An error if the Packer template is invalid.
func (b *Blueprint) ValidatePackerTemplate() error {
	for _, pTmpl := range b.PackerTemplates {
		requiredFields := map[string]string{
			"ImageValues.Name":           pTmpl.ImageValues.Name,
			"ImageValues.Version":        pTmpl.ImageValues.Version,
			"User":                       pTmpl.User,
			"Blueprint.Name":             b.Name,
			"Blueprint.Path":             b.Path,
			"Blueprint.ProvisioningRepo": b.ProvisioningRepo,
		}

		missingFields := []string{}
		for fieldName, fieldValue := range requiredFields {
			if fieldValue == "" {
				missingFields = append(missingFields, fieldName)
			}
		}

		if len(missingFields) > 0 {
			return fmt.Errorf("packer template '%s' has uninitialized fields: %s",
				b.Name, strings.Join(missingFields, ", "))
		}
	}

	return nil
}

// BuildImageAttempt attempts to build the image for the blueprint.
//
// **Parameters:**
//
// attempt: The attempt number for the build.
//
// **Returns:**
//
// map[string]string: The image hashes generated by the build.
// error: An error if any issue occurs during the build process.
func (b *Blueprint) BuildImageAttempt(attempt int) ([]packer.ImageHash, error) {
	if len(b.PackerTemplates) == 0 {
		return nil, fmt.Errorf("no packer templates found")
	}

	if err := viper.UnmarshalKey(ContainerKey, &b.PackerTemplates[0].Container); err != nil {
		return nil, fmt.Errorf("failed to unmarshal container parameters from %s config file: %v", b.Name, err)
	}

	args := b.PreparePackerArgs()

	fmt.Printf("Attempt %d - packer parameters: %s\n", attempt, hideSensitiveArgs(args))

	// Initialize the packer templates directory
	if err := b.PackerTemplates[0].RunInit([]string{"."}, filepath.Join(b.Path, "packer_templates")); err != nil {
		return nil, fmt.Errorf("error initializing packer command: %v", err)
	}

	// Verify the template directory contents
	cmd := sys.Cmd{
		CmdString: "ls",
		Args:      []string{"-la", b.Path},
	}

	if _, err := cmd.RunCmd(); err != nil {
		return nil, fmt.Errorf("failed to list contents of template directory: %v", err)
	}

	// Run the build command
	hashes, amiID, err := b.PackerTemplates[0].RunBuild(args, filepath.Join(b.Path, "packer_templates"))
	if err != nil {
		return nil, fmt.Errorf("error running build command: %v", err)
	}

	// Filter out entries without a hash
	validHashes := []packer.ImageHash{}
	for _, hash := range hashes {
		if hash.Hash != "" {
			validHashes = append(validHashes, hash)
		}
	}

	switch {
	case len(validHashes) > 0:
		fmt.Printf("Image hashes: %v\n", validHashes)
	case amiID != "":
		fmt.Printf("Built AMI ID: %s\n", amiID)
	default:
		fmt.Printf("No container image or AMI found in the build output\n")
	}

	return validHashes, nil
}

func hideSensitiveArgs(args []string) []string {
	logArgs := make([]string, len(args))
	copy(logArgs, args)
	for i, arg := range logArgs {
		if strings.Contains(arg, "registry_cred=") {
			logArgs[i] = "-var registry_cred=<HIDDEN>"
		}
	}
	return logArgs
}
