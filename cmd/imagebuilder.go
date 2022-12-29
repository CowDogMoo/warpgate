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
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bitfield/script"
	"github.com/fatih/color"
	goutils "github.com/l50/goutils"
	cp "github.com/otiai10/copy"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// PackerTemplate is used to hold the information used
// with a blueprint's packer templates.
type PackerTemplate struct {
	// Name of the Packer template
	Name string `yaml:"name"`

	// Base represents the base image used
	// by Packer for image building.
	Base struct {
		// Base image to use
		Name string `yaml:"name"`
		// Base image version to use
		Version string `yaml:"version"`
	} `yaml:"base"`

	// Systemd dictates creation of a container with systemd.
	Systemd bool `yaml:"systemd"`

	// Tag represents the new tag for the image built by Packer.
	Tag struct {
		// Name of the new image
		Name string `yaml:"name"`
		// Version of the new image
		Version string `yaml:"version"`
	} `yaml:"tag"`
}

// imageBuilderCmd represents the imageBuilder command
var imageBuilderCmd = &cobra.Command{
	Use:   "imageBuilder",
	Short: "Build a container image using packer and a provisoning repo",
	Run: func(cmd *cobra.Command, args []string) {
		var packerTemplates []PackerTemplate

		// Viper key
		packerTemplatesKey := "packer_templates"

		/* Retrieve CLI args */

		// Get local path to provision repo from CLI args
		inputPath, err := cmd.Flags().GetString("provisionPath")
		if err != nil {
			log.WithError(err).Errorf(
				"failed to get provisionPath from CLI input: %v", err)
			os.Exit(1)
		}

		/* Unmarshal viper values */

		// Get packer templates
		if err = viper.UnmarshalKey(packerTemplatesKey, &packerTemplates); err != nil {
			log.WithError(err).Errorf(
				"failed to unmarshal packer templates from config file: %v", err)
			os.Exit(1)
		}

		// Get blueprint information
		if err = viper.UnmarshalKey(blueprint.Key, &blueprint); err != nil {
			log.WithError(err).Errorf(
				"failed to unmarshal blueprint path from config file: %v", err)
			os.Exit(1)
		}

		// Change into the blueprint directory
		if err := goutils.Cd(blueprint.Path); err != nil {
			log.WithError(err).Errorf(
				"failed to change into the %s directory: %v", blueprint.Path, err)
			os.Exit(1)
		}

		// Get current user's home directory
		usr, err := user.Current()
		if err != nil {
			log.WithError(err).Errorf(
				"failed to get current user's home directory: %v", err)
			os.Exit(1)
		}

		homedir := usr.HomeDir
		// Resource: https://stackoverflow.com/questions/17609732/expand-tilde-to-home-directory
		// Account for ~ being passed in by itself
		if strings.HasPrefix(inputPath, "~/") {
			// Use strings.HasPrefix so we don't match paths like
			// "/something/~/something/"
			blueprint.ProvisioningRepo = filepath.Join(homedir, inputPath[2:])
		} else if inputPath == "~" {
			blueprint.ProvisioningRepo = homedir
		}

		// Build each template listed in the blueprint's config.yaml
		var wg sync.WaitGroup
		for _, pTmpl := range packerTemplates {
			wg.Add(1)
			go func(pTmpl PackerTemplate, blueprint Blueprint) {
				defer wg.Done()
				fmt.Print(color.YellowString(
					"Now building the %s template as part of the %s blueprint, please wait.\n",
					pTmpl.Name, blueprint.Name))

				if err := buildPackerImage(pTmpl, blueprint); err != nil {
					log.WithError(err)
					os.Exit(1)
				}
			}(pTmpl, blueprint)
		}
		wg.Wait()

		s := "All packer templates in the " + blueprint.Name + " blueprint were built successfully!"
		fmt.Print(color.GreenString(s))
	},
}

func init() {
	rootCmd.AddCommand(imageBuilderCmd)

	imageBuilderCmd.Flags().StringP(
		"provisionPath", "p", "", "Local path to the repo with provisioning logic that will be used by packer")
	if err := imageBuilderCmd.MarkFlagRequired("provisionPath"); err != nil {
		log.WithError(err)
		os.Exit(1)
	}
}

func createBuildDir(pTmpl PackerTemplate, blueprint Blueprint) (string, error) {
	// Create random name for the build directory
	dirName, err := goutils.RandomString(8)
	if err != nil {
		log.WithError(err).Errorf(
			"failed to get random string for buildDir: %v", err)
		return "", err
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.WithError(err).Errorf(
			"failed to get working directory: %v", err)
		return "", err
	}

	// Create buildDir
	buildDir := filepath.Join("/tmp", "builds", dirName)
	if err := goutils.CreateDirectory(buildDir); err != nil {
		log.WithError(err).Errorf(
			"failed to create %s build directory for image building: %v", dirName, err)
		return "", err
	}

	files, err := os.ReadDir(cwd)
	if err != nil {
		log.WithError(err).Errorf(
			"failed to read files from %s: %v", cwd, err)
		return "", err
	}

	// packer_templates is excluded here since we need
	// to get the specific packer template specified
	// in `pTmpl.Name`
	excludedFiles := []string{"builds", "packer_templates",
		"config.yaml", ".gitignore", "README.md"}

	// Copy blueprint to build dir
	for _, file := range files {
		if !goutils.StringInSlice(file.Name(), excludedFiles) {
			dst := filepath.Join(buildDir, file.Name())
			if err := cp.Copy(file.Name(), dst); err != nil {
				log.WithError(err).Errorf(
					"failed to copy %s to %s: %v", file.Name(), buildDir, err)
				return "", err
			}
		}
	}

	// Copy packer template into build dir
	src := filepath.Join(cwd, "packer_templates", pTmpl.Name)
	dst := filepath.Join(buildDir, pTmpl.Name)
	if err := cp.Copy(src, dst); err != nil {
		log.WithError(err).Errorf(
			"failed to transfer packer template to %s: %v", buildDir, err)
		return "", err
	}

	return buildDir, nil
}

func buildPackerImage(pTmpl PackerTemplate, blueprint Blueprint) error {
	buildDir, err := createBuildDir(pTmpl, blueprint)
	if err != nil {
		log.WithError(err).Errorf(
			"failed to create build directory: %v", err)
		return err
	}

	// Create the provision command for the input packer template
	provisionCmd := fmt.Sprintf(
		"packer build "+
			"-var 'base_image=%s' "+
			"-var 'base_image_version=%s' "+
			"-var 'new_image_tag=%s' "+
			"-var 'new_image_version=%s' "+
			"-var 'provision_repo_path=%s' "+
			"-var 'setup_systemd=%t' "+
			"-var 'registry_cred=%s' %s",
		pTmpl.Base.Name,
		pTmpl.Base.Version,
		pTmpl.Tag.Name,
		pTmpl.Tag.Version,
		blueprint.ProvisioningRepo,
		pTmpl.Systemd,
		os.Getenv("PAT"),
		buildDir)

	log.Debug(provisionCmd)

	if _, err := script.Exec(provisionCmd).Stdout(); err != nil {
		if err != nil {
			log.WithError(err).Errorf(
				"failed to build container image from the %s "+
					"packer template that's part of the %s blueprint: %v",
				pTmpl.Name, blueprint.Name, err)
			return err
		}
	}

	return nil

}
