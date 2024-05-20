package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	packer "github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/fatih/color"
	fileutils "github.com/l50/goutils/v2/file/fileutils"
	log "github.com/l50/goutils/v2/logging"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	base         []string
	blueprintCmd = &cobra.Command{
		Use:   "blueprint",
		Short: "All blueprint oriented operations for Warp Gate.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				log.L().Error("Failed to show help")
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(blueprintCmd)

	blueprintCmd.PersistentFlags().StringSliceVarP(
		&base, "base", "b", []string{}, "Base image(s) to use for blueprint creation.")
	blueprintCmd.PersistentFlags().StringP(
		"template", "t", "", "Template to use for blueprint "+
			"(all templates can be found in templates/).")
	blueprintCmd.PersistentFlags().BoolP(
		"systemd", "", false, "Blueprint includes a systemd-based image.")
	blueprintCmd.PersistentFlags().StringP(
		"tag", "", "", "Tag information for the created container image.")

	blueprintCmd.AddCommand(createBlueprintCmd, listBlueprintsCmd)
}

var createBlueprintCmd = &cobra.Command{
	Use:   "create [blueprint name]",
	Short: "Create a new blueprint.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		createBlueprint(cmd, args[0])
	},
}

var listBlueprintsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all blueprints.",
	Run: func(cmd *cobra.Command, args []string) {
		listBlueprints()
	},
}

func genBPDirs(newBlueprint string) error {
	if newBlueprint != "" {
		bpDirs := []string{"packer_templates", "scripts"}
		for _, d := range bpDirs {
			dirPath := filepath.Join("blueprints", newBlueprint, d)
			if _, err := fileutils.Create(dirPath, nil, fileutils.CreateDirectory); err != nil {
				log.L().Errorf("Failed to create %s directory for new %s blueprint: %v", d, newBlueprint, err)
				return err
			}
		}
	}
	return nil
}

func validateBaseInput(baseInput []string) error {
	if len(baseInput) == 0 {
		return errors.New("base input is required")
	}
	for _, input := range baseInput {
		parts := strings.Split(input, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid base input format for %s, expected name:version", input)
		}
	}
	return nil
}

func validateTagInput(tagInput string) error {
	if !strings.Contains(tagInput, ":") || len(strings.Split(tagInput, ":")) != 2 {
		return fmt.Errorf("invalid tag input format, expected 'name:version', got: %s", tagInput)
	}
	return nil
}

func processCfgInputs(cmd *cobra.Command) error {
	tagInfo, err := cmd.Flags().GetString("tag")
	if err != nil || validateTagInput(tagInfo) != nil {
		log.L().Errorf("Failed to get tag info from input: %v", err)
		return err
	}

	parts := strings.Split(tagInfo, ":")
	if len(parts) != 2 {
		log.L().Error("Tag info must be in the format 'name:version'")
		return errors.New("invalid tag format")
	}
	tagName, tagVersion := parts[0], parts[1]

	systemd, err := cmd.Flags().GetBool("systemd")
	if err != nil {
		log.L().Error("Failed to get systemd flag from input: %v", err)
		return err
	}

	baseInput, err := cmd.Flags().GetStringSlice("base")
	if err != nil || validateBaseInput(baseInput) != nil {
		log.L().Error("Failed to get base input from CLI or invalid format: %v", err)
		return err
	}

	if systemd {
		// Systemd enabled; prepare both systemd and standard templates
		packerTemplates = append(packerTemplates, generatePackerTemplate(blueprint.Name+"-systemd.pkr.hcl", baseInput[0], tagName+"-systemd", tagVersion, true))
		if len(baseInput) > 1 {
			// Create non-systemd templates for the rest of the base inputs
			for _, base := range baseInput[1:] {
				packerTemplates = append(packerTemplates, generatePackerTemplate(blueprint.Name+".pkr.hcl", base, tagName, tagVersion, false))
			}
		}
	} else {
		// No systemd; create standard templates for all base inputs
		for _, base := range baseInput {
			packerTemplates = append(packerTemplates, generatePackerTemplate(blueprint.Name+".pkr.hcl", base, tagName, tagVersion, false))
		}
	}

	log.L().Debugf("Configured packer templates: %#v", packerTemplates)

	return nil
}

func generatePackerTemplate(templateName, baseInput, tagName, tagVersion string, systemd bool) packer.PackerTemplate {
	parts := strings.Split(baseInput, ":")
	return packer.PackerTemplate{
		AMI: packer.AMI{
			InstanceType: "",
			Region:       "",
			SSHUser:      "",
		},
		Container: packer.Container{
			ImageHashes: map[string]string{},
			ImageRegistry: packer.ContainerImageRegistry{
				Server:     "",
				Username:   "",
				Credential: "",
			},
		},
		ImageValues: packer.ImageValues{
			Name:    parts[0],
			Version: parts[1],
		},
		User:    "root",
		Systemd: systemd,
	}
}

