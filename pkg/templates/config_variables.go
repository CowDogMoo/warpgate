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

package templates

import (
	"fmt"
	"os"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/pkg/errors"
	"gopkg.in/yaml.v3"
)

// ParseVariables parses and merges variables from CLI flags and YAML variable files.
// Variables are applied with the following precedence (highest to lowest):
//  1. CLI flags (--var key=value)
//  2. Var files loaded in order (later files override earlier files)
func ParseVariables(vars, varFiles []string) (map[string]string, error) {
	variables := make(map[string]string)

	// First, load variables from var files (lower precedence)
	for _, varFile := range varFiles {
		fileVars, err := LoadVariablesFromFile(varFile)
		if err != nil {
			return nil, errors.Wrap("load var file", varFile, err)
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

// LoadVariablesFromFile loads variables from a YAML file containing key-value pairs.
// The YAML file should be a flat map of strings (e.g., `KEY: value`).
func LoadVariablesFromFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap("read file", path, err)
	}

	var variables map[string]string
	if err := yaml.Unmarshal(data, &variables); err != nil {
		return nil, errors.Wrap("parse YAML", path, err)
	}

	return variables, nil
}

// ParseKeyValue parses a key=value string into separate key and value components.
// The string must contain exactly one '=' character with a non-empty key.
// Values can be empty. Returns an error if the format is invalid or the key is empty.
func ParseKeyValue(s string) (key, value string, err error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected format: key=value")
	}
	if parts[0] == "" {
		return "", "", fmt.Errorf("key cannot be empty")
	}
	return parts[0], parts[1], nil
}
