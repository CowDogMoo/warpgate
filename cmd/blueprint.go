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
	"text/template"

	"github.com/fatih/color"
	goutils "github.com/l50/goutils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Blueprint represents the contents of a Blueprint.
type Blueprint struct {
	// Name of the Blueprint
	Name string `yaml:"name"`
	// Path to the provisioning repo
	ProvisioningRepo string
	Key              string
}

// Data holds a blueprint and its associated packer templates.
type Data struct {
	Blueprint       Blueprint
	PackerTemplates []PackerTemplate
}

var blueprintCmd = &cobra.Command{
	Use:   "blueprint",
	Short: "All blueprint oriented operations for Warp Gate.",
	Run: func(cmd *cobra.Command, args []string) {
		listBlueprints(cmd)
		createBlueprint(cmd)

	},
}

func init() {
	rootCmd.AddCommand(blueprintCmd)
	blueprintCmd.Flags().StringP(
		"create", "c", "", "Create a blueprint.")
	blueprintCmd.Flags().BoolP(
		"list", "l", false, "List all blueprints.")
	blueprintCmd.Flags().IntP(
		"numImages", "n", 2, "Number of images in the target blueprint.")
	blueprintCmd.Flags().BoolP(
		"systemd", "", true, "Blueprint contains a systemd image")
	blueprintCmd.Flags().StringP(
		"tag-name", "", "", "Name of the created container image.")
	blueprintCmd.Flags().StringP(
		"tag-version", "", "", "Version of the created container image.")
}

func genBPDirs(newBlueprint string) error {
	if newBlueprint != "" {
		bpDirs := []string{"packer_templates", "scripts"}
		for _, d := range bpDirs {
			if err := goutils.CreateDirectory(filepath.Join("blueprints", newBlueprint, d)); err != nil {
				log.WithError(err).Errorf(
					"failed to create %s directory for new %s blueprint: %v", d, newBlueprint, err)
				return err
			}
		}
	}

	return nil

}

func genPkrTmpl(cmd *cobra.Command) error {
	tagName, err := cmd.Flags().GetString("tag-name")
	if err != nil {
		log.WithError(err).Error()
		os.Exit(1)
	}

	tagVersion, err := cmd.Flags().GetString("tag-version")
	if err != nil {
		log.WithError(err).Error()
		os.Exit(1)
	}

	systemd, err := cmd.Flags().GetBool("systemd")
	if err != nil {
		log.WithError(err).Error()
		os.Exit(1)
	}

	var tagNames []string
	var tmplNames []string

	if systemd {
		tagNames = []string{tagName, tagName + "-systemd"}
		tmplNames = []string{blueprint.Name + ".pkr.hcl",
			blueprint.Name + "-systemd.pkr.hcl"}
	} else {
		tagNames = []string{tagName}
		tmplNames = []string{blueprint.Name + ".pkr.hcl"}
	}

	for i, n := range tagNames {
		packerTemplates = append(packerTemplates, PackerTemplate{
			Name: tmplNames[i],
			Tag: struct {
				Name    string `yaml:"name"`
				Version string `yaml:"version"`
			}{
				Name:    n,
				Version: tagVersion,
			},
		})
	}

	log.Debugf("Packer templates: %#v\n", packerTemplates)

	return nil
}

func createCfg(data Data) error {
	// TODO: this should be an input param
	tmplName := "ubuntu-config"

	// TODO: The base image and version should be input params
	// i.e. ./wg blueprint -c fucks --base-image ubuntu --base-image-version latest --tag-name cowdogmoo/fucks --tag-version latest

	// Parse the input config template
	tmpl := template.Must(
		template.ParseFiles(filepath.Join("templates", tmplName+".yaml.tmpl")))

	// Create the templated config file for the new blueprint
	cfgFile := filepath.Join("blueprints", data.Blueprint.Name, "config.yaml")
	f, err := os.Create(cfgFile)
	if err != nil {
		return fmt.Errorf(color.RedString("failed to create config.yaml: %v", err))
	}

	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf(color.RedString("failed to populate config.yaml: %v", err))
	}

	return nil
}

func createBlueprint(cmd *cobra.Command) {
	blueprint.Name, err = cmd.Flags().GetString("create")
	if err != nil {
		log.WithError(err).Errorf(
			"failed to retrieve blueprint to create from CLI input: %v", err)
		os.Exit(1)
	}

	// Create blueprint directories
	if err := genBPDirs(blueprint.Name); err != nil {
		log.WithError(err).Error()
		os.Exit(1)
	}

	// Get packer template contents from input
	if err := genPkrTmpl(cmd); err != nil {
		log.WithError(err).Error()
		os.Exit(1)
	}

	data := Data{
		Blueprint:       blueprint,
		PackerTemplates: packerTemplates,
	}

	if err := createCfg(data); err != nil {
		log.WithError(err).Error(err)
		os.Exit(1)
	}

	fmt.Print(color.GreenString("Successfully created %s blueprint!", blueprint.Name))
	os.Exit(0)
}

func listBlueprints(cmd *cobra.Command) {
	ls, err := cmd.Flags().GetBool("list")
	if err != nil {
		log.WithError(err).Errorf(
			"failed to get ls from CLI input: %v", err)
		os.Exit(1)
	}
	if ls {
		files, err := os.ReadDir("blueprints")
		if err != nil {
			log.WithError(err).Errorf(
				"failed to read blueprints: %v", err)
			os.Exit(1)
		}
		for _, f := range files {
			fmt.Println(f.Name())
		}
		os.Exit(0)
	}
}
