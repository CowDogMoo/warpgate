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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helper: create a local bare git repo with a commit, tag, and branch
// using go-git (no shell required).
// ---------------------------------------------------------------------------

// createTestGitRepoGoGit creates a bare git repo with a tag and branch,
// returning a file:// URL suitable for cloning with go-git's built-in transport.
func createTestGitRepoGoGit(t *testing.T) (repoURL string) {
	t.Helper()

	bareDir := filepath.Join(t.TempDir(), "bare.git")
	workDir := filepath.Join(t.TempDir(), "work")

	// Init bare repo
	_, err := git.PlainInit(bareDir, true)
	require.NoError(t, err)

	// Init work dir separately (can't clone empty bare repo)
	repo, err := git.PlainInit(workDir, false)
	require.NoError(t, err)

	// Use file:// URL to force go-git's built-in transport (avoids shelling out to git)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://" + bareDir},
	})
	require.NoError(t, err)

	w, err := repo.Worktree()
	require.NoError(t, err)

	// Create a file and commit
	err = os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Test repo"), 0644)
	require.NoError(t, err)

	_, err = w.Add("README.md")
	require.NoError(t, err)

	commitHash, err := w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create a tag
	_, err = repo.CreateTag("v1.0.0", commitHash, nil)
	require.NoError(t, err)

	// Create a branch
	err = w.Checkout(&git.CheckoutOptions{
		Branch: "refs/heads/dev-branch",
		Create: true,
	})
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(workDir, "dev.txt"), []byte("dev content"), 0644)
	require.NoError(t, err)
	_, err = w.Add("dev.txt")
	require.NoError(t, err)
	_, err = w.Commit("dev commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Push all to bare using file:// transport
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{"refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*"},
	})
	require.NoError(t, err)

	return "file://" + bareDir
}

// createTestGitRepoWithTemplates creates a bare repo that contains a templates/
// directory with a warpgate.yaml inside, returning a file:// URL.
func createTestGitRepoWithTemplates(t *testing.T) string {
	t.Helper()

	bareDir := filepath.Join(t.TempDir(), "bare.git")
	workDir := filepath.Join(t.TempDir(), "work")

	_, err := git.PlainInit(bareDir, true)
	require.NoError(t, err)

	repo, err := git.PlainInit(workDir, false)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://" + bareDir},
	})
	require.NoError(t, err)

	w, err := repo.Worktree()
	require.NoError(t, err)

	// Create templates directory
	tmplDir := filepath.Join(workDir, "templates", "test-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	tmplContent := "metadata:\n  description: Test template\n  version: 1.0.0\n  author: tester\n  tags:\n    - test\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(tmplContent), 0644))

	_, err = w.Add("templates")
	require.NoError(t, err)
	_, err = w.Commit("add templates", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{"refs/heads/*:refs/heads/*"},
	})
	require.NoError(t, err)

	return "file://" + bareDir
}

// ---------------------------------------------------------------------------
// manager.go: saveConfigValue, saveTemplatesConfig
// ---------------------------------------------------------------------------

func TestSaveConfigValue_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories:\n    official: https://github.com/cowdogmoo/warpgate-templates.git\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: map[string]string{
				"official": "https://github.com/cowdogmoo/warpgate-templates.git",
				"custom":   "https://github.com/acme/templates.git",
			},
		},
	}
	manager := NewManager(cfg)

	err := manager.saveConfigValue("templates.repositories", cfg.Templates.Repositories)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "custom")
}

func TestSaveConfigValue_NoConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/config/path")

	cfg := &config.Config{}
	manager := NewManager(cfg)

	err := manager.saveConfigValue("templates.repositories", map[string]string{})
	assert.Error(t, err)
}

func TestSaveTemplatesConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories:\n    official: https://github.com/cowdogmoo/warpgate-templates.git\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: map[string]string{
				"official": "https://github.com/cowdogmoo/warpgate-templates.git",
			},
			LocalPaths: []string{"/some/new/path"},
		},
	}
	manager := NewManager(cfg)

	err := manager.saveTemplatesConfig()
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "/some/new/path")
}

