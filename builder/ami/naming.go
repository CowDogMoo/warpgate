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
	"fmt"
	"strings"
)

// normalizeAMIName ensures the AMI name contains a valid timestamp placeholder for AWS Image Builder.
// AWS Image Builder requires AMI names to contain '{{ imagebuilder:buildDate }}' for uniqueness.
// This function converts common timestamp patterns to the required format.
func normalizeAMIName(amiName, defaultName string) string {
	if amiName == "" {
		return fmt.Sprintf("%s-{{ imagebuilder:buildDate }}", defaultName)
	}

	replacements := []struct {
		old string
		new string
	}{
		{"{{timestamp}}", "{{ imagebuilder:buildDate }}"},
		{"{{ timestamp }}", "{{ imagebuilder:buildDate }}"},
		{"{{imagebuilder:buildDate}}", "{{ imagebuilder:buildDate }}"},
	}

	result := amiName
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}

	// If no timestamp placeholder found, append one
	if !strings.Contains(result, "{{ imagebuilder:buildDate }}") {
		result += "-{{ imagebuilder:buildDate }}"
	}

	return result
}

// NormalizeSemanticVersion converts non-standard version strings to valid semantic versions.
// AWS Image Builder requires versions to match ^[0-9]+\.[0-9]+\.[0-9]+$
// This function handles common cases like "latest", empty strings, or partial versions.
func NormalizeSemanticVersion(version string) string {
	// Handle empty or "latest" - default to 1.0.0
	if version == "" || strings.EqualFold(version, "latest") {
		return "1.0.0"
	}

	// Parse and reformat to ensure valid format
	major, minor, patch := ParseSemanticVersion(version)

	// If parsing yielded all zeros from a non-empty string that wasn't "latest",
	// it means the string wasn't a valid version - default to 1.0.0
	if major == 0 && minor == 0 && patch == 0 {
		parts := strings.Split(version, ".")
		// Check if it's actually "0.0.0" vs an invalid string
		if len(parts) < 3 || parts[0] != "0" {
			return "1.0.0"
		}
	}

	return FormatSemanticVersion(major, minor, patch)
}
