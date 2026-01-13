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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/builder"
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
	document, err := g.createComponentDocument(provisioner)
	if err != nil {
		return nil, fmt.Errorf("failed to create component document: %w", err)
	}

	componentName := fmt.Sprintf("%s-%s", name, provisioner.Type)

	platform := determinePlatform(provisioner)

	input := &imagebuilder.CreateComponentInput{
		Name:            aws.String(componentName),
		SemanticVersion: aws.String(version),
		Platform:        platform,
		Data:            aws.String(document),
		Description:     aws.String(fmt.Sprintf("Component for %s provisioner", provisioner.Type)),
		Tags: map[string]string{
			"warpgate:type": provisioner.Type,
			"warpgate:name": name,
		},
	}

	result, err := g.clients.ImageBuilder.CreateComponent(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
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
	case "powershell":
		return g.createPowerShellComponent(provisioner)
	default:
		return "", fmt.Errorf("unsupported provisioner type: %s", provisioner.Type)
	}
}

// createShellComponent creates a component document for shell provisioner
func (g *ComponentGenerator) createShellComponent(provisioner builder.Provisioner) (string, error) {
	if len(provisioner.Inline) == 0 {
		return "", fmt.Errorf("shell provisioner has no inline commands")
	}

	commands := append([]string{}, provisioner.Inline...)

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

	if len(provisioner.Environment) > 0 {
		envVars := make([]string, 0, len(provisioner.Environment))
		for key, value := range provisioner.Environment {
			envVars = append(envVars, fmt.Sprintf("export %s=%s", key, value))
		}
		doc["phases"].([]map[string]interface{})[0]["steps"].([]map[string]interface{})[0]["inputs"].(map[string]interface{})["commands"] = append(envVars, commands...)
	}

	return marshalComponentDocument(doc)
}

// createScriptComponent creates a component document for script provisioner
func (g *ComponentGenerator) createScriptComponent(provisioner builder.Provisioner) (string, error) {
	if len(provisioner.Scripts) == 0 {
		return "", fmt.Errorf("script provisioner has no scripts")
	}

	var scriptContents []string
	for _, scriptPath := range provisioner.Scripts {
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script %s: %w", scriptPath, err)
		}
		scriptContents = append(scriptContents, string(content))
	}

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

	platform := determinePlatform(provisioner)
	if platform == types.PlatformWindows {
		return g.createWindowsAnsibleComponent(provisioner)
	}

	return g.createLinuxAnsibleComponent(provisioner)
}

