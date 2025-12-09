# Installation Guide

Comprehensive installation instructions for Warpgate on all platforms.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
- [Platform-Specific Instructions](#platform-specific-instructions)
- [Verify Installation](#verify-installation)

## Prerequisites

### Required

- **Go 1.21+** - For building from source ([install Go](https://go.dev/doc/install))
- **Docker or Podman** - For containerized execution (recommended for macOS/Windows)
- **Linux** - Required for native Buildah integration

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

Warpgate runs natively on Linux with full Buildah integration.

#### Requirements

- Linux kernel with container support
- runc or crun runtime

#### Configuration Steps

**1. Configure storage and runtime:**

Create `~/.config/warpgate/config.yaml`:

```yaml
storage:
  driver: vfs

container:
  runtime: runc
```

**2. Configure container runtime:**

Create or edit `/etc/containers/containers.conf`:

```toml
[engine]
runtime = "runc"

[engine.runtimes]
runc = [
  "/usr/bin/runc"
]
```

**3. Build templates:**

```bash
# Build with sudo for container operations
sudo warpgate build attack-box --arch amd64

# Pass variables via CLI (recommended)
sudo warpgate build sliver \
  --var ARSENAL_REPO_PATH=/path/to/arsenal \
  --arch amd64

# Or use variable files
sudo warpgate build sliver --var-file vars.yaml --arch amd64

# Legacy: preserve environment variables with sudo -E
export ARSENAL_REPO_PATH=/path/to/arsenal
sudo -E warpgate build sliver --arch amd64
```

**Variable precedence:** CLI flags (`--var`) > Variable files
(`--var-file`) > Environment variables

**Important:** The `runc` runtime and `vfs` storage driver are critical for
proper builds on Linux.

### macOS (Containerized)

On macOS, use Docker Desktop or the containerized version of Warpgate.

#### Option 1: Using Build Scripts

```bash
# Build any template using the provided script
bash scripts/build-template.sh sliver

# The script automatically handles Docker Desktop integration
```

#### Option 2: Container Image

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

### Windows (Containerized)

Use Docker Desktop on Windows with the containerized version.

**Prerequisites:**

- [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/)
- WSL2 enabled (recommended)

**Installation steps:**

```powershell
# Pull warpgate image
docker pull ghcr.io/cowdogmoo/warpgate:latest

# Build from template
docker run --rm `
  -v ${PWD}:/workspace `
  ghcr.io/cowdogmoo/warpgate:latest `
  build /workspace/warpgate.yaml --arch amd64

# Create alias (PowerShell)
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
warpgate discover

# Validate a template
warpgate validate <template-name>
```

## Next Steps

After installing Warpgate:

1. **Configure Warpgate** - Set up global configuration ([Configuration Guide](configuration.md))
2. **Discover templates** - Find available templates (`warpgate discover`)
3. **Build your first image** - Follow the [Usage Guide](usage-guide.md)
4. **Create custom templates** - See [Template Format](template-format.md)

## Troubleshooting

### Installation Issues

#### Go install fails with "module not found"

```bash
# Ensure Go is properly installed
go version

# Clear module cache and retry
go clean -modcache
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest
```

#### Binary not found after installation

```bash
# Check if $GOPATH/bin is in PATH
echo $PATH | grep "$(go env GOPATH)/bin"

# Add to PATH if missing
export PATH="$PATH:$(go env GOPATH)/bin"
```

#### Container image pull fails

```bash
# Check Docker is running
docker ps

# Try explicit version
docker pull ghcr.io/cowdogmoo/warpgate:v1.x.x

# Check network/registry connectivity
ping ghcr.io
```

For more help, see the [Troubleshooting Guide](troubleshooting.md) or [open an issue](https://github.com/CowDogMoo/warpgate/issues).

---

**Need help?** [Open an issue](https://github.com/CowDogMoo/warpgate/issues) or
ask in [Discussions](https://github.com/CowDogMoo/warpgate/discussions).
