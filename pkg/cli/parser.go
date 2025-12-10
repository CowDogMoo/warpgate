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

// Package cli provides utilities for parsing, validating, and formatting CLI input and output.
//
// This package serves as the interface layer between the command-line interface and
// the core domain logic, handling:
//
//   - Parsing: Converting raw CLI flags into structured data (key=value pairs, labels, build args)
//   - Validation: Ensuring CLI options are valid before passing to business logic
//   - Output: Formatting build results and errors for human-readable display
//
// The package is designed to keep CLI concerns separate from the core builder logic,
// following the adapter pattern to translate between CLI representations and domain models.
//
// # Key Components
//
// Parser: Handles parsing of key=value pairs from CLI flags:
//
//	parser := cli.NewParser()
//	labels, err := parser.ParseLabels([]string{"env=prod", "version=1.0"})
//
// Validator: Validates CLI options before processing:
//
//	validator := cli.NewValidator()
//	if err := validator.ValidateBuildOptions(opts); err != nil {
//	    return fmt.Errorf("invalid options: %w", err)
//	}
//
// OutputFormatter: Formats build results for display:
//
//	formatter := cli.NewOutputFormatter("text")
//	formatter.DisplayBuildResults(ctx, results)
//
// # Design Principles
//
//   - Separation of Concerns: CLI parsing logic is isolated from domain logic
//   - Fail Fast: Validation occurs at the CLI boundary before business logic
//   - Adapter Pattern: Translates between CLI and domain representations
//   - Type Safety: Strongly typed options structs prevent runtime errors
package cli

import (
	"fmt"
	"strings"
)

// Parser handles parsing of CLI input into structured data.
type Parser struct{}

// NewParser creates a new CLI parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseKeyValuePairs parses key=value pairs from CLI flags.
// Returns a map and an error if any pair is malformed.
//
// Example:
//
//	pairs := []string{"key1=value1", "key2=value2"}
//	result, err := parser.ParseKeyValuePairs(pairs)
//	// result == map[string]string{"key1": "value1", "key2": "value2"}
func (p *Parser) ParseKeyValuePairs(pairs []string) (map[string]string, error) {
	result := make(map[string]string, len(pairs))

	for _, pair := range pairs {
		key, value, err := ParseKeyValue(pair)
		if err != nil {
			return nil, fmt.Errorf("invalid pair %q: %w", pair, err)
		}
		result[key] = value
	}

	return result, nil
}

// ParseKeyValue parses a single key=value string.
// Returns the key, value, and an error if the format is invalid.
func ParseKeyValue(pair string) (string, string, error) {
	parts := strings.SplitN(pair, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected format key=value, got %q", pair)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if key == "" {
		return "", "", fmt.Errorf("key cannot be empty")
	}

	return key, value, nil
}

// ParseLabels parses label flags in key=value format.
// Returns a map of labels and an error if any label is malformed.
func (p *Parser) ParseLabels(labels []string) (map[string]string, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	return p.ParseKeyValuePairs(labels)
}

// ParseBuildArgs parses build argument flags in key=value format.
// Returns a map of build arguments and an error if any arg is malformed.
func (p *Parser) ParseBuildArgs(buildArgs []string) (map[string]string, error) {
	if len(buildArgs) == 0 {
		return nil, nil
	}
	return p.ParseKeyValuePairs(buildArgs)
}

// ParseVariables parses variable flags in key=value format.
// Returns a map of variables and an error if any variable is malformed.
func (p *Parser) ParseVariables(vars []string) (map[string]string, error) {
	if len(vars) == 0 {
		return nil, nil
	}
	return p.ParseKeyValuePairs(vars)
}

// ValidateKeyValueFormat checks if a string is in key=value format without parsing.
// Returns true if the format is valid, false otherwise.
func ValidateKeyValueFormat(pair string) bool {
	parts := strings.SplitN(pair, "=", 2)
	if len(parts) != 2 {
		return false
	}
	return strings.TrimSpace(parts[0]) != ""
}