func TestSaveTemplatesConfig_NoConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/config/path")

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths:   []string{},
			Repositories: map[string]string{},
		},
	}
	manager := NewManager(cfg)

	err := manager.saveTemplatesConfig()
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// manager.go: AddGitRepository with working config persistence
// ---------------------------------------------------------------------------

func TestAddGitRepositoryWithPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: make(map[string]string),
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddGitRepository(ctx, "my-repo", "https://github.com/acme/templates.git")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/templates.git", manager.config.Templates.Repositories["my-repo"])
}

func TestAddGitRepositoryAutoName(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: make(map[string]string),
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddGitRepository(ctx, "", "https://github.com/acme/my-awesome-repo.git")
	require.NoError(t, err)
	assert.Contains(t, manager.config.Templates.Repositories, "my-awesome-repo")
}

func TestAddGitRepositorySameURLNoError(t *testing.T) {
	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			Repositories: map[string]string{
				"existing": "https://github.com/acme/templates.git",
			},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	// Same name, same URL - should return nil (warn only)
	err := manager.AddGitRepository(ctx, "existing", "https://github.com/acme/templates.git")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// manager.go: AddLocalPath with config persistence
// ---------------------------------------------------------------------------

func TestAddLocalPathWithPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	localDir := filepath.Join(tmpDir, "my-templates")
	require.NoError(t, os.MkdirAll(localDir, 0755))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddLocalPath(ctx, localDir)
	require.NoError(t, err)
	assert.Len(t, manager.config.Templates.LocalPaths, 1)
}

// ---------------------------------------------------------------------------
// manager.go: RemoveSource with config persistence
// ---------------------------------------------------------------------------

func TestRemoveSourceLocalPathPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	localDir := filepath.Join(tmpDir, "my-templates")
	require.NoError(t, os.MkdirAll(localDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories: {}\n  local_paths:\n    - " + localDir + "\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths:   []string{localDir},
			Repositories: map[string]string{},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.RemoveSource(ctx, localDir)
	require.NoError(t, err)
	assert.Empty(t, manager.config.Templates.LocalPaths)
}

func TestRemoveSourceRepoPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	initialContent := "templates:\n  repositories:\n    my-repo: https://github.com/acme/templates.git\n  local_paths: []\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initialContent), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{},
			Repositories: map[string]string{
				"my-repo": "https://github.com/acme/templates.git",
			},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.RemoveSource(ctx, "my-repo")
	require.NoError(t, err)
	assert.Empty(t, manager.config.Templates.Repositories)
}

// ---------------------------------------------------------------------------
// git.go: checkoutVersion (was 0% in coverage)
// ---------------------------------------------------------------------------

// savedPATH is captured at init time, before any test can modify the env.
var savedPATH = os.Getenv("PATH")

// ensurePATH restores the PATH env var if it was cleared by other test cleanups.
func ensurePATH(t *testing.T) {
	t.Helper()
	if os.Getenv("PATH") == "" && savedPATH != "" {
		t.Setenv("PATH", savedPATH)
		t.Cleanup(func() {
			// no-op: we just need PATH available during this test
		})
	}
}

func TestCheckoutVersionTagAndBranch(t *testing.T) {
	ensurePATH(t)
	repoURL := createTestGitRepoGoGit(t)

	// Test 1: Clone with tag version -- exercises clone() -> cloneWithRetry() -> checkoutVersion()
	cacheDir1 := t.TempDir()
	gitOps1 := NewGitOperations(cacheDir1)
	ctx := context.Background()
	path, err := gitOps1.clone(ctx, repoURL, "v1.0.0", filepath.Join(cacheDir1, "tag-clone"))
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Test 2: Clone with branch version -- exercises the tag-fail->branch-retry path
	cacheDir2 := t.TempDir()
	gitOps2 := NewGitOperations(cacheDir2)
	path2, err := gitOps2.clone(ctx, repoURL, "dev-branch", filepath.Join(cacheDir2, "branch-clone"))
	require.NoError(t, err)
	assert.NotEmpty(t, path2)

	// Test 3: Directly test checkoutVersion with a cloned repo that has local refs
	cloneDir := filepath.Join(t.TempDir(), "full-clone")
	repo, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:  repoURL,
		Tags: git.AllTags,
	})
	require.NoError(t, err)

	// Checkout a tag (succeeds on first try)
	err = checkoutVersion(repo, "v1.0.0")
	assert.NoError(t, err)

	// Checkout nonexistent version - should fail both tag and branch path
	err = checkoutVersion(repo, "nonexistent-version-xyz")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// git.go: clone, cloneWithRetry full paths
