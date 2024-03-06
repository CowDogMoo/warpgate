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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fatih/color"
	fileutils "github.com/l50/goutils/v2/file/fileutils"

	git "github.com/l50/goutils/v2/git"
	log "github.com/l50/goutils/v2/logging"
	"github.com/l50/goutils/v2/str"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var githubToken string

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
	var err error
	blueprint.ProvisioningRepo, err = cmd.Flags().GetString("provisionPath")
	if err != nil {
		return fmt.Errorf("failed to get provisionPath: %v", err)
	}

	// If the provisioning repo path contains a tilde, expand it
	if strings.Contains(blueprint.ProvisioningRepo, "~") {
		blueprint.ProvisioningRepo = sys.ExpandHomeDir(blueprint.ProvisioningRepo)
	}

	blueprint.Name, err = cmd.Flags().GetString(blueprintKey)
	if err != nil {
		return fmt.Errorf("failed to retrieve blueprint: %v", err)
	}

	if err := SetBlueprintConfigPath(filepath.Join("blueprints", blueprint.Name)); err != nil {
		return err
	}

	if err := viper.MergeInConfig(); err != nil {
		return fmt.Errorf("failed to merge blueprint config: %v", err)
	}

	var packerTemplates []PackerTemplate
	if err = viper.UnmarshalKey(packerTemplatesKey, &packerTemplates); err != nil {
		return fmt.Errorf("failed to unmarshal packer templates: %v", err)
	}

	// Get the GitHub token from the command-line flag or environment variable
	if githubToken == "" {
		githubToken = os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			return fmt.Errorf("no GitHub token provided")
		}
	}

	// Validate the GitHub token
	if err := validateToken(githubToken); err != nil {
		return fmt.Errorf("GitHub token validation failed: %v", err)
	}

	errChan := make(chan error, len(packerTemplates))
	var wg sync.WaitGroup
	for _, pTmpl := range packerTemplates {
		wg.Add(1)
		go func(pTmpl PackerTemplate) {
			defer wg.Done()
			log.L().Printf("Now building %s template as part of %s blueprint, please wait.\n",
				pTmpl.Name, blueprint.Name)

			if err := buildPackerImage(pTmpl, blueprint); err != nil {
				errChan <- fmt.Errorf("error building %s: %v", pTmpl.Name, err)
			}
		}(pTmpl)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

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

	fmt.Print(color.GreenString(
		"All packer templates in %s blueprint built successfully!\n", blueprint.Name))

	return nil
}

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

	imageBuilderCmd.Flags().StringVarP(
		&githubToken, "github-token", "t", "", "GitHub token to authenticate with (optional, will use GITHUB_TOKEN env var if not provided)")
}

func createBuildDir(pTmpl PackerTemplate, blueprint Blueprint) (string, error) {
	// Create random name for the build directory
	dirName, err := str.GenRandom(8)
	if err != nil {
		log.L().Errorf("Failed to get random string for buildDir: %v", err)
		return "", err
	}

	buildBaseDir := filepath.Join(os.TempDir(), "builds")
	if _, err := os.Stat(buildBaseDir); os.IsNotExist(err) {
		if err := os.Mkdir(buildBaseDir, 0755); err != nil {
			log.L().Errorf("Failed to create base build directory: %v", err)
			return "", err
		}
	}

	// Create buildDir
	buildDir := filepath.Join(buildBaseDir, dirName)
	if err := fileutils.Create(buildDir, nil, fileutils.CreateDirectory); err != nil {
		log.L().Errorf(
			"Failed to create %s build directory for image building: %v", dirName, err)
		return "", err
	}

	bpDir := filepath.Dir(bpConfig)
	files, err := os.ReadDir(bpDir)
	if err != nil {
		log.L().Errorf("Failed to list files in %s: %v", bpDir, err)
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
				log.L().Errorf("Failed to copy %s to %s: %v", src, dst, err)
				return "", err
			}
		}
	}

	// Copy packer template into build dir
	src := filepath.Join(bpDir, "packer_templates", pTmpl.Name)
	dst := filepath.Join(buildDir, pTmpl.Name)
	if err := sys.Cp(src, dst); err != nil {
		log.L().Errorf(
			"Failed to transfer packer template to %s: %v", buildDir, err)
		return "", err
	}

	return buildDir, nil
}

