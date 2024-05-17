package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/cowdogmoo/warpgate/pkg/registry"
	"github.com/fatih/color"
	log "github.com/l50/goutils/v2/logging"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	githubToken string

	imageBuilderCmd = &cobra.Command{
		Use:   "imageBuilder",
		Short: "Build a container image using packer and a provisioning repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			blueprint.Name, err = cmd.Flags().GetString("blueprint")
			if err != nil {
				return err
			}

			githubToken, err = cmd.Flags().GetString("github-token")
			if err != nil {
				return err
			}

			if err := blueprint.CreateBuildDir(); err != nil {
				return err
			}

			if err := blueprint.ParseCommandLineFlags(cmd); err != nil {
				return err
			}

			if err := blueprint.SetConfigPath(); err != nil {
				return err
			}

			if err := registry.ValidateToken(githubToken); err != nil {
				return err
			}

			return RunImageBuilder(cmd, args, blueprint)
		},
	}
)

func init() {
	rootCmd.AddCommand(imageBuilderCmd)

	imageBuilderCmd.Flags().StringP(
		"provisionPath", "p", "", "Local path to the repo with provisioning logic that will be used by packer")
	if err := imageBuilderCmd.MarkFlagRequired("provisionPath"); err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}

	imageBuilderCmd.Flags().StringP(
		"blueprint", "b", "", "The blueprint to use for image building.")
	if err := imageBuilderCmd.MarkFlagRequired("blueprint"); err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}

	imageBuilderCmd.Flags().StringP(
		"github-token", "t", "", "GitHub token to authenticate with (optional, will use GITHUB_TOKEN env var if not provided)")
	err := viper.BindPFlag("container.registry.token", imageBuilderCmd.Flags().Lookup("github-token"))
	if err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}
}

// RunImageBuilder is the main function for the imageBuilder command
// that builds container images using Packer.
//
// **Parameters:**
//
// - cmd: A Cobra command object containing flags and arguments for the command.
// - args: A slice of strings containing additional arguments passed to the command.
// - blueprint: A Blueprint struct containing the blueprint configuration.
//
// **Returns:**
//
// - error: An error if any issue occurs while building the images.
func RunImageBuilder(cmd *cobra.Command, args []string, blueprint bp.Blueprint) error {
	packerTemplates, err := packer.LoadPackerTemplates()
	if err != nil || packerTemplates == nil {
		return fmt.Errorf("no packer templates found: %v", err)
	}

	if len(packerTemplates) == 0 {
		return fmt.Errorf("no packer templates found")
	}

	if err := buildPackerImages(blueprint, packerTemplates); err != nil {
		return err
	}

	dockerClient, err := docker.NewDockerClient()
	if err != nil {
		return err
	}

	if err := dockerClient.TagAndPushImages(packerTemplates); err != nil {
		return err
	}

	return nil
}

func validatePackerTemplate(pTmpl packer.BlueprintPacker, blueprint bp.Blueprint) error {
	requiredFields := map[string]string{
		"Base.Name":                  pTmpl.Base.Name,
		"Base.Version":               pTmpl.Base.Version,
		"Tag.Name":                   pTmpl.Tag.Name,
		"Tag.Version":                pTmpl.Tag.Version,
		"User":                       pTmpl.User,
		"Blueprint.Name":             blueprint.Name,
		"Blueprint.Path":             blueprint.Path,
		"Blueprint.ProvisioningRepo": blueprint.ProvisioningRepo,
	}

	missingFields := []string{}

	for fieldName, fieldValue := range requiredFields {
		if fieldValue == "" {
			missingFields = append(missingFields, fieldName)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("packer template '%s' has uninitialized fields: %s", pTmpl.Base.Name, strings.Join(missingFields, ", "))
	}

	return nil
}

func buildPackerImages(blueprint bp.Blueprint, packerTemplates []packer.BlueprintPacker) error {
	errChan := make(chan error, len(packerTemplates))
	var wg sync.WaitGroup

	for _, pTmpl := range packerTemplates {
		if err := validatePackerTemplate(pTmpl, blueprint); err != nil {
			return err
		}

		wg.Add(1)
		go func(pTmpl packer.BlueprintPacker, blueprint bp.Blueprint) {
			defer wg.Done()

			log.L().Printf("Building %s packer template as part of the %s blueprint", pTmpl.Base.Name, blueprint.Name)
			if err := buildPackerImage(&pTmpl, blueprint); err != nil {
				errChan <- fmt.Errorf("error building %s: %v", pTmpl.Base.Name, err)
			}
		}(pTmpl, blueprint)
	}

	wg.Wait()
	close(errChan)

	var errOccurred bool
	for err := range errChan {
		if err != nil {
			log.L().Error(err)
			errOccurred = true
		}
	}

	if errOccurred {
		return fmt.Errorf("errors occurred while building packer images")
	}

	log.L().Printf(color.GreenString("All packer templates in %s blueprint built successfully!\n", blueprint.Name))
	return nil
}

func buildPackerImage(pTmpl *packer.BlueprintPacker, blueprint bp.Blueprint) error {
	if err := blueprint.Initialize(); err != nil {
		log.L().Errorf("Failed to initialize blueprint: %v", err)
		return err
	}

	const maxRetries = 3
	var lastError error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := buildImageAttempt(pTmpl, blueprint, attempt); err != nil {
			lastError = err
			continue // retry
		}

		log.L().Printf("Successfully built container image from the %s packer template as part of the %s blueprint",
			pTmpl.Base.Name, blueprint.Name)

		return nil // success
	}

	// If the loop completes, it means all attempts failed
	return fmt.Errorf("all attempts failed to build container image from %s packer template: %v", pTmpl.Base.Name, lastError)
}

