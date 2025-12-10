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

package manifests

import (
	"strings"
)

// PlatformInfo represents parsed platform information from a platform string.
// Platform strings follow the format: [os]/[architecture][/variant]
// Examples:
//   - "linux/amd64" -> OS: "linux", Architecture: "amd64", Variant: ""
//   - "linux/arm64/v8" -> OS: "linux", Architecture: "arm64", Variant: "v8"
//   - "amd64" -> OS: "linux" (default), Architecture: "amd64", Variant: ""
//   - "arm/v7" -> OS: "linux" (default), Architecture: "arm", Variant: "v7"
type PlatformInfo struct {
	// OS is the operating system (e.g., "linux", "windows", "darwin")
	OS string

	// Architecture is the CPU architecture (e.g., "amd64", "arm64", "arm")
	Architecture string

	// Variant is the architecture variant (e.g., "v7", "v8" for ARM)
	Variant string
}

// ParsePlatform parses a platform string into its constituent parts.
// It handles various platform string formats and normalizes them into a PlatformInfo struct.
//
// Supported formats:
//   - "os/architecture/variant" (e.g., "linux/arm64/v8")
//   - "os/architecture" (e.g., "linux/amd64")
//   - "architecture/variant" (e.g., "arm/v7") - assumes OS is "linux"
//   - "architecture" (e.g., "amd64") - assumes OS is "linux"
//
// Parameters:
//   - platformStr: The platform string to parse
//
// Returns:
//   - PlatformInfo with parsed components. Defaults OS to "linux" if not specified.
//
// Examples:
//
//	ParsePlatform("linux/amd64")      // -> {OS: "linux", Architecture: "amd64", Variant: ""}
//	ParsePlatform("linux/arm64/v8")   // -> {OS: "linux", Architecture: "arm64", Variant: "v8"}
//	ParsePlatform("amd64")            // -> {OS: "linux", Architecture: "amd64", Variant: ""}
//	ParsePlatform("arm/v7")           // -> {OS: "linux", Architecture: "arm", Variant: "v7"}
func ParsePlatform(platformStr string) PlatformInfo {
	info := PlatformInfo{
		OS: "linux", // Default to linux
	}

	// If no slash, it's just an architecture
	if !strings.Contains(platformStr, "/") {
		info.Architecture = platformStr
		return info
	}

	// Split the platform string by "/"
	parts := strings.Split(platformStr, "/")

	// Determine the format based on the number of parts
	switch len(parts) {
	case 1:
		// Just architecture (shouldn't reach here due to Contains check above)
		info.Architecture = parts[0]
	case 2:
		// Could be "os/arch" or "arch/variant"
		// We distinguish by checking if the first part looks like an OS
		// Common OS values: linux, windows, darwin, freebsd
		// Common architectures: amd64, arm64, arm, 386, ppc64le, s390x, etc.
		if isLikelyOS(parts[0]) {
			info.OS = parts[0]
			info.Architecture = parts[1]
		} else {
			// Assume it's "arch/variant" (e.g., "arm/v7")
			info.Architecture = parts[0]
			info.Variant = parts[1]
		}
	default:
		// Three or more parts: "os/arch/variant/..."
		// Take the first three parts
		info.OS = parts[0]
		info.Architecture = parts[1]
		if len(parts) >= 3 {
			info.Variant = parts[2]
		}
	}

	return info
}

// isLikelyOS checks if a string is likely an operating system name rather than an architecture.
// This is a heuristic used to disambiguate "os/arch" from "arch/variant" format.
func isLikelyOS(s string) bool {
	// Common OS names used in container platforms
	switch strings.ToLower(s) {
	case "linux", "windows", "darwin", "freebsd", "openbsd", "netbsd", "solaris", "aix", "plan9", "js":
		return true
	default:
		return false
	}
}

// FormatPlatform formats a PlatformInfo into a platform string.
// It creates a properly formatted platform string from the components.
//
// Examples:
//
//	FormatPlatform(PlatformInfo{OS: "linux", Architecture: "amd64"})
//	  -> "linux/amd64"
//	FormatPlatform(PlatformInfo{OS: "linux", Architecture: "arm64", Variant: "v8"})
//	  -> "linux/arm64/v8"
func FormatPlatform(info PlatformInfo) string {
	if info.Variant != "" {
		return info.OS + "/" + info.Architecture + "/" + info.Variant
	}
	return info.OS + "/" + info.Architecture
}