func (g *ComponentGenerator) createLinuxAnsibleComponent(provisioner builder.Provisioner) (string, error) {
	playbookContent, err := os.ReadFile(provisioner.PlaybookPath)
	if err != nil {
		return "", fmt.Errorf("failed to read playbook %s: %w", provisioner.PlaybookPath, err)
	}

	commands := []string{
		"# Install Ansible",
		"if ! command -v ansible-playbook &> /dev/null; then",
		"  apt-get update && apt-get install -y ansible || yum install -y ansible",
		"fi",
	}

	// Use base64 encoding to avoid AWS Image Builder interpreting Jinja2/Ansible template syntax
	if provisioner.GalaxyFile != "" {
		galaxyContent, err := os.ReadFile(provisioner.GalaxyFile)
		if err != nil {
			return "", fmt.Errorf("failed to read galaxy file %s: %w", provisioner.GalaxyFile, err)
		}
		galaxyBase64 := base64.StdEncoding.EncodeToString(galaxyContent)
		commands = append(commands,
			fmt.Sprintf("echo '%s' | base64 -d > /tmp/requirements.yml", galaxyBase64),
			"ansible-galaxy install -r /tmp/requirements.yml",
		)
	}

	playbookName := filepath.Base(provisioner.PlaybookPath)
	playbookBase64 := base64.StdEncoding.EncodeToString(playbookContent)
	commands = append(commands,
		fmt.Sprintf("echo '%s' | base64 -d > /tmp/%s", playbookBase64, playbookName),
	)

	ansibleCmd := fmt.Sprintf("ansible-playbook /tmp/%s", playbookName)

	if len(provisioner.ExtraVars) > 0 {
		var extraVars []string
		for key, value := range provisioner.ExtraVars {
			extraVars = append(extraVars, fmt.Sprintf("%s=%s", key, value))
		}
		ansibleCmd += fmt.Sprintf(" -e '%s'", strings.Join(extraVars, " "))
	}

	if provisioner.Inventory != "" {
		ansibleCmd += fmt.Sprintf(" -i %s", provisioner.Inventory)
	} else {
		ansibleCmd += " --connection=local -i localhost,"
	}

	commands = append(commands, ansibleCmd)

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

func (g *ComponentGenerator) createWindowsAnsibleComponent(provisioner builder.Provisioner) (string, error) {
	playbookContent, err := os.ReadFile(provisioner.PlaybookPath)
	if err != nil {
		return "", fmt.Errorf("failed to read playbook %s: %w", provisioner.PlaybookPath, err)
	}

	commands := []string{
		"$ErrorActionPreference = 'Stop'",
		"$ProgressPreference = 'SilentlyContinue'",
		"",
		"# Use Chocolatey for compatibility with older Windows versions (Server 2016)",
		"if (-not (Get-Command python -ErrorAction SilentlyContinue)) {",
		"    if (-not (Get-Command choco -ErrorAction SilentlyContinue)) {",
		"        Set-ExecutionPolicy Bypass -Scope Process -Force",
		"        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072",
		"        Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))",
		"    }",
		"    choco install python311 -y --no-progress",
		"    $env:Path = [System.Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path','User')",
		"    refreshenv",
		"}",
		"",
		"python -m pip install --upgrade pip",
		"python -m pip install ansible pywinrm",
		"",
	}

	commands = append(commands,
		"New-Item -ItemType Directory -Force -Path 'C:\\temp' | Out-Null",
		"",
	)

	if provisioner.GalaxyFile != "" {
		galaxyContent, err := os.ReadFile(provisioner.GalaxyFile)
		if err != nil {
			return "", fmt.Errorf("failed to read galaxy file %s: %w", provisioner.GalaxyFile, err)
		}
		galaxyBase64 := base64.StdEncoding.EncodeToString(galaxyContent)
		commands = append(commands,
			fmt.Sprintf("$requirementsBase64 = '%s'", galaxyBase64),
			"[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($requirementsBase64)) | Out-File -FilePath 'C:\\temp\\requirements.yml' -Encoding UTF8",
			"ansible-galaxy install -r C:\\temp\\requirements.yml",
			"",
		)
	}

	playbookName := filepath.Base(provisioner.PlaybookPath)
	playbookBase64 := base64.StdEncoding.EncodeToString(playbookContent)
	commands = append(commands,
		fmt.Sprintf("$playbookBase64 = '%s'", playbookBase64),
		fmt.Sprintf("[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($playbookBase64)) | Out-File -FilePath 'C:\\temp\\%s' -Encoding UTF8", playbookName),
		"",
	)

	ansibleCmd := fmt.Sprintf("ansible-playbook 'C:\\temp\\%s'", playbookName)

	if len(provisioner.ExtraVars) > 0 {
		var extraVars []string
		for key, value := range provisioner.ExtraVars {
			// Skip ansible connection vars for local execution
			if key == "ansible_connection" || key == "ansible_shell_type" ||
				key == "ansible_aws_ssm_bucket_name" || key == "ansible_aws_ssm_region" {
				continue
			}
			extraVars = append(extraVars, fmt.Sprintf("%s=%s", key, value))
		}
		if len(extraVars) > 0 {
			ansibleCmd += fmt.Sprintf(" -e '%s'", strings.Join(extraVars, " "))
		}
	}

	// Force local connection for Windows AMI builds
	ansibleCmd += " --connection=local -i 'localhost,'"

	commands = append(commands,
		ansibleCmd,
	)

	doc := map[string]interface{}{
		"schemaVersion": 1.0,
		"name":          "AnsibleProvisioner",
		"description":   "Ansible provisioner component for Windows",
		"phases": []map[string]interface{}{
			{
				"name": "build",
				"steps": []map[string]interface{}{
					{
						"name":   "ExecuteAnsible",
						"action": "ExecutePowerShell",
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

// createPowerShellComponent creates a component document for PowerShell provisioner
func (g *ComponentGenerator) createPowerShellComponent(provisioner builder.Provisioner) (string, error) {
	if len(provisioner.PSScripts) == 0 {
		return "", fmt.Errorf("powershell provisioner has no scripts")
	}

	var scriptContents []string
	for _, scriptPath := range provisioner.PSScripts {
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read PowerShell script %s: %w", scriptPath, err)
		}

		scriptContent := processPowerShellScript(content)

		if err := validatePowerShellSyntax(scriptContent); err != nil {
			return "", fmt.Errorf("PowerShell script %s has syntax issues: %w", scriptPath, err)
		}

		scriptContents = append(scriptContents, scriptContent)
	}

	executionPolicy := provisioner.ExecutionPolicy
	if executionPolicy == "" {
		executionPolicy = "Bypass"
	}

	steps := []map[string]interface{}{}

	for i, script := range scriptContents {
		stepName := fmt.Sprintf("ExecutePowerShellScript_%d", i)
		wrappedScript := wrapPowerShellWithErrorHandling(script, executionPolicy)

		steps = append(steps, map[string]interface{}{
			"name":   stepName,
			"action": "ExecutePowerShell",
			"inputs": map[string]interface{}{
				"commands": []string{wrappedScript},
			},
		})

		if needsReboot(script) {
			steps = append(steps, map[string]interface{}{
				"name":   fmt.Sprintf("RebootAfterScript_%d", i),
				"action": "Reboot",
			})
		}
	}

	doc := map[string]interface{}{
		"schemaVersion": 1.0,
		"name":          "PowerShellProvisioner",
		"description":   "PowerShell provisioner component",
		"phases": []map[string]interface{}{
			{
				"name":  "build",
				"steps": steps,
			},
		},
	}

	return marshalComponentDocument(doc)
}

func processPowerShellScript(content []byte) string {
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		content = content[3:]
	}

	script := string(content)
	script = strings.ReplaceAll(script, "\r\n", "\n")

	return script
}

// validatePowerShellSyntax performs basic PowerShell syntax validation
func validatePowerShellSyntax(script string) error {
	if err := checkCharacterBalance(script, '{', '}', "braces"); err != nil {
		return err
	}
	if err := checkCharacterBalance(script, '(', ')', "parentheses"); err != nil {
		return err
	}
	return nil
}

// checkCharacterBalance verifies that opening and closing characters are balanced
func checkCharacterBalance(script string, open, close rune, name string) error {
	count := 0
	for _, char := range script {
		switch char {
		case open:
			count++
		case close:
			count--
			if count < 0 {
				return fmt.Errorf("unbalanced %s: unexpected '%c'", name, close)
			}
		}
	}
	if count != 0 {
		return fmt.Errorf("unbalanced %s: %d unclosed '%c'", name, count, open)
	}
	return nil
}

func wrapPowerShellWithErrorHandling(script string, executionPolicy string) string {
	wrapper := `# Set execution policy for this session
Set-ExecutionPolicy -ExecutionPolicy %s -Scope Process -Force

$ErrorActionPreference = 'Stop'
$VerbosePreference = 'Continue'

try {
%s
} catch {
    Write-Error "Script failed with error: $_"
    Write-Error "Stack trace: $($_.ScriptStackTrace)"
    exit 1
}`
	return fmt.Sprintf(wrapper, executionPolicy, script)
}

// needsReboot checks if the script contains reboot indicators
func needsReboot(script string) bool {
	rebootIndicators := []string{
		"Restart-Computer",
		"shutdown /r",
		"shutdown -r",
		"#REQUIRES_REBOOT",
		"# REQUIRES_REBOOT",
	}

	scriptLower := strings.ToLower(script)
	for _, indicator := range rebootIndicators {
		if strings.Contains(scriptLower, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
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
