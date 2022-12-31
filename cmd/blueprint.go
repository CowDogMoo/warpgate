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
	blueprintCmd.Flags().BoolP(
		"create", "c", false, "Create a blueprint.")
	blueprintCmd.Flags().BoolP(
		"list", "l", false, "List all blueprints.")
}

func createBlueprint(cmd *cobra.Command) {
	newBlueprint, err := cmd.Flags().GetString("create")
	if err != nil {
		log.WithError(err).Errorf(
			"failed to retrieve blueprint to create from CLI input: %v", err)
		os.Exit(1)
	}
	fmt.Println(newBlueprint)
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