// ---------------------------------------------------------------------------

func TestCloneWithSpecificTag(t *testing.T) {
	ensurePATH(t)
	repoURL := createTestGitRepoGoGit(t)
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	path, err := gitOps.CloneOrUpdate(ctx, repoURL, "v1.0.0")
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.True(t, dirExists(path))
}

func TestCloneWithBranch(t *testing.T) {
	ensurePATH(t)
	repoURL := createTestGitRepoGoGit(t)
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	path, err := gitOps.CloneOrUpdate(ctx, repoURL, "dev-branch")
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.True(t, dirExists(path))
}

func TestCloneInvalidURL(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	_, err := gitOps.CloneOrUpdate(ctx, "https://nonexistent-host-that-does-not-resolve.invalid/repo.git", "")
	assert.Error(t, err)
}

func TestPullUpdatesAlreadyUpToDate(t *testing.T) {
	ensurePATH(t)
	repoURL := createTestGitRepoGoGit(t)
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	path, err := gitOps.CloneOrUpdate(ctx, repoURL, "")
	require.NoError(t, err)

	err = gitOps.pullUpdates(ctx, path)
	assert.NoError(t, err)
}

func TestPullUpdatesNotARepo(t *testing.T) {
	cacheDir := t.TempDir()
	gitOps := NewGitOperations(cacheDir)
	ctx := context.Background()

	err := gitOps.pullUpdates(ctx, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open repository")
}

func TestIsSpecificVersionCoverage(t *testing.T) {
	assert.False(t, isSpecificVersion(""))
	assert.False(t, isSpecificVersion("main"))
	assert.False(t, isSpecificVersion("master"))
	assert.True(t, isSpecificVersion("v1.0.0"))
	assert.True(t, isSpecificVersion("dev-branch"))
	assert.True(t, isSpecificVersion("latest"))
}

// ---------------------------------------------------------------------------
// registry.go: UpdateCache with local templates
// ---------------------------------------------------------------------------

func TestUpdateCacheWithRepo(t *testing.T) {
	ensurePATH(t)
	repoURL := createTestGitRepoWithTemplates(t)

	cacheDir := t.TempDir()
	registry := &TemplateRegistry{
		repos: map[string]string{
			"test-repo": repoURL,
		},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	err := registry.UpdateCache(ctx, "test-repo")
	require.NoError(t, err)

	cachePath := filepath.Join(cacheDir, "test-repo.json")
	assert.True(t, fileExists(cachePath))
}

func TestUpdateAllCachesWithFailures(t *testing.T) {
	cacheDir := t.TempDir()
	registry := &TemplateRegistry{
		repos: map[string]string{
			"bad-repo": "https://nonexistent-host.invalid/repo.git",
		},
		cacheDir:      cacheDir,
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	err := registry.UpdateAllCaches(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update some caches")
}

func TestDiscoverTemplatesWithInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	validDir := filepath.Join(tmpDir, "templates", "valid")
	invalidDir := filepath.Join(tmpDir, "templates", "invalid")
	require.NoError(t, os.MkdirAll(validDir, 0755))
	require.NoError(t, os.MkdirAll(invalidDir, 0755))

	validContent := "metadata:\n  description: Valid\n  version: 1.0.0\n  author: test\n  tags:\n    - test\n"
	require.NoError(t, os.WriteFile(filepath.Join(validDir, "warpgate.yaml"), []byte(validContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "warpgate.yaml"), []byte("{{{invalid"), 0644))

	registry := &TemplateRegistry{
		pathValidator: NewPathValidator(),
	}

	templates, err := registry.discoverTemplates(tmpDir)
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "valid", templates[0].Name)
}

func TestRegistryListWithFreshCacheHit(t *testing.T) {
	tmpDir := t.TempDir()

	cache := CacheMetadata{
		LastUpdated: time.Now(),
		Templates: map[string]TemplateInfo{
			"cached-tmpl": {
				Name:        "cached-tmpl",
				Description: "Cached template",
				Version:     "1.0.0",
			},
		},
		Repositories: map[string]string{},
	}

	cacheData, err := json.MarshalIndent(cache, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "remote-repo.json"), cacheData, 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"remote-repo": "https://github.com/test/repo.git",
		},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates, err := registry.List(ctx, "remote-repo")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "cached-tmpl", templates[0].Name)
}

func TestScanLocalPathsWithMixedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	validPath := filepath.Join(tmpDir, "valid-local")
	tmplDir := filepath.Join(validPath, "templates", "local-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	tmplContent := "metadata:\n  description: Local\n  version: 1.0.0\n  author: test\n  tags:\n    - local\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(tmplContent), 0644))

	noTmplsPath := filepath.Join(tmpDir, "no-templates")
	require.NoError(t, os.MkdirAll(noTmplsPath, 0755))

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		localPaths:    []string{validPath, noTmplsPath, "/nonexistent/path"},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates := registry.scanLocalPaths(ctx)
	assert.Len(t, templates, 1)
	assert.Equal(t, "local-tmpl", templates[0].Name)
}

func TestListLocalWithMixedRepos(t *testing.T) {
	tmpDir := t.TempDir()

	localRepo := filepath.Join(tmpDir, "local-repo")
	tmplDir := filepath.Join(localRepo, "templates", "local-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	tmplContent := "metadata:\n  description: Local template\n  version: 1.0.0\n  author: test\n  tags:\n    - local\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(tmplContent), 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"local":  localRepo,
			"remote": "https://github.com/test/repo.git",
		},
		localPaths:    []string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates, err := registry.ListLocal(ctx)
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "local-tmpl", templates[0].Name)
}

// ---------------------------------------------------------------------------
// loader.go: SetVariables, LoadTemplateWithVars, List
// ---------------------------------------------------------------------------

func TestSetVariablesReplace(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	vars := map[string]string{"key1": "val1", "key2": "val2"}
	loader.SetVariables(vars)
	assert.Equal(t, "val1", loader.variables["key1"])
	assert.Equal(t, "val2", loader.variables["key2"])

	newVars := map[string]string{"key3": "val3"}
	loader.SetVariables(newVars)
	assert.Len(t, loader.variables, 1)
	assert.Equal(t, "val3", loader.variables["key3"])
}