func createCfg(blueprint bp.Blueprint) error {
	// Parse the input config template
	tmpl := template.Must(
		template.ParseFiles(filepath.Join("templates", "config.yaml.tmpl")))

	// Create the templated config file for the new blueprint
	cfgFile := filepath.Join("blueprints", blueprint.Name, "config.yaml")
	f, err := os.Create(cfgFile)
	if err != nil {
		return fmt.Errorf(color.RedString("failed to create config file: %v", err))
	}

	defer f.Close()

	if err := tmpl.Execute(f, blueprint); err != nil {
		return fmt.Errorf(color.RedString("failed to create config.yaml: %v", err))
	}

	return nil
}

func createPacker(blueprint bp.Blueprint) error {
	// Parse the input config template
	tmpl := template.Must(
		template.ParseFiles(filepath.Join("templates", "packer.pkr.hcl.tmpl")))
	newBPPath := filepath.Join("blueprints", blueprint.Name)

	viper.AddConfigPath(newBPPath)
	viper.SetConfigName("config.yaml")
	if err := viper.MergeInConfig(); err != nil {
		log.L().Errorf("Failed to merge %s blueprint config into the existing config: %v", blueprint.Name, err)
		return err
	}

	blueprint.PackerTemplates = packerTemplates

	// Create the templated config file for the new blueprint
	packerDir := filepath.Join(newBPPath, "packer_templates")

	// Get blueprint information
	if err := viper.UnmarshalKey("container", &blueprint.PackerTemplates[0].Container); err != nil {
		log.L().Errorf("Failed to unmarshal container data from config file: %v", err)
		cobra.CheckErr(err)
	}

	set := false
	blueprint.PackerTemplates[0].Container.ImageRegistry.Credential, set = os.LookupEnv("GITHUB_TOKEN")
	if !set {
		return errors.New("required env var $GITHUB_TOKEN is not set, please " +
			"set it with a correct Personal Access Token and try again")
	}

	for _, pkr := range blueprint.PackerTemplates {
		tmplData := struct {
			Blueprint      bp.Blueprint
			PackerTemplate packer.PackerTemplate
			Container      packer.Container
		}{
			Blueprint:      bp.Blueprint{},
			PackerTemplate: pkr,
			Container:      packer.Container{},
		}
		f, err := os.Create(filepath.Join(packerDir, blueprint.Name))
		if err != nil {
			return fmt.Errorf(color.RedString("failed to create %s: %v", filepath.Join(packerDir, blueprint.Name), err))
		}
		defer f.Close()
		if err := tmpl.Execute(f, tmplData); err != nil {
			return fmt.Errorf(color.RedString("failed to populate %s with templated data: %v", filepath.Join(packerDir, blueprint.Name), err))
		}
	}

	if err := sys.Cp(filepath.Join("files", "variables.pkr.hcl"), filepath.Join(newBPPath, "variables.pkr.hcl")); err != nil {
		return fmt.Errorf(color.RedString("failed to transfer packer variables file to new blueprint directory: %v", err))
	}

	log.L().Debugf("Contents of all Packer templates: %#v\n", packerTemplates)

	return nil
}

func createScript(bpName string) error {
	tmpl := template.Must(
		template.ParseFiles(filepath.Join("templates", "provision.sh.tmpl")))

	// Create the templated config file for the new blueprint
	scriptFile := filepath.Join("blueprints", bpName, "scripts", "provision.sh")
	f, err := os.Create(scriptFile)
	if err != nil {
		return fmt.Errorf(color.RedString("failed to create provision script file: %v", err))
	}

	defer f.Close()

	if err := tmpl.Execute(f, bpName); err != nil {
		return fmt.Errorf(color.RedString("failed to create provision.sh: %v", err))
	}

	return nil
}

func createBlueprint(cmd *cobra.Command, blueprintName string) {
	blueprint.Name = blueprintName

	// Create blueprint directories
	if err := genBPDirs(blueprint.Name); err != nil {
		log.L().Error()
		cobra.CheckErr(err)
	}

	// Get packer template contents from input
	if err := processCfgInputs(cmd); err != nil {
		log.L().Error()
		cobra.CheckErr(err)
	}

	blueprint.PackerTemplates = packerTemplates

	if err := createCfg(blueprint); err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}

	if err := createPacker(blueprint); err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}

	if err := createScript(blueprint.Name); err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}

	fmt.Print(color.GreenString("Successfully created %s blueprint!", blueprint.Name))
	os.Exit(0)
}

func listBlueprints() {
	files, err := os.ReadDir("blueprints")
	if err != nil {
		log.L().Errorf(
			"failed to read blueprints: %v", err)
		cobra.CheckErr(err)
	}
	for _, f := range files {
		fmt.Println(f.Name())
	}
	os.Exit(0)
}
