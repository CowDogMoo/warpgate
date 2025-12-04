/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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

package provisioner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// CollectionManager manages Ansible Galaxy collections
// This replaces the ansible-galaxy-setup from GitHub Actions
type CollectionManager struct {
	collectionsPath string
	galaxyFile      string
}

// NewCollectionManager creates a new collection manager
func NewCollectionManager(collectionsPath, galaxyFile string) *CollectionManager {
	return &CollectionManager{
		collectionsPath: collectionsPath,
		galaxyFile:      galaxyFile,
	}
}

// InstallFromFile installs collections from a requirements.yml file
func (cm *CollectionManager) InstallFromFile() error {
	logging.Info("Installing Ansible collections from %s", cm.galaxyFile)

	if _, err := os.Stat(cm.galaxyFile); os.IsNotExist(err) {
		return fmt.Errorf("galaxy file not found: %s", cm.galaxyFile)
	}

	// Ensure collections directory exists
	if err := os.MkdirAll(cm.collectionsPath, 0755); err != nil {
		return fmt.Errorf("failed to create collections directory: %w", err)
	}

	// Run ansible-galaxy collection install
	cmd := exec.Command("ansible-galaxy", "collection", "install",
		"-r", cm.galaxyFile,
		"-p", cm.collectionsPath,
		"--force",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install collections: %w", err)
	}

	logging.Info("Successfully installed Ansible collections to %s", cm.collectionsPath)
	return nil
}

// InstallCollection installs a specific collection
func (cm *CollectionManager) InstallCollection(collection string) error {
	logging.Info("Installing Ansible collection: %s", collection)

	cmd := exec.Command("ansible-galaxy", "collection", "install",
		collection,
		"-p", cm.collectionsPath,
		"--force",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install collection %s: %w", collection, err)
	}

	logging.Info("Successfully installed collection: %s", collection)
	return nil
}

// CloneCustomCollection clones a custom collection from git
func (cm *CollectionManager) CloneCustomCollection(repoURL, namespace, collection string) error {
	logging.Info("Cloning custom collection from %s", repoURL)

	collectionPath := filepath.Join(cm.collectionsPath, "ansible_collections", namespace, collection)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(collectionPath), 0755); err != nil {
		return fmt.Errorf("failed to create collection directory: %w", err)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", repoURL, collectionPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone collection: %w", err)
	}

	logging.Info("Successfully cloned collection to %s", collectionPath)
	return nil
}

// ListInstalled lists all installed collections
func (cm *CollectionManager) ListInstalled() ([]string, error) {
	collectionsDir := filepath.Join(cm.collectionsPath, "ansible_collections")

	if _, err := os.Stat(collectionsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	var collections []string

	// Walk through ansible_collections directory
	namespaces, err := os.ReadDir(collectionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read collections directory: %w", err)
	}

	for _, ns := range namespaces {
		if !ns.IsDir() {
			continue
		}

		nsPath := filepath.Join(collectionsDir, ns.Name())
		colls, err := os.ReadDir(nsPath)
		if err != nil {
			continue
		}

		for _, coll := range colls {
			if coll.IsDir() {
				collections = append(collections, fmt.Sprintf("%s.%s", ns.Name(), coll.Name()))
			}
		}
	}

	return collections, nil
}

// Cleanup removes all installed collections
func (cm *CollectionManager) Cleanup() error {
	logging.Info("Cleaning up Ansible collections")

	if err := os.RemoveAll(cm.collectionsPath); err != nil {
		return fmt.Errorf("failed to remove collections directory: %w", err)
	}

	logging.Info("Ansible collections cleaned up")
	return nil
}
