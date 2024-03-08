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

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	packer "github.com/cowdogmoo/warpgate/pkg/packer"
	log "github.com/l50/goutils/v2/logging"
	"github.com/l50/goutils/v2/str"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	githubToken     string
	bpConfig        string
	imageBuilderCmd = &cobra.Command{
		Use:   "imageBuilder",
		Short: "Build a container image using packer and a provisioning repo",
		RunE:  RunImageBuilder,
	}
)

// SetBlueprintConfigPath sets the configuration path for the blueprint.
//
// **Parameters:**
//
// blueprintDir: The directory where the blueprint configuration file is located.
//
// **Returns:**
//
// error: An error if any issue occurs while setting the configuration path.
func SetBlueprintConfigPath(blueprintDir string) error {
	bpConfig = filepath.Join(blueprintDir, "config.yaml")
	viper.SetConfigFile(bpConfig)
	if err := viper.ReadInConfig(); err != nil { // Read the configuration file
		return fmt.Errorf("failed to read config file: %v", err)
	}
	return nil
}

// RunImageBuilder is the main function for the imageBuilder command
// that builds container images using Packer.
//
// **Parameters:**
//
// cmd: A Cobra command object containing flags and arguments for the command.
// args: A slice of strings containing additional arguments passed to the command.
//
// **Returns:**
//
// error: An error if any issue occurs while building the images.
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

	// Set the path for the blueprint configuration file
	if err := SetBlueprintConfigPath(filepath.Join("blueprints", blueprint.Name)); err != nil {
		return err
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

	// Populate packerTemplates from the blueprint configuration
	if err := viper.UnmarshalKey("packer_templates", &packerTemplates); err != nil {
		return fmt.Errorf("failed to unmarshal packer templates: %v", err)
	}

	// Check if packerTemplates is empty
	if len(packerTemplates) == 0 {
		return fmt.Errorf("no packer templates found")
	}

	errChan := make(chan error, len(packerTemplates))
	var wg sync.WaitGroup
	for _, pTmpl := range packerTemplates {
		wg.Add(1)
		go func(pTmpl packer.BlueprintPacker) {
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

func createBuildDir(blueprint bp.Blueprint) (string, error) {
	// Create random name for the build directory
	dirName, err := str.GenRandom(8)
	if err != nil {
		log.L().Errorf("Failed to get random string for buildDir: %v", err)
		return "", err
	}

	buildDir := filepath.Join(os.TempDir(), "builds", dirName)
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		if err := os.Mkdir(buildDir, 0755); err != nil {
			log.L().Errorf("Failed to create build directory %s: %v", buildDir, err)
			return "", err
		}
	}

	// Set bpDir to the correct blueprint directory
	bpDir := filepath.Join("blueprints", blueprint.Name)
	if err := sys.Cp(bpDir, buildDir); err != nil {
		log.L().Errorf("Failed to copy %s to %s: %v", bpDir, buildDir, err)
		return "", err
	}

	if err := SetBlueprintConfigPath(buildDir); err != nil {
		return "", err
	}

	if err = viper.UnmarshalKey(packerTemplatesKey, &packerTemplates); err != nil {
		return "", fmt.Errorf("failed to unmarshal packer templates: %v", err)
	}

	return buildDir, nil
}

func initializeBlueprint(blueprintDir string) error {
	// Path to the directory where plugins would be installed
	pluginsDir := filepath.Join(blueprintDir, "packer_templates")

	// Ensure the packer templates directory exists
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		log.L().Errorf("Expected packer_templates directory not found in %s: %v", blueprintDir, err)
		return err
	}

	// Change to the blueprint's directory
	if err := os.Chdir(pluginsDir); err != nil {
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

	return nil
}

func buildPackerImage(pTmpl packer.BlueprintPacker, blueprint bp.Blueprint) error {
	const maxRetries = 3
	var lastError error
	var buildDir string
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		buildDir, err = createBuildDir(blueprint)
		if err != nil {
			log.L().Errorf(
				"Failed to create build directory: %v", err)
			return err
		}

		if err := initializeBlueprint(buildDir); err != nil {
			log.L().Errorf(
				"Failed to initialize blueprint: %v", err)
			return err
		}

		if err := viper.UnmarshalKey(containerKey, &pTmpl.Container); err != nil {
			log.L().Errorf("Failed to unmarshal container parameters "+
				"from %s config file: %v", blueprint.Name, err)
			return err
		}

		templateDir := filepath.Join(buildDir, "packer_templates")
		log.L().Printf("Packer template path: %s\n", templateDir)

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
			templateDir,
		}

		// Log the arguments with the token hidden
		logArgs := make([]string, len(args))
		copy(logArgs, args)
		for i, arg := range logArgs {
			if strings.Contains(arg, "registry_cred=") {
				logArgs[i] = "-var registry_cred=<HIDDEN>"
			}
		}
		log.L().Debugf("Attempt %d - Packer Parameters: %s", attempt, logArgs)

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
				"Attempt %d: Failed to build container image from %s packer template: %s/%v", attempt, buildDir, pTmpl.Name, err)
			lastError = err
			continue // retry
		}

		log.L().Printf("Successfully built container image from the %s packer template as part of the %s blueprint",
			pTmpl.Name, blueprint.Name)

		return nil // success
	}

	// If the loop completes, it means all attempts failed
	return fmt.Errorf("all attempts failed to build container image from %s packer template: %v", pTmpl.Name, lastError)
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
