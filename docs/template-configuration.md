# Template Configuration and Repository Management

Complete guide to managing Warpgate template sources, repositories, and discovery.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
  - [Configuration File Locations](#configuration-file-locations)
  - [Basic Configuration](#basic-configuration)
  - [Repository Types](#repository-types)
  - [Configuration Options](#configuration-options)
- [Managing Template Sources](#managing-template-sources)
  - [Adding Sources](#adding-sources)
  - [Removing Sources](#removing-sources)
  - [Updating Cache](#updating-cache)
- [Using Templates](#using-templates)
  - [Discovering Templates](#discovering-templates)
  - [Building from Templates](#building-from-templates)
  - [Template Discovery Order](#template-discovery-order)
- [Template Directory Structure](#template-directory-structure)
- [Private Repositories](#private-repositories)
- [Environment Variables](#environment-variables)
- [Common Scenarios](#common-scenarios)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

Warpgate supports flexible template management from multiple sources:

- **Git repositories** - Public or private, with automatic caching
- **Local directories** - Development templates or shared network paths
- **Mixed sources** - Combine git repos and local paths seamlessly

Template sources are configured in `~/.config/warpgate/config.yaml` and
automatically discovered and cached.

**Key Features:**

- Automatic template discovery from multiple sources
- Git repository caching for offline use
- Local directory scanning
- Template versioning and metadata
- Private repository support

## Quick Start

Get started with template management in 60 seconds:

```bash
# List default templates
warpgate templates list

# Add a Git repository
warpgate templates add https://github.com/myorg/security-templates.git

# Add a local directory
warpgate templates add ~/my-templates

# Discover all templates
warpgate discover

# Build from template
warpgate build attack-box

# Get template info
warpgate templates info attack-box
```

## Configuration

### Configuration File Locations

Warpgate follows the XDG Base Directory Specification:

**Primary config location:**

```text
~/.config/warpgate/config.yaml
```

**Legacy location (still supported):**

```text
~/.warpgate/config.yaml
```

**Cache directory:**

```text
~/.cache/warpgate/templates/
```

### Basic Configuration

Minimal configuration (uses official repository by default):

```yaml
# No configuration needed - uses default repository:
# https://github.com/cowdogmoo/warpgate-templates.git
```

Custom configuration:

```yaml
templates:
  # Named repositories (git URLs or local paths)
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    private: git@github.com:myorg/private-templates.git
    local: /Users/username/my-templates

  # Additional local paths to scan
  local_paths:
    - /opt/shared/templates
    - ~/dev/templates

  # Custom cache directory (optional)
  cache_dir: ~/.cache/warpgate/templates
```

### Repository Types

#### Git Repositories

**Public repositories:**

```yaml
repositories:
  official: https://github.com/cowdogmoo/warpgate-templates.git
  community: https://github.com/security-team/templates.git
```

**Private repositories (SSH):**

```yaml
repositories:
  private: git@github.com:myorg/private-templates.git
  company: git@gitlab.company.com:infra/templates.git
```

**Private repositories (HTTPS with credentials):**

```yaml
repositories:
  # Requires git credential helper configured
  private: https://github.com/myorg/private-templates.git
```

#### Local Directories

**Absolute paths:**

```yaml
repositories:
  dev: /Users/username/warpgate-templates
  shared: /opt/company/templates
```

**Relative paths (from config file location):**

```yaml
repositories:
  local: ../templates
  nearby: ./my-templates
```

**Home directory paths:**

```yaml
repositories:
  home: ~/warpgate-templates
```

#### Mixed Configuration

Combine git repositories and local paths:

```yaml
templates:
  repositories:
    # Git repos (cached locally)
    official: https://github.com/cowdogmoo/warpgate-templates.git
    private: git@github.com:myorg/templates.git

    # Local directories (used directly)
    dev: ~/dev/warpgate-templates
    shared: /mnt/nfs/shared-templates

  # Additional local scan paths
  local_paths:
    - ~/projects/*/templates # Glob patterns supported
    - /opt/templates
```

### Configuration Options

#### `repositories`

Maps repository names to their sources (git URLs or local paths).

**Format:**

```yaml
repositories:
  <name>: <git-url-or-local-path>
```

**Behaviors:**

- Git URLs are cloned and cached in `cache_dir`
- Local paths are scanned directly (no caching)
- Repositories are searched in definition order
- First match wins when building by template name

**Examples:**

```yaml
repositories:
  # Named for easy reference
  official: https://github.com/cowdogmoo/warpgate-templates.git

  # Custom name for organization
  security-tools: git@github.com:security/templates.git

  # Local development path
  dev: /Users/username/templates
```

#### `local_paths`

Additional local directories to scan for templates.

**When to use:**

- Development/testing directories
- Multiple template collections
- Shared team locations
- Network-mounted directories

**Format:**

```yaml
local_paths:
  - /path/to/directory
  - ~/relative/path
  - ../another/path
```

**Examples:**

```yaml
local_paths:
  - ~/dev/templates # Your development templates
  - /opt/shared/templates # Shared team templates
  - /mnt/nfs/company-templates # Network share
```

**How it works:**

Local paths are searched **after** repositories when building by template name.
This allows you to use `--template` flag with templates in local directories:

```bash
# Searches repositories first, then local_paths
warpgate build --template sliver
```

If `sliver` exists in both a repository and a local_path, the repository version
is used (repositories have higher priority).

#### `cache_dir`

Override the default cache directory for git repositories.

**Default:** `~/.cache/warpgate/templates` (XDG compliant)

**When to override:**

- Custom cache location needed
- Shared cache for multiple users
- Network-mounted cache

**Example:**

```yaml
cache_dir: /var/cache/warpgate/templates
```

## Managing Template Sources

### Adding Sources

#### Add Git Repository (Auto-named)

The repository name is extracted from the URL:

```bash
# Adds as 'warpgate-templates'
warpgate templates add https://github.com/cowdogmoo/warpgate-templates.git

# Adds as 'security-templates'
warpgate templates add https://github.com/security/security-templates.git
```

#### Add Git Repository (Custom Name)

Specify a custom name for easy reference:

```bash
# Adds as 'official'
warpgate templates add official https://github.com/cowdogmoo/warpgate-templates.git

# Adds as 'security'
warpgate templates add security git@github.com:myorg/templates.git
```

#### Add Local Directory

Provide an absolute or relative path:

```bash
# Absolute path
warpgate templates add /Users/username/my-templates

# Home directory
warpgate templates add ~/warpgate-templates

# Relative path
warpgate templates add ../templates
```

**What happens:**

- Git URLs are added to `repositories` and cloned
- Local paths are added to `repositories`
- Configuration file is updated automatically
- Templates are immediately available

### Removing Sources

#### Remove by Name (Git Repositories)

```bash
# Remove named repository
warpgate templates remove official

# Remove private repository
warpgate templates remove security-tools
```

#### Remove by Path (Local Directories)

```bash
# Remove by absolute path
warpgate templates remove /Users/username/my-templates

# Remove by home directory path
warpgate templates remove ~/warpgate-templates

# Remove by relative path
warpgate templates remove ../templates
```

**What happens:**

- Repository is removed from configuration
- Cached data remains (for git repos) but won't be used
- To fully clean up, delete cache directory manually

### Updating Cache

Force refresh of all git repository caches:

```bash
warpgate templates update
```

**When to update:**

- After upstream template changes
- Periodic refresh (daily/weekly)
- Before important builds
- After adding new repositories

**What happens:**

- Pulls latest changes for all git repositories
- Local directories are always live (no caching)
- Template metadata is refreshed

## Using Templates

### Discovering Templates

**List all available templates:**

```bash
warpgate templates list
```

**Output:**

```text
NAME              VERSION   SOURCE      DESCRIPTION
attack-box        1.0.0     official    Security testing environment
sliver            1.0.0     official    Sliver C2 framework
atomic-red-team   1.0.0     official    Atomic Red Team test platform
custom-tool       2.1.0     private     Custom security tool
dev-template      0.1.0     local       Development template
```

**Discover templates (alias for list):**

```bash
warpgate discover
```

**Get detailed template information:**

```bash
warpgate templates info attack-box
```

**Output:**

```text
Template: attack-box
Version: 1.0.0
Source: official
Description: Comprehensive security testing environment

Base Image: ubuntu:22.04

Provisioners:
  1. shell - Update system packages
  2. ansible - Install security tools
  3. shell - Configure environment

Targets:
  - container (linux/amd64, linux/arm64)

Variables:
  TOOLS_PATH: /opt/tools (Installation path for tools)
  ENABLE_GUI: false (Install GUI tools)
```

### Building from Templates

#### By Template Name

Searches all repositories for a match:

```bash
# Build from any configured repository
warpgate build attack-box

# Build specific architecture
warpgate build attack-box --arch amd64

# Build with variables
warpgate build sliver --var ARSENAL_PATH=/custom/path
```

#### By Local Directory Path

Directly reference a template directory:

```bash
# Build from local template directory
warpgate build /Users/username/templates/attack-box

# Relative path
warpgate build ../templates/custom-tool
```

#### By Local File

Reference a specific warpgate.yaml file:

```bash
# Build from specific file
warpgate build /path/to/warpgate.yaml

# Current directory
warpgate build ./warpgate.yaml
```

#### By Git URL

Build directly from a git repository:

```bash
# Public repository
warpgate build --from-git https://github.com/user/repo.git//templates/attack-box

# Private repository (requires SSH key)
warpgate build --from-git git@github.com:myorg/repo.git//path/to/template

# Specific branch or tag
warpgate build --from-git https://github.com/user/repo.git//templates/tool?ref=v2.0.0
```

### Template Discovery Order

When building by template name with `--template` flag, warpgate searches in
this order:

1. **Repositories** (in configuration order)

   ```yaml
   repositories:
     official: ... # Searched first
     private: ... # Searched second
     local: ... # Searched third
   ```

2. **Local Paths** (in configuration order, searched after all repositories)

   ```yaml
   local_paths:
     - ~/dev/templates # Searched after repositories
     - /opt/templates # Searched last
   ```

**First match wins!**

**Examples:**

**Example 1:** Template in repository takes precedence

```yaml
repositories:
  official: https://github.com/cowdogmoo/warpgate-templates.git
local_paths:
  - ~/dev/templates
```

If `sliver` exists in both `official` and `~/dev/templates`, the `official`
version is used (repositories are searched first).

**Example 2:** Template only in local_paths

```yaml
repositories:
  official: https://github.com/cowdogmoo/warpgate-templates.git
local_paths:
  - ~/dev/templates # Contains 'custom-tool'
```

```bash
# This now works! Searches repositories, then local_paths
warpgate build --template custom-tool
```

**Tip:** Order repositories by priority in your config. Templates in repositories
always have higher priority than templates in local_paths.

## Template Directory Structure

Templates must follow this structure for automatic discovery:

### Standard Structure

```text
repository-root/
└── templates/
    ├── attack-box/
    │   ├── warpgate.yaml       # Required: template definition
    │   ├── playbook.yml        # Optional: provisioner files
    │   ├── requirements.yml    # Optional: dependencies
    │   └── scripts/            # Optional: provisioner scripts
    │       └── setup.sh
    ├── sliver/
    │   ├── warpgate.yaml
    │   └── ansible/
    │       └── ...
    └── custom-tool/
        └── warpgate.yaml
```

**Requirements:**

- Top-level `templates/` directory is **required**
- Each template in its own subdirectory
- Each subdirectory must contain `warpgate.yaml`
- Additional files (playbooks, scripts) are relative to template directory

### Local Development Structure

For local development or single templates:

```text
my-template/
├── warpgate.yaml
├── playbook.yml
└── scripts/
    └── setup.sh
```

Use this by:

```bash
warpgate build /path/to/my-template
```

### Multi-template Local Structure

For multiple templates in a local directory:

```text
my-templates/
└── templates/
    ├── template1/
    │   └── warpgate.yaml
    └── template2/
        └── warpgate.yaml
```

Add to config:

```yaml
repositories:
  local: /path/to/my-templates
```

## Private Repositories

### SSH Authentication (Recommended)

**Setup:**

```bash
# Generate SSH key (if needed)
ssh-keygen -t ed25519 -C "your.email@example.com"

# Add to SSH agent
ssh-add ~/.ssh/id_ed25519

# Add public key to GitHub/GitLab
cat ~/.ssh/id_ed25519.pub
# Copy and add to your account settings
```

**Configuration:**

```yaml
templates:
  repositories:
    private: git@github.com:myorg/private-templates.git
    gitlab: git@gitlab.company.com:infra/templates.git
```

**Test access:**

```bash
# Test GitHub access
ssh -T git@github.com

# Test GitLab access
ssh -T git@gitlab.company.com
```

### HTTPS Authentication

**Using Git Credential Helper:**

```bash
# Configure credential helper
git config --global credential.helper store

# Or use OS-specific helpers:
# macOS
git config --global credential.helper osxkeychain

# Linux (GNOME Keyring)
git config --global credential.helper /usr/share/doc/git/contrib/credential/libsecret/git-credential-libsecret

# Windows
git config --global credential.helper wincred
```

**Configuration:**

```yaml
templates:
  repositories:
    private: https://github.com/myorg/private-templates.git
```

**First Use:**

```bash
# First template operation will prompt for credentials
warpgate templates add https://github.com/myorg/private-templates.git
# Enter username and password/token when prompted
# Credentials are saved by helper for future use
```

### GitHub Personal Access Token

For GitHub HTTPS access:

```bash
# Create token at: https://github.com/settings/tokens
# Required scopes: repo (full control)

# Use token as password
git config --global credential.helper store
warpgate templates add https://github.com/myorg/private-templates.git
# Username: your-github-username
# Password: ghp_your_personal_access_token
```

## Environment Variables

Override configuration using environment variables:

### Override Repositories

```bash
# Single repository (JSON format)
export WARPGATE_TEMPLATES_REPOSITORIES='{"local": "/path/to/templates"}'

# Multiple repositories
export WARPGATE_TEMPLATES_REPOSITORIES='{"official": "https://github.com/cowdogmoo/warpgate-templates.git", "local": "/path/to/local"}'
```

### Override Local Paths

```bash
# Single path
export WARPGATE_TEMPLATES_LOCAL_PATHS='["/path/to/templates"]'

# Multiple paths
export WARPGATE_TEMPLATES_LOCAL_PATHS='["/path/one", "/path/two", "/path/three"]'
```

### Override Cache Directory

```bash
export WARPGATE_TEMPLATES_CACHE_DIR="/custom/cache/location"
```

### Use in Scripts

```bash
#!/bin/bash
# Build with temporary template source
export WARPGATE_TEMPLATES_REPOSITORIES='{"temp": "/tmp/test-templates"}'
warpgate build test-template

# Environment override takes precedence over config file
```

## Common Scenarios

### Development Setup

Local development with official templates as fallback:

```yaml
templates:
  repositories:
    dev: ~/dev/warpgate-templates # Local development (checked first)
    official: https://github.com/cowdogmoo/warpgate-templates.git # Fallback
```

### Production/Enterprise Setup

Private company templates with official fallback:

```yaml
templates:
  repositories:
    company: git@gitlab.company.com:infra/templates.git # Private company templates
    official: https://github.com/cowdogmoo/warpgate-templates.git # Official templates
  local_paths:
    - /mnt/nfs/shared-templates # Shared team templates
  cache_dir: /var/cache/warpgate/templates # Shared cache
```

### CI/CD Setup

Use environment variables for flexibility:

```bash
export WARPGATE_TEMPLATES_REPOSITORIES='{"ci": "https://github.com/myorg/templates.git?ref=v2.0.0"}'
export WARPGATE_TEMPLATES_CACHE_DIR="/tmp/warpgate-cache"
warpgate build production-image --arch amd64,arm64
```

## Troubleshooting

### Templates Not Found

**Symptoms:**

```text
Error: template 'attack-box' not found
```

**Diagnosis:**

```bash
# Check configuration
cat ~/.config/warpgate/config.yaml

# List available templates
warpgate templates list

# Check specific template
warpgate templates info attack-box
```

**Solutions:**

1. **Update cache (for git repositories):**

   ```bash
   warpgate templates update
   ```

2. **Verify repository is configured:**

   ```bash
   # Add the repository
   warpgate templates add https://github.com/cowdogmoo/warpgate-templates.git
   ```

3. **Check if template is in local_paths:**

   If your template is in a local directory, ensure it's configured:

   ```yaml
   templates:
     local_paths:
       - /path/to/your/templates
   ```

   Then verify the structure:

   ```bash
   # Template should be at:
   ls /path/to/your/templates/templates/attack-box/warpgate.yaml
   ```

   Now you can use:

   ```bash
   warpgate build --template attack-box
   ```

4. **Check repository accessibility:**

   ```bash
   # For git repositories
   git ls-remote https://github.com/cowdogmoo/warpgate-templates.git

   # For local directories
   ls -la /path/to/templates/templates/
   ```

5. **Verify directory structure:**

   ```bash
   # Templates must be in templates/ subdirectory
   # ✓ Correct: repository/templates/attack-box/warpgate.yaml
   # ✗ Wrong: repository/attack-box/warpgate.yaml
   ```

### Duplicate Templates

**Symptoms:**

Multiple templates with the same name from different sources.

**Diagnosis:**

```bash
warpgate templates list
# Shows source for each template
```

**Understanding precedence:**

Templates are found in configuration order:

1. First `repositories` entry
2. Second `repositories` entry
3. First `local_paths` entry
4. Second `local_paths` entry

**Solutions:**

1. **Remove duplicate source:**

   ```bash
   warpgate templates remove duplicate-source
   ```

2. **Reorder repositories:**

   ```yaml
   repositories:
     priority-first: ... # This wins for duplicates
     priority-second: ...
   ```

3. **Use full path:**

   ```bash
   # Build from specific location
   warpgate build /path/to/specific/template
   ```

### Private Repository Access Failed

**Symptoms:**

```text
Error: failed to clone repository: authentication failed
```

**Diagnosis:**

```bash
# Test SSH access
ssh -T git@github.com

# Test git access
git ls-remote git@github.com:myorg/private-templates.git
```

**Solutions:**

1. **Setup SSH key:**

   ```bash
   ssh-keygen -t ed25519 -C "your.email@example.com"
   ssh-add ~/.ssh/id_ed25519
   # Add public key to GitHub/GitLab
   ```

2. **Configure git credentials:**

   ```bash
   git config --global credential.helper store
   # Or use OS-specific helper
   ```

3. **Use HTTPS with token:**

   ```bash
   # GitHub: Create personal access token
   # Use as password when prompted
   ```

4. **Verify repository URL:**

   ```yaml
   # SSH format
   git@github.com:org/repo.git

   # HTTPS format
   https://github.com/org/repo.git
   ```

### Cache is Stale

**Symptoms:**

Old template versions are used despite upstream changes.

**Solution:**

```bash
# Update all git repository caches
warpgate templates update

# Or manually clear cache
rm -rf ~/.cache/warpgate/templates/*
warpgate templates update
```

### Template Build Fails

**Symptoms:**

Template builds successfully from direct path but fails by name.

**Diagnosis:**

```bash
# Check template source
warpgate templates info template-name

# Verify it's finding the right template
warpgate templates list | grep template-name
```

**Solutions:**

1. **Use full path:**

   ```bash
   warpgate build /full/path/to/template
   ```

2. **Check discovery order:**

   ```yaml
   # Ensure correct repository is first
   repositories:
     correct-source: ... # Move this up
   ```

3. **Verify template structure:**

   ```bash
   # Must have warpgate.yaml
   ls /path/to/template/warpgate.yaml
   ```

### Permission Denied on Cache Directory

**Symptoms:**

```text
Error: failed to write cache: permission denied
```

**Solution:**

```bash
# Fix cache directory permissions
mkdir -p ~/.cache/warpgate/templates
chmod 755 ~/.cache/warpgate/templates

# Or use custom cache directory
export WARPGATE_TEMPLATES_CACHE_DIR=/tmp/warpgate-cache
```

## Best Practices

### Organization

1. **Use descriptive repository names:**

   ```yaml
   repositories:
     official: ... # ✓ Clear
     company: ... # ✓ Clear
     repo1: ... # ✗ Vague
   ```

2. **Order by priority:**

   ```yaml
   repositories:
     production: ... # Highest priority
     staging: ... # Medium priority
     development: ... # Lowest priority
   ```

3. **Group related templates:**

   ```text
   templates/
   ├── security/
   │   ├── attack-box/
   │   ├── defense-box/
   │   └── forensics/
   └── infrastructure/
       ├── base-ubuntu/
       └── base-alpine/
   ```

### Security

1. **Use SSH for private repositories:**

   ```yaml
   repositories:
     private: git@github.com:org/repo.git # ✓ SSH
     # Not: https://github.com/org/repo.git  # ✗ Requires token management
   ```

2. **Never commit credentials:**

   - Use git credential helpers
   - Use SSH keys, not passwords
   - Don't store tokens in config files

3. **Audit template sources:**

   ```bash
   # Regularly review configured sources
   cat ~/.config/warpgate/config.yaml
   ```

### Maintenance

1. **Update caches regularly:**

   ```bash
   # Weekly or daily
   warpgate templates update
   ```

2. **Clean old caches:**

   ```bash
   # Remove unused cached repositories
   rm -rf ~/.cache/warpgate/templates/unused-repo
   ```

3. **Version your templates:**

   ```yaml
   metadata:
     version: 2.1.0 # Use semantic versioning
   ```

4. **Document templates:**

   ```yaml
   metadata:
     description: "Clear description of what this builds"
     author: "Team or person responsible"
   ```

### Performance

1. **Use local paths for frequently used templates:**

   ```yaml
   # Faster than git repositories
   repositories:
     dev: ~/dev/templates # No network access needed
   ```

2. **Set reasonable cache expiry:**

   ```bash
   # Update daily for active development
   # Update weekly for stable templates
   ```

3. **Minimize local_paths entries:**

   ```yaml
   # Fewer paths = faster discovery
   local_paths:
     - /opt/templates # Only what you need
   ```

### Collaboration

1. **Share template repositories:**

   - Use git for version control
   - Provide clear README in repository
   - Use branches for development

2. **Document configuration:**

   ```yaml
   # Add comments in config.yaml
   repositories:
     # Official Warpgate templates (maintained by community)
     official: https://github.com/cowdogmoo/warpgate-templates.git

     # Company-internal templates (requires VPN)
     company: git@gitlab.internal.com:infra/templates.git
   ```

3. **Use consistent template structure:**
   - Follow standard directory layout
   - Include metadata in all templates
   - Document variables and requirements

## Summary

**Key Takeaways:**

- ✅ Templates can come from git repositories or local directories
- ✅ Configuration is in `~/.config/warpgate/config.yaml`
- ✅ Git repositories are automatically cached
- ✅ Templates are discovered in configuration order
- ✅ Private repositories require SSH or HTTPS authentication
- ✅ Use `warpgate templates` commands to manage sources
- ✅ Update cache regularly with `warpgate templates update`

**Quick Reference:**

```bash
# List templates
warpgate templates list

# Add source
warpgate templates add <url-or-path>

# Remove source
warpgate templates remove <name-or-path>

# Update cache
warpgate templates update

# Build from template
warpgate build <template-name>
```
