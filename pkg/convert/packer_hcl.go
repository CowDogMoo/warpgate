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

package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// PackerVariable represents a Packer variable definition
type PackerVariable struct {
	Name    string
	Type    string
	Default string
}

// PackerProvisioner represents a Packer provisioner block
type PackerProvisioner struct {
	Type            string
	Only            []string
	Except          []string
	Inline          []string
	PlaybookFile    string
	GalaxyFile      string
	InventoryFile   string
	ExtraArguments  []string
	AnsibleEnvVars  []string
	CollectionsPath string
	UseProxy        bool
	User            string
	Script          string
	Scripts         []string
}

// PackerBuild represents a Packer build block
type PackerBuild struct {
	Name           string
	Sources        []string
	Provisioners   []PackerProvisioner
	PostProcessors []PackerPostProcessor
}

// PackerPostProcessor represents a Packer post-processor block
type PackerPostProcessor struct {
	Type       string
	Only       []string
	Except     []string
	Output     string
	StripPath  bool
	Repository string
	Tags       []string
	Force      bool
}

// PackerDockerSource represents a Packer docker source block
type PackerDockerSource struct {
	Name       string
	Image      string
	Platform   string
	Commit     bool
	Privileged bool
	Pull       bool
	Volumes    map[string]string
	RunCommand []string
	Changes    []string
}

// PackerAMISource represents a Packer amazon-ebs source block
type PackerAMISource struct {
	Name         string
	InstanceType string
	Region       string
	AMIName      string
	SubnetID     string
	VolumeSize   int
}

// HCLParser parses Packer HCL templates using the official HCL library
type HCLParser struct {
	parser    *hclparse.Parser
	evalCtx   *hcl.EvalContext
	variables map[string]PackerVariable
}

// NewHCLParser creates a new HCL parser
func NewHCLParser() *HCLParser {
	return &HCLParser{
		parser:    hclparse.NewParser(),
		variables: make(map[string]PackerVariable),
		evalCtx:   createEvalContext(),
	}
}

// createEvalContext creates an evaluation context with common Packer functions
func createEvalContext() *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{}),
			// Add path object with root and cwd properties
			"path": cty.ObjectVal(map[string]cty.Value{
				"root": cty.StringVal("${TEMPLATE_DIR}"),
				"cwd":  cty.StringVal("${TEMPLATE_DIR}"),
			}),
		},
		Functions: map[string]function.Function{},
	}
}

// ParseVariablesFile parses a Packer variables.pkr.hcl file
func (p *HCLParser) ParseVariablesFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read variables file: %w", err)
	}

	file, diags := p.parser.ParseHCL(content, filepath.Base(path))
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse HCL: %s", diags.Error())
	}

	// Extract variable blocks
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return fmt.Errorf("unexpected body type")
	}

	for _, block := range body.Blocks {
		if block.Type == "variable" {
			if len(block.Labels) == 0 {
				continue
			}

			varName := block.Labels[0]
			variable := PackerVariable{
				Name: varName,
			}

			// Extract default value
			if attr, exists := block.Body.Attributes["default"]; exists {
				val, diags := attr.Expr.Value(p.evalCtx)
				if !diags.HasErrors() && val.Type() == cty.String {
					variable.Default = val.AsString()
				}
			}

			// Extract type
			if attr, exists := block.Body.Attributes["type"]; exists {
				// Type is typically an expression like `string`, `number`, etc.
				variable.Type = string(attr.Expr.Range().SliceBytes(content))
			}

			p.variables[varName] = variable
		}
	}

	// Update eval context with parsed variables
	varValues := make(map[string]cty.Value)
	for name, v := range p.variables {
		if v.Default != "" {
			varValues[name] = cty.StringVal(v.Default)
		} else {
			// Use placeholder for variables without defaults
			// This allows expressions like ${var.foo} to evaluate to ${FOO}
			varValues[name] = cty.StringVal(fmt.Sprintf("${%s}", strings.ToUpper(name)))
		}
	}
	p.evalCtx.Variables["var"] = cty.ObjectVal(varValues)

	return nil
}

// ParseBuildFile parses a Packer build file (docker.pkr.hcl, ami.pkr.hcl)
func (p *HCLParser) ParseBuildFile(path string) ([]PackerBuild, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read build file: %w", err)
	}

	file, diags := p.parser.ParseHCL(content, filepath.Base(path))
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL: %s", diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected body type")
	}

	var builds []PackerBuild
	for _, block := range body.Blocks {
		if block.Type == "build" {
			build, err := p.parseBuildBlock(block, content)
			if err != nil {
				return nil, fmt.Errorf("failed to parse build block: %w", err)
			}
			builds = append(builds, build)
		}
	}

	return builds, nil
}

// parseBuildBlock parses a build block
func (p *HCLParser) parseBuildBlock(block *hclsyntax.Block, content []byte) (PackerBuild, error) {
	build := PackerBuild{}

	// Extract name if present
	if attr, exists := block.Body.Attributes["name"]; exists {
		val, diags := attr.Expr.Value(p.evalCtx)
		if !diags.HasErrors() && val.Type() == cty.String {
			build.Name = val.AsString()
		}
	}

	// Extract sources
	if attr, exists := block.Body.Attributes["sources"]; exists {
		val, diags := attr.Expr.Value(p.evalCtx)
		if !diags.HasErrors() {
			if val.Type().IsListType() || val.Type().IsTupleType() {
				for _, v := range val.AsValueSlice() {
					if v.Type() == cty.String {
						build.Sources = append(build.Sources, v.AsString())
					}
				}
			}
		}
	}

	// Extract provisioners and post-processors
	for _, provBlock := range block.Body.Blocks {
		if provBlock.Type == "provisioner" {
			provisioner, err := p.parseProvisionerBlock(provBlock, content)
			if err != nil {
				// Log but continue - don't fail entire parse for one provisioner
				continue
			}
			build.Provisioners = append(build.Provisioners, provisioner)
		} else if provBlock.Type == "post-processor" {
			postProc, err := p.parsePostProcessorBlock(provBlock, content)
			if err != nil {
				// Log but continue - don't fail entire parse for one post-processor
				continue
			}
			build.PostProcessors = append(build.PostProcessors, postProc)
		}
	}

	return build, nil
}

