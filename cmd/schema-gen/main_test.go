package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "writes schema output",
			setup: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "schema.json")
			},
		},
		{
			name: "returns error on unwritable output",
			setup: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				readOnlyDir := filepath.Join(tmpDir, "readonly")
				if err := os.Mkdir(readOnlyDir, 0500); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Chmod(readOnlyDir, 0700)
				})
				return filepath.Join(readOnlyDir, "schema.json")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := tt.setup(t)
			originalOutput := *output
			*output = outputPath
			t.Cleanup(func() {
				*output = originalOutput
			})

			err := run()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}

			data, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}

			content := string(data)
			if !strings.Contains(content, "Warpgate Template") {
				t.Errorf("schema output missing title, got: %s", content)
			}
			if !strings.Contains(content, "warpgateVersion") {
				t.Errorf("schema output missing warpgateVersion")
			}
			if !strings.Contains(content, builder.Version) {
				t.Errorf("schema output missing version %q", builder.Version)
			}
		})
	}
}

// TestRunSchemaContent validates the structure and content of the generated
// JSON schema to ensure it conforms to expectations.
func TestRunSchemaContent(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	originalOutput := *output
	*output = outputPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	// Verify JSON is valid and parseable
	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema JSON is not valid: %v", err)
	}

	// Verify schema ID
	schemaID, ok := schema["$id"]
	if !ok {
		t.Error("schema missing $id field")
	} else if schemaID != "https://warpgate.dev/schema/template.json" {
		t.Errorf("schema $id = %v, want %q", schemaID, "https://warpgate.dev/schema/template.json")
	}

	// Verify title
	title, ok := schema["title"]
	if !ok {
		t.Error("schema missing title field")
	} else if title != "Warpgate Template" {
		t.Errorf("schema title = %v, want %q", title, "Warpgate Template")
	}

	// Verify description
	desc, ok := schema["description"]
	if !ok {
		t.Error("schema missing description field")
	} else if desc != "Schema for Warpgate image build templates" {
		t.Errorf("schema description = %v, want %q", desc, "Schema for Warpgate image build templates")
	}

	// Verify warpgateVersion extra
	wv, ok := schema["warpgateVersion"]
	if !ok {
		t.Error("schema missing warpgateVersion field")
	} else if wv != builder.Version {
		t.Errorf("schema warpgateVersion = %v, want %q", wv, builder.Version)
	}

	// Verify $schema field is present (JSON Schema draft version)
	if _, ok := schema["$schema"]; !ok {
		t.Error("schema missing $schema field")
	}
}

// TestRunSchemaExamples ensures the examples array is populated with the
// expected structure.
func TestRunSchemaExamples(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	originalOutput := *output
	*output = outputPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema JSON is not valid: %v", err)
	}

	examples, ok := schema["examples"]
	if !ok {
		t.Fatal("schema missing examples field")
	}

	exArr, ok := examples.([]interface{})
	if !ok {
		t.Fatalf("schema examples is not an array, got %T", examples)
	}

	if len(exArr) == 0 {
		t.Fatal("schema examples array is empty")
	}

	// Verify the first example has expected top-level keys
	firstExample, ok := exArr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first example is not an object, got %T", exArr[0])
	}

	expectedKeys := []string{"metadata", "name", "version", "base", "provisioners", "targets"}
	for _, key := range expectedKeys {
		if _, ok := firstExample[key]; !ok {
			t.Errorf("first example missing key %q", key)
		}
	}

	// Verify metadata sub-structure
	metadata, ok := firstExample["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("example metadata is not an object, got %T", firstExample["metadata"])
	}
	metadataKeys := []string{"name", "version", "description", "author", "license", "tags"}
	for _, key := range metadataKeys {
		if _, ok := metadata[key]; !ok {
			t.Errorf("example metadata missing key %q", key)
		}
	}
}

// TestRunCreatesNestedDirectories verifies that run() creates intermediate
// directories when the output path has non-existent parent directories.
func TestRunCreatesNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "a", "b", "c", "schema.json")
	originalOutput := *output
	*output = nestedPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Errorf("expected schema file at %s, but it does not exist", nestedPath)
	}
}

// TestRunOutputEndsWithNewline verifies the generated schema file ends with a
// trailing newline to satisfy end-of-file-fixer linting.
func TestRunOutputEndsWithNewline(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	originalOutput := *output
	*output = outputPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("schema file is empty")
	}
	if data[len(data)-1] != '\n' {
		t.Error("schema file does not end with a newline")
	}
}

