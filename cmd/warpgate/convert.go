/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/convert"
	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
)

// Convert command options
type convertOptions struct {
	author     string
	license    string
	version    string
	baseImage  string
	includeAMI bool
	output     string
	dryRun     bool
}

var convertOpts = &convertOptions{}

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert templates to Warpgate format",
	Long: `Convert templates from other formats (e.g., Packer) to Warpgate YAML format.

This command helps migrate existing infrastructure templates to Warpgate's
unified configuration format.`,
}

var convertPackerCmd = &cobra.Command{
	Use:   "packer [template-dir]",
	Short: "Convert Packer template to Warpgate format",
	Long: `Convert a Packer HCL template to Warpgate YAML format.

The converter will parse Packer template files (variables.pkr.hcl, docker.pkr.hcl,
ami.pkr.hcl) and generate a unified warpgate.yaml configuration file.

Examples:
  # Convert Packer template and write to default location
  warpgate convert packer ./my-template

  # Convert with custom output path
  warpgate convert packer ./my-template --output ./converted.yaml

  # Preview conversion without writing file
  warpgate convert packer ./my-template --dry-run

  # Convert with custom metadata
  warpgate convert packer ./my-template --author "John Doe" --version "v1.0.0"

  # Convert container-only template (exclude AMI)
  warpgate convert packer ./my-template --include-ami=false`,
	Args: cobra.ExactArgs(1),
	RunE: runConvertPacker,
}

func init() {
	// Convert packer subcommand flags
	convertPackerCmd.Flags().StringVar(&convertOpts.author, "author", "", "Template author")
	convertPackerCmd.Flags().StringVar(&convertOpts.license, "license", "", "Template license (default from config)")
	convertPackerCmd.Flags().StringVar(&convertOpts.version, "version", "", "Template version (default from config)")
	convertPackerCmd.Flags().StringVar(&convertOpts.baseImage, "base-image", "", "Override base image (default: extracted from template)")
	convertPackerCmd.Flags().BoolVar(&convertOpts.includeAMI, "include-ami", true, "Include AMI target configuration (default true)")
	convertPackerCmd.Flags().StringVarP(&convertOpts.output, "output", "o", "", "Output file path (default: <template-dir>/warpgate.yaml)")
	convertPackerCmd.Flags().BoolVar(&convertOpts.dryRun, "dry-run", false, "Print converted YAML without writing file")

	// Add subcommands to convert
	convertCmd.AddCommand(convertPackerCmd)
}

func runConvertPacker(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	absTemplateDir, err := resolveTemplatePath(args[0])
	if err != nil {
		return err
	}

	logging.InfoContext(ctx, "Converting Packer template to Warpgate format: %s", absTemplateDir)

	// If author not provided, try to get from git config
	author := convertOpts.author
	if author == "" {
		author = getGitAuthor(ctx)
		if author != "" {
			logging.InfoContext(ctx, "Using git config for author: %s", author)
		}
	}

	// Create converter options
	opts := convert.PackerConverterOptions{
		TemplateDir: absTemplateDir,
		Author:      author,
		License:     convertOpts.license,
		Version:     convertOpts.version,
		BaseImage:   convertOpts.baseImage,
		IncludeAMI:  convertOpts.includeAMI,
	}

	// Create converter
	converter, err := convert.NewPackerConverter(opts)
	if err != nil {
		return fmt.Errorf("failed to create converter: %w", err)
	}

	// Perform conversion
	buildConfig, err := converter.Convert()
	if err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	// Count provisioners and post-processors
	provisionerCount := len(buildConfig.Provisioners)
	postProcessorCount := len(buildConfig.PostProcessors)
	targetCount := len(buildConfig.Targets)

	logging.InfoContext(ctx, "Conversion complete: %d provisioners, %d post-processors, %d targets",
		provisionerCount, postProcessorCount, targetCount)

	// Marshal to YAML
	yamlData, err := yaml.Marshal(buildConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Prepend schema comment for IDE autocomplete/validation
	yamlWithSchema := append([]byte(builder.SchemaComment), yamlData...)

	// Determine output path
	outputPath, err := determineOutputPath(absTemplateDir)
	if err != nil {
		return err
	}

	// Write output or print dry-run
	if err := writeConvertedTemplate(ctx, yamlWithSchema, outputPath); err != nil {
		return err
	}

	// Display summary if not dry-run
	if !convertOpts.dryRun {
		displayConversionSummary(buildConfig, outputPath)
	}

	return nil
}

// resolveTemplatePath resolves the template directory path to an absolute path
func resolveTemplatePath(templateDir string) (string, error) {
	// Expand home directory if present
	if strings.HasPrefix(templateDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		templateDir = filepath.Join(home, templateDir[2:])
	}

	// Convert to absolute path
	absTemplateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve template directory: %w", err)
	}

	// Verify template directory exists
	if _, err := os.Stat(absTemplateDir); os.IsNotExist(err) {
		return "", fmt.Errorf("template directory does not exist: %s", absTemplateDir)
	}

	return absTemplateDir, nil
}

