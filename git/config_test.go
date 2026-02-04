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

package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

func TestNewConfigReader(t *testing.T) {
	t.Parallel()
	reader := NewConfigReader()
	assert.NotNil(t, reader)
}

func TestFormatAuthor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		pName string
		email string
		want  string
	}{
		{"both name and email", "John Doe", "john@example.com", "John Doe <john@example.com>"},
		{"only name", "John Doe", "", "John Doe"},
		{"only email", "", "john@example.com", "john@example.com"},
		{"neither", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatAuthor(tt.pName, tt.email)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractUserInfo(t *testing.T) {
	t.Parallel()

	reader := NewConfigReader()

	t.Run("extracts from valid gitconfig", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		gitconfigPath := filepath.Join(tmpDir, ".gitconfig")
		content := `[user]
	name = Test User
	email = test@example.com
`
		require.NoError(t, os.WriteFile(gitconfigPath, []byte(content), 0644))

		cfg, err := ini.Load(gitconfigPath)
		require.NoError(t, err)

		name, email := reader.extractUserInfo(cfg)
		assert.Equal(t, "Test User", name)
		assert.Equal(t, "test@example.com", email)
	})

	t.Run("returns empty for missing user keys", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		gitconfigPath := filepath.Join(tmpDir, ".gitconfig")
		content := `[core]
	autocrlf = true
`
		require.NoError(t, os.WriteFile(gitconfigPath, []byte(content), 0644))

		cfg, err := ini.Load(gitconfigPath)
		require.NoError(t, err)

		name, email := reader.extractUserInfo(cfg)
		assert.Empty(t, name)
		assert.Empty(t, email)
	})
}

func TestLoadGitConfig(t *testing.T) {
	t.Parallel()

	reader := NewConfigReader()
	ctx := context.Background()

	t.Run("returns nil for nonexistent directory", func(t *testing.T) {
		t.Parallel()
		cfg := reader.loadGitConfig(ctx, "/nonexistent/path")
		assert.Nil(t, cfg)
	})

	t.Run("returns nil for directory without gitconfig", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := reader.loadGitConfig(ctx, tmpDir)
		assert.Nil(t, cfg)
	})

	t.Run("loads valid gitconfig", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		content := `[user]
	name = Test User
	email = test@example.com
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(content), 0644))
		cfg := reader.loadGitConfig(ctx, tmpDir)
		assert.NotNil(t, cfg)
	})
}

func TestTryIncludedConfig(t *testing.T) {
	t.Parallel()

	reader := NewConfigReader()
	ctx := context.Background()

	t.Run("fills missing name from included config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// Create included config with name
		includedPath := filepath.Join(tmpDir, "included.gitconfig")
		includedContent := `[user]
	name = Included User
	email = included@example.com
`
		require.NoError(t, os.WriteFile(includedPath, []byte(includedContent), 0644))

		// Create main config referencing included config
		mainContent := `[include]
	path = ` + includedPath + `
[user]
	email = main@example.com
`
		mainPath := filepath.Join(tmpDir, ".gitconfig")
		require.NoError(t, os.WriteFile(mainPath, []byte(mainContent), 0644))

		cfg, err := ini.Load(mainPath)
		require.NoError(t, err)

		name, email := reader.tryIncludedConfig(ctx, cfg, tmpDir, "", "main@example.com")
		assert.Equal(t, "Included User", name)
		assert.Equal(t, "main@example.com", email) // Should NOT override existing
	})

	t.Run("returns existing values when no include path", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mainContent := `[user]
	name = Main User
`
		mainPath := filepath.Join(tmpDir, ".gitconfig")
		require.NoError(t, os.WriteFile(mainPath, []byte(mainContent), 0644))

		cfg, err := ini.Load(mainPath)
		require.NoError(t, err)

		name, email := reader.tryIncludedConfig(ctx, cfg, tmpDir, "Main User", "")
		assert.Equal(t, "Main User", name)
		assert.Empty(t, email)
	})

	t.Run("handles nonexistent included config gracefully", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mainContent := `[include]
	path = /nonexistent/included.gitconfig
[user]
	name = Main User
`
		mainPath := filepath.Join(tmpDir, ".gitconfig")
		require.NoError(t, os.WriteFile(mainPath, []byte(mainContent), 0644))

		cfg, err := ini.Load(mainPath)
		require.NoError(t, err)

		name, email := reader.tryIncludedConfig(ctx, cfg, tmpDir, "Main User", "")
		assert.Equal(t, "Main User", name)
		assert.Empty(t, email)
	})
}

// TestGetAuthor_NoGitConfig tests the GetAuthor path where no .gitconfig exists.
func TestGetAuthor_NoGitConfig(t *testing.T) {
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()
	// No .gitconfig file exists in tmpDir
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	author := reader.GetAuthor(ctx)
	assert.Empty(t, author, "expected empty author when no .gitconfig exists")
}

// TestGetAuthor_IncludedConfigNoUserSection tests the path where included config
// has no [user] section.
func TestGetAuthor_IncludedConfigNoUserSection(t *testing.T) {
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create included config WITHOUT user section
	includedPath := filepath.Join(tmpDir, "included.gitconfig")
	includedContent := "[core]\n\tautocrlf = true\n"
	require.NoError(t, os.WriteFile(includedPath, []byte(includedContent), 0644))

	// Create main config with only name, referencing included config
	mainContent := "[user]\n\tname = Main User\n[include]\n\tpath = " + includedPath + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(mainContent), 0644))

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Since included config has no user section, email stays empty
	author := reader.GetAuthor(ctx)
	assert.Equal(t, "Main User", author)
}

// TestGetAuthor_IncludedConfigEmptyPath tests the path where include section
// exists but path is empty.
func TestGetAuthor_IncludedConfigEmptyPath(t *testing.T) {
	reader := NewConfigReader()
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create main config with include section but empty path
	mainContent := "[user]\n\tname = User Only\n[include]\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(mainContent), 0644))

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	author := reader.GetAuthor(ctx)
	assert.Equal(t, "User Only", author)
}

func TestGetAuthor(t *testing.T) {
	// Not parallel - modifies HOME
	reader := NewConfigReader()
	ctx := context.Background()

	t.Run("returns empty when no gitconfig exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalHome := os.Getenv("HOME")
		t.Setenv("HOME", tmpDir)
		defer func() { _ = os.Setenv("HOME", originalHome) }()

		author := reader.GetAuthor(ctx)
		assert.Empty(t, author)
	})

	t.Run("returns author from gitconfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `[user]
	name = Test User
	email = test@example.com
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(content), 0644))

		originalHome := os.Getenv("HOME")
		t.Setenv("HOME", tmpDir)
		defer func() { _ = os.Setenv("HOME", originalHome) }()

		author := reader.GetAuthor(ctx)
		assert.Equal(t, "Test User <test@example.com>", author)
	})
}
