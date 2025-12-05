# Warp Gate

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![🚨 Semgrep Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-Commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)
[![Renovate](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml)

<img src="docs/images/wg-logo.jpeg" alt="Warp Gate Logo" width="100%">

**Warp Gate** is a Go CLI tool for building container images and AWS AMIs. It
uses [Buildah](https://buildah.io/) as a library for container builds and AWS
[SDK](https://aws.amazon.com/sdk-for-go/) for AMI creation.

---

## Installation

### Using Go Install

```bash
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest
```

### From Source

```bash
# Clone the repository
gh repo clone CowDogMoo/warpgate
cd warpgate

# Build using Taskfile (recommended)
task go:build

# Or build directly with go
go build -ldflags "-s -w -X main.version=$(git describe --tags --always)" -o warpgate ./cmd/warpgate

# Or install to $GOPATH/bin
task go:install
```

---

## Quick Start

### Build from a template file

```bash
# Validate template
warpgate validate warpgate.yaml

# Build container image
warpgate build warpgate.yaml

# Build with custom architectures
warpgate build warpgate.yaml --arch amd64,arm64

# Build and push
warpgate build warpgate.yaml --push --registry ghcr.io/myorg
```

### Build from git repository

```bash
# Build from a git URL
warpgate build --from-git https://github.com/cowdogmoo/warpgate-templates.git//templates/attack-box
```

### Template discovery

```bash
# Discover templates in configured sources
warpgate discover

# List templates
warpgate templates list

# Get template info
warpgate templates info attack-box
```

### Multi-architecture manifests

```bash
# Create and push multi-arch manifest
warpgate manifest create \
  --name myorg/myimage:latest \
  --images myorg/myimage:latest-amd64,myorg/myimage:latest-arm64

# Push manifest
warpgate manifest push myorg/myimage:latest
```

---

## Configuration

Warpgate uses two configuration systems:

1. **Global config** (`~/.warpgate/config.yaml`) - User preferences, registry
   credentials, build defaults
2. **Template config** (`warpgate.yaml`) - Image definitions (portable and shareable)

### Example Template

```yaml
metadata:
  name: my-image
  version: 1.0.0
  description: "My custom image"

name: my-image

base:
  image: ubuntu:22.04

provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y python3 ansible

  - type: ansible
    playbook_path: playbook.yml
    galaxy_file: requirements.yml

targets:
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64
```

---

## Development

### Prerequisites

**For building from source:**

- [Go 1.21+](https://go.dev/doc/install)
- [go-task](https://taskfile.dev/installation/) (optional but recommended)
  (`brew install go-task/tap/go-task` or see docs)

**For runtime (depending on use case):**

- [Docker](https://www.docker.com/) or [Podman](https://podman.io/)
  (for container builds)
- [Buildah](https://buildah.io/) (embedded as library, no installation needed)

**For provisioners (optional, depending on template):**

- [Ansible](https://www.ansible.com/) (for ansible provisioner)
- [PowerShell](https://github.com/PowerShell/PowerShell) (for pwsh provisioner)

### Clone the Repo

```bash
gh repo clone CowDogMoo/warpgate
cd warpgate
```

### Building and Testing

```bash
# Build for current platform
task go:build

# Run tests
task go:test

# Run tests with coverage
task go:test-coverage

# Build for all platforms
task go:build-all

# Format code
task go:fmt

# Run linter
task go:lint

# Clean build artifacts
task go:clean
```

---

## Commands

### Build

```bash
# Build from local template file
warpgate build warpgate.yaml

# Build from git repository
warpgate build --from-git https://github.com/user/repo.git//path/to/template

# Build specific architectures
warpgate build warpgate.yaml --arch amd64,arm64

# Build and push to registry
warpgate build warpgate.yaml --push --registry ghcr.io/myorg

# Save digests after build and push
warpgate build warpgate.yaml --push --save-digests --digest-dir ./digests
```

### Templates

```bash
# Discover templates from configured sources
warpgate discover

# List available templates
warpgate templates list

# Show template information
warpgate templates info <template-name>
```

### Validation

```bash
# Validate template configuration
warpgate validate warpgate.yaml
```

### Manifests

```bash
# Create multi-arch manifest
warpgate manifest create \
  --name myorg/myimage:latest \
  --images myorg/myimage:amd64,myorg/myimage:arm64

# Push manifest to registry
warpgate manifest push myorg/myimage:latest

# Inspect manifest
warpgate manifest inspect myorg/myimage:latest
```

### Convert

```bash
# Convert Packer template to warpgate format
warpgate convert packer-template.pkr.hcl
```

---

## Available Taskfile Commands

The project uses modular taskfiles for development tasks. All Go-specific
tasks use the `go:` namespace:

### Build Commands

- `task go:build` - Build for current platform
- `task go:build OS=linux ARCH=amd64` - Build for specific platform
- `task go:build-all` - Build for all supported platforms (Linux, macOS, Windows)
- `task go:install` - Install to $GOPATH/bin

### Development Commands

- `task go:test` - Run tests with race detector
- `task go:test-coverage` - Generate HTML coverage report
- `task go:lint` - Run golangci-lint
- `task go:fmt` - Format code with go fmt
- `task go:tidy` - Tidy go modules
- `task go:clean` - Clean build artifacts

### Running

- `task go:run` - Build and run warpgate
- `task go:dev` - Run in development mode with verbose logging
- `task go:show-arch` - Show detected architecture

### CI/CD Commands

- `task go:ci-build` - Build for CI/CD environment
- `task go:ci-test` - Run tests with JSON output for CI
- `task go:release TAG=v1.0.0` - Create and push release tag

### Other Commands

- `task pre-commit:run` - Run pre-commit hooks
- `task github:pr-merge` - Merge current PR with cleanup
- See [taskfile-templates](https://github.com/CowDogMoo/taskfile-templates)
  for full list of available tasks

---

## Registry Authentication

You must have a **Classic GitHub Personal Access Token** with `write:packages`,
`read:packages`, and `delete:packages` scopes. If you are only pushing to a
single namespace in CI, you can use a fine-grained token.

**Web:**

- Go to GitHub > Settings > Developer Settings > Personal Access Tokens
- Create a token with the required scopes.

**CLI:**

```bash
gh auth refresh --scopes write:packages,read:packages,delete:packages
gh auth status --show-token
```

**Login for Docker:**

```bash
echo "<your_token>" | docker login ghcr.io -u yourusername --password-stdin
```

---

## Contributing

We welcome contributions! Please open issues for bug reports and feature requests.

**Before contributing:**

1. Install pre-commit hooks: `pre-commit install`
2. Run tests and linting: `task go:test && task go:lint`
3. Format your code: `task go:fmt`

Pre-commit hooks will automatically:

- Run security scans
- Format code and run linters

### Development Workflow

```bash
# 1. Fork and clone the repository
gh repo fork CowDogMoo/warpgate --clone

# 2. Create a feature branch
git checkout -b feature/my-feature

# 3. Make your changes and test
task go:build
task go:test

# 4. Format and lint
task go:fmt
task go:lint

# 5. Run pre-commit checks
task pre-commit:run

# 6. Commit and push
git commit -am "feat: add my feature"
git push origin feature/my-feature

# 7. Create a pull request
gh pr create
```
