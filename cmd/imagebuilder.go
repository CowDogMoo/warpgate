/*
Copyright © 2022 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/bitfield/script"
	"github.com/fatih/color"
	goutils "github.com/l50/goutils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// imageBuilderCmd represents the imageBuilder command
var imageBuilderCmd = &cobra.Command{
	Use:   "imageBuilder",
	Short: "Build a container image using packer and a provisoning repo",
	Run: func(cmd *cobra.Command, args []string) {
		// Local vars
		var packerTemplates []string
		var blueprintName string
		var blueprintPath string

		// Viper keys
		blueprintPathKey := "blueprint_path"
		blueprintNameKey := "blueprint_name"
		packerTemplatesKey := "packer_templates"

		// Get base version from CLI args
		baseImgVer, err := cmd.Flags().GetString("baseImgVer")
		if err != nil {
			log.WithError(err).Errorf(
				"failed to get baseImgVer from CLI input: %v", err)
			os.Exit(1)
		}

		// Get new image tag from CLI args
		newImgTag, err := cmd.Flags().GetString("newImgTag")
		if err != nil {
			log.WithError(err).Errorf(
				"failed to get newImgTag from CLI input: %v", err)
			os.Exit(1)
		}

		// Get new image version from CLI args
		newVer, err := cmd.Flags().GetString("newVer")
		if err != nil {
			log.WithError(err).Errorf(
				"failed to get newVer from CLI input: %v", err)
			os.Exit(1)
		}

		// Get local path to provision repo from CLI args
		provisionRepoPath, err := cmd.Flags().GetString("provisionPath")
		if err != nil {
			log.WithError(err).Errorf(
				"failed to get provisionPath from CLI input: %v", err)
			os.Exit(1)
		}

		// Get packer templates to build
		if err = viper.UnmarshalKey(packerTemplatesKey, &packerTemplates); err != nil {
			log.WithError(err).Errorf(
				"failed to unmarshal packer templates from config file: %v", err)
			os.Exit(1)
		}

		// Get blueprint path
		if err = viper.UnmarshalKey(blueprintPathKey, &blueprintPath); err != nil {
			log.WithError(err).Errorf(
				"failed to unmarshal blueprint path from config file: %v", err)
			os.Exit(1)
		}

		// Get blueprint name
		if err = viper.UnmarshalKey(blueprintNameKey, &blueprintName); err != nil {
			log.WithError(err).Errorf(
				"failed to unmarshal blueprint name from config file: %v", err)
			os.Exit(1)
		}

		// Set env vars required for image building
		os.Setenv("IMAGE_TAG", newImgTag)
		os.Setenv("BASE_IMAGE_VERSION", baseImgVer)
		os.Setenv("NEW_IMAGE_VERSION", newVer)

		// Change into the blueprint directory
		if err := goutils.Cd(blueprintPath); err != nil {
			log.WithError(err).Errorf(
				"failed to change into the %s directory: %v", blueprintPath, err)
			os.Exit(1)
		}

		// Build each template listed in the blueprint's config.yaml
		for _, pTmpl := range packerTemplates {
			color.YellowString("Now building the %s template in the %s blueprint, please wait.", blueprintName, pTmpl)

			cmd := fmt.Sprintf("packer build -var \"provision_repo_path=%s\" %s ", provisionRepoPath, pTmpl)

			if _, err := script.Exec(cmd).Stdout(); err != nil {
				log.WithError(err).Errorf(
					"failed to build container image for %s blueprint from the %s packer template: %v", blueprintName, pTmpl, err)
				os.Exit(1)
			}
		}

		s := "All packer templates in the " + blueprintName + " blueprint were built successfully!"
		fmt.Print(color.GreenString(s))
	},
}

func init() {
	rootCmd.AddCommand(imageBuilderCmd)

	imageBuilderCmd.Flags().StringP(
		"baseImgVer", "b", "", "Version of the base image to use")

	imageBuilderCmd.Flags().StringP(
		"newImgTag", "n", "", "Tag for the created image")

	imageBuilderCmd.Flags().StringP(
		"newVer", "v", "", "Version for the created image")

	imageBuilderCmd.Flags().StringP(
		"provisionPath", "p", "", "Local path to the repo with provisioning logic that will be used by packer")
}
