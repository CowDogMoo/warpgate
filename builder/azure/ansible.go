/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

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

package azure

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/cowdogmoo/warpgate/v3/builder"
)

// ansibleCustomizer turns an ansible provisioner into a single AIB customizer.
// On Linux targets we emit a Shell customizer that installs ansible, decodes
// the playbook (and optional galaxy requirements) from base64, and runs
// ansible-playbook. On Windows targets we emit a PowerShell customizer that
// installs ansible via Chocolatey + pip, then runs the playbook the same way.
//
// Files are embedded as base64 inside the inline commands rather than uploaded
// to blob storage so the build remains self-contained — no extra storage
// account is required for the common ansible case.
func ansibleCustomizer(p *builder.Provisioner, index int, osType string) (armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	if p.PlaybookPath == "" {
		return nil, fmt.Errorf("provisioner[%d] ansible: playbook_path is required", index)
	}

	playbookBytes, err := os.ReadFile(p.PlaybookPath)
	if err != nil {
		return nil, fmt.Errorf("provisioner[%d] ansible: read playbook %s: %w", index, p.PlaybookPath, err)
	}

	var galaxyBytes []byte
	if p.GalaxyFile != "" {
		galaxyBytes, err = os.ReadFile(p.GalaxyFile)
		if err != nil {
			return nil, fmt.Errorf("provisioner[%d] ansible: read galaxy file %s: %w", index, p.GalaxyFile, err)
		}
	}

	if isWindowsAnsibleTarget(p, osType) {
		return windowsAnsibleCustomizer(p, index, playbookBytes, galaxyBytes), nil
	}
	return linuxAnsibleCustomizer(p, index, playbookBytes, galaxyBytes), nil
}

// isWindowsAnsibleTarget reports whether an ansible provisioner is targeting
// Windows. Target OSType (set on the build target) takes precedence; if the
// caller didn't set it, the provisioner's ExtraVars are inspected.
func isWindowsAnsibleTarget(p *builder.Provisioner, osType string) bool {
	if strings.EqualFold(osType, "windows") {
		return true
	}
	if shellType, ok := p.ExtraVars["ansible_shell_type"]; ok {
		if shellType == "powershell" || shellType == "cmd" {
			return true
		}
	}
	return false
}

// linuxAnsibleCustomizer emits a Shell customizer that runs the playbook on a
// Linux target. The playbook (and optional galaxy requirements) are embedded
// as base64 so no blob staging is required.
func linuxAnsibleCustomizer(p *builder.Provisioner, index int, playbookBytes, galaxyBytes []byte) armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification {
	commands := []string{
		"set -eu",
		"if ! command -v ansible-playbook >/dev/null 2>&1; then",
		"  if command -v apt-get >/dev/null 2>&1; then",
		"    sudo apt-get update -y && sudo DEBIAN_FRONTEND=noninteractive apt-get install -y ansible",
		"  elif command -v dnf >/dev/null 2>&1; then",
		"    sudo dnf install -y ansible",
		"  elif command -v yum >/dev/null 2>&1; then",
		"    sudo yum install -y ansible",
		"  else",
		"    echo 'no supported package manager found to install ansible' >&2",
		"    exit 1",
		"  fi",
		"fi",
		"mkdir -p /tmp/warpgate-ansible",
	}

	if len(galaxyBytes) > 0 {
		galaxyB64 := base64.StdEncoding.EncodeToString(galaxyBytes)
		commands = append(commands,
			fmt.Sprintf("echo '%s' | base64 -d > /tmp/warpgate-ansible/requirements.yml", galaxyB64),
			"ansible-galaxy install -r /tmp/warpgate-ansible/requirements.yml",
		)
	}

	playbookName := filepath.Base(p.PlaybookPath)
	playbookB64 := base64.StdEncoding.EncodeToString(playbookBytes)
	playbookOnDisk := "/tmp/warpgate-ansible/" + playbookName
	commands = append(commands,
		fmt.Sprintf("echo '%s' | base64 -d > %s", playbookB64, playbookOnDisk),
	)

	cmd := "ansible-playbook " + playbookOnDisk
	for _, key := range sortedExtraVarKeys(p.ExtraVars) {
		if isLinuxIgnoredExtraVar(key) {
			continue
		}
		cmd += fmt.Sprintf(" -e '%s=%s'", key, shellEscapeSingleQuote(p.ExtraVars[key]))
	}
	if p.Inventory != "" {
		cmd += " -i " + shellSingleQuote(p.Inventory)
	} else {
		cmd += " --connection=local -i 'localhost,'"
	}
	commands = append(commands, cmd)

	return &armvirtualmachineimagebuilder.ImageTemplateShellCustomizer{
		Type:   to.Ptr("Shell"),
		Name:   to.Ptr(fmt.Sprintf("ansible-%d", index)),
		Inline: stringSliceToPointerSlice(commands),
	}
}

