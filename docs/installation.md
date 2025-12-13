# Installation Guide

Comprehensive installation instructions for Warpgate on all platforms.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
- [Platform-Specific Instructions](#platform-specific-instructions)
- [Verify Installation](#verify-installation)
- [Shell Completion (Optional)](#shell-completion-optional)
- [Next Steps](#next-steps)

## Prerequisites

### Required

- **Go 1.25+** - For building from source ([install Go](https://go.dev/doc/install))
- **Docker** - Docker daemon with BuildKit support (required for all platforms)
  - [Docker Desktop](https://www.docker.com/products/docker-desktop/) for macOS/Windows
  - [Docker Engine](https://docs.docker.com/engine/install/) for Linux

### Optional

- [Task](https://taskfile.dev/) - For development tasks (`brew install go-task`)
- [Ansible](https://www.ansible.com/) - If using Ansible provisioners in templates

## Installation Methods

### Option 1: Go Install (Recommended)

Install the latest release directly from source:

```bash
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest
```

This installs the `warpgate` binary to `$GOPATH/bin` (usually `~/go/bin`).

**Ensure `$GOPATH/bin` is in your PATH:**

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Option 2: From Source

Build from source for development or customization:

```bash
# Clone repository
gh repo clone CowDogMoo/warpgate
cd warpgate

# Build using Taskfile (recommended)
task go:build

# Or build with go directly
go build -ldflags "-s -w -X main.version=$(git describe --tags --always)" -o warpgate ./cmd/warpgate

# Install to $GOPATH/bin
task go:install
```

### Option 3: Container Image

Use the pre-built container image:

```bash
# Pull the latest image
docker pull ghcr.io/cowdogmoo/warpgate:latest

# Create an alias for convenience
alias warpgate='docker run --rm -v $(pwd):/workspace ghcr.io/cowdogmoo/warpgate:latest'

# Use it like the native command
warpgate --version
```

**Add to your shell profile for persistence:**

```bash
# For bash: ~/.bashrc
# For zsh: ~/.zshrc
echo "alias warpgate='docker run --rm -v \$(pwd):/workspace ghcr.io/cowdogmoo/warpgate:latest'" >> ~/.bashrc
```

### Option 4: Pre-built Binaries

Download pre-built binaries from the [releases page](https://github.com/CowDogMoo/warpgate/releases).

**Installation steps:**

```bash
# Download for your platform (example: Linux amd64)
curl -LO https://github.com/CowDogMoo/warpgate/releases/download/v1.x.x/warpgate-linux-amd64

# Make executable
chmod +x warpgate-linux-amd64

# Move to PATH location
sudo mv warpgate-linux-amd64 /usr/local/bin/warpgate

# Verify
warpgate --version
```

## Platform-Specific Instructions

### Linux (Native Execution)

Warpgate runs natively on Linux using Docker BuildKit for container builds.

#### Requirements

- **Docker Engine** installed and running ([installation guide](https://docs.docker.com/engine/install/))
- **Docker group membership** to access Docker daemon

#### Docker Setup

**1. Install Docker Engine:**

Follow the official Docker installation guide for your Linux distribution:

- [Ubuntu](https://docs.docker.com/engine/install/ubuntu/)
- [Fedora](https://docs.docker.com/engine/install/fedora/)
- [Debian](https://docs.docker.com/engine/install/debian/)

**2. Add user to docker group:**

```bash
# Add current user to docker group
sudo usermod -aG docker $USER

# Log out and back in for changes to take effect
# Or run: newgrp docker

# Verify Docker access
docker ps
```

**3. Configure warpgate (optional):**

Create `~/.config/warpgate/config.yaml` to customize settings:

```yaml
buildkit:
  endpoint: "" # Empty = auto-detect local buildx builder
  tls_enabled: false

container:
  default_platforms: [linux/amd64, linux/arm64]
```

See [CLI Configuration Guide](cli-configuration.md) for all available options.

#### Building Templates

Once Docker group membership is configured, build templates:

```bash
# Build templates
warpgate build attack-box --arch amd64

# Pass variables via CLI (recommended)
warpgate build sliver \
  --var PROVISION_REPO_PATH=/path/to/arsenal \
  --arch amd64

# Or use variable files
warpgate build sliver --var-file vars.yaml --arch amd64

# Multi-architecture builds
warpgate build attack-box --arch amd64,arm64
```

**Variable precedence:** CLI flags (`--var`) > Variable files
(`--var-file`) > Environment variables

**Note:** Warpgate uses Docker BuildKit for all container image builds.
BuildKit runs as a Docker container (`buildx_buildkit_*`) and is auto-detected
by warpgate.

### macOS

On macOS, use Docker Desktop with either native warpgate or the containerized version.

#### macOS: Native Installation

```bash
# Install via Go
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

# Ensure Docker Desktop is running
docker ps

# Build templates
warpgate build attack-box --arch amd64
```

#### macOS: Containerized

```bash
# Pull warpgate image
docker pull ghcr.io/cowdogmoo/warpgate:latest

# Validate template
docker run --rm \
  -v $(pwd):/workspace \
  ghcr.io/cowdogmoo/warpgate:latest \
  validate /workspace/warpgate.yaml

# Build image
docker run --rm \
  -v $(pwd):/workspace \
  ghcr.io/cowdogmoo/warpgate:latest \
  build /workspace/warpgate.yaml --arch amd64

# Create alias for convenience
alias warpgate='docker run --rm -v $(pwd):/workspace ghcr.io/cowdogmoo/warpgate:latest'
warpgate build mytemplate
```

### Windows

On Windows, use Docker Desktop with either native warpgate or the
containerized version.

**Prerequisites:**

- [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/)
- WSL2 enabled (recommended)
- [Go 1.25+](https://go.dev/doc/install) for native installation

#### Windows: Native Installation

```powershell
# Install via Go
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

# Ensure Go bin is in PATH
$env:Path += ";$env:USERPROFILE\go\bin"

# Ensure Docker Desktop is running
docker ps

# Build templates
warpgate build attack-box --arch amd64
```

**Add Go bin to PATH permanently:**

```powershell
# Add to user PATH
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:USERPROFILE\go\bin", "User")

# Restart PowerShell to apply changes
```

#### Windows: Containerized

```powershell
# Pull warpgate image
docker pull ghcr.io/cowdogmoo/warpgate:latest

# Build from template
docker run --rm `
  -v ${PWD}:/workspace `
  ghcr.io/cowdogmoo/warpgate:latest `
  build /workspace/warpgate.yaml --arch amd64

# Create function alias (PowerShell)
function warpgate { docker run --rm -v ${PWD}:/workspace ghcr.io/cowdogmoo/warpgate:latest @args }

# Use the alias
warpgate validate mytemplate.yaml
```

**Add alias to PowerShell profile for persistence:**

```powershell
# Open profile
notepad $PROFILE

# Add function
function warpgate { docker run --rm -v ${PWD}:/workspace ghcr.io/cowdogmoo/warpgate:latest @args }

# Save and reload
. $PROFILE
```

## Verify Installation

Check that Warpgate is installed correctly:

```bash
# Check version
warpgate --version
# Expected output: warpgate version v1.x.x

# Test basic functionality
warpgate templates list

# Validate a template
warpgate validate <template-name>
```

## Shell Completion (Optional)

Warpgate supports shell completion for bash, zsh, fish, and PowerShell.

### Bash

```bash
# Load completions for current session
source <(warpgate completion bash)

# Load completions for all sessions (Linux)
warpgate completion bash > /etc/bash_completion.d/warpgate

# Load completions for all sessions (macOS)
warpgate completion bash > $(brew --prefix)/etc/bash_completion.d/warpgate
```

### Zsh

```bash
# Enable completion system (if not already enabled)
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Install completion script
warpgate completion zsh > "${fpath[1]}/_warpgate"

# Restart shell or reload
exec zsh
```

### Fish

```bash
# Load completions for current session
warpgate completion fish | source

# Install for all sessions
warpgate completion fish > ~/.config/fish/completions/warpgate.fish
```

### PowerShell

```powershell
# Load completions for current session
warpgate completion powershell | Out-String | Invoke-Expression

# Install for all sessions
warpgate completion powershell > warpgate.ps1
# Then add to your PowerShell profile: . /path/to/warpgate.ps1
```

## Next Steps

After installing Warpgate:

1. **Configure Warpgate** - Set up global configuration ([CLI Configuration Guide](cli-configuration.md))
2. **List templates** - Find available templates (`warpgate templates list`)
3. **Build your first image** - Follow the [Usage Guide](usage-guide.md)
4. **Create custom templates** - See [Template Reference](template-reference.md)
