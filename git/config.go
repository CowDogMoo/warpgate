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

// Package git provides utilities for reading and parsing git configuration.
package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/cowdogmoo/warpgate/v3/templates"
	"gopkg.in/ini.v1"
)

// ConfigReader reads git configuration from .gitconfig files.
type ConfigReader struct{}

// NewConfigReader creates a new git configuration reader.
func NewConfigReader() *ConfigReader {
	return &ConfigReader{}
}

// GetAuthor retrieves author information from git config.
// Returns a formatted string in the form "Name <email>", "Name", "email", or empty string.
func (r *ConfigReader) GetAuthor(ctx context.Context) string {
	home, err := os.UserHomeDir()
	if err != nil {
		logging.DebugContext(ctx, "Failed to get home directory: %v", err)
		return ""
	}

	// Load main git config
	cfg := r.loadGitConfig(ctx, home)
	if cfg == nil {
		return ""
	}

	// Extract name and email
	name, email := r.extractUserInfo(cfg)

	// Try included configs if needed
	if name == "" || email == "" {
		name, email = r.tryIncludedConfig(ctx, cfg, home, name, email)
	}

	return formatAuthor(name, email)
}

// loadGitConfig loads the main .gitconfig file from the user's home directory.
func (r *ConfigReader) loadGitConfig(ctx context.Context, home string) *ini.File {
	gitconfigPath := filepath.Join(home, ".gitconfig")
	cfg, err := ini.Load(gitconfigPath)
	if err != nil {
		logging.DebugContext(ctx, "Failed to load .gitconfig: %v", err)
		return nil
	}
	return cfg
}

// extractUserInfo extracts name and email from a git config [user] section.
func (r *ConfigReader) extractUserInfo(cfg *ini.File) (name, email string) {
	userSection := cfg.Section("user")
	if userSection != nil {
		name = userSection.Key("name").String()
		email = userSection.Key("email").String()
	}
	return name, email
}

// tryIncludedConfig attempts to load user info from included config files.
// Git supports [include] sections that reference other config files.
func (r *ConfigReader) tryIncludedConfig(ctx context.Context, cfg *ini.File, home, currentName, currentEmail string) (name, email string) {
	name, email = currentName, currentEmail

	includeSection := cfg.Section("include")
	if includeSection == nil {
		return name, email
	}

	includePath := includeSection.Key("path").String()
	if includePath == "" {
		return name, email
	}

	// Expand path (handles ~ and environment variables)
	includePath = templates.MustExpandPath(includePath)

	includedCfg, err := ini.Load(includePath)
	if err != nil {
		logging.DebugContext(ctx, "Failed to load included config from %s: %v", includePath, err)
		return name, email
	}

	includedUserSection := includedCfg.Section("user")
	if includedUserSection == nil {
		return name, email
	}

	// Only override empty values
	if name == "" {
		name = includedUserSection.Key("name").String()
	}
	if email == "" {
		email = includedUserSection.Key("email").String()
	}

	return name, email
}

// formatAuthor formats name and email as a git author string.
// Returns formats in order of preference:
// - "Name <email>" if both present
// - "Name" if only name present
// - "email" if only email present
// - "" if neither present
func formatAuthor(name, email string) string {
	switch {
	case name != "" && email != "":
		return fmt.Sprintf("%s <%s>", name, email)
	case name != "":
		return name
	case email != "":
		return email
	default:
		return ""
	}
}
