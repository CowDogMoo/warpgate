# Template Configuration and Repository Management

Complete guide to managing Warpgate template sources, repositories, and discovery.

## Table of Contents

- [Overview](#overview)
- [Configuration](#configuration)
  - [Configuration File Locations](#configuration-file-locations)
  - [Basic Configuration](#basic-configuration)
  - [Repository Types](#repository-types)
  - [Configuration Options](#configuration-options)
- [Template Discovery Order](#template-discovery-order)
- [Template Directory Structure](#template-directory-structure)
- [Private Repositories](#private-repositories)
- [Environment Variables](#environment-variables)
- [Common Scenarios](#common-scenarios)
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

**Default behavior:** Warpgate always includes the official repository
(`https://github.com/cowdogmoo/warpgate-templates.git`) automatically.

**Add custom repositories:**

```yaml
templates:
  # Add your custom repositories (official repo is included automatically)
  repositories:
    private: git@github.com:myorg/private-templates.git
    company: https://gitlab.com/company/templates.git

  # Additional local paths to scan
  local_paths:
    - /opt/shared/templates
    - ~/dev/templates

  # Custom cache directory (optional)
  cache_dir: ~/.cache/warpgate/templates
```

**Disable the official repository:**

```yaml
templates:
  repositories:
    official: "" # Empty string disables the official repository
    private: git@github.com:myorg/private-templates.git
```

### Repository Types

#### Git Repositories

The `repositories` field accepts **Git URLs only**. Local directories must be
configured in `local_paths`.

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

Local template directories must be configured in `local_paths`, not `repositories`.

```yaml
templates:
  # Git repositories ONLY
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    private: git@github.com:myorg/private-templates.git

  # Local directories ONLY
  local_paths:
    - /opt/company/templates # Absolute path
    - ~/dev/warpgate-templates # Home directory
    - /mnt/nfs/shared-templates # Network mount
```

**Key distinction:**

- `repositories`: Git URLs only (cloned and cached)
- `local_paths`: Local filesystem directories only (used directly)

### Configuration Options

#### `repositories`

Maps repository names to Git URLs. **Local paths are not allowed** - use
`local_paths` instead.

**Format:**

```yaml
repositories:
  <name>: <git-url>
```

**Behaviors:**

- Only Git URLs are accepted (https://, git@)
- Repositories are cloned and cached in `cache_dir`
- Searched in definition order
- First match wins when building by template name
- Attempting to use a local path will result in an error

**Examples:**

```yaml
repositories:
  # Named for easy reference
  official: https://github.com/cowdogmoo/warpgate-templates.git

  # Custom name for organization
  security-tools: git@github.com:security/templates.git

  # Private repository
  company: git@gitlab.company.com:infra/templates.git
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

See [Commands Reference](commands.md#templates) for template management commands.

## Template Discovery Order

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
local_paths:
  - /path/to/my-templates
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
# Single repository (JSON format, Git URLs only)
export WARPGATE_TEMPLATES_REPOSITORIES='{"official": "https://github.com/cowdogmoo/warpgate-templates.git"}'

# Multiple repositories (Git URLs only)
export WARPGATE_TEMPLATES_REPOSITORIES='{"official": "https://github.com/cowdogmoo/warpgate-templates.git", "private": "git@github.com:myorg/templates.git"}'
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
# Build with temporary Git repository
export WARPGATE_TEMPLATES_REPOSITORIES='{"ci": "https://github.com/myorg/templates.git"}'
warpgate build test-template

# Or use local path for testing
export WARPGATE_TEMPLATES_LOCAL_PATHS='["/tmp/test-templates"]'
warpgate build test-template

# Environment override takes precedence over config file
```

## Common Scenarios

### Development Setup

Local development with official templates as fallback:

```yaml
templates:
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git # Git repos
  local_paths:
    - ~/dev/warpgate-templates # Local development (checked after repos)
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
   # Faster than git repositories (no cloning/caching)
   local_paths:
     - ~/dev/templates # Direct filesystem access
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
