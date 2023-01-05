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
	"path/filepath"
	"sync"

	"github.com/bitfield/script"
	"github.com/fatih/color"
	goutils "github.com/l50/goutils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// PackerTemplate is used to hold the information used
// with a blueprint's packer templates.
type PackerTemplate struct {
	// Name of the Packer template
	Name      string    `yaml:"name"`
	Container Container `yaml:"container"`
	Base      Base      `yaml:"base"`
	Tag       Tag       `yaml:"tag"`
	// Systemd dictates creation of a container with systemd.
	Systemd bool `yaml:"systemd"`
}

// Base represents the base image used
// by Packer for image building.
type Base struct {
	// Base image(s) to use
	Name string `yaml:"name"`
	// Base image version(s) to use
	Version string `yaml:"version"`
}

// Tag represents the new tag for the image built by Packer.
type Tag struct {
	// Name of the new image
	Name string `yaml:"name"`
	// Version of the new image
	Version string `yaml:"version"`
}

// Container stores information that is used for
// the container build process.
type Container struct {
	Workdir    string `yaml:"container.workdir"`
	User       string `yaml:"container.user"`
	Entrypoint string `yaml:"container.entrypoint"`
	Registry   Registry
}

// Registry stores container registry information that is
// used after image building to commit the image.
type Registry struct {
	// Registry server to connect to.
	Server string `yaml:"registry.server"`
	// Username to authenticate to the registry server.
	Username string `yaml:"registry.username"`
	// Credential to authenticate to the registry server.
	Credential string
}

var (
	bpConfig        string
	imageBuilderCmd = &cobra.Command{
		Use:   "imageBuilder",
		Short: "Build a container image using packer and a provisoning repo",
		Run: func(cmd *cobra.Command, args []string) {
			// Get local path to provision repo from CLI args
			blueprint.ProvisioningRepo, err = cmd.Flags().GetString("provisionPath")
			if err != nil {
				log.WithError(err).Errorf(
					"failed to get provisionPath from CLI input: %v", err)
				os.Exit(1)
			}

			// Get the blueprint to build.
			blueprint.Name, err = cmd.Flags().GetString(blueprintKey)
			if err != nil {
				log.WithError(err).Errorf(
					"failed to retrieve blueprint to build from CLI input: %v", err)
				os.Exit(1)
			}

			// Get blueprint information
			if err = viper.UnmarshalKey(blueprintKey, &blueprint); err != nil {
				log.WithError(err).Errorf(
					"failed to unmarshal blueprint path from config file: %v", err)
				os.Exit(1)
			}

			bpConfig = filepath.Join("blueprints", blueprint.Name, "config.yaml")

			// Validate input provisioning repo path.
			if _, err := os.Stat(blueprint.ProvisioningRepo); err != nil {
				log.WithError(err).Errorf("invalid provisionPath %s input: %v", blueprint.ProvisioningRepo, err)
				os.Exit(1)
			}

			viper.AddConfigPath(filepath.Dir(bpConfig))
			viper.SetConfigName(filepath.Base(bpConfig))

			if err := viper.MergeInConfig(); err != nil {
				log.WithError(err).Errorf(
					"failed to merge %s blueprint config into the existing config: %v", blueprint.Name, err)
				os.Exit(1)
			}

			log.Debug("Using config file: ", viper.ConfigFileUsed())

			var packerTemplates []PackerTemplate

			// Get packer templates
			if err = viper.UnmarshalKey(packerTemplatesKey, &packerTemplates); err != nil {
				log.WithError(err).Errorf(
					"failed to unmarshal packer templates from config file: %v", err)
				os.Exit(1)
			}

			log.Debug(viper.Get("blueprint.name"))

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
)

func init() {
	rootCmd.AddCommand(imageBuilderCmd)

	imageBuilderCmd.Flags().StringP(
		"provisionPath", "p", "", "Local path to the repo with provisioning logic that will be used by packer")
	if err := imageBuilderCmd.MarkFlagRequired("provisionPath"); err != nil {
		log.WithError(err)
		os.Exit(1)
	}

	imageBuilderCmd.Flags().StringP(
		"blueprint", "b", "", "The blueprint to use for image building.")
	if err := imageBuilderCmd.MarkFlagRequired("blueprint"); err != nil {
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

	// Create buildDir
	buildDir := filepath.Join("/tmp", "builds", dirName)
	if err := goutils.CreateDirectory(buildDir); err != nil {
		log.WithError(err).Errorf(
			"failed to create %s build directory for image building: %v", dirName, err)
		return "", err
	}

	bpDir := filepath.Dir(bpConfig)
	files, err := os.ReadDir(bpDir)
	if err != nil {
		log.WithError(err).Errorf(
			"failed to list files in %s: %v", bpDir, err)
		return "", err
	}

	// packer_templates is excluded here because we need
	// to get the specific packer template specified
	// in `pTmpl.Name`.
	excludedFiles := []string{"builds", "packer_templates",
		"config.yaml", ".gitignore", "README.md"}

	// Copy blueprint to build dir
	for _, file := range files {
		if !goutils.StringInSlice(file.Name(), excludedFiles) {
			src := filepath.Join(bpDir, file.Name())
			dst := filepath.Join(buildDir, file.Name())
			if err := goutils.Cp(src, dst); err != nil {
				log.WithError(err).Errorf(
					"failed to copy %s to %s: %v", src, dst, err)
				return "", err
			}
		}
	}

	// Copy packer template into build dir
	src := filepath.Join(bpDir, "packer_templates", pTmpl.Name)
	dst := filepath.Join(buildDir, pTmpl.Name)
	if err := goutils.Cp(src, dst); err != nil {
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

	// Get container parameters
	if err = viper.UnmarshalKey(containerKey, &pTmpl.Container); err != nil {
		log.WithError(err).Errorf(
			"failed to unmarshal container parameters "+
				"from %s config file: %v", blueprint.Name, err)
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
			"-var 'registry_server=%s' "+
			"-var 'registry_username=%s' "+
			"-var 'workdir=%s' "+
			"-var 'entrypoint=%s' "+
			"-var 'container_user=%s' "+
			"-var 'registry_cred=%s' %s",
		pTmpl.Base.Name,
		pTmpl.Base.Version,
		pTmpl.Tag.Name,
		pTmpl.Tag.Version,
		blueprint.ProvisioningRepo,
		pTmpl.Container.Registry.Server,
		pTmpl.Container.Registry.Username,
		pTmpl.Container.Workdir,
		pTmpl.Container.Entrypoint,
		pTmpl.Container.User,
		os.Getenv("PAT"),
		buildDir)

	log.Debug(provisionCmd)

	// Run packer from buildDir
	if err := goutils.Cd(buildDir); err != nil {
		log.WithError(err).Errorf(
			"failed to change into the %s directory: %v", buildDir, err)
		return err
	}

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
