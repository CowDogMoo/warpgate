package cmd

import (
	"fmt"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprintName, err := cmd.Flags().GetString("blueprint")
			if err != nil {
				return err
			}

			githubToken, err = cmd.Flags().GetString("github-token")
			if err != nil {
				return err
			}

			if blueprintName == "" {
				return fmt.Errorf("blueprint name must not be empty")
			}

			// Read the specific blueprint configuration file
			blueprintConfigFile := filepath.Join("blueprints", blueprintName, "config.yaml")
			viper.SetConfigFile(blueprintConfigFile)

			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("error reading blueprint config file: %w", err)
			}

			tagName := viper.GetString("blueprint.packer_templates.tag.name")
			tagVersion := viper.GetString("blueprint.packer_templates.tag.version")

			if tagName == "" || tagVersion == "" {
				return fmt.Errorf("blueprint tag name and version must not be empty")
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
	cobra.CheckErr(imageBuilderCmd.MarkFlagRequired("provisionPath"))

	imageBuilderCmd.Flags().StringP(
		"blueprint", "b", "", "The blueprint to use for image building.")
	cobra.CheckErr(imageBuilderCmd.MarkFlagRequired("blueprint"))

	imageBuilderCmd.Flags().StringP(
		"github-token", "t", "", "GitHub token to authenticate with (optional, will use GITHUB_TOKEN env var if not provided)")
	cobra.CheckErr(viper.BindPFlag("blueprint.image_hashes.container.registry.token", imageBuilderCmd.Flags().Lookup("github-token")))
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

	buildResult, err := blueprint.BuildPackerImages()
	if err != nil {
		return err
	}

	// Ensure at least one image is built
	if len(buildResult) == 0 {
		return fmt.Errorf("no images were built")
	}

	// Check if container configuration is needed
	if blueprint.PackerTemplates.Container.ImageRegistry.Server != "" {
		// Create ContainerImageRegistry object
		registryConfig := packer.ContainerImageRegistry{
			Server:     viper.GetString("blueprint.packer_templates.container.registry.server"),
			Username:   viper.GetString("blueprint.packer_templates.container.registry.username"),
			Credential: githubToken,
		}

		if registryConfig.Server == "" || registryConfig.Username == "" || registryConfig.Credential == "" {
			return fmt.Errorf("container registry configuration must not be empty")
		}

		// New DockerClient initialization with username and token
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
