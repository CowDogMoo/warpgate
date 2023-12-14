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
	fileutils "github.com/l50/goutils/v2/file/fileutils"

	log "github.com/cowdogmoo/warpgate/pkg/logging"
	git "github.com/l50/goutils/v2/git"
	"github.com/l50/goutils/v2/str"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// PackerTemplate is used to hold the information used
// with a blueprint's packer templates.
type PackerTemplate struct {
	Name      string    `yaml:"name"`
	Container Container `yaml:"container"`
	Base      Base      `yaml:"base"`
	Tag       Tag       `yaml:"tag"`
	Systemd   bool      `yaml:"systemd"`
}

// Base represents the base image used
// by Packer for image building.
type Base struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// Tag represents the new tag for the image built by Packer.
type Tag struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// Container stores information that is used for
// the container build process.
type Container struct {
	Workdir    string   `yaml:"container.workdir"`
	User       string   `yaml:"container.user"`
	Entrypoint string   `yaml:"container.entrypoint"`
	Registry   Registry `yaml:"container.registry"`
}

// Registry stores container registry information that is
// used after image building to commit the image.
type Registry struct {
	Server     string `yaml:"registry.server"`
	Username   string `yaml:"registry.username"`
	Credential string `yaml:"registry.credential"`
}

var (
	bpConfig        string
	imageBuilderCmd = &cobra.Command{
		Use:   "imageBuilder",
		Short: "Build a container image using packer and a provisioning repo",
		RunE:  RunImageBuilder,
	}
)

// SetBlueprintConfigPath sets the configuration path for the blueprint
func SetBlueprintConfigPath(blueprintDir string) error {
	bpConfig = filepath.Join(blueprintDir, "config.yaml")
	viper.AddConfigPath(filepath.Dir(bpConfig))
	viper.SetConfigName(filepath.Base(bpConfig))
	return nil
}