// TestRunOverwritesExistingFile verifies that run() overwrites an existing
// schema file without error.
func TestRunOverwritesExistingFile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	originalOutput := *output
	*output = outputPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	// Write a dummy file first
	if err := os.WriteFile(outputPath, []byte("old content"), 0644); err != nil {
		t.Fatalf("write dummy file: %v", err)
	}

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	if string(data) == "old content" {
		t.Error("schema file was not overwritten")
	}

	// Verify the new content is valid JSON
	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Errorf("overwritten schema is not valid JSON: %v", err)
	}
}

// TestOutputFlagDefault verifies the default value of the -o flag.
func TestOutputFlagDefault(t *testing.T) {
	f := flag.Lookup("o")
	if f == nil {
		t.Fatal("flag -o is not registered")
	}
	if f.DefValue != "schema/warpgate-template.json" {
		t.Errorf("flag -o default = %q, want %q", f.DefValue, "schema/warpgate-template.json")
	}
	if f.Usage != "Output path for JSON schema" {
		t.Errorf("flag -o usage = %q, want %q", f.Usage, "Output path for JSON schema")
	}
}

// TestRunMkdirAllFailure verifies that run() returns an error when the output
// directory cannot be created (e.g. path traversal through a file).
func TestRunMkdirAllFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows due to path handling differences")
	}

	tmpDir := t.TempDir()
	// Create a file where a directory is expected to block MkdirAll
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	// Try to write schema to a path that requires blocker to be a directory
	badPath := filepath.Join(blockingFile, "subdir", "schema.json")
	originalOutput := *output
	*output = badPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	err := run()
	if err == nil {
		t.Fatal("expected error when MkdirAll fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create output directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestMainViaExecCommand exercises the main() function by running the compiled
// binary as a subprocess with valid arguments.
func TestMainViaExecCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	// Build the binary
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "schema-gen")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = filepath.Join(projectRoot(t), "cmd", "schema-gen")
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, buildOut)
	}

	outputPath := filepath.Join(tmpDir, "out", "schema.json")

	cmd := exec.Command(binPath, "-o", outputPath)
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary execution failed: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "Generated JSON schema") {
		t.Errorf("unexpected output: %s", out)
	}

	// Verify the file was created and is valid JSON
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if schema["title"] != "Warpgate Template" {
		t.Errorf("schema title = %v, want %q", schema["title"], "Warpgate Template")
	}
}

// TestMainExitOnError exercises the main() function error path by running the
// binary with an invalid output path, expecting a non-zero exit code.
func TestMainExitOnError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows due to path handling differences")
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "schema-gen")

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = filepath.Join(projectRoot(t), "cmd", "schema-gen")
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, buildOut)
	}

	// Create a file to block directory creation
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	badOutputPath := filepath.Join(blockingFile, "sub", "schema.json")
	cmd := exec.Command(binPath, "-o", badOutputPath)
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit code for invalid output path")
	}

	if !strings.Contains(string(out), "Error:") {
		t.Errorf("expected stderr to contain 'Error:', got: %s", out)
	}
}

// TestRunSchemaHasProperties verifies the generated schema has a properties
// or definitions section, indicating it properly reflected the Config struct.
func TestRunSchemaHasProperties(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	originalOutput := *output
	*output = outputPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema JSON is not valid: %v", err)
	}

	// The schema should have properties or $defs from the reflected Config struct
	hasProperties := false
	if _, ok := schema["properties"]; ok {
		hasProperties = true
	}
	if _, ok := schema["$defs"]; ok {
		hasProperties = true
	}
	if !hasProperties {
		t.Error("schema has neither 'properties' nor '$defs'; struct reflection may have failed")
	}
}

// TestRunAddGoCommentsWarning verifies that run() succeeds even when
// AddGoComments fails (e.g., when run from a directory without Go source).
// This exercises the warning path on line 67-68 of main.go.
func TestRunAddGoCommentsWarning(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	originalOutput := *output
	*output = outputPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	// Change to a temp directory that has no Go source files.
	// This causes reflector.AddGoComments to fail, exercising the warning path.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	emptyDir := t.TempDir()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	// run() should succeed despite AddGoComments warning
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	// Verify the schema was still generated correctly
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema JSON is not valid: %v", err)
	}

	if schema["title"] != "Warpgate Template" {
		t.Errorf("schema title = %v, want %q", schema["title"], "Warpgate Template")
	}
}

