/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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

package ami

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/pkg/builder"
	"gopkg.in/yaml.v3"
)

// ComponentGenerator generates EC2 Image Builder components from provisioners
type ComponentGenerator struct {
	clients *AWSClients
}

// NewComponentGenerator creates a new component generator
func NewComponentGenerator(clients *AWSClients) *ComponentGenerator {
	return &ComponentGenerator{
		clients: clients,
	}
}

// GenerateComponent creates an Image Builder component from a provisioner
func (g *ComponentGenerator) GenerateComponent(ctx context.Context, provisioner builder.Provisioner, name, version string) (*string, error) {
	// Generate component document based on provisioner type
	document, err := g.createComponentDocument(provisioner)
	if err != nil {
		return nil, fmt.Errorf("failed to create component document: %w", err)
	}

	// Create the component in AWS
	componentName := fmt.Sprintf("%s-%s", name, provisioner.Type)
	input := &imagebuilder.CreateComponentInput{
		Name:            aws.String(componentName),
		SemanticVersion: aws.String(version),
		Platform:        types.PlatformLinux,
		Data:            aws.String(document),
		Description:     aws.String(fmt.Sprintf("Component for %s provisioner", provisioner.Type)),
		Tags: map[string]string{
			"warpgate:type": provisioner.Type,
			"warpgate:name": name,
		},
	}

	result, err := g.clients.ImageBuilder.CreateComponent(ctx, input)
	if err != nil {
		// Check if component already exists
		if strings.Contains(err.Error(), "already exists") {
			// Get existing component ARN
			arn, getErr := g.getComponentARN(ctx, componentName, version)
			if getErr != nil {
				return nil, fmt.Errorf("component exists but failed to get ARN: %w", getErr)
			}
			return arn, nil
		}
		return nil, fmt.Errorf("failed to create component: %w", err)
	}

	return result.ComponentBuildVersionArn, nil
}

// createComponentDocument generates a YAML document for the Image Builder component
func (g *ComponentGenerator) createComponentDocument(provisioner builder.Provisioner) (string, error) {
	switch provisioner.Type {
	case "shell":
		return g.createShellComponent(provisioner)
	case "script":
		return g.createScriptComponent(provisioner)
	case "ansible":
		return g.createAnsibleComponent(provisioner)
	default:
		return "", fmt.Errorf("unsupported provisioner type: %s", provisioner.Type)
	}
}

// createShellComponent creates a component document for shell provisioner
func (g *ComponentGenerator) createShellComponent(provisioner builder.Provisioner) (string, error) {
	if len(provisioner.Inline) == 0 {
		return "", fmt.Errorf("shell provisioner has no inline commands")
	}

	// Build the shell script
	commands := append([]string{}, provisioner.Inline...)

	// Create component document
	doc := map[string]interface{}{
		"schemaVersion": 1.0,
		"name":          "ShellProvisioner",
		"description":   "Shell provisioner component",
		"phases": []map[string]interface{}{
			{
				"name": "build",
				"steps": []map[string]interface{}{
					{
						"name":   "ExecuteShellCommands",
						"action": "ExecuteBash",
						"inputs": map[string]interface{}{
							"commands": commands,
						},
					},
				},
			},
		},
	}

	// Add environment variables if present
	if len(provisioner.Environment) > 0 {
		envVars := make([]string, 0, len(provisioner.Environment))
		for key, value := range provisioner.Environment {
			envVars = append(envVars, fmt.Sprintf("export %s=%s", key, value))
		}
		// Prepend environment variables to commands
		doc["phases"].([]map[string]interface{})[0]["steps"].([]map[string]interface{})[0]["inputs"].(map[string]interface{})["commands"] = append(envVars, commands...)
	}

	return marshalComponentDocument(doc)
}

// createScriptComponent creates a component document for script provisioner
func (g *ComponentGenerator) createScriptComponent(provisioner builder.Provisioner) (string, error) {
	if len(provisioner.Scripts) == 0 {
		return "", fmt.Errorf("script provisioner has no scripts")
	}

	// Read and combine all scripts
	var scriptContents []string
	for _, scriptPath := range provisioner.Scripts {
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script %s: %w", scriptPath, err)
		}
		scriptContents = append(scriptContents, string(content))
	}

	// Create component document
	doc := map[string]interface{}{
		"schemaVersion": 1.0,
		"name":          "ScriptProvisioner",
		"description":   "Script provisioner component",
		"phases": []map[string]interface{}{
			{
				"name": "build",
				"steps": []map[string]interface{}{
					{
						"name":   "ExecuteScripts",
						"action": "ExecuteBash",
						"inputs": map[string]interface{}{
							"commands": scriptContents,
						},
					},
				},
			},
		},
	}

	return marshalComponentDocument(doc)
}

