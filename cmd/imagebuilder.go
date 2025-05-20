package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/docker"
	packer "github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/cowdogmoo/warpgate/pkg/registry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	githubToken string

	imageBuilderCmd = &cobra.Command{
		Use:   "imageBuilder",
		Short: "Build a container image using packer and a provisioning repo",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate required flags
			blueprintName, err := cmd.Flags().GetString("blueprint")
			if err != nil {
				return err
			}
			if blueprintName == "" {
				return fmt.Errorf("blueprint name must not be empty")
			}

			provisionPath, err := cmd.Flags().GetString("provisionPath")
			if err != nil {
				return err
			}
			if provisionPath == "" {
				return fmt.Errorf("provision path must not be empty")
			}

			// Check if the provision path exists
			if _, err := os.Stat(provisionPath); os.IsNotExist(err) {
				return fmt.Errorf("provision path does not exist: %s", provisionPath)
			}

			// Handle GitHub token - try flag first, then environment variable
			token, err := cmd.Flags().GetString("github-token")
			if err != nil {
				return err
			}

			// If token is not provided via flag, try environment variable
			if token == "" {
				token = os.Getenv("GITHUB_TOKEN")
				// Set the flag value for consistent access later
				if token != "" {
					cmd.Flags().Set("github-token", token)
				}
			}

			githubToken = token

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprintName, _ := cmd.Flags().GetString("blueprint")

			// Read the specific blueprint configuration file
			blueprintConfigFile := filepath.Join("blueprints", blueprintName, "config.yaml")
			viper.SetConfigFile(blueprintConfigFile)

			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("error reading blueprint config file: %w", err)
			}

			tagName := viper.GetString("blueprint.packer_templates.tag.name")
			tagVersion := viper.GetString("blueprint.packer_templates.tag.version")

			if tagName == "" || tagVersion == "" {
				return fmt.Errorf("blueprint tag name and version must not be empty in config file")
			}

			blueprint := bp.Blueprint{
				Name: blueprintName,
				PackerTemplates: &packer.PackerTemplates{
					Tag: packer.Tag{
						Name:    tagName,
						Version: tagVersion,
					},
				},
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

			// Only validate token if it's provided and we're pushing to registry
			noPush, _ := cmd.Flags().GetBool("no-push")
			if !noPush && githubToken != "" {
				if err := registry.ValidateToken(githubToken); err != nil {
					return fmt.Errorf("invalid GitHub token: %w", err)
				}
			} else if !noPush && githubToken == "" {
				return fmt.Errorf("GitHub token is required when pushing to registry. Provide it with -t/--github-token flag or set GITHUB_TOKEN environment variable")
			}

			return RunImageBuilder(cmd, args, blueprint)
		},
	}
)

func init() {
	rootCmd.AddCommand(imageBuilderCmd)

	imageBuilderCmd.Flags().StringP(
		"provisionPath", "p", "", "Local path to the repo with provisioning logic that will be used by packer")
	cobra.CheckErr(imageBuilderCmd.MarkFlagRequired("provisionPath"))

	imageBuilderCmd.Flags().StringP(
		"blueprint", "b", "", "The blueprint to use for image building.")
	cobra.CheckErr(imageBuilderCmd.MarkFlagRequired("blueprint"))

	imageBuilderCmd.Flags().StringP(
		"github-token", "t", "", "GitHub token to authenticate with (optional, will use GITHUB_TOKEN env var if not provided)")
	viper.BindPFlag("blueprint.image_hashes.container.registry.token", imageBuilderCmd.Flags().Lookup("github-token"))

	imageBuilderCmd.Flags().BoolP(
		"no-push", "", false, "Build container images locally without pushing to registry")

	imageBuilderCmd.Flags().BoolP(
		"no-containers", "", false, "Skip building container images")

	imageBuilderCmd.Flags().BoolP(
		"no-ami", "", false, "Skip building AMI images")
}

// RunImageBuilder is the main function for the imageBuilder command
// that builds container images using Packer.
//
// **Parameters:**
//
// cmd: A Cobra command object containing flags and arguments for the command.
// args: A slice of strings containing additional arguments passed to the command.
// blueprint: A Blueprint struct containing the blueprint configuration.
//
// **Returns:**
//
// error: An error if any issue occurs while building the images.
func RunImageBuilder(cmd *cobra.Command, args []string, blueprint bp.Blueprint) error {
	err := blueprint.LoadPackerTemplates(githubToken)
	if err != nil {
		return fmt.Errorf("no packer templates found: %v", err)
	}

	// Get the flag values and set them in the PackerTemplates
	noAMI, _ := cmd.Flags().GetBool("no-ami")
	noContainers, _ := cmd.Flags().GetBool("no-containers")
	blueprint.PackerTemplates.NoAMI = noAMI
	blueprint.PackerTemplates.NoContainers = noContainers

	imageHashes, err := blueprint.BuildPackerImages()
	if err != nil {
		return err
	}

	// Get the no-push flag value
	noPush, _ := cmd.Flags().GetBool("no-push")

	// Only process Docker images if there are actually image hashes to push and we didn't disable containers
	if !noPush && len(imageHashes) > 0 && !noContainers && blueprint.PackerTemplates.Container.ImageRegistry.Server != "" {
		registryConfig := packer.ContainerImageRegistry{
			Server:     viper.GetString("blueprint.packer_templates.container.registry.server"),
			Username:   viper.GetString("blueprint.packer_templates.container.registry.username"),
			Credential: githubToken,
		}
		if registryConfig.Server == "" || registryConfig.Username == "" {
			return fmt.Errorf("container registry server and username configuration must not be empty")
		}

		// Only require credential if we're pushing
		if registryConfig.Credential == "" && !noPush {
			return fmt.Errorf("GitHub token is required for pushing to container registry")
		}

		dockerClient, err := docker.NewDockerClient(registryConfig)
		if err != nil {
			return err
		}
		if err := dockerClient.ProcessTemplates(blueprint.PackerTemplates, blueprint.Name); err != nil {
			return fmt.Errorf("error processing Packer template: %v", err)
		}
	}

	return nil
}
