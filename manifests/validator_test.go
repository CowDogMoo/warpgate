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

package manifests

import (
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDigestFiles(t *testing.T) {
	now := time.Now()
	validDigest, _ := digest.Parse("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	tests := []struct {
		name        string
		digestFiles []DigestFile
		opts        ValidationOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid digest files",
			digestFiles: []DigestFile{
				{
					Path:         "/tmp/digest-test-amd64.txt",
					ImageName:    "test",
					Architecture: "amd64",
					Digest:       validDigest,
					ModTime:      now,
				},
				{
					Path:         "/tmp/digest-test-arm64.txt",
					ImageName:    "test",
					Architecture: "arm64",
					Digest:       validDigest,
					ModTime:      now,
				},
			},
			opts: ValidationOptions{
				ImageName: "test",
				MaxAge:    0,
			},
			expectError: false,
		},
		{
			name: "mismatched image name",
			digestFiles: []DigestFile{
				{
					Path:         "/tmp/digest-test-amd64.txt",
					ImageName:    "wrong-name",
					Architecture: "amd64",
					Digest:       validDigest,
					ModTime:      now,
				},
			},
			opts: ValidationOptions{
				ImageName: "test",
				MaxAge:    0,
			},
			expectError: true,
			errorMsg:    "incorrect image name",
		},
		{
			name: "file too old",
			digestFiles: []DigestFile{
				{
					Path:         "/tmp/digest-test-amd64.txt",
					ImageName:    "test",
					Architecture: "amd64",
					Digest:       validDigest,
					ModTime:      now.Add(-2 * time.Hour),
				},
			},
			opts: ValidationOptions{
				ImageName: "test",
				MaxAge:    1 * time.Hour,
			},
			expectError: true,
			errorMsg:    "too old",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate digest files
			err := ValidateDigestFiles(tt.digestFiles, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestFilterArchitectures(t *testing.T) {
	tests := []struct {
		name        string
		digestFiles []DigestFile
		opts        FilterOptions
		expectedLen int
		expectError bool
		errorMsg    string
	}{
		{
			name: "no filter - return all",
			digestFiles: []DigestFile{
				{Architecture: "amd64"},
				{Architecture: "arm64"},
				{Architecture: "arm-v7"},
			},
			opts: FilterOptions{
				RequiredArchitectures: nil,
				BestEffort:            false,
			},
			expectedLen: 3,
			expectError: false,
		},
		{
			name: "filter to specific architectures",
			digestFiles: []DigestFile{
				{Architecture: "amd64"},
				{Architecture: "arm64"},
				{Architecture: "arm-v7"},
			},
			opts: FilterOptions{
				RequiredArchitectures: []string{"amd64", "arm64"},
				BestEffort:            false,
			},
			expectedLen: 2,
			expectError: false,
		},
		{
			name: "missing required arch - strict mode",
			digestFiles: []DigestFile{
				{Architecture: "amd64"},
			},
			opts: FilterOptions{
				RequiredArchitectures: []string{"amd64", "arm64"},
				BestEffort:            false,
			},
			expectedLen: 0,
			expectError: true,
			errorMsg:    "missing required architectures",
		},
		{
			name: "missing required arch - best effort mode",
			digestFiles: []DigestFile{
				{Architecture: "amd64"},
			},
			opts: FilterOptions{
				RequiredArchitectures: []string{"amd64", "arm64"},
				BestEffort:            true,
			},
			expectedLen: 1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Filter architectures
			filtered, err := FilterArchitectures(tt.digestFiles, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedLen, len(filtered))
		})
	}
}