// createAnsibleComponent creates a component document for ansible provisioner
func (g *ComponentGenerator) createAnsibleComponent(provisioner builder.Provisioner) (string, error) {
	if provisioner.PlaybookPath == "" {
		return "", fmt.Errorf("ansible provisioner has no playbook path")
	}

	// Read playbook content
	playbookContent, err := os.ReadFile(provisioner.PlaybookPath)
	if err != nil {
		return "", fmt.Errorf("failed to read playbook %s: %w", provisioner.PlaybookPath, err)
	}

	// Build commands to install Ansible and run playbook
	commands := []string{
		"# Install Ansible",
		"if ! command -v ansible-playbook &> /dev/null; then",
		"  apt-get update && apt-get install -y ansible || yum install -y ansible",
		"fi",
	}

	// Install Galaxy collections if specified
	if provisioner.GalaxyFile != "" {
		galaxyContent, err := os.ReadFile(provisioner.GalaxyFile)
		if err != nil {
			return "", fmt.Errorf("failed to read galaxy file %s: %w", provisioner.GalaxyFile, err)
		}
		commands = append(commands,
			"# Install Galaxy requirements",
			fmt.Sprintf("cat > /tmp/requirements.yml << 'EOF'\n%s\nEOF", string(galaxyContent)),
			"ansible-galaxy install -r /tmp/requirements.yml",
		)
	}

	// Create playbook file
	playbookName := filepath.Base(provisioner.PlaybookPath)
	commands = append(commands,
		"# Create playbook",
		fmt.Sprintf("cat > /tmp/%s << 'EOF'\n%s\nEOF", playbookName, string(playbookContent)),
	)

	// Build ansible-playbook command
	ansibleCmd := fmt.Sprintf("ansible-playbook /tmp/%s", playbookName)

	// Add extra vars if present
	if len(provisioner.ExtraVars) > 0 {
		var extraVars []string
		for key, value := range provisioner.ExtraVars {
			extraVars = append(extraVars, fmt.Sprintf("%s=%s", key, value))
		}
		ansibleCmd += fmt.Sprintf(" -e '%s'", strings.Join(extraVars, " "))
	}

	// Add inventory if present
	if provisioner.Inventory != "" {
		ansibleCmd += fmt.Sprintf(" -i %s", provisioner.Inventory)
	} else {
		ansibleCmd += " --connection=local -i localhost,"
	}

	commands = append(commands, ansibleCmd)

	// Create component document
	doc := map[string]interface{}{
		"schemaVersion": 1.0,
		"name":          "AnsibleProvisioner",
		"description":   "Ansible provisioner component",
		"phases": []map[string]interface{}{
			{
				"name": "build",
				"steps": []map[string]interface{}{
					{
						"name":   "ExecuteAnsible",
						"action": "ExecuteBash",
						"inputs": map[string]interface{}{
							"commands": commands,
						},
					},
				},
			},
		},
	}

	return marshalComponentDocument(doc)
}

// marshalComponentDocument converts a component document to YAML string
func marshalComponentDocument(doc map[string]interface{}) (string, error) {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal component document: %w", err)
	}
	return string(data), nil
}

// getComponentARN retrieves the ARN of an existing component
func (g *ComponentGenerator) getComponentARN(ctx context.Context, name, version string) (*string, error) {
	input := &imagebuilder.ListComponentsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{name},
			},
		},
	}

	result, err := g.clients.ImageBuilder.ListComponents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list components: %w", err)
	}

	// Find matching component version
	for _, component := range result.ComponentVersionList {
		if component.Version != nil && *component.Version == version {
			return component.Arn, nil
		}
	}

	return nil, fmt.Errorf("component %s version %s not found", name, version)
}

// DeleteComponent deletes an Image Builder component
func (g *ComponentGenerator) DeleteComponent(ctx context.Context, componentARN string) error {
	input := &imagebuilder.DeleteComponentInput{
		ComponentBuildVersionArn: aws.String(componentARN),
	}

	_, err := g.clients.ImageBuilder.DeleteComponent(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete component: %w", err)
	}

	return nil
}