func TestLoadTemplateWithVarsVariableMerge(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "warpgate.yaml")

	content := `metadata:
  name: merge-test
  version: 1.0.0
  description: Merge test
  author: Test
  license: MIT
  requires:
    warpgate: ">=1.0.0"
name: merge-test
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo "test"
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	loader.SetVariables(map[string]string{"INSTANCE_VAR": "instance_val"})
	cfg, err := loader.LoadTemplateWithVars(context.Background(), configPath, map[string]string{"CALL_VAR": "call_val"})
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoadTemplateWithVarsByNameNotFound(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	_, err = loader.LoadTemplateWithVars(context.Background(), "absolutely-nonexistent-template-xyz-999", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// config_loader.go: LoadFromFileWithVars error paths
// ---------------------------------------------------------------------------

func TestLoadFromFileWithVarsNonExistent(t *testing.T) {
	loader := NewLoader()
	_, err := loader.LoadFromFileWithVars("/nonexistent/path/config.yaml", nil)
	assert.Error(t, err)
}

func TestLoadFromFileWithVarsInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	require.NoError(t, os.WriteFile(badFile, []byte("{{{invalid yaml"), 0644))

	loader := NewLoader()
	_, err := loader.LoadFromFileWithVars(badFile, nil)
	assert.Error(t, err)
}

func TestExpandVariablesSourcesPreserved(t *testing.T) {
	loader := NewLoader()

	input := "path: ${sources.my_source}/file.txt"
	result := loader.expandVariables(input, nil)
	assert.Equal(t, "path: ${sources.my_source}/file.txt", result)
}

func TestExpandVariablesEmptyVar(t *testing.T) {
	loader := NewLoader()

	input := "value: ${TOTALLY_UNKNOWN_VAR_XYZ_12345}"
	result := loader.expandVariables(input, nil)
	assert.Equal(t, "value: ", result)
}

func TestResolveRelativePathsNilDockerfile(t *testing.T) {
	loader := NewLoader()

	cfg := &builder.Config{
		Dockerfile: nil,
	}
	loader.resolveRelativePaths(cfg, "/base")
	assert.Nil(t, cfg.Dockerfile)
}

func TestSaveToFileValid(t *testing.T) {
	loader := NewLoader()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.yaml")

	cfg := &builder.Config{
		Name:    "test-save",
		Version: "latest",
	}
	err := loader.SaveToFile(cfg, outputPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-save")
}

func TestSaveToFileInvalidDir(t *testing.T) {
	loader := NewLoader()
	err := loader.SaveToFile(&builder.Config{Name: "test"}, "/nonexistent/dir/output.yaml")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// version.go: edge cases
// ---------------------------------------------------------------------------

func TestCompareVersionsV2Nil(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.CompareVersions("1.0.0", "latest")
	require.NoError(t, err)
	assert.Equal(t, -1, result)
}

func TestCompareVersionsInvalid(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.CompareVersions("invalid", "1.0.0")
	assert.Error(t, err)

	_, err = vm.CompareVersions("1.0.0", "invalid")
	assert.Error(t, err)
}

func TestIsBreakingChangeNilVersions(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.IsBreakingChange("latest", "latest")
	require.NoError(t, err)
	assert.False(t, result)

	result, err = vm.IsBreakingChange("1.0.0", "latest")
	require.NoError(t, err)
	assert.False(t, result)

	result, err = vm.IsBreakingChange("latest", "2.0.0")
	require.NoError(t, err)
	assert.False(t, result)
}

func TestIsBreakingChangeInvalid(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.IsBreakingChange("invalid", "2.0.0")
	assert.Error(t, err)

	_, err = vm.IsBreakingChange("1.0.0", "invalid")
	assert.Error(t, err)
}

func TestValidateConstraintInvalidVersion(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.ValidateConstraint("invalid", ">=1.0.0")
	assert.Error(t, err)
}

func TestValidateConstraintInvalidConstraint(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.ValidateConstraint("1.0.0", ">>>invalid<<<")
	assert.Error(t, err)
}

func TestCheckCompatibilityInvalidConstraint(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, _, err = vm.CheckCompatibility("1.0.0", ">>>invalid<<<")
	assert.Error(t, err)
}

func TestGetLatestVersionAllLatest(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.GetLatestVersion(context.Background(), []string{"latest", "", "latest"})
	require.NoError(t, err)
	assert.Equal(t, "latest", result)
}

func TestValidateVersionRangeInvalidMinMax(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.ValidateVersionRange("1.0.0", "invalid", "2.0.0")
	assert.Error(t, err)

	_, err = vm.ValidateVersionRange("1.0.0", "1.0.0", "invalid")
	assert.Error(t, err)

	_, err = vm.ValidateVersionRange("invalid", "1.0.0", "2.0.0")
	assert.Error(t, err)
}

func TestValidateVersionRangeNoMin(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.ValidateVersionRange("0.5.0", "", "2.0.0")
	require.NoError(t, err)
	assert.True(t, result)
}

// ---------------------------------------------------------------------------
// paths.go: ExtractRepoName edge cases
// ---------------------------------------------------------------------------

func TestExtractRepoNameCoverage(t *testing.T) {
	tests := []struct {
		gitURL string
		want   string
	}{
		{"git@github.com:user/repo.git", "repo"},
		{"git@github.com:user/my-repo", "my-repo"},
		{"https://github.com/user/repo.git/", "templates"},
		{"", "templates"},
		{"https://github.com", "github.com"},
		{"https://gitlab.com/a/b/c/d/my-template.git", "my-template"},
	}

	for _, tt := range tests {
		result := ExtractRepoName(tt.gitURL)
		assert.Equal(t, tt.want, result, "ExtractRepoName(%q)", tt.gitURL)
	}
}

func TestMustExpandPathWithEnvVar(t *testing.T) {
	t.Setenv("WARPGATE_TEST_EXPAND_COV", "/custom/expanded/path")
	result := MustExpandPath("${WARPGATE_TEST_EXPAND_COV}/subdir")
	assert.Equal(t, "/custom/expanded/path/subdir", result)
}

func TestPathValidatorIsLocalPathEdgeCases(t *testing.T) {
	pv := NewPathValidator()

	assert.False(t, pv.IsLocalPath("./nonexistent-dir-xyz"))
	assert.False(t, pv.IsLocalPath("~/nonexistent-dir-xyz"))
}

// ---------------------------------------------------------------------------
// registry.go: isPlaceholderURL edge cases
// ---------------------------------------------------------------------------

func TestIsPlaceholderURLGitAtVariants(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"git@example.com:user/repo.git", true},
		{"git@foo.test:user/repo.git", true},
		{"git@foo.invalid:user/repo.git", true},
		{"git@foo.localhost:user/repo.git", true},
		{"git@github.com:user/repo.git", false},
		{"git@example.com", true},
	}

	for _, tt := range tests {
		result := isPlaceholderURL(tt.url)
		assert.Equal(t, tt.expected, result, "isPlaceholderURL(%q)", tt.url)
	}
}

// ---------------------------------------------------------------------------
// registry.go: GetLocalPaths returns copy
// ---------------------------------------------------------------------------

func TestGetLocalPathsCopy(t *testing.T) {
	registry := &TemplateRegistry{
		repos:      map[string]string{},
		localPaths: []string{"/path/one", "/path/two"},
	}

	paths := registry.GetLocalPaths()
	assert.Len(t, paths, 2)

	paths[0] = "/modified"
	assert.Equal(t, "/path/one", registry.localPaths[0])
}

// ---------------------------------------------------------------------------
// registry.go: LoadRepositories with corrupt JSON
// ---------------------------------------------------------------------------

func TestLoadRepositoriesCorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "repositories.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{invalid json}"), 0644))

	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	err := registry.LoadRepositories()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

// ---------------------------------------------------------------------------
// scaffold.go: saveTemplateConfig error path
// ---------------------------------------------------------------------------

func TestSaveTemplateConfigNilConfig(t *testing.T) {
	s := NewScaffolder()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &builder.Config{
		Name:    "nil-test",
		Version: "latest",
	}
	err := s.saveTemplateConfig(cfg, configPath)
	assert.NoError(t, err)
	assert.True(t, fileExists(configPath))
}

func TestSaveTemplateConfigBadPath(t *testing.T) {
	s := NewScaffolder()
	cfg := &builder.Config{Name: "test"}
	err := s.saveTemplateConfig(cfg, "/nonexistent/dir/that/cannot/be/created/config.yaml")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: scaffold.go Create and Fork
// ---------------------------------------------------------------------------

func TestScaffolderCreateSuccess(t *testing.T) {
	s := NewScaffolder()
	outputDir := t.TempDir()
	ctx := context.Background()

	err := s.Create(ctx, "my-new-template", outputDir)
	require.NoError(t, err)

	// Verify created structure
	assert.True(t, fileExists(filepath.Join(outputDir, "my-new-template", "warpgate.yaml")))
	assert.True(t, dirExists(filepath.Join(outputDir, "my-new-template", "scripts")))
	assert.True(t, fileExists(filepath.Join(outputDir, "my-new-template", "README.md")))
}

func TestScaffolderCreateReadOnlyDir(t *testing.T) {
	s := NewScaffolder()
	ctx := context.Background()

	err := s.Create(ctx, "template", "/nonexistent/readonly/dir")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: paths.go NormalizePath, ExpandPath (method), ExtractRepoName
// ---------------------------------------------------------------------------

func TestNormalizePathWithSlash(t *testing.T) {
	pv := NewPathValidator()

	// Test path that contains "/" but is not absolute -> should become absolute
	result, err := pv.NormalizePath("some/relative/path")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

func TestExtractRepoNameColonWithoutSlash(t *testing.T) {
	// This exercises the colon-parsing branch for git@host:repo format without /
	result := ExtractRepoName("git@myhost.com:single-repo.git")
	assert.Equal(t, "single-repo", result)

	// Just whitespace after cleaning
	result2 := ExtractRepoName("   ")
	assert.Equal(t, "templates", result2)

	// Colon with single part (no path after colon)
	result3 := ExtractRepoName("git@host.com:")
	assert.Equal(t, "templates", result3)
}

func TestPathValidatorExpandPathRelative(t *testing.T) {
	pv := NewPathValidator()

	// Relative path should become absolute
	result, err := pv.ExpandPath("relative/path")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

func TestPathValidatorExpandPathAbsolute(t *testing.T) {
	pv := NewPathValidator()

	result, err := pv.ExpandPath("/absolute/path")
	require.NoError(t, err)
	assert.Equal(t, "/absolute/path", result)
}

// ---------------------------------------------------------------------------
// Additional coverage: registry.go List with local path, SaveRepositories
// ---------------------------------------------------------------------------

func TestRegistryListWithLocalPathRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a local repo structure with templates
	tmplDir := filepath.Join(tmpDir, "templates", "my-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	tmplContent := "metadata:\n  description: Local test\n  version: 1.0.0\n  author: test\n  tags:\n    - test\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"), []byte(tmplContent), 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"local-repo": tmpDir,
		},
		localPaths:    []string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	templates, err := registry.List(ctx, "local-repo")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "my-tmpl", templates[0].Name)
}

func TestRegistryListUnknownRepoError(t *testing.T) {
	registry := &TemplateRegistry{
		repos:         map[string]string{},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	_, err := registry.List(ctx, "unknown-repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repository")
}

func TestSaveRepositoriesSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	registry := &TemplateRegistry{
		repos: map[string]string{
			"test": "https://github.com/test/repo.git",
		},
		cacheDir:      tmpDir,
		pathValidator: NewPathValidator(),
	}

	err := registry.SaveRepositories()
	require.NoError(t, err)
	assert.True(t, fileExists(filepath.Join(tmpDir, "repositories.json")))
}

func TestSaveRepositoriesBadPath(t *testing.T) {
	registry := &TemplateRegistry{
		repos:         map[string]string{"test": "url"},
		cacheDir:      "/nonexistent/path",
		pathValidator: NewPathValidator(),
	}

	err := registry.SaveRepositories()
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: config_loader.go LoadFromFileWithVars relative base dir
// ---------------------------------------------------------------------------

func TestLoadFromFileWithVarsRelativePath(t *testing.T) {
	// Create a valid config in a subdir of the cwd
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sub", "warpgate.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))

	content := `metadata:
  name: rel-test
  version: 1.0.0
  description: Relative path test
  author: Test
  license: MIT
  requires:
    warpgate: ">=1.0.0"
