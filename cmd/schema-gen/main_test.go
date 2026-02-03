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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_GeneratesValidSchema(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "schema", "test-schema.json")

	// Override the output flag
	oldOutput := *output
	*output = outputPath
	defer func() { *output = oldOutput }()

	if err := run(); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Verify it's valid JSON
	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify schema metadata
	if title, ok := schema["title"].(string); !ok || title != "Warpgate Template" {
		t.Errorf("schema title = %v, want %q", schema["title"], "Warpgate Template")
	}

	if desc, ok := schema["description"].(string); !ok || !strings.Contains(desc, "Warpgate") {
		t.Errorf("schema description = %v, should contain 'Warpgate'", schema["description"])
	}

	if _, ok := schema["$id"]; !ok {
		t.Error("schema missing $id field")
	}

	if _, ok := schema["warpgateVersion"]; !ok {
		t.Error("schema missing warpgateVersion field")
	}

	if examples, ok := schema["examples"]; !ok || examples == nil {
		t.Error("schema missing examples")
	}

	// Verify file ends with newline
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("schema file should end with newline")
	}
}

func TestRun_CreatesOutputDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "deep", "nested", "dir", "schema.json")

	oldOutput := *output
	*output = outputPath
	defer func() { *output = oldOutput }()

	if err := run(); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output file was not created")
	}
}