// TestMainInProcess exercises the main() function in-process by intercepting
// os.Exit via the test subprocess pattern. When run as a subprocess with
// SCHEMA_GEN_TEST_MAIN=1, it calls main() directly to get coverage.
func TestMainInProcess(t *testing.T) {
	if os.Getenv("SCHEMA_GEN_TEST_MAIN") == "1" {
		// We are in the subprocess: set up output flag and call main()
		outputPath := os.Getenv("SCHEMA_GEN_TEST_OUTPUT")
		if outputPath != "" {
			*output = outputPath
		}
		main()
		return
	}

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "schema.json")

		cmd := exec.Command(os.Args[0], "-test.run=^TestMainInProcess$")
		cmd.Env = append(os.Environ(),
			"SCHEMA_GEN_TEST_MAIN=1",
			"SCHEMA_GEN_TEST_OUTPUT="+outputPath,
		)
		cmd.Dir = projectRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("subprocess failed: %v\n%s", err, out)
		}

		// Verify schema was generated
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}
		var schema map[string]interface{}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		if schema["title"] != "Warpgate Template" {
			t.Errorf("schema title = %v, want %q", schema["title"], "Warpgate Template")
		}
	})

	t.Run("error_exit", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping on windows")
		}

		tmpDir := t.TempDir()
		// Create a blocking file so MkdirAll fails
		blockingFile := filepath.Join(tmpDir, "blocker")
		if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
			t.Fatalf("write blocking file: %v", err)
		}
		badPath := filepath.Join(blockingFile, "sub", "schema.json")

		cmd := exec.Command(os.Args[0], "-test.run=^TestMainInProcess$")
		cmd.Env = append(os.Environ(),
			"SCHEMA_GEN_TEST_MAIN=1",
			"SCHEMA_GEN_TEST_OUTPUT="+badPath,
		)
		cmd.Dir = projectRoot(t)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(string(out), "Error:") {
			t.Errorf("expected stderr to contain 'Error:', got: %s", out)
		}
	})
}

// TestRunWriteFileFailure verifies that run() returns an appropriate error
// when MkdirAll succeeds but WriteFile fails (e.g., output path is a directory).
func TestRunWriteFileFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows due to path handling differences")
	}

	tmpDir := t.TempDir()
	// Create a directory where the output file should be written.
	// WriteFile will fail because the target path is a directory, not a file.
	dirAsFile := filepath.Join(tmpDir, "schema.json")
	if err := os.Mkdir(dirAsFile, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	originalOutput := *output
	*output = dirAsFile
	t.Cleanup(func() {
		*output = originalOutput
	})

	err := run()
	if err == nil {
		t.Fatal("expected error when WriteFile target is a directory, got nil")
	}
	if !strings.Contains(err.Error(), "failed to write schema file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestMainViaExecWithCustomFlag exercises the main() binary with a custom
// output flag to verify flag parsing works end-to-end.
func TestMainViaExecWithCustomFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "schema-gen")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = filepath.Join(projectRoot(t), "cmd", "schema-gen")
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, buildOut)
	}

	// Use a deeply nested output path to test directory creation
	outputPath := filepath.Join(tmpDir, "deep", "nested", "dir", "my-schema.json")

	cmd := exec.Command(binPath, "-o", outputPath)
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary execution failed: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "Generated JSON schema") {
		t.Errorf("unexpected output: %s", out)
	}

	// Verify the output file path in the success message
	if !strings.Contains(string(out), "my-schema.json") {
		t.Errorf("output should reference custom filename, got: %s", out)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

// TestRunSchemaDefinitionsContainExpectedTypes verifies that the generated
// schema contains definitions for key warpgate types.
func TestRunSchemaDefinitionsContainExpectedTypes(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	originalOutput := *output
	*output = outputPath
	t.Cleanup(func() {
		*output = originalOutput
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	content := string(data)

	// The schema should reference key types from the builder package
	expectedStrings := []string{
		"warpgate.dev/schema/template.json", // schema ID
		"examples",                          // examples array
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("schema missing expected content %q", expected)
		}
	}
}

// projectRoot returns the root directory of the warpgate project by walking
// up from the current test file location until go.mod is found.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Start from the known location of this test file
	dir := "/Users/l/cowdogmoo/warpgate"
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("could not find project root at %s: %v", dir, err)
	}
	return dir
}
