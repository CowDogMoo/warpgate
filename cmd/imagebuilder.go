package cmd

import (
	"fmt"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/docker"
	"github.com/cowdogmoo/warpgate/pkg/registry"
	log "github.com/l50/goutils/v2/logging"
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
	if err := blueprint.LoadPackerTemplates(); err != nil {
		return fmt.Errorf("no packer templates found: %v", err)
	}

	if len(blueprint.PackerTemplates) == 0 {
		return fmt.Errorf("no packer templates found")
	}

	imageHashes, err := blueprint.BuildPackerImages()
	if err != nil {
		return err
	}

	dockerClient, err := docker.NewDockerClient()
	if err != nil {
		return err
	}

	if err := dockerClient.TagAndPushImages(blueprint.PackerTemplates, githubToken, blueprint.Name, imageHashes); err != nil {
		return err
	}

	return nil
}
