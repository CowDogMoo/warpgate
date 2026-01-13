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

package ami

import (
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/builder"
)

// determinePlatform determines the target platform based on provisioner configuration.
// Windows is detected if the provisioner type is powershell, or if ansible extra_vars
// indicate Windows (e.g., ansible_shell_type: powershell).
func determinePlatform(provisioner builder.Provisioner) types.Platform {
	// PowerShell provisioner is always Windows
	if provisioner.Type == "powershell" {
		return types.PlatformWindows
	}

	// For Ansible, check extra_vars for Windows indicators
	if provisioner.Type == "ansible" {
		shellType, ok := provisioner.ExtraVars["ansible_shell_type"]
		if ok && (shellType == "powershell" || shellType == "cmd") {
			return types.PlatformWindows
		}
		// Also check for SSM connection which is commonly used for Windows
		connection, ok := provisioner.ExtraVars["ansible_connection"]
		if ok && connection == "aws_ssm" {
			// SSM with powershell shell type indicates Windows
			if shellType == "powershell" {
				return types.PlatformWindows
			}
		}
	}

	return types.PlatformLinux
}
