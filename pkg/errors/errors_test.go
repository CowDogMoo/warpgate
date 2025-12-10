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

package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestWrap(t *testing.T) {
	baseErr := errors.New("something went wrong")

	tests := []struct {
		name           string
		action         string
		detail         string
		err            error
		expectedPrefix string
		shouldContain  []string
	}{
		{
			name:           "wrap with action only",
			action:         "create builder",
			detail:         "",
			err:            baseErr,
			expectedPrefix: "failed to create builder:",
			shouldContain:  []string{"failed to create builder:", "something went wrong"},
		},
		{
			name:           "wrap with action and detail",
			action:         "parse config",
			detail:         "/path/to/config.yaml",
			err:            baseErr,
			expectedPrefix: "failed to parse config (/path/to/config.yaml):",
			shouldContain:  []string{"failed to parse config", "/path/to/config.yaml", "something went wrong"},
		},
		{
			name:           "wrap nil error returns nil",
			action:         "do something",
			detail:         "details",
			err:            nil,
			expectedPrefix: "",
			shouldContain:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Wrap(tt.action, tt.detail, tt.err)

			if tt.err == nil {
				if result != nil {
					t.Errorf("Expected nil error, got: %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected wrapped error, got nil")
			}

			errMsg := result.Error()

			// Check that the error message starts with expected prefix
			if !strings.HasPrefix(errMsg, tt.expectedPrefix) {
				t.Errorf("Expected error to start with %q, got: %q", tt.expectedPrefix, errMsg)
			}

			// Check that all expected strings are contained
			for _, expected := range tt.shouldContain {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Expected error to contain %q, got: %q", expected, errMsg)
				}
			}

			// Verify error unwrapping works
			if !errors.Is(result, baseErr) {
				t.Error("Expected wrapped error to unwrap to original error")
			}
		})
	}
}
