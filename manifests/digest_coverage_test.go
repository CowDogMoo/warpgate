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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveDigestToFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		imageName string
		arch      string
		digestStr string
		setupDir  func(t *testing.T) string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "saves valid digest",
			imageName: "attack-box",
			arch:      "amd64",
			digestStr: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: false,
		},
		{
			name:      "creates subdirectory if needed",
			imageName: "myimage",
			arch:      "arm64",
			digestStr: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			setupDir: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nested", "subdir")
			},
			wantErr: false,
		},
		{
			name:      "fails with empty digest",
			imageName: "test",
			arch:      "amd64",
			digestStr: "",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: true,
			errMsg:  "empty digest provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dir := tt.setupDir(t)

			err := SaveDigestToFile(ctx, tt.imageName, tt.arch, tt.digestStr, dir)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)

			// Verify the file was written
			expectedFile := filepath.Join(dir, "digest-"+tt.imageName+"-"+tt.arch+".txt")
			content, err := os.ReadFile(expectedFile)
			require.NoError(t, err)
			assert.Equal(t, tt.digestStr, string(content))
		})
	}
}

func TestSaveDigestToFile_ThenParse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()

	digestStr := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := SaveDigestToFile(ctx, "roundtrip", "arm64", digestStr, dir)
	require.NoError(t, err)

	// Now parse the saved file
	filePath := filepath.Join(dir, "digest-roundtrip-arm64.txt")
	df, err := ParseDigestFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "roundtrip", df.ImageName)
	assert.Equal(t, "arm64", df.Architecture)
	assert.Equal(t, digestStr, df.Digest.String())
}

func TestDiscoverDigestFiles_WithInvalidFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Write a valid digest file
	validContent := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "digest-myimage-amd64.txt"),
		[]byte(validContent), 0644,
	))

	// Write an invalid digest file (bad digest content) that matches the glob
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "digest-myimage-arm64.txt"),
		[]byte("not-a-valid-digest"), 0644,
	))

	digestFiles, err := DiscoverDigestFiles(context.Background(), DiscoveryOptions{
		ImageName: "myimage",
		Directory: tmpDir,
	})

	require.NoError(t, err)
	// The invalid file should be skipped, only the valid one returned
	assert.Len(t, digestFiles, 1)
	assert.Equal(t, "amd64", digestFiles[0].Architecture)
}

func TestParseDigestFile_UnknownArchitecture(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// A file with a valid prefix/suffix pattern but unknown architecture
	filePath := filepath.Join(tmpDir, "digest-myimage-mips64.txt")
	require.NoError(t, os.WriteFile(filePath, []byte(
		"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	), 0644))

	_, err := ParseDigestFile(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown architecture")
}

func TestParseDigestFile_AllKnownArchitectures(t *testing.T) {
	t.Parallel()

	arches := []string{"amd64", "arm64", "arm-v7", "arm-v6", "arm-v8", "ppc64le", "s390x", "386", "riscv64"}

	for _, arch := range arches {
		t.Run(arch, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "digest-test-"+arch+".txt")
			validDigest := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
			require.NoError(t, os.WriteFile(filePath, []byte(validDigest), 0644))

			df, err := ParseDigestFile(filePath)
			require.NoError(t, err)
			assert.Equal(t, arch, df.Architecture)
			assert.Equal(t, "test", df.ImageName)
		})
	}
}

func TestParseDigestFile_WhitespaceInDigest(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "digest-myimage-amd64.txt")
	// Digest with trailing newline
	require.NoError(t, os.WriteFile(filePath, []byte(
		"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef\n",
	), 0644))

	df, err := ParseDigestFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "amd64", df.Architecture)
	assert.Equal(t, "myimage", df.ImageName)
}

func TestParseDigestFile_NoDigestPrefix(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "notdigest-myimage-amd64.txt")
	require.NoError(t, os.WriteFile(filePath, []byte(
		"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	), 0644))

	_, err := ParseDigestFile(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filename format")
}