name: rel-test
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo hello
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	loader := NewLoader()
	cfg, err := loader.LoadFromFileWithVars(configPath, map[string]string{"MY_VAR": "my_val"})
	require.NoError(t, err)
	assert.Equal(t, "rel-test", cfg.Name)
}

// ---------------------------------------------------------------------------
// Additional coverage: registry.go matchesQuery - fuzzy match on description
// ---------------------------------------------------------------------------

func TestMatchesQueryFuzzyDescription(t *testing.T) {
	registry := &TemplateRegistry{pathValidator: NewPathValidator()}

	tmpl := TemplateInfo{
		Name:        "attack-box",
		Description: "Security penetration testing toolkit",
		Tags:        []string{"security"},
	}

	// Fuzzy match on description word
	assert.True(t, registry.matchesQuery(tmpl, "penetration"))
	// No match
	assert.False(t, registry.matchesQuery(tmpl, "zzzzzzzzzzz"))
}

// ---------------------------------------------------------------------------
// Additional coverage: registry.go listAll
// ---------------------------------------------------------------------------

func TestRegistryListAllWithMixedSources(t *testing.T) {
	tmpDir := t.TempDir()

	// Create local repo with templates
	tmplDir := filepath.Join(tmpDir, "repo1", "templates", "tmpl1")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "warpgate.yaml"),
		[]byte("metadata:\n  description: T1\n  version: 1.0.0\n  author: test\n  tags:\n    - test\n"), 0644))

	// Create local path with templates
	localPath := filepath.Join(tmpDir, "local")
	localTmplDir := filepath.Join(localPath, "templates", "tmpl2")
	require.NoError(t, os.MkdirAll(localTmplDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(localTmplDir, "warpgate.yaml"),
		[]byte("metadata:\n  description: T2\n  version: 1.0.0\n  author: test\n  tags:\n    - local\n"), 0644))

	registry := &TemplateRegistry{
		repos: map[string]string{
			"local-repo": filepath.Join(tmpDir, "repo1"),
		},
		localPaths:    []string{localPath},
		cacheDir:      t.TempDir(),
		pathValidator: NewPathValidator(),
	}

	ctx := context.Background()
	all, err := registry.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