// parseProvisionerBlock parses a provisioner block
func (p *HCLParser) parseProvisionerBlock(block *hclsyntax.Block, content []byte) (PackerProvisioner, error) {
	if len(block.Labels) == 0 {
		return PackerProvisioner{}, fmt.Errorf("provisioner block missing type label")
	}

	provisioner := PackerProvisioner{
		Type: block.Labels[0],
	}

	// Extract only/except conditionals using helper function
	provisioner.Only = p.getStringListAttribute(block, "only")
	provisioner.Except = p.getStringListAttribute(block, "except")

	// Parse based on type
	switch provisioner.Type {
	case "shell":
		p.parseShellProvisionerHCL(block, &provisioner)
	case "ansible":
		p.parseAnsibleProvisionerHCL(block, &provisioner)
	}

	return provisioner, nil
}

// getStringAttribute extracts a string value from an HCL attribute
func (p *HCLParser) getStringAttribute(block *hclsyntax.Block, attrName string) string {
	if attr, exists := block.Body.Attributes[attrName]; exists {
		val, diags := attr.Expr.Value(p.evalCtx)
		if !diags.HasErrors() && val.Type() == cty.String {
			return val.AsString()
		}
	}
	return ""
}

// getStringListAttribute extracts a list of strings from an HCL attribute
func (p *HCLParser) getStringListAttribute(block *hclsyntax.Block, attrName string) []string {
	var result []string
	if attr, exists := block.Body.Attributes[attrName]; exists {
		val, diags := attr.Expr.Value(p.evalCtx)
		if !diags.HasErrors() && (val.Type().IsListType() || val.Type().IsTupleType()) {
			for _, v := range val.AsValueSlice() {
				if v.Type() == cty.String {
					result = append(result, v.AsString())
				}
			}
		}
	}
	return result
}

// parseShellProvisionerHCL parses a shell provisioner block
func (p *HCLParser) parseShellProvisionerHCL(block *hclsyntax.Block, provisioner *PackerProvisioner) {
	provisioner.Inline = p.getStringListAttribute(block, "inline")
	provisioner.Script = p.getStringAttribute(block, "script")
	provisioner.Scripts = p.getStringListAttribute(block, "scripts")
}

// parseAnsibleProvisionerHCL parses an ansible provisioner block
func (p *HCLParser) parseAnsibleProvisionerHCL(block *hclsyntax.Block, provisioner *PackerProvisioner) {
	provisioner.PlaybookFile = p.getStringAttribute(block, "playbook_file")
	provisioner.GalaxyFile = p.getStringAttribute(block, "galaxy_file")
	provisioner.InventoryFile = p.getStringAttribute(block, "inventory_file")
	provisioner.CollectionsPath = p.getStringAttribute(block, "ansible_collections_path")
	provisioner.ExtraArguments = p.getStringListAttribute(block, "extra_arguments")
	provisioner.AnsibleEnvVars = p.getStringListAttribute(block, "ansible_env_vars")
	provisioner.User = p.getStringAttribute(block, "user")

	// Extract use_proxy boolean
	if attr, exists := block.Body.Attributes["use_proxy"]; exists {
		val, diags := attr.Expr.Value(p.evalCtx)
		if !diags.HasErrors() && val.Type() == cty.Bool {
			provisioner.UseProxy = val.True()
		}
	}
}

// parsePostProcessorBlock parses a post-processor block
func (p *HCLParser) parsePostProcessorBlock(block *hclsyntax.Block, content []byte) (PackerPostProcessor, error) {
	if len(block.Labels) == 0 {
		return PackerPostProcessor{}, fmt.Errorf("post-processor block missing type label")
	}

	postProc := PackerPostProcessor{
		Type: block.Labels[0],
	}

	// Extract only/except conditionals
	postProc.Only = p.getStringListAttribute(block, "only")
	postProc.Except = p.getStringListAttribute(block, "except")

	// Extract fields based on type
	switch postProc.Type {
	case "manifest":
		postProc.Output = p.getStringAttribute(block, "output")
		// Extract strip_path boolean
		if attr, exists := block.Body.Attributes["strip_path"]; exists {
			val, diags := attr.Expr.Value(p.evalCtx)
			if !diags.HasErrors() && val.Type() == cty.Bool {
				postProc.StripPath = val.True()
			}
		}
	case "docker-tag":
		postProc.Repository = p.getStringAttribute(block, "repository")
		postProc.Tags = p.getStringListAttribute(block, "tags")
		// Extract force boolean
		if attr, exists := block.Body.Attributes["force"]; exists {
			val, diags := attr.Expr.Value(p.evalCtx)
			if !diags.HasErrors() && val.Type() == cty.Bool {
				postProc.Force = val.True()
			}
		}
	}

	return postProc, nil
}

// GetVariable returns a parsed variable by name
func (p *HCLParser) GetVariable(name string) (PackerVariable, bool) {
	v, ok := p.variables[name]
	return v, ok
}

// GetAllVariables returns all parsed variables
func (p *HCLParser) GetAllVariables() map[string]PackerVariable {
	return p.variables
}
