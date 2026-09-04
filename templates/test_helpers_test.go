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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
)

// TestMain redirects HOME and the XDG base directories to a throwaway
// directory so no test in this package can read or write the developer's
// real warpgate config or cache.
func TestMain(m *testing.M) {
	os.Exit(runTestMain(m))
}

// isolateConfigHome gives one test its own config home, seeded with an empty
// config file, and returns the directory holding it.
//
// The package-wide home from TestMain is a single file. Manager.saveConfigValue
// and Manager.saveTemplatesConfig read it, edit it and write it back, so two
// tests persisting a source at the same time can read what the other left
// half-written. That surfaces as "failed to read config: While parsing config:
// yaml: ... could not find expected ':'".
//
// This mutates process-global environment, so a test that calls it must not call
// t.Parallel(); t.Setenv panics if it does. Serialising these few tests is the
// price of each having a config file nothing else touches.
func isolateConfigHome(t *testing.T) string {
	t.Helper()

	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	configDir := filepath.Join(configHome, "warpgate")
	if err := os.MkdirAll(configDir, config.DirPermReadWriteExec); err != nil {
		t.Fatalf("failed to create isolated config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), nil, config.FilePermReadWrite); err != nil {
		t.Fatalf("failed to seed isolated config file: %v", err)
	}

	return configDir
}

func runTestMain(m *testing.M) int {
	home, err := os.MkdirTemp("", "warpgate-testhome")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create isolated test home: %v\n", err)
		return 1
	}
	defer func() {
		if err := os.RemoveAll(home); err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove isolated test home: %v\n", err)
		}
	}()

	for key, value := range map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
	} {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "failed to set %s: %v\n", key, err)
			return 1
		}
	}

	// Seed an empty global config so code paths that update it in place
	// (e.g. Manager.saveConfigValue) find a file to read.
	configDir := filepath.Join(home, ".config", "warpgate")
	if err := os.MkdirAll(configDir, config.DirPermReadWriteExec); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create isolated config dir: %v\n", err)
		return 1
	}
	sharedConfig := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(sharedConfig, nil, config.FilePermReadWrite); err != nil {
		fmt.Fprintf(os.Stderr, "failed to seed isolated config file: %v\n", err)
		return 1
	}

	code := m.Run()
	if code != 0 {
		return code
	}

	// This one file is shared by every test in the package, so anything that
	// writes it is racing every other writer. A test that persists a template
	// source has to call isolateConfigHome first; the seeded file staying empty
	// is what proves none of them skipped it.
	info, err := os.Stat(sharedConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to stat the shared config file: %v\n", err)
		return 1
	}
	if info.Size() != 0 {
		fmt.Fprintf(os.Stderr,
			"a test wrote the package-wide config file %s (%d bytes); call isolateConfigHome(t) in every test that persists a template source\n",
			sharedConfig, info.Size())
		return 1
	}

	return code
}