// RunImageBuilder is the main function for the imageBuilder command
// that is used to build container images using packer.
func RunImageBuilder(cmd *cobra.Command, args []string) error {
	provisioningRepo, err := cmd.Flags().GetString("provisionPath")
	if err != nil {
		return fmt.Errorf("failed to get provisionPath: %v", err)
	}

	blueprintName, err := cmd.Flags().GetString(blueprintKey)
	if err != nil {
		return fmt.Errorf("failed to retrieve blueprint: %v", err)
	}

	// Set the configuration path for the blueprint
	if err := SetBlueprintConfigPath(filepath.Join("blueprints", blueprintName)); err != nil {
		return err
	}

	if err := viper.MergeInConfig(); err != nil {
		return fmt.Errorf("failed to merge blueprint config: %v", err)
	}

	var packerTemplates []PackerTemplate
	if err = viper.UnmarshalKey(packerTemplatesKey, &packerTemplates); err != nil {
		return fmt.Errorf("failed to unmarshal packer templates: %v", err)
	}

	errChan := make(chan error)
	var wg sync.WaitGroup
	for _, pTmpl := range packerTemplates {
		wg.Add(1)
		go func(pTmpl PackerTemplate) {
			defer wg.Done()
			log.L().Printf("Now building %s template as part of %s blueprint, please wait.\n",
				pTmpl.Name, blueprintName)

			blueprint.Name = blueprintName
			blueprint.ProvisioningRepo = provisioningRepo
			if err := buildPackerImage(pTmpl, blueprint); err != nil {
				errChan <- fmt.Errorf("error building %s: %v", pTmpl.Name, err)
			}
		}(pTmpl)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	fmt.Print(color.GreenString(
		"All packer templates in %s blueprint built successfully!\n", blueprintName))
	return nil
}

func init() {
	rootCmd.AddCommand(imageBuilderCmd)

	imageBuilderCmd.Flags().StringP(
		"provisionPath", "p", "", "Local path to the repo with provisioning logic that will be used by packer")
	if err := imageBuilderCmd.MarkFlagRequired("provisionPath"); err != nil {
		log.L()
		cobra.CheckErr(err)
	}

	imageBuilderCmd.Flags().StringP(
		"blueprint", "b", "", "The blueprint to use for image building.")
	if err := imageBuilderCmd.MarkFlagRequired("blueprint"); err != nil {
		log.L()
		cobra.CheckErr(err)
	}
}

func createBuildDir(pTmpl PackerTemplate, blueprint Blueprint) (string, error) {
	// Create random name for the build directory
	dirName, err := str.GenRandom(8)
	if err != nil {
		log.L().Errorf(
			"failed to get random string for buildDir: %v", err)
		return "", err
	}

	// Create buildDir
	buildDir := filepath.Join("/tmp", "builds", dirName)
	if err := fileutils.Create(buildDir, nil, fileutils.CreateDirectory); err != nil {
		log.L().Errorf(
			"failed to create %s build directory for image building: %v", dirName, err)
		return "", err
	}

	bpDir := filepath.Dir(bpConfig)
	files, err := os.ReadDir(bpDir)
	if err != nil {
		log.L().Errorf(
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
		if !str.InSlice(file.Name(), excludedFiles) {
			src := filepath.Join(bpDir, file.Name())
			dst := filepath.Join(buildDir, file.Name())
			if err := sys.Cp(src, dst); err != nil {
				log.L().Errorf(
					"failed to copy %s to %s: %v", src, dst, err)
				return "", err
			}
		}
	}

	// Copy packer template into build dir
	src := filepath.Join(bpDir, "packer_templates", pTmpl.Name)
	dst := filepath.Join(buildDir, pTmpl.Name)
	if err := sys.Cp(src, dst); err != nil {
		log.L().Errorf(
			"failed to transfer packer template to %s: %v", buildDir, err)
		return "", err
	}

	return buildDir, nil
}

func initializeBlueprint(blueprintDir string) error {
	repoRoot, err := git.RepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get the root of the git repo: %v", err)
	}

	// Path to the directory where plugins would be installed
	pluginsDir := filepath.Join(repoRoot, blueprintDir, "packer_templates", "github.com")

	// Check if the plugins directory exists
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		// Change to the blueprint's directory
		if err := os.Chdir(filepath.Join(blueprintDir, "packer_templates")); err != nil {
			return fmt.Errorf("failed to change directory to %s: %v", blueprintDir, err)
		}

		// Run packer init .
		initCmd := "packer init ."
		if _, err := script.Exec(initCmd).Stdout(); err != nil {
			return fmt.Errorf("failed to initialize blueprint with packer init: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check the status of the plugins directory: %v", err)
	}

	return nil
}

func buildPackerImage(pTmpl PackerTemplate, blueprint Blueprint) error {
	buildDir, err := createBuildDir(pTmpl, blueprint)
	if err != nil {
		log.L().Errorf(
			"failed to create build directory: %v", err)
		return err
	}

	// Initialize the blueprint before building the image
	if err := initializeBlueprint(filepath.Dir(bpConfig)); err != nil {
		log.L().Errorf(
			"failed to initialize blueprint: %v", err)
		return err
	}

	// Get container parameters
	if err = viper.UnmarshalKey(containerKey, &pTmpl.Container); err != nil {
		log.L().Errorf(
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
		os.Getenv("GITHUB_TOKEN"),
		buildDir)

	// Run packer from buildDir
	if err := sys.Cd(buildDir); err != nil {
		log.L().Errorf(
			"failed to change into the %s directory: %v", buildDir, err)
		return err
	}

	if _, err := script.Exec(provisionCmd).Stdout(); err != nil {
		if err != nil {
			log.L().Errorf(
				"failed to build container image from the %s "+
					"packer template that's part of the %s blueprint: %v",
				pTmpl.Name, blueprint.Name, err)
			return err
		}
	}

	return nil
}