// determineOutputPath determines the output path for the converted template
func determineOutputPath(absTemplateDir string) (string, error) {
	outputPath := convertOpts.output
	if outputPath == "" {
		return filepath.Join(absTemplateDir, "warpgate.yaml"), nil
	}

	// Handle relative paths
	if !filepath.IsAbs(outputPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		outputPath = filepath.Join(cwd, outputPath)
	}

	// If output path is a directory, append filename
	if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
		return "", fmt.Errorf("output path is a directory, please specify a file: %s", outputPath)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	return outputPath, nil
}

// writeConvertedTemplate writes the converted template to file or prints it for dry-run
func writeConvertedTemplate(ctx context.Context, yamlWithSchema []byte, outputPath string) error {
	// Dry run: print to stdout
	if convertOpts.dryRun {
		fmt.Println("# Dry run - converted YAML:")
		fmt.Println("---")
		fmt.Print(string(yamlWithSchema))
		return nil
	}

	// Write to file
	if err := os.WriteFile(outputPath, yamlWithSchema, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	logging.InfoContext(ctx, "Successfully converted template to: %s", outputPath)
	return nil
}

// displayConversionSummary displays a summary of the conversion results
func displayConversionSummary(buildConfig *builder.Config, outputPath string) {
	fmt.Println("\n✓ Conversion successful!")
	fmt.Printf("  Template:     %s\n", buildConfig.Name)
	fmt.Printf("  Description:  %s\n", buildConfig.Metadata.Description)
	fmt.Printf("  Provisioners: %d\n", len(buildConfig.Provisioners))
	fmt.Printf("  Targets:      %d\n", len(buildConfig.Targets))

	// Show target types
	if len(buildConfig.Targets) > 0 {
		targetTypes := make([]string, 0, len(buildConfig.Targets))
		for _, target := range buildConfig.Targets {
			targetTypes = append(targetTypes, target.Type)
		}
		fmt.Printf("  Target types: %s\n", strings.Join(targetTypes, ", "))
	}

	fmt.Printf("\n  Output file:  %s\n", outputPath)
}

// getGitAuthor retrieves author information from git config
func getGitAuthor(ctx context.Context) string {
	home, err := os.UserHomeDir()
	if err != nil {
		logging.DebugContext(ctx, "Failed to get home directory: %v", err)
		return ""
	}

	// Load main git config
	cfg := loadGitConfig(ctx, home)
	if cfg == nil {
		return ""
	}

	// Extract name and email
	name, email := extractUserInfo(cfg)

	// Try included configs if needed
	if name == "" || email == "" {
		name, email = tryIncludedConfig(ctx, cfg, home, name, email)
	}

	return formatGitAuthor(name, email)
}

// loadGitConfig loads the main .gitconfig file
func loadGitConfig(ctx context.Context, home string) *ini.File {
	gitconfigPath := filepath.Join(home, ".gitconfig")
	cfg, err := ini.Load(gitconfigPath)
	if err != nil {
		logging.DebugContext(ctx, "Failed to load .gitconfig: %v", err)
		return nil
	}
	return cfg
}

// extractUserInfo extracts name and email from a git config section
func extractUserInfo(cfg *ini.File) (name, email string) {
	userSection := cfg.Section("user")
	if userSection != nil {
		name = userSection.Key("name").String()
		email = userSection.Key("email").String()
	}
	return name, email
}

// tryIncludedConfig tries to load user info from included config files
func tryIncludedConfig(ctx context.Context, cfg *ini.File, home, currentName, currentEmail string) (name, email string) {
	name, email = currentName, currentEmail

	includeSection := cfg.Section("include")
	if includeSection == nil {
		return name, email
	}

	includePath := includeSection.Key("path").String()
	if includePath == "" {
		return name, email
	}

	// Expand ~ to home directory
	if strings.HasPrefix(includePath, "~/") {
		includePath = filepath.Join(home, includePath[2:])
	}

	// Try to load the included file
	includedCfg, err := ini.Load(includePath)
	if err != nil {
		return name, email
	}

	includedUserSection := includedCfg.Section("user")
	if includedUserSection == nil {
		return name, email
	}

	if name == "" {
		name = includedUserSection.Key("name").String()
	}
	if email == "" {
		email = includedUserSection.Key("email").String()
	}

	return name, email
}

// formatGitAuthor formats name and email as a git author string
func formatGitAuthor(name, email string) string {
	switch {
	case name != "" && email != "":
		return fmt.Sprintf("%s <%s>", name, email)
	case name != "":
		return name
	case email != "":
		return email
	default:
		return ""
	}
}
