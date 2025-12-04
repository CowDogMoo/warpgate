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

package container

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/pkg/logging"
)

// StorageConfig manages container storage configuration
// This replaces the storage configuration setup from GitHub Actions
type StorageConfig struct {
	root       string
	runroot    string
	driver     string
	configured bool
}

// NewStorageConfig creates a new storage configuration
func NewStorageConfig() *StorageConfig {
	return &StorageConfig{
		root:    "/var/lib/containers/storage",
		runroot: "/run/containers/storage",
		driver:  "overlay",
	}
}

// Configure sets up the container storage directories and configuration
func (sc *StorageConfig) Configure() error {
	logging.Info("Configuring container storage")

	// Create storage directories
	dirs := []string{
		sc.root,
		sc.runroot,
		filepath.Join(sc.root, "overlay"),
		filepath.Join(sc.root, "overlay-images"),
		filepath.Join(sc.root, "overlay-layers"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create storage directory %s: %w", dir, err)
		}
		logging.Debug("Created storage directory: %s", dir)
	}

	sc.configured = true
	logging.Info("Container storage configured successfully")
	return nil
}

// SetRoot sets the storage root directory
func (sc *StorageConfig) SetRoot(root string) {
	sc.root = root
}

// SetRunRoot sets the storage runroot directory
func (sc *StorageConfig) SetRunRoot(runroot string) {
	sc.runroot = runroot
}

// SetDriver sets the storage driver
func (sc *StorageConfig) SetDriver(driver string) {
	sc.driver = driver
}

// GetRoot returns the storage root directory
func (sc *StorageConfig) GetRoot() string {
	return sc.root
}

// GetRunRoot returns the storage runroot directory
func (sc *StorageConfig) GetRunRoot() string {
	return sc.runroot
}

// GetDriver returns the storage driver
func (sc *StorageConfig) GetDriver() string {
	return sc.driver
}

// IsConfigured returns whether storage has been configured
func (sc *StorageConfig) IsConfigured() bool {
	return sc.configured
}

// WriteStorageConf writes a storage.conf file
func (sc *StorageConfig) WriteStorageConf(path string) error {
	content := fmt.Sprintf(`[storage]
driver = "%s"
runroot = "%s"
graphroot = "%s"

[storage.options]
additionalimagestores = []

[storage.options.overlay]
mountopt = "nodev,metacopy=on"
`, sc.driver, sc.runroot, sc.root)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write storage.conf: %w", err)
	}

	logging.Info("Wrote storage configuration to %s", path)
	return nil
}

// Cleanup removes storage directories
func (sc *StorageConfig) Cleanup() error {
	logging.Info("Cleaning up container storage")

	if err := os.RemoveAll(sc.root); err != nil {
		return fmt.Errorf("failed to remove storage root: %w", err)
	}

	if err := os.RemoveAll(sc.runroot); err != nil {
		return fmt.Errorf("failed to remove storage runroot: %w", err)
	}

	sc.configured = false
	logging.Info("Container storage cleaned up")
	return nil
}
