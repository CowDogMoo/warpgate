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

package builder

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuildManifest(t *testing.T) {
	config := &Config{
		Metadata: Metadata{
			Name:    "test-template",
			Version: "1.0.0",
		},
	}

	results := []BuildResult{
		{
			ImageRef:     "ghcr.io/test/image:latest",
			Digest:       "sha256:abc123",
			Architecture: "amd64",
			Platform:     "linux/amd64",
			Duration:     "2m30s",
		},
		{
			ImageRef:     "ghcr.io/test/image:latest",
			Digest:       "sha256:def456",
			Architecture: "arm64",
			Platform:     "linux/arm64",
			Duration:     "3m15s",
		},
	}

	duration := 5*time.Minute + 45*time.Second

	manifest := NewBuildManifest(config, results, duration)

	assert.Equal(t, "test-template", manifest.Template)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.Equal(t, "5m45s", manifest.Duration)
	assert.Len(t, manifest.Builds, 2)

	// Verify first build
	assert.Equal(t, "container", manifest.Builds[0].Type)
	assert.Equal(t, "linux/amd64", manifest.Builds[0].Platform)
	assert.Equal(t, "amd64", manifest.Builds[0].Architecture)
	assert.Equal(t, "sha256:abc123", manifest.Builds[0].Digest)

	// Verify second build
	assert.Equal(t, "container", manifest.Builds[1].Type)
	assert.Equal(t, "linux/arm64", manifest.Builds[1].Platform)
	assert.Equal(t, "arm64", manifest.Builds[1].Architecture)
}

func TestNewBuildManifestWithAMI(t *testing.T) {
	config := &Config{
		Metadata: Metadata{
			Name:    "ami-template",
			Version: "2.0.0",
		},
	}

	results := []BuildResult{
		{
			AMIID:    "ami-12345678",
			Region:   "us-east-1",
			Duration: "15m30s",
		},
	}

	manifest := NewBuildManifest(config, results, 15*time.Minute+30*time.Second)

	assert.Len(t, manifest.Builds, 1)
	assert.Equal(t, "ami", manifest.Builds[0].Type)
	assert.Equal(t, "ami-12345678", manifest.Builds[0].AMIID)
	assert.Equal(t, "us-east-1", manifest.Builds[0].Region)
}

func TestWriteAndReadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "build.json")

	original := &BuildManifest{
		Template:        "test-app",
		Version:         "1.2.3",
		Timestamp:       time.Now().UTC().Truncate(time.Millisecond),
		Duration:        "5m30s",
		WarpgateVersion: "dev",
		Pushed:          true,
		Builds: []ManifestBuild{
			{
				Type:         "container",
				Platform:     "linux/amd64",
				Architecture: "amd64",
				ImageRef:     "ghcr.io/org/app:latest",
				Digest:       "sha256:abc123def456",
				Duration:     "2m15s",
			},
		},
		Manifest: &ManifestRef{
			Ref:    "ghcr.io/org/app:latest",
			Digest: "sha256:manifest789",
		},
	}

	// Write manifest
	err := WriteManifest(manifestPath, original)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(manifestPath)
	require.NoError(t, err)

	// Read manifest
	loaded, err := ReadManifest(manifestPath)
	require.NoError(t, err)

	// Compare fields
	assert.Equal(t, original.Template, loaded.Template)
	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.Duration, loaded.Duration)
	assert.Equal(t, original.Pushed, loaded.Pushed)
	assert.Len(t, loaded.Builds, 1)
	assert.Equal(t, original.Builds[0].Digest, loaded.Builds[0].Digest)
	assert.Equal(t, original.Manifest.Ref, loaded.Manifest.Ref)
}

func TestWriteManifestCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "nested", "dir", "build.json")

	manifest := &BuildManifest{
		Template: "test",
		Version:  "1.0.0",
		Builds:   []ManifestBuild{},
	}

	err := WriteManifest(manifestPath, manifest)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(manifestPath)
	require.NoError(t, err)
}

func TestReadManifestNotFound(t *testing.T) {
	_, err := ReadManifest("/nonexistent/path/manifest.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read manifest file")
}

func TestReadManifestInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(manifestPath, []byte("not valid json"), 0644)
	require.NoError(t, err)

	_, err = ReadManifest(manifestPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse manifest")
}

func TestBuildManifestWithNotes(t *testing.T) {
	config := &Config{
		Metadata: Metadata{
			Name:    "test",
			Version: "1.0.0",
		},
	}

	results := []BuildResult{
		{
			ImageRef:     "test:latest",
			Architecture: "amd64",
			Platform:     "linux/amd64",
			Notes:        []string{"Warning: deprecated base image", "Build used cache"},
		},
	}

	manifest := NewBuildManifest(config, results, time.Minute)

	assert.Len(t, manifest.Builds[0].Notes, 2)
	assert.Contains(t, manifest.Builds[0].Notes, "Warning: deprecated base image")
	assert.Contains(t, manifest.Builds[0].Notes, "Build used cache")
}