func buildImageAttempt(pTmpl *packer.BlueprintPacker, blueprint bp.Blueprint, attempt int) error {
	if err := viper.UnmarshalKey(bp.ContainerKey, &pTmpl.Container); err != nil {
		log.L().Errorf("Failed to unmarshal container parameters from %s config file: %v", blueprint.Name, err)
		return err
	}

	args := preparePackerArgs(pTmpl, blueprint)

	log.L().Debugf("Attempt %d - Packer Parameters: %s\n", attempt, hideSensitiveArgs(args))

	// Initialize the packer templates directory
	log.L().Printf("Initializing %s packer template as part of the %s blueprint", pTmpl.Base.Name, blueprint.Name)
	if err := pTmpl.RunInit([]string{"."}, filepath.Join(blueprint.Path, "packer_templates")); err != nil {
		return fmt.Errorf("error initializing packer command: %v", err)
	}

	// Verify the template directory contents
	log.L().Debugf("Contents of the %s build directory\n", blueprint.Name)
	cmd := sys.Cmd{
		CmdString: "ls",
		Args:      []string{"-la", blueprint.Path},
	}

	if _, err := cmd.RunCmd(); err != nil {
		log.L().Errorf("Failed to list contents of template directory: %v", err)
		return err
	}

	// Run the build command
	log.L().Printf("Building %s packer template as part of the %s blueprint", pTmpl.Base.Name, blueprint.Name)
	if err := pTmpl.RunBuild(args, filepath.Join(blueprint.Path, "packer_templates")); err != nil {
		return fmt.Errorf("error running build command: %v", err)
	}

	return nil
}

func preparePackerArgs(pTmpl *packer.BlueprintPacker, blueprint bp.Blueprint) []string {
	args := []string{
		"-var", fmt.Sprintf("base_image=%s", pTmpl.Base.Name),
		"-var", fmt.Sprintf("base_image_version=%s", pTmpl.Base.Version),
		"-var", fmt.Sprintf("blueprint_name=%s", blueprint.Name),
		"-var", fmt.Sprintf("user=%s", pTmpl.User),
		"-var", fmt.Sprintf("provision_repo_path=%s", blueprint.ProvisioningRepo),
		"-var", fmt.Sprintf("workdir=%s", pTmpl.Container.Workdir),
	}

	// Add AMI specific variables if they exist
	if amiConfig := pTmpl.AMI; amiConfig.InstanceType != "" {
		args = append(args, "-var", fmt.Sprintf("instance_type=%s", pTmpl.AMI.InstanceType))
		args = append(args, "-var", fmt.Sprintf("region=%s", pTmpl.AMI.Region))
		args = append(args, "-var", fmt.Sprintf("ssh_user=%s", pTmpl.AMI.SSHUser))
	}

	// Add entrypoint if it's set
	if pTmpl.Container.Entrypoint != "" {
		args = append(args, "-var", fmt.Sprintf("entrypoint=%s", pTmpl.Container.Entrypoint))
	}
	args = append(args, ".")
	log.L().Debugf("Packer Parameters: %s\n", hideSensitiveArgs(args))

	return args
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
