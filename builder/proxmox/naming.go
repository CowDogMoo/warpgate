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
	"regexp"
	"strings"
	"time"
)

// pveNameInvalidRune matches characters not permitted in a Proxmox VM name.
// PVE only allows DNS-style names: letters, digits, and dashes.
var pveNameInvalidRune = regexp.MustCompile(`[^A-Za-z0-9-]+`)

// normalizeVMName converts an arbitrary build name into a valid Proxmox VM
// name. Invalid characters are replaced with dashes and the result is
// truncated to 63 characters to satisfy DNS-label limits. If templateName is
// empty, defaultName is used.
func normalizeVMName(templateName, defaultName string) string {
	name := strings.TrimSpace(templateName)
	if name == "" {
		name = defaultName
	}
	name = pveNameInvalidRune.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	if name == "" {
		return "warpgate-build"
	}
	return name
}

// fmtBuildStamp produces a compact UTC timestamp suitable for resource names.
// Format matches the Azure builder's stamp so logs across builders line up.
func fmtBuildStamp(t time.Time) string {
	return t.UTC().Format("20060102150405")
}

// buildResourceName composes a build-stamped resource name from a base.
//
//	buildResourceName("kali", t) -> "kali-20260601120000"
//
// The result is always Proxmox-name safe (max 63 chars, trimmed dashes).
// When base is empty, the name falls back to "warpgate-build-<stamp>".
func buildResourceName(base string, t time.Time) string {
	stamp := fmtBuildStamp(t)
	fallback := "warpgate-build-" + stamp
	if strings.TrimSpace(base) == "" {
		return fallback
	}
	return normalizeVMName(base+"-"+stamp, fallback)
}
