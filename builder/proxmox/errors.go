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

package proxmox

import (
	"fmt"
	"strings"
)

// BuildError wraps a Proxmox build failure with a contextual message and an
// actionable remediation hint. Mirrors the AMI builder's BuildError.
type BuildError struct {
	Context     string
	Remediation string
	Err         error
}

// Error formats the wrapped error with the available context and remediation.
func (e *BuildError) Error() string {
	parts := []string{}
	if e.Context != "" {
		parts = append(parts, e.Context)
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	msg := strings.Join(parts, ": ")
	if e.Remediation != "" {
		msg = msg + "\n  remediation: " + e.Remediation
	}
	return msg
}

// Unwrap exposes the wrapped error so errors.Is/As work as expected.
func (e *BuildError) Unwrap() error { return e.Err }

// errorPattern matches a substring in a Proxmox error message and pairs it
// with a remediation hint shown to the user.
type errorPattern struct {
	Match       string
	Remediation string
}

// proxmoxErrorPatterns lists the most common Proxmox failure substrings and
// the action a user should take. Order matters: the first match wins.
var proxmoxErrorPatterns = []errorPattern{
	{
		Match:       "401",
		Remediation: "verify the API token/username has the required PVEVMAdmin role and the credential has not expired",
	},
	{
		Match:       "authentication failed",
		Remediation: "verify the API token/username has the required PVEVMAdmin role and the credential has not expired",
	},
	{
		Match:       "403",
		Remediation: "the token/user lacks permission for the requested operation; grant PVEVMAdmin on the target node or VM",
	},
	{
		Match:       "permission check",
		Remediation: "the token/user lacks permission for the requested operation; grant PVEVMAdmin on the target node or VM",
	},
	{
		Match:       "no such node",
		Remediation: "check the target.node value in your template config matches an existing PVE node",
	},
	{
		Match:       "configuration file already exists",
		Remediation: "VMID already in use; let warpgate auto-allocate a VMID via /cluster/nextid or remove the existing VM",
	},
	{
		Match:       "VM is locked",
		Remediation: "another task is running against this VM; wait for it to finish or unlock with `qm unlock <vmid>` on the node",
	},
	{
		Match:       "storage",
		Remediation: "verify the configured storage exists on the node and has free space (see `pvesm status`)",
	},
	{
		Match:       "no such logical volume",
		Remediation: "the source template's disk is missing on the target storage; verify the template VMID exists on this node",
	},
	{
		Match:       "QEMU guest agent is not running",
		Remediation: "the source template must have the QEMU guest agent installed and `agent: 1` set in its config",
	},
	{
		Match:       "connection refused",
		Remediation: "the Proxmox API endpoint is unreachable; verify the URL, that the network allows access, and that the API is up",
	},
	{
		Match:       "x509",
		Remediation: "set insecure_skip_verify: true in client config to accept self-signed certs, or install a trusted CA on the cluster",
	},
}

// WrapWithRemediation wraps err with the supplied context and the best
// matching remediation hint, or returns err unchanged when nothing matches.
func WrapWithRemediation(err error, context string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, p := range proxmoxErrorPatterns {
		if matchesPattern(msg, p) {
			return &BuildError{Context: context, Remediation: p.Remediation, Err: err}
		}
	}
	if context == "" {
		return err
	}
	return fmt.Errorf("%s: %w", context, err)
}

// matchesPattern reports whether errMsg contains the pattern's Match string
// in a case-insensitive way.
func matchesPattern(errMsg string, p errorPattern) bool {
	if p.Match == "" {
		return false
	}
	return strings.Contains(strings.ToLower(errMsg), strings.ToLower(p.Match))
}
