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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/fatih/color"
	fileutils "github.com/l50/goutils/v2/file/fileutils"
	log "github.com/l50/goutils/v2/logging"
	"github.com/l50/goutils/v2/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Blueprint represents the contents of a Blueprint.
type Blueprint struct {
	// Name of the Blueprint
	Name string `yaml:"name"`
	// Path to the provisioning repo
	ProvisioningRepo string
}

// Data holds a blueprint and its associated packer templates.
type Data struct {
	Blueprint       Blueprint
	PackerTemplates []PackerTemplate
	Container       Container
}

var (
	base         []string
	blueprintCmd = &cobra.Command{
		Use:   "blueprint",
		Short: "All blueprint oriented operations for Warp Gate.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				log.L().Error("failed to show help")
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
			if err := fileutils.Create(dirPath, nil, fileutils.CreateDirectory); err != nil {
				log.L().Errorf("failed to create %s directory for new %s blueprint: %v", d, newBlueprint, err)
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
	parts := strings.Split(tagInput, ":")
	if len(parts) != 2 {
		return errors.New("invalid tag input format, expected name:version")
	}
	return nil
}

func processCfgInputs(cmd *cobra.Command) error {
	tagInfo, err := cmd.Flags().GetString("tag")
	if err != nil || validateTagInput(tagInfo) != nil {
		log.L().Error("failed to get tag info from input or invalid format")
		cobra.CheckErr(err)
	}

	var tagName string
	var tagVersion string

	parts := strings.Split(tagInfo, ":")
	tagName = parts[0]
	tagVersion = parts[1]

	systemd, err := cmd.Flags().GetBool("systemd")
	if err != nil {
		log.L().Error()
		cobra.CheckErr(err)
	}

	baseInput, err := cmd.Flags().GetStringSlice("base")
	if err != nil || validateBaseInput(baseInput) != nil {
		log.L().Error("failed to get base input from CLI or invalid format")
		cobra.CheckErr(err)
	}

	var systemdContainer []bool
	var tagNames []string
	var tmplNames []string

	if systemd {
		systemdContainer = []bool{false, true}
		tagNames = []string{tagName, tagName + "-systemd"}
		tmplNames = []string{blueprint.Name + ".pkr.hcl",
			blueprint.Name + "-systemd.pkr.hcl"}
	} else {
		tagNames = []string{tagName}
		tmplNames = []string{blueprint.Name + ".pkr.hcl"}
	}

	if len(baseInput) < len(tagNames) {
		errMsg := fmt.Sprintf("not enough base inputs for the specified tag names: %v", tagNames)
		log.L().Error(errMsg)
		cobra.CheckErr(errors.New(errMsg))
	}

	for i, n := range tagNames {
		parts = strings.Split(baseInput[i], ":")
		packerTemplates = append(packerTemplates, PackerTemplate{
			Name: tmplNames[i],
			Base: Base{
				Name:    parts[0],
				Version: parts[1],
			},
			Systemd: systemdContainer[i],
			Tag: Tag{
				Name:    n,
				Version: tagVersion,
			},
		})
	}

	log.L().Debugf("Templated packer config data: %#v\n", packerTemplates)

	return nil
}

func createCfg(cmd *cobra.Command, data Data) error {
	// Parse the input config template
	tmpl := template.Must(
		template.ParseFiles(filepath.Join("templates", "config.yaml.tmpl")))

	// Create the templated config file for the new blueprint
	cfgFile := filepath.Join("blueprints", data.Blueprint.Name, "config.yaml")
	f, err := os.Create(cfgFile)
	if err != nil {
		return fmt.Errorf(color.RedString("failed to create config file: %v", err))
	}

	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf(color.RedString("failed to create config.yaml: %v", err))
	}

	return nil
}

func createPacker(cmd *cobra.Command, data Data) error {
	// Parse the input config template
	tmpl := template.Must(
		template.ParseFiles(filepath.Join("templates", "packer.pkr.hcl.tmpl")))
	newBPPath := filepath.Join("blueprints", data.Blueprint.Name)

	viper.AddConfigPath(newBPPath)
	viper.SetConfigName("config.yaml")
	if err := viper.MergeInConfig(); err != nil {
		log.L().Errorf(
			"failed to merge %s blueprint config into the existing config: %v", data.Blueprint.Name, err)
		return err
	}

	// Create the templated config file for the new blueprint
	packerDir := filepath.Join(newBPPath, "packer_templates")

	// Get blueprint information
	if err := viper.UnmarshalKey("container", &data.Container); err != nil {
		log.L().Errorf(
			"failed to unmarshal container data from config file: %v", err)
		cobra.CheckErr(err)
	}

	set := false
	data.Container.Registry.Credential, set = os.LookupEnv("GITHUB_TOKEN")
	if !set {
		return errors.New("required env var $GITHUB_TOKEN is not set, please " +
			"set it with a correct Personal Access Token and try again")
	}

	for _, pkr := range data.PackerTemplates {
		tmplData := struct {
			Blueprint      Blueprint
			PackerTemplate PackerTemplate
			Container      Container
		}{
			Blueprint:      data.Blueprint,
			PackerTemplate: pkr,
			Container:      data.Container,
		}
		f, err := os.Create(filepath.Join(packerDir, pkr.Name))
		if err != nil {
			return fmt.Errorf(color.RedString("failed to create %s: %v", filepath.Join(packerDir, pkr.Name), err))
		}
		defer f.Close()
		if err := tmpl.Execute(f, tmplData); err != nil {
			return fmt.Errorf(color.RedString("failed to populate %s with templated data: %v", filepath.Join(packerDir, pkr.Name), err))
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

	// Create data to hold the blueprint and
	// associated packer templates.
	data := Data{
		Blueprint:       blueprint,
		PackerTemplates: packerTemplates,
	}

	if err := createCfg(cmd, data); err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}

	if err := createPacker(cmd, data); err != nil {
		log.L().Error(err)
		cobra.CheckErr(err)
	}

	if err := createScript(data.Blueprint.Name); err != nil {
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
