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
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDigestFile(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		content       string
		expectedArch  string
		expectedImage string
		expectError   bool
	}{
		{
			name:          "valid amd64 digest",
			filename:      "digest-attack-box-amd64.txt",
			content:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expectedArch:  "amd64",
			expectedImage: "attack-box",
			expectError:   false,
		},
		{
			name:          "valid arm64 digest",
			filename:      "digest-sliver-arm64.txt",
			content:       "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expectedArch:  "arm64",
			expectedImage: "sliver",
			expectError:   false,
		},
		{
			name:          "valid arm/v7 digest",
			filename:      "digest-test-arm-v7.txt",
			content:       "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
			expectedArch:  "arm-v7",
			expectedImage: "test",
			expectError:   false,
		},
		{
			name:        "invalid filename format",
			filename:    "invalid-filename.txt",
			content:     "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expectError: true,
		},
		{
			name:        "invalid digest format",
			filename:    "digest-test-amd64.txt",
			content:     "not-a-valid-digest",
			expectError: true,
		},
		{
			name:        "empty digest",
			filename:    "digest-test-amd64.txt",
			content:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, tt.filename)

			// Write test file
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			// Parse digest file
			df, err := ParseDigestFile(filePath)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedArch, df.Architecture)
			assert.Equal(t, tt.expectedImage, df.ImageName)
			assert.NotEmpty(t, df.Digest.String())
			assert.Equal(t, digest.SHA256, df.Digest.Algorithm())
		})
	}
}

func TestDiscoverDigestFiles(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string // filename -> content
		imageName     string
		expectedCount int
		expectError   bool
	}{
		{
			name: "discover multiple architecture files",
			files: map[string]string{
				"digest-attack-box-amd64.txt": "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				"digest-attack-box-arm64.txt": "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			imageName:     "attack-box",
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "filter by image name",
			files: map[string]string{
				"digest-attack-box-amd64.txt": "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				"digest-sliver-arm64.txt":     "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			imageName:     "attack-box",
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "no files found",
			files:         map[string]string{},
			imageName:     "nonexistent",
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Write test files
			for filename, content := range tt.files {
				filePath := filepath.Join(tmpDir, filename)
				err := os.WriteFile(filePath, []byte(content), 0644)
				require.NoError(t, err)
			}

			// Discover digest files
			digestFiles, err := DiscoverDigestFiles(DiscoveryOptions{
				ImageName: tt.imageName,
				Directory: tmpDir,
			})

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(digestFiles))
		})
	}
}