func initializeBlueprint(blueprintDir string) error {
	repoRoot, err := git.RepoRoot()
	if err != nil {
		log.L().Errorf(
			"Failed to get the root of the git repo: %v", err)
		return err
	}

	// Path to the directory where plugins would be installed
	pluginsDir := filepath.Join(repoRoot, blueprintDir, "packer_templates", "github.com")

	// Validate the directory structure
	if _, err := os.Stat(filepath.Join(blueprintDir, "packer_templates")); os.IsNotExist(err) {
		log.L().Errorf("Expected packer_templates directory not found in %s: %v", blueprintDir, err)
		return err
	}

	// Check if the plugins directory exists
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		// Change to the blueprint's directory
		if err := os.Chdir(filepath.Join(blueprintDir, "packer_templates")); err != nil {
			log.L().Errorf(
				"Failed to change directory to %s: %v", blueprintDir, err)
			return err
		}

		// Run packer init .
		cmd := sys.Cmd{
			CmdString:     "packer",
			Args:          []string{"init", "."},
			OutputHandler: func(s string) { log.L().Println(s) },
		}

		if _, err := cmd.RunCmd(); err != nil {
			log.L().Errorf(
				"Failed to initialize blueprint with packer init: %v", err)
			return err
		}
	} else if err != nil {
		log.L().Errorf(
			"Failed to check the status of the plugins directory: %v", err)
		return err
	}

	return nil
}

func buildPackerImage(pTmpl PackerTemplate, blueprint Blueprint) error {
	buildDir, err := createBuildDir(pTmpl, blueprint)
	if err != nil {
		log.L().Errorf(
			"Failed to create build directory: %v", err)
		return err
	}

	// Initialize the blueprint before building the image
	if err := initializeBlueprint(filepath.Dir(bpConfig)); err != nil {
		log.L().Errorf(
			"Failed to initialize blueprint: %v", err)
		return err
	}

	// Get container parameters
	if err := viper.UnmarshalKey(containerKey, &pTmpl.Container); err != nil {
		log.L().Errorf("Failed to unmarshal container parameters "+
			"from %s config file: %v", blueprint.Name, err)
		return err
	}

	args := []string{
		"build",
		"-var", fmt.Sprintf("base_image=%s", pTmpl.Base.Name),
		"-var", fmt.Sprintf("base_image_version=%s", pTmpl.Base.Version),
		"-var", fmt.Sprintf("container_user=%s", pTmpl.Container.User),
		"-var", fmt.Sprintf("entrypoint=%s", pTmpl.Container.Entrypoint),
		"-var", fmt.Sprintf("new_image_tag=%s", pTmpl.Tag.Name),
		"-var", fmt.Sprintf("new_image_version=%s", pTmpl.Tag.Version),
		"-var", fmt.Sprintf("provision_repo_path=%s", blueprint.ProvisioningRepo),
		"-var", fmt.Sprintf("registry_server=%s", pTmpl.Container.Registry.Server),
		"-var", fmt.Sprintf("registry_username=%s", pTmpl.Container.Registry.Username),
		"-var", fmt.Sprintf("registry_cred=%s", githubToken),
		"-var", fmt.Sprintf("workdir=%s", pTmpl.Container.Workdir),
		buildDir,
	}

	log.L().Println("ARGS: ", args)

	// Change to the build directory
	if err := os.Chdir(buildDir); err != nil {
		log.L().Errorf(
			"Failed to change into the %s directory: %v", buildDir, err)
		return err
	}

	cmd := sys.Cmd{
		CmdString:     "packer",
		Args:          args,
		OutputHandler: func(s string) { log.L().Println(s) },
	}

	if _, err := cmd.RunCmd(); err != nil {
		log.L().Errorf(
			"Failed to build container image from %s packer template: %v", pTmpl.Name, err)
		return err
	}

	log.L().Printf("Successfully built container image from the %s packer template as part of the %s blueprint",
		pTmpl.Name, blueprint.Name)

	return nil
}

func validateToken(token string) error {
	// Define the GitHub API URL for user authentication
	url := "https://api.github.com/user"

	// Create a new request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set the Authorization header with the token
	req.Header.Set("Authorization", "token "+token)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	// Check if the status code is not 200
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token validation failed with status code: %d", resp.StatusCode)
	}

	return nil
}
