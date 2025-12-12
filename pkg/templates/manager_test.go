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

package templates

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
)

func TestNewManager(t *testing.T) {
	cfg := &globalconfig.Config{}
	manager := NewManager(cfg)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	if manager.config != cfg {
		t.Error("NewManager() config not set correctly")
	}

	if manager.validator == nil {
		t.Error("NewManager() validator not initialized")
	}
}

func TestAddGitRepository(t *testing.T) {
	// Skip this test if config file doesn't exist (e.g., in CI)
	// This test requires filesystem persistence which should be an integration test
	configPath, err := globalconfig.ConfigFile("config.yaml")
	if err != nil || !fileExists(configPath) {
		t.Skip("Skipping test that requires config file persistence - should be an integration test")
	}

	tests := []struct {
		name     string
		repoName string
		url      string
		existing map[string]string
		wantErr  bool
	}{
		{
			name:     "add new repository with name",
			repoName: "my-templates",
			url:      "https://github.com/acme/templates.git",
			existing: map[string]string{},
			wantErr:  false,
		},
		{
			name:     "add new repository without name (auto-generate)",
			repoName: "",
			url:      "https://github.com/acme/my-repo.git",
			existing: map[string]string{},
			wantErr:  false,
		},
		{
			name:     "add duplicate repository with same URL",
			repoName: "existing",
			url:      "https://github.com/acme/templates.git",
			existing: map[string]string{
				"existing": "https://github.com/acme/templates.git",
			},
			wantErr: false, // Should warn but not error
		},
		{
			name:     "add repository with conflicting name but different URL",
			repoName: "existing",
			url:      "https://github.com/acme/different.git",
			existing: map[string]string{
				"existing": "https://github.com/acme/templates.git",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &globalconfig.Config{
				Templates: globalconfig.TemplatesConfig{
					Repositories: tt.existing,
				},
			}
			manager := NewManager(cfg)

			ctx := context.Background()
			err := manager.AddGitRepository(ctx, tt.repoName, tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddGitRepository() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Note: We can't easily test the config file persistence in unit tests
			// as it requires a real config file. That would be better as an integration test.
		})
	}
}

func TestAddLocalPath(t *testing.T) {
	// Skip this test if config file doesn't exist (e.g., in CI)
	// This test requires filesystem persistence which should be an integration test
	configPath, err := globalconfig.ConfigFile("config.yaml")
	if err != nil || !fileExists(configPath) {
		t.Skip("Skipping test that requires config file persistence - should be an integration test")
	}

	// Create temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		existing []string
		wantErr  bool
	}{
		{
			name:     "add valid local path",
			path:     tmpDir,
			existing: []string{},
			wantErr:  false,
		},
		{
			name:     "add duplicate path",
			path:     tmpDir,
			existing: []string{tmpDir},
			wantErr:  false, // Should warn but not error
		},
		{
			name:     "add non-existent path",
			path:     filepath.Join(tmpDir, "nonexistent"),
			existing: []string{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &globalconfig.Config{
				Templates: globalconfig.TemplatesConfig{
					LocalPaths: tt.existing,
				},
			}
			manager := NewManager(cfg)

			ctx := context.Background()
			err := manager.AddLocalPath(ctx, tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddLocalPath() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Note: We can't easily test the config file persistence in unit tests
		})
	}
}

func TestAddLocalPath_HomePath(t *testing.T) {
	// Skip this test if config file doesn't exist (e.g., in CI)
	// This test requires filesystem persistence which should be an integration test
	configPath, err := globalconfig.ConfigFile("config.yaml")
	if err != nil || !fileExists(configPath) {
		t.Skip("Skipping test that requires config file persistence - should be an integration test")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()

	// We can't easily test ~ expansion without mocking os.UserHomeDir
	// But we can test that the path validator is called
	cfg := &globalconfig.Config{
		Templates: globalconfig.TemplatesConfig{
			LocalPaths: []string{},
		},
	}
	manager := NewManager(cfg)

	ctx := context.Background()

	// Test with existing directory
	err = manager.AddLocalPath(ctx, tmpDir)
	if err != nil {
		t.Errorf("AddLocalPath() with valid path error = %v", err)
	}
}

func TestRemoveSource_LocalPath(t *testing.T) {
	// Skip this test if config file doesn't exist (e.g., in CI)
	// This test requires filesystem persistence which should be an integration test
	configPath, err := globalconfig.ConfigFile("config.yaml")
	if err != nil || !fileExists(configPath) {
		t.Skip("Skipping test that requires config file persistence - should be an integration test")
	}

	tmpDir := t.TempDir()
	anotherPath := filepath.Join(tmpDir, "another")
	_ = os.MkdirAll(anotherPath, 0755)

	cfg := &globalconfig.Config{
		Templates: globalconfig.TemplatesConfig{
			LocalPaths: []string{tmpDir, anotherPath},
			Repositories: map[string]string{
				"official": "https://github.com/official/templates.git",
			},
		},
	}
	manager := NewManager(cfg)

	ctx := context.Background()

	// Remove existing local path
	err = manager.RemoveSource(ctx, tmpDir)
	if err != nil {
		t.Errorf("RemoveSource() error = %v", err)
	}

	// Verify it was removed
	if len(manager.config.Templates.LocalPaths) != 1 {
		t.Errorf("RemoveSource() local_paths length = %d, want 1", len(manager.config.Templates.LocalPaths))
	}
}

func TestRemoveSource_Repository(t *testing.T) {
	// Skip this test if config file doesn't exist (e.g., in CI)
	// This test requires filesystem persistence which should be an integration test
	configPath, err := globalconfig.ConfigFile("config.yaml")
	if err != nil || !fileExists(configPath) {
		t.Skip("Skipping test that requires config file persistence - should be an integration test")
	}

	cfg := &globalconfig.Config{
		Templates: globalconfig.TemplatesConfig{
			LocalPaths: []string{},
			Repositories: map[string]string{
				"official": "https://github.com/official/templates.git",
				"custom":   "https://github.com/acme-corp/templates.git",
			},
		},
	}
	manager := NewManager(cfg)

	ctx := context.Background()

	// Remove existing repository
	err = manager.RemoveSource(ctx, "official")
	if err != nil {
		t.Errorf("RemoveSource() error = %v", err)
	}

	// Verify it was removed
	if len(manager.config.Templates.Repositories) != 1 {
		t.Errorf("RemoveSource() repositories length = %d, want 1", len(manager.config.Templates.Repositories))
	}

	if _, exists := manager.config.Templates.Repositories["official"]; exists {
		t.Error("RemoveSource() did not remove 'official' repository")
	}
}

func TestRemoveSource_NotFound(t *testing.T) {
	cfg := &globalconfig.Config{
		Templates: globalconfig.TemplatesConfig{
			LocalPaths:   []string{},
			Repositories: map[string]string{},
		},
	}
	manager := NewManager(cfg)

	ctx := context.Background()

	// Try to remove non-existent source
	err := manager.RemoveSource(ctx, "nonexistent")
	if err == nil {
		t.Error("RemoveSource() expected error for non-existent source, got nil")
	}
}

func TestRemoveFromLocalPaths(t *testing.T) {
	tmpDir1 := "/path/to/templates1"
	tmpDir2 := "/path/to/templates2"

	cfg := &globalconfig.Config{
		Templates: globalconfig.TemplatesConfig{
			LocalPaths: []string{tmpDir1, tmpDir2},
		},
	}
	manager := NewManager(cfg)

	// Remove first path
	removed := manager.removeFromLocalPaths(tmpDir1, tmpDir1)

	if !removed {
		t.Error("removeFromLocalPaths() should return true when path is removed")
	}

	if len(manager.config.Templates.LocalPaths) != 1 {
		t.Errorf("removeFromLocalPaths() local_paths length = %d, want 1", len(manager.config.Templates.LocalPaths))
	}

	if manager.config.Templates.LocalPaths[0] != tmpDir2 {
		t.Errorf("removeFromLocalPaths() remaining path = %s, want %s", manager.config.Templates.LocalPaths[0], tmpDir2)
	}
}

func TestRemoveFromRepositories(t *testing.T) {
	cfg := &globalconfig.Config{
		Templates: globalconfig.TemplatesConfig{
			Repositories: map[string]string{
				"official": "https://github.com/official/templates.git",
				"custom":   "https://github.com/acme-corp/templates.git",
			},
		},
	}
	manager := NewManager(cfg)

	// Remove repository
	removed := manager.removeFromRepositories("official")

	if !removed {
		t.Error("removeFromRepositories() should return true when repository is removed")
	}

	if len(manager.config.Templates.Repositories) != 1 {
		t.Errorf("removeFromRepositories() repositories length = %d, want 1", len(manager.config.Templates.Repositories))
	}

	if _, exists := manager.config.Templates.Repositories["official"]; exists {
		t.Error("removeFromRepositories() did not remove 'official' repository")
	}
}

func TestRemoveFromRepositories_NotFound(t *testing.T) {
	cfg := &globalconfig.Config{
		Templates: globalconfig.TemplatesConfig{
			Repositories: map[string]string{
				"official": "https://github.com/official/templates.git",
			},
		},
	}
	manager := NewManager(cfg)

	// Try to remove non-existent repository
	removed := manager.removeFromRepositories("nonexistent")

	if removed {
		t.Error("removeFromRepositories() should return false for non-existent repository")
	}

	if len(manager.config.Templates.Repositories) != 1 {
		t.Error("removeFromRepositories() should not modify repositories when key not found")
	}
}
