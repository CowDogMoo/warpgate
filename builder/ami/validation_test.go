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
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
)

func TestValidationResultAddError(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true}
	result.AddError("error: %s", "test")

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "error: test", result.Errors[0])
}

func TestValidationResultAddWarning(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true}
	result.AddWarning("warning: %d", 42)

	assert.True(t, result.Valid)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "warning: 42", result.Warnings[0])
}

func TestValidationResultAddInfo(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true}
	result.AddInfo("info: %s %s", "hello", "world")

	assert.Len(t, result.Info, 1)
	assert.Equal(t, "info: hello world", result.Info[0])
}

func TestValidationResultString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     ValidationResult
		wantParts  []string
		wantAbsent []string
	}{
		{
			name:      "passed with no messages",
			result:    ValidationResult{Valid: true},
			wantParts: []string{"Validation PASSED"},
		},
		{
			name: "failed with errors",
			result: ValidationResult{
				Valid:  false,
				Errors: []string{"missing region", "missing profile"},
			},
			wantParts: []string{"Validation FAILED", "Errors:", "missing region", "missing profile"},
		},
		{
			name: "passed with warnings and info",
			result: ValidationResult{
				Valid:    true,
				Warnings: []string{"instance type not set"},
				Info:     []string{"template: my-template"},
			},
			wantParts:  []string{"Validation PASSED", "Warnings:", "instance type not set", "Info:", "template: my-template"},
			wantAbsent: []string{"Errors:"},
		},
		{
			name: "all sections present",
			result: ValidationResult{
				Valid:    false,
				Errors:   []string{"err1"},
				Warnings: []string{"warn1"},
				Info:     []string{"info1"},
			},
			wantParts: []string{"FAILED", "Errors:", "err1", "Warnings:", "warn1", "Info:", "info1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			output := tc.result.String()
			for _, part := range tc.wantParts {
				assert.Contains(t, output, part)
			}
			for _, absent := range tc.wantAbsent {
				assert.NotContains(t, output, absent)
			}
		})
	}
}

func TestNewValidator(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)
	assert.NotNil(t, v)
	assert.Nil(t, v.clients)
}

func TestValidateProvisioners(t *testing.T) {
	t.Parallel()

	v := NewValidator(nil)

	tests := []struct {
		name          string
		provisioners  []builder.Provisioner
		wantErrors    int
		wantWarnings  int
		wantInfoCount int
	}{
		{
			name:         "no provisioners warns",
			provisioners: nil,
			wantWarnings: 1,
		},
		{
			name: "valid shell provisioner",
			provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo hello"}},
			},
			wantInfoCount: 2, // count + detail
		},
		{
			name: "shell without inline errors",
			provisioners: []builder.Provisioner{
				{Type: "shell"},
			},
			wantErrors:    1,
			wantInfoCount: 1, // count only
		},
		{
			name: "valid ansible provisioner",
			provisioners: []builder.Provisioner{
				{Type: "ansible", PlaybookPath: "/path/to/playbook.yml"},
			},
			wantInfoCount: 2,
		},
		{
			name: "ansible without playbook errors",
			provisioners: []builder.Provisioner{
				{Type: "ansible"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "valid script provisioner",
			provisioners: []builder.Provisioner{
				{Type: "script", Scripts: []string{"setup.sh"}},
			},
			wantInfoCount: 2,
		},
		{
			name: "script without scripts errors",
			provisioners: []builder.Provisioner{
				{Type: "script"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "valid powershell provisioner",
			provisioners: []builder.Provisioner{
				{Type: "powershell", PSScripts: []string{"setup.ps1"}},
			},
			wantInfoCount: 2,
		},
		{
			name: "powershell without scripts errors",
			provisioners: []builder.Provisioner{
				{Type: "powershell"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "unknown provisioner type errors",
			provisioners: []builder.Provisioner{
				{Type: "terraform"},
			},
			wantErrors:    1,
			wantInfoCount: 1,
		},
		{
			name: "multiple provisioners mixed",
			provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo hello"}},
				{Type: "ansible"},
			},
			wantErrors:    1,
			wantInfoCount: 2, // count + 1 valid shell detail
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := &ValidationResult{Valid: true}
			cfg := builder.Config{Provisioners: tc.provisioners}
			v.validateProvisioners(result, cfg)

			assert.Len(t, result.Errors, tc.wantErrors)
			assert.Len(t, result.Warnings, tc.wantWarnings)
			assert.Len(t, result.Info, tc.wantInfoCount)
		})
	}
}