// windowsAnsibleCustomizer emits a PowerShell customizer that installs
// ansible (via Chocolatey + pip) and runs the playbook against the local host.
func windowsAnsibleCustomizer(p *builder.Provisioner, index int, playbookBytes, galaxyBytes []byte) armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification {
	commands := []string{
		"$ErrorActionPreference = 'Stop'",
		"$ProgressPreference = 'SilentlyContinue'",
		"if (-not (Get-Command python -ErrorAction SilentlyContinue)) {",
		"    if (-not (Get-Command choco -ErrorAction SilentlyContinue)) {",
		"        Set-ExecutionPolicy Bypass -Scope Process -Force",
		"        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072",
		"        Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))",
		"    }",
		"    choco install python311 -y --no-progress",
		"    $env:Path = [System.Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path','User')",
		"}",
		"python -m pip install --upgrade pip",
		"python -m pip install ansible pywinrm",
		"New-Item -ItemType Directory -Force -Path 'C:\\warpgate-ansible' | Out-Null",
	}

	if len(galaxyBytes) > 0 {
		galaxyB64 := base64.StdEncoding.EncodeToString(galaxyBytes)
		commands = append(commands,
			fmt.Sprintf("$reqB64 = '%s'", galaxyB64),
			"[System.IO.File]::WriteAllBytes('C:\\warpgate-ansible\\requirements.yml', [System.Convert]::FromBase64String($reqB64))",
			"ansible-galaxy install -r C:\\warpgate-ansible\\requirements.yml",
		)
	}

	playbookName := filepath.Base(p.PlaybookPath)
	playbookB64 := base64.StdEncoding.EncodeToString(playbookBytes)
	playbookOnDisk := "C:\\warpgate-ansible\\" + playbookName
	commands = append(commands,
		fmt.Sprintf("$pbB64 = '%s'", playbookB64),
		fmt.Sprintf("[System.IO.File]::WriteAllBytes('%s', [System.Convert]::FromBase64String($pbB64))", powerShellEscape(playbookOnDisk)),
	)

	cmd := "ansible-playbook '" + powerShellEscape(playbookOnDisk) + "'"
	for _, key := range sortedExtraVarKeys(p.ExtraVars) {
		if isWindowsIgnoredExtraVar(key) {
			continue
		}
		cmd += fmt.Sprintf(" -e '%s=%s'", key, shellEscapeSingleQuote(p.ExtraVars[key]))
	}
	cmd += " --connection=local -i 'localhost,'"
	commands = append(commands, cmd)

	return &armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer{
		Type:        to.Ptr("PowerShell"),
		Name:        to.Ptr(fmt.Sprintf("ansible-%d", index)),
		Inline:      stringSliceToPointerSlice(commands),
		RunElevated: to.Ptr(true),
	}
}

// sortedExtraVarKeys returns the ExtraVars keys in stable lexical order so the
// generated commands are deterministic (important for tests + diff stability).
func sortedExtraVarKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isLinuxIgnoredExtraVar reports whether an extra var should be skipped when
// running ansible locally on Linux. Connection-related vars only make sense
// for remote executions.
func isLinuxIgnoredExtraVar(key string) bool {
	switch key {
	case "ansible_connection", "ansible_aws_ssm_bucket_name", "ansible_aws_ssm_region":
		return true
	}
	return false
}

// isWindowsIgnoredExtraVar reports whether an extra var should be skipped when
// running ansible locally on Windows. We always force --connection=local so
// connection-flavor vars would conflict.
func isWindowsIgnoredExtraVar(key string) bool {
	switch key {
	case "ansible_connection", "ansible_shell_type", "ansible_aws_ssm_bucket_name", "ansible_aws_ssm_region":
		return true
	}
	return false
}

// shellEscapeSingleQuote escapes a string for safe use inside a single-quoted
// shell argument. POSIX shell does not allow a literal single quote inside
// single quotes, so each one is replaced with: '\”
func shellEscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}

// shellSingleQuote wraps s in single quotes after escaping any embedded ones.
func shellSingleQuote(s string) string {
	return "'" + shellEscapeSingleQuote(s) + "'"
}

// powerShellEscape escapes a string for embedding inside a single-quoted
// PowerShell literal. PowerShell single quotes are escaped by doubling.
func powerShellEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