// ---------------------------------------------------------------------------
// Additional coverage: loader.go LoadTemplateWithVars with directory ref
// ---------------------------------------------------------------------------

func TestLoadTemplateWithVarsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	content := `metadata:
  name: dir-test
  version: 1.0.0
  description: Directory test
  author: Test
  license: MIT
  requires:
    warpgate: ">=1.0.0"
name: dir-test
version: latest
base:
  image: alpine:latest
  pull: true
provisioners:
  - type: shell
    inline:
      - echo dir
targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - latest
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "warpgate.yaml"), []byte(content), 0644))

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	cfg, err := loader.LoadTemplateWithVars(context.Background(), tmpDir, nil)
	require.NoError(t, err)
	assert.Equal(t, "dir-test", cfg.Name)
}

func TestLoadTemplateWithVarsDirNoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	_, err = loader.LoadTemplateWithVars(context.Background(), tmpDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no warpgate.yaml found")
}

func TestLoadTemplateWithVarsUnknownRef(t *testing.T) {
	loader, err := NewTemplateLoader(context.Background())
	require.NoError(t, err)

	_, err = loader.LoadTemplateWithVars(context.Background(), "some/path/with/slashes", nil)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: manager.go AddLocalPath duplicate path
// ---------------------------------------------------------------------------

func TestAddLocalPathDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "warpgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("templates:\n  repositories: {}\n  local_paths: []\n"), 0644))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	localDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.MkdirAll(localDir, 0755))

	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{localDir},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	// Adding duplicate should not error
	err := manager.AddLocalPath(ctx, localDir)
	assert.NoError(t, err)
	assert.Len(t, manager.config.Templates.LocalPaths, 1)
}

func TestAddLocalPathNonExistent(t *testing.T) {
	cfg := &config.Config{
		Templates: config.TemplatesConfig{
			LocalPaths: []string{},
		},
	}
	manager := NewManager(cfg)
	ctx := context.Background()

	err := manager.AddLocalPath(ctx, "/nonexistent/path/xyz")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Additional coverage: version.go GetLatestVersion with mixed versions
// ---------------------------------------------------------------------------

func TestGetLatestVersionMixed(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.GetLatestVersion(context.Background(), []string{"1.0.0", "2.0.0", "1.5.0"})
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", result)
}

func TestGetLatestVersionSingleValid(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	result, err := vm.GetLatestVersion(context.Background(), []string{"latest", "invalid!", "3.0.0"})
	require.NoError(t, err)
	assert.Equal(t, "3.0.0", result)
}

func TestGetLatestVersionEmpty(t *testing.T) {
	vm, err := NewVersionManager("1.0.0")
	require.NoError(t, err)

	_, err = vm.GetLatestVersion(context.Background(), []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no versions provided")
}
