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

package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseVariables parses variables from CLI flags and var files
// CLI flags take precedence over var files
func ParseVariables(vars, varFiles []string) (map[string]string, error) {
	variables := make(map[string]string)

	// First, load variables from var files (lower precedence)
	for _, varFile := range varFiles {
		fileVars, err := LoadVariablesFromFile(varFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load var file %s: %w", varFile, err)
		}
		// Merge file variables (later files override earlier ones)
		for k, v := range fileVars {
			variables[k] = v
		}
	}

	// Then, apply CLI variables (higher precedence)
	for _, v := range vars {
		key, value, err := ParseKeyValue(v)
		if err != nil {
			return nil, fmt.Errorf("invalid variable format %q: %w", v, err)
		}
		variables[key] = value
	}

	return variables, nil
}

// LoadVariablesFromFile loads variables from a YAML file
func LoadVariablesFromFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var variables map[string]string
	if err := yaml.Unmarshal(data, &variables); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return variables, nil
}

// ParseKeyValue parses a key=value string
func ParseKeyValue(s string) (string, string, error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected format: key=value")
	}
	if parts[0] == "" {
		return "", "", fmt.Errorf("key cannot be empty")
	}
	return parts[0], parts[1], nil
}
