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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildErrorError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  BuildError
		want string
	}{
		{
			name: "with remediation",
			err: BuildError{
				Message:     "build failed",
				Remediation: "try again",
			},
			want: "build failed\n\nRemediation: try again",
		},
		{
			name: "without remediation",
			err: BuildError{
				Message: "build failed",
			},
			want: "build failed",
		},
		{
			name: "empty message with remediation",
			err: BuildError{
				Remediation: "fix it",
			},
			want: "\n\nRemediation: fix it",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.err.Error())
		})
	}
}

func TestBuildErrorUnwrap(t *testing.T) {
	t.Parallel()

	t.Run("with cause", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("root cause")
		be := &BuildError{Message: "wrapper", Cause: cause}
		assert.Equal(t, cause, be.Unwrap())
		assert.True(t, errors.Is(be, cause))
	})

	t.Run("without cause", func(t *testing.T) {
		t.Parallel()
		be := &BuildError{Message: "no cause"}
		assert.Nil(t, be.Unwrap())
	})
}

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errMsg  string
		pattern errorPattern
		want    bool
	}{
		{
			name:   "anyPatterns match first",
			errMsg: "ResourceAlreadyExistsException: foo",
			pattern: errorPattern{
				anyPatterns: []string{"ResourceAlreadyExistsException", "already exists"},
			},
			want: true,
		},
		{
			name:   "anyPatterns match second",
			errMsg: "the resource already exists",
			pattern: errorPattern{
				anyPatterns: []string{"ResourceAlreadyExistsException", "already exists"},
			},
			want: true,
		},
		{
			name:   "anyPatterns no match",
			errMsg: "something else entirely",
			pattern: errorPattern{
				anyPatterns: []string{"ResourceAlreadyExistsException", "already exists"},
			},
			want: false,
		},
		{
			name:   "required patterns all match",
			errMsg: "ami-12345 not found in region",
			pattern: errorPattern{
				patterns:    []string{"ami-"},
				anyPatterns: []string{"not found", "does not exist"},
			},
			want: true,
		},
		{
			name:   "required pattern missing",
			errMsg: "resource not found",
			pattern: errorPattern{
				patterns:    []string{"ami-"},
				anyPatterns: []string{"not found", "does not exist"},
			},
			want: false,
		},
		{
			name:   "required patterns match but anyPatterns don't",
			errMsg: "ami-12345 is available",
			pattern: errorPattern{
				patterns:    []string{"ami-"},
				anyPatterns: []string{"not found", "does not exist"},
			},
			want: false,
		},
		{
			name:   "empty patterns with anyPatterns",
			errMsg: "pipeline failed",
			pattern: errorPattern{
				patterns:    []string{"pipeline", "failed"},
				anyPatterns: nil,
			},
			want: true,
		},
		{
			name:    "empty pattern matches everything",
			errMsg:  "any message",
			pattern: errorPattern{},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, matchesPattern(tc.errMsg, tc.pattern))
		})
	}
}

func TestWrapWithRemediation(t *testing.T) {
	t.Parallel()

	t.Run("nil error returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, WrapWithRemediation(nil, "context"))
	})

	t.Run("matching error returns BuildError", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("ResourceAlreadyExistsException: component already exists")
		wrapped := WrapWithRemediation(err, "creating component")
		require.NotNil(t, wrapped)

		var be *BuildError
		require.True(t, errors.As(wrapped, &be))
		assert.Contains(t, be.Message, "creating component")
		assert.Contains(t, be.Message, "resource already exists")
		assert.NotEmpty(t, be.Remediation)
		assert.Equal(t, err, be.Unwrap())
	})

	t.Run("access denied error returns BuildError", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("AccessDenied: user is not authorized")
		wrapped := WrapWithRemediation(err, "building AMI")

		var be *BuildError
		require.True(t, errors.As(wrapped, &be))
		assert.Contains(t, be.Message, "permission denied")
	})

	t.Run("non-matching error wraps normally", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("some random error")
		wrapped := WrapWithRemediation(err, "doing stuff")
		assert.Contains(t, wrapped.Error(), "doing stuff")
		assert.Contains(t, wrapped.Error(), "some random error")

		var be *BuildError
		assert.False(t, errors.As(wrapped, &be))
	})
}

func TestValidatePrerequisites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		region          string
		instanceProfile string
		parentImage     string
		wantErr         bool
		wantMsgParts    []string
	}{
		{
			name:            "all valid",
			region:          "us-east-1",
			instanceProfile: "my-profile",
			parentImage:     "ami-12345",
			wantErr:         false,
		},
		{
			name:            "missing region",
			region:          "",
			instanceProfile: "my-profile",
			parentImage:     "ami-12345",
			wantErr:         true,
			wantMsgParts:    []string{"region"},
		},
		{
			name:            "missing instance profile",
			region:          "us-east-1",
			instanceProfile: "",
			parentImage:     "ami-12345",
			wantErr:         true,
			wantMsgParts:    []string{"Instance profile"},
		},
		{
			name:            "missing parent image",
			region:          "us-east-1",
			instanceProfile: "my-profile",
			parentImage:     "",
			wantErr:         true,
			wantMsgParts:    []string{"Parent image"},
		},
		{
			name:            "all missing",
			region:          "",
			instanceProfile: "",
			parentImage:     "",
			wantErr:         true,
			wantMsgParts:    []string{"region", "Instance profile", "Parent image"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePrerequisites(tc.region, tc.instanceProfile, tc.parentImage)
			if tc.wantErr {
				require.Error(t, err)
				var be *BuildError
				require.True(t, errors.As(err, &be))
				assert.Equal(t, "Build prerequisites not met", be.Message)
				for _, part := range tc.wantMsgParts {
					assert.Contains(t, be.Remediation, part)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
