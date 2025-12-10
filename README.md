# Warp Gate

**Build container images and AWS AMIs with speed, simplicity, and security.**

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/CowDogMoo/warpgate?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/CowDogMoo/warpgate?label=Release&logo=github)](https://github.com/CowDogMoo/warpgate/releases)

[![🚨 Semgrep](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-Commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)
[![Renovate](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml)

<img src="docs/images/wg-logo.jpeg" alt="Warp Gate Logo" width="100%">

---

## Overview

Warp Gate is a modern, Go-based CLI tool that simplifies building container
images and AWS AMIs using declarative YAML templates. Built on
[BuildKit](https://github.com/moby/buildkit) and the [AWS SDK](https://aws.amazon.com/sdk-for-go/),
it provides a faster, more maintainable alternative to Packer for
infrastructure image creation.

**Why Warp Gate?**

- **Unified workflow** - Build from Dockerfiles OR provisioners
  (Ansible/shell) with the same tool
- **Flexible provisioning** - Use native Dockerfiles for simple images,
  provisioners for complex ones
- **Faster builds** - Native Go performance with BuildKit integration
- **Simpler syntax** - Clean YAML templates instead of HCL/JSON
- **Better portability** - Run natively on Linux or containerized anywhere
- **Template discovery** - Built-in template repository management
- **Multi-arch support** - Build for amd64 and arm64 simultaneously

**Perfect for:**

- Security teams building attack/defense infrastructure
- DevOps engineers creating base images
- Platform teams standardizing environments
- Anyone frustrated with Packer's complexity

## Quick Start

Get started in under 60 seconds:

```bash
# Install warpgate
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

# Discover available templates
warpgate discover

# Build a container image from template
warpgate build attack-box --arch amd64

# Verify the image
docker images | grep attack-box
```

**New to Warpgate?** See the [Installation Guide](docs/installation.md)
and [Usage Guide](docs/usage-guide.md).

## Documentation

### Getting Started

- **[Installation Guide](docs/installation.md)** - Platform-specific
  installation instructions
- **[Quick Start](#quick-start)** - Get running in 60 seconds
- **[Usage Guide](docs/usage-guide.md)** - Common workflows and practical examples

### Configuration

- **[Configuration Guide](docs/configuration.md)** - Global and template
  configuration
- **[Template Format](docs/template-format.md)** - Complete YAML syntax
  reference
- **[Template Management](docs/template-configuration.md)** - Repository
  management and discovery

### Reference

- **[Commands Reference](docs/commands.md)** - Complete CLI documentation
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions
- **[FAQ](docs/faq.md)** - Frequently asked questions

### Examples & Guides

- **[Official Templates](https://github.com/CowDogMoo/warpgate-templates)**
  - Ready-to-use templates

### Contributing

- **[Contributing Guide](CONTRIBUTING.md)** - Development guide and standards
- **[License](#license)** - MIT License details

## Features

### Core Capabilities

| Feature | Description | Status |
| --- | --- | --- |
| **Container Images** | Build OCI images with BuildKit | ✅ Stable |
| **Dockerfile Support** | Native Dockerfile builds | ✅ Stable |
| **AWS AMIs** | Create EC2 AMIs | ✅  Stable |
| **Multi-arch Builds** | Build amd64/arm64 simultaneously | ✅  Stable |
| **Template Discovery** | Git/local template repo mgmt | ✅  Stable |
| **Ansible Provisioner** | Run Ansible playbooks | ✅  Stable |
| **Shell Provisioner** | Execute shell scripts | ✅  Stable |
| **PowerShell Provisioner** | Run PowerShell (Windows) | ✅  Stable |
| **Variable Substitution** | CLI flags/files/env vars | ✅  Stable |
| **Packer Conversion** | Convert Packer to Warpgate | ⚠️  Beta |
| **Registry Push** | Push images to registries | ✅  Stable |
| **Multi-arch Manifests** | Create/push multi-arch images | ✅  Stable |

### Why Warpgate vs Packer?

| Aspect                 | Warpgate                  | Packer                  |
| ---------------------- | ------------------------- | ----------------------- |
| **Performance**        | Fast (Go native)          | Slower (plugins)        |
| **Syntax**             | Simple YAML               | Complex HCL/JSON        |
| **Container Focus**    | Native BuildKit           | Docker plugin only      |
| **Template Discovery** | Built-in repo mgmt        | Manual                  |
| **Learning Curve**     | Gentle                    | Steep                   |
| **Maintenance**        | Single binary             | Multiple plugins        |
| **Multi-arch**         | First-class support       | Manual configuration    |

### Security Features

- **Credential-free config** - Uses standard auth methods (docker login, AWS profiles)
- **No token storage** - Integrates with credential helpers
- **Security scanning** - Automated Semgrep analysis in CI
- **Least privilege** - Supports IAM roles and SSO
- **Supply chain security** - Pre-commit hooks and dependency updates

## Installation

### Quick Install

```bash
# Go install (recommended)
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

# Container image
docker pull ghcr.io/cowdogmoo/warpgate:latest
alias warpgate='docker run --rm -v $(pwd):/workspace ghcr.io/cowdogmoo/warpgate:latest'

# Pre-built binaries
# Download from: https://github.com/CowDogMoo/warpgate/releases
```

**Prerequisites:**

- Go 1.21+ (for building from source)
- Docker or Podman (for container builds via BuildKit)

**See [Installation Guide](docs/installation.md) for detailed
platform-specific instructions.**

## Configuration Setup

Warpgate uses a two-tier configuration system:

1. **Global config** (`~/.config/warpgate/config.yaml`) - User preferences and defaults
2. **Template config** (`warpgate.yaml`) - Image definitions (portable, version-controlled)

### Quick Configuration

Create `~/.config/warpgate/config.yaml`:

```yaml
# Storage and Runtime
storage:
  driver: vfs
container:
  runtime: runc

# Registry Configuration
registry:
  default: ghcr.io

# AWS Configuration
aws:
  region: us-west-2
  profile: lab

# Build Defaults
build:
  default_arch: amd64
  parallel_builds: true
```

**See [Configuration Guide](docs/configuration.md) for complete configuration reference.**

## Basic Usage

### Build Container Images

```bash
# Build from template
warpgate build attack-box --arch amd64

# Build with variables
warpgate build sliver --var ARSENAL_PATH=/opt/arsenal --var VERSION=1.0.0

# Build and push to registry
warpgate build myimage --push --registry ghcr.io/myorg
```

### Build AWS AMIs

```bash
# Configure AWS credentials
aws sso login --profile myprofile
export AWS_PROFILE=myprofile

# Build AMI
warpgate build my-ami-template --target ami
```

### Manage Templates

```bash
# Discover available templates
warpgate discover

# Add template repository
warpgate templates add https://github.com/myorg/templates.git

# Get template info
warpgate templates info attack-box
```

**See [Usage Guide](docs/usage-guide.md) for comprehensive examples and workflows.**

## Creating Templates

Templates use simple YAML syntax:

```yaml
metadata:
  name: my-image
  version: 1.0.0
  description: "My custom security image"

name: my-image

base:
  image: ubuntu:22.04

provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y curl wget

targets:
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64
```

**See [Template Format Reference](docs/template-format.md) for complete
syntax documentation.**

## How to Contribute

We welcome contributions! Warpgate is built for and by the community.

**Ways to contribute:**

- Report bugs and request features via [Issues](https://github.com/CowDogMoo/warpgate/issues)
- Submit pull requests for fixes and features
- Improve documentation
- Share templates in [warpgate-templates](https://github.com/CowDogMoo/warpgate-templates)

### Quick Start for Contributors

```bash
# Fork and clone
gh repo fork CowDogMoo/warpgate --clone
cd warpgate

# Install dependencies
go mod download

# Build and test
task go:build
task go:test

# Run pre-commit checks
task pre-commit:run
```

**See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.**

## Troubleshooting

**Common Issues:**

- **"Permission denied" on Linux** - Use `sudo warpgate build` or
  configure rootless containers
- **"Storage driver" errors** - Use `vfs` driver in config
- **Templates not found** - Run `warpgate templates update`
- **Registry push fails** - Authenticate with `docker login ghcr.io`

**See [Troubleshooting Guide](docs/troubleshooting.md) for detailed solutions.**

## FAQ

**Q: What's the difference between Warpgate and Packer?**

A: Warpgate offers simpler YAML syntax, sane licensing, faster builds,
built-in template discovery, and a stronger focus on containerization support.

**Q: Can I use my existing Packer templates?**

A: Yes! Use `warpgate convert packer-template.pkr.hcl`.

**Q: Is Warpgate production-ready?**

A: Yes. Core features are stable and used in production environments.

**See [FAQ](docs/faq.md) for more questions and answers.**

## License

This project is licensed under the **MIT License** - see the
[LICENSE](LICENSE) file for details.

### Third-Party Licenses

Warpgate uses open-source libraries:

- [BuildKit](https://github.com/moby/buildkit) - Apache 2.0
- [AWS SDK for Go](https://github.com/aws/aws-sdk-go-v2) - Apache 2.0
- [Cobra](https://github.com/spf13/cobra) - Apache 2.0
- [Viper](https://github.com/spf13/viper) - MIT
