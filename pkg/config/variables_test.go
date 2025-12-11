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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedKey string
		expectedVal string
		expectError bool
	}{
		{
			name:        "valid key=value",
			input:       "KEY=value",
			expectedKey: "KEY",
			expectedVal: "value",
			expectError: false,
		},
		{
			name:        "value with equals sign",
			input:       "KEY=value=with=equals",
			expectedKey: "KEY",
			expectedVal: "value=with=equals",
			expectError: false,
		},
		{
			name:        "empty value",
			input:       "KEY=",
			expectedKey: "KEY",
			expectedVal: "",
			expectError: false,
		},
		{
			name:        "no equals sign",
			input:       "KEYVALUE",
			expectError: true,
		},
		{
			name:        "empty key",
			input:       "=value",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, val, err := ParseKeyValue(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedKey, key)
			assert.Equal(t, tt.expectedVal, val)
		})
	}
}

func TestLoadVariablesFromFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    map[string]string
		expectError bool
	}{
		{
			name: "valid YAML file",
			content: `KEY1: value1
KEY2: value2
KEY3: value3`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
			expectError: false,
		},
		{
			name:        "empty file",
			content:     "",
			expected:    nil, // YAML unmarshalling empty content returns nil
			expectError: false,
		},
		{
			name:        "invalid YAML",
			content:     "key: [unclosed",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "vars.yaml")

			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			vars, err := LoadVariablesFromFile(filePath)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, vars)
		})
	}
}

func TestParseVariables(t *testing.T) {
	tests := []struct {
		name        string
		vars        []string
		varFiles    []string
		fileContent map[string]string // filename -> content
		expected    map[string]string
		expectError bool
	}{
		{
			name: "CLI vars only",
			vars: []string{"KEY1=value1", "KEY2=value2"},
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expectError: false,
		},
		{
			name:     "file vars only",
			varFiles: []string{"vars.yaml"},
			fileContent: map[string]string{
				"vars.yaml": "KEY1: filevalue1\nKEY2: filevalue2",
			},
			expected: map[string]string{
				"KEY1": "filevalue1",
				"KEY2": "filevalue2",
			},
			expectError: false,
		},
		{
			name:     "CLI overrides file",
			vars:     []string{"KEY1=clivalue"},
			varFiles: []string{"vars.yaml"},
			fileContent: map[string]string{
				"vars.yaml": "KEY1: filevalue\nKEY2: filevalue2",
			},
			expected: map[string]string{
				"KEY1": "clivalue",
				"KEY2": "filevalue2",
			},
			expectError: false,
		},
		{
			name:     "multiple files - later overrides",
			varFiles: []string{"vars1.yaml", "vars2.yaml"},
			fileContent: map[string]string{
				"vars1.yaml": "KEY1: value1\nKEY2: value2",
				"vars2.yaml": "KEY1: overridden",
			},
			expected: map[string]string{
				"KEY1": "overridden",
				"KEY2": "value2",
			},
			expectError: false,
		},
		{
			name:        "invalid CLI var format",
			vars:        []string{"INVALID"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create var files
			var varFilePaths []string
			for _, filename := range tt.varFiles {
				filePath := filepath.Join(tmpDir, filename)
				content := tt.fileContent[filename]
				err := os.WriteFile(filePath, []byte(content), 0644)
				require.NoError(t, err)
				varFilePaths = append(varFilePaths, filePath)
			}

			vars, err := ParseVariables(tt.vars, varFilePaths)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, vars)
		})
	}
}
