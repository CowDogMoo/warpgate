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

// Package main generates a JSON schema from the warpgate template configuration structure.
// The generated schema enables IDE autocompletion and validation for template YAML files.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/cowdogmoo/warpgate/pkg/config"
	"github.com/invopop/jsonschema"
)

var (
	output = flag.String("o", "schema/warpgate-template.json", "Output path for JSON schema")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Generate schema from Config struct
	reflector := jsonschema.Reflector{
		// Use full type paths for clarity
		ExpandedStruct: true,
		// Don't allow additional properties by default (strict validation)
		DoNotReference: false,
		// Include descriptions from struct tags
		AllowAdditionalProperties: false,
	}

	// Extract type-level doc comments from Go source files
	// This adds descriptions to type definitions (e.g., "BaseImage specifies...")
	// Field-level descriptions (e.g., "Image is the base container...") are handled automatically by the reflector
	if err := reflector.AddGoComments("github.com/cowdogmoo/warpgate", "./"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to extract type-level comments: %v\n", err)
		// Continue anyway - we still have field-level descriptions from inline comments
	}

	schema := reflector.Reflect(&builder.Config{})

	// Add schema metadata
	schema.ID = jsonschema.ID("https://warpgate.dev/schema/template.json")
	schema.Title = "Warpgate Template"
	schema.Description = "Schema for Warpgate image build templates"
	// Note: schema.Version is the JSON Schema version (e.g., "https://json-schema.org/draft/2020-12/schema")
	// and is automatically set by the reflector. We add warpgate version to extras instead.
	if schema.Extras == nil {
		schema.Extras = make(map[string]interface{})
	}
	schema.Extras["warpgateVersion"] = builder.Version

	// Example template to help users understand the structure
	schema.Examples = []interface{}{
		map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":        "example-template",
				"version":     "1.0.0",
				"description": "Example security research template",
				"author":      "Security Team <security@example.com>",
				"license":     "MIT",
				"tags":        []string{"security", "research", "linux"},
				"requires": map[string]interface{}{
					"warpgate": ">=1.0.0",
				},
			},
			"name":    "example-image",
			"version": "1.0.0",
			"base": map[string]interface{}{
				"image": "ubuntu:22.04",
				"pull":  true,
			},
			"provisioners": []interface{}{
				map[string]interface{}{
					"type": "shell",
					"inline": []string{
						"apt-get update",
						"apt-get install -y curl git",
					},
				},
			},
			"targets": []interface{}{
				map[string]interface{}{
					"type":      "container",
					"platforms": []string{"linux/amd64", "linux/arm64"},
					"registry":  "ghcr.io/example/repo",
					"tags":      []string{"latest", "v1.0.0"},
					"push":      false,
				},
			},
		},
	}

	// Marshal to pretty JSON
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Ensure output directory exists
	dir := filepath.Dir(*output)
	if err := os.MkdirAll(dir, config.DirPermReadWriteExec); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Append newline to satisfy end-of-file-fixer
	data = append(data, '\n')

	// Write schema file
	if err := os.WriteFile(*output, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	fmt.Printf("✓ Generated JSON schema: %s\n", *output)
	return nil
}
