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

package cli

import (
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/templates"
)

// Validator validates CLI input before passing to business logic.
type Validator struct {
	parser        *Parser
	pathValidator *templates.PathValidator
}

// NewValidator creates a new CLI validator.
func NewValidator() *Validator {
	return &Validator{
		parser:        NewParser(),
		pathValidator: templates.NewPathValidator(),
	}
}

// ValidateBuildOptions validates build command options for correctness and consistency.
func (v *Validator) ValidateBuildOptions(opts BuildCLIOptions) error {
	if err := v.validateKeyValueFormats(opts); err != nil {
		return err
	}

	if err := v.validateOptionDependencies(opts); err != nil {
		return err
	}

	if err := v.validateInputSources(opts); err != nil {
		return err
	}

	return nil
}

// validateKeyValueFormats validates all key=value format options.
func (v *Validator) validateKeyValueFormats(opts BuildCLIOptions) error {
	// Validate labels format
	for _, label := range opts.Labels {
		if !ValidateKeyValueFormat(label) {
			return fmt.Errorf("invalid label format: %s (expected key=value)", label)
		}
	}

	// Validate build-args format
	for _, arg := range opts.BuildArgs {
		if !ValidateKeyValueFormat(arg) {
			return fmt.Errorf("invalid build-arg format: %s (expected key=value)", arg)
		}
	}

	// Validate variable format
	for _, variable := range opts.Variables {
		if !ValidateKeyValueFormat(variable) {
			return fmt.Errorf("invalid variable format: %s (expected key=value)", variable)
		}
	}

	return nil
}

// validateOptionDependencies validates that dependent options are correctly specified.
func (v *Validator) validateOptionDependencies(opts BuildCLIOptions) error {
	if opts.Push && opts.Registry == "" {
		return fmt.Errorf("--push requires --registry to be specified")
	}

	if opts.SaveDigests && !opts.Push {
		return fmt.Errorf("--save-digests requires --push to be enabled")
	}

	return nil
}

// validateInputSources validates that only one input source is specified (mutually exclusive).
func (v *Validator) validateInputSources(opts BuildCLIOptions) error {
	inputSources := 0
	if opts.Template != "" {
		inputSources++
	}
	if opts.FromGit != "" {
		inputSources++
	}
	if opts.ConfigFile != "" {
		inputSources++
	}
	if inputSources > 1 {
		return fmt.Errorf("only one of --template, --from-git, or config file can be specified")
	}

	return nil
}

// ValidateTemplateAddOptions validates template add command options.
func (v *Validator) ValidateTemplateAddOptions(name, urlOrPath string) error {
	if urlOrPath == "" {
		return fmt.Errorf("URL or path is required")
	}

	// If name is provided, urlOrPath must be a git URL
	if name != "" && !v.pathValidator.IsGitURL(urlOrPath) {
		return fmt.Errorf("when providing a name, the URL must be a git URL (not a local path)")
	}

	return nil
}

// ValidateConfigSetOptions validates config set command options.
func (v *Validator) ValidateConfigSetOptions(key, value string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}

	if value == "" {
		return fmt.Errorf("value is required")
	}

	// Validate key format (dot notation for nested keys)
	if !isValidConfigKey(key) {
		return fmt.Errorf("invalid config key format: %s (use dot notation like log.level)", key)
	}

	return nil
}

// isValidConfigKey checks if a config key is in valid format.
func isValidConfigKey(key string) bool {
	if key == "" {
		return false
	}

	// Must not start or end with dot
	if strings.HasPrefix(key, ".") || strings.HasSuffix(key, ".") {
		return false
	}

	// Must not have consecutive dots
	if strings.Contains(key, "..") {
		return false
	}

	return true
}
