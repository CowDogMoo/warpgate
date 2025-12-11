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

Warp Gate creates standardized, reproducible environments. Build container
images and AWS AMIs from simple YAML templates, then reproduce them anywhere
with a single command. It handles everything from simple Dockerfiles to complex
multi-step provisioning with Ansible or shell scripts, and supports building
for multiple architectures simultaneously.

**Why Warp Gate?**

- Simple YAML templates
- One tool for containers and cloud images

**Useful for:**

- Security teams building attack/defense infrastructure
- DevOps engineers creating base images
- Platform teams standardizing environments
- Collaboration on infrastructure deployments across teams

## Quick Start

```bash
# Install warpgate
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

# List available templates
warpgate templates list

# Build a container image from template
warpgate build attack-box --arch amd64

# Verify the image
docker images | grep attack-box
```

## Documentation

### Getting Started

- **[Installation Guide](docs/installation.md)** - Platform-specific
  installation instructions
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

## Features

### Core Capabilities

| Feature                    | Description                      |
| -------------------------- | -------------------------------- |
| **Container Images**       | Build OCI images with BuildKit   |
| **Dockerfile Support**     | Native Dockerfile builds         |
| **AWS AMIs**               | Create EC2 AMIs                  |
| **Multi-arch Builds**      | Build amd64/arm64 simultaneously |
| **Template Discovery**     | Git/local template repo mgmt     |
| **Ansible Provisioner**    | Run Ansible playbooks            |
| **Shell Provisioner**      | Execute shell scripts            |
| **PowerShell Provisioner** | Run PowerShell (Windows)         |
| **Variable Substitution**  | CLI flags/files/env vars         |
| **Packer Conversion**      | Convert Packer to Warpgate       |
| **Registry Push**          | Push images to registries        |
| **Multi-arch Manifests**   | Create/push multi-arch images    |

## Installation

### Quick Install

```bash
# Go install
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

# Container image
docker pull ghcr.io/cowdogmoo/warpgate:latest
alias warpgate='docker run --rm -v $(pwd):/workspace ghcr.io/cowdogmoo/warpgate:latest'
```

Alternatively, pre-built binaries can be downloaded from: https://github.com/CowDogMoo/warpgate/releases

**Prerequisites:**

- Go 1.25+ (for building from source)
- Docker with BuildKit support (for container builds)

**See [Installation Guide](docs/installation.md) for detailed
platform-specific instructions.**

### Quick Configuration

```bash
# Initialize default configuration
warpgate config init

# Or customize settings
warpgate config set registry.default ghcr.io
warpgate config set aws.region your-aws-region
warpgate config set aws.profile my-aws-profile
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
# Add the official template repository
warpgate templates add https://github.com/CowDogMoo/warpgate-templates.git

# Update template cache
warpgate templates update

# List available templates
warpgate templates list

# Get template info
warpgate templates info attack-box

# Search for templates
warpgate templates search security
```

**See [Usage Guide](docs/usage-guide.md) for comprehensive examples and workflows.**

## Creating Templates

Templates use simple YAML syntax:

```yaml
metadata:
  name: my-image
  version: 1.0.0
  description: "My custom security image"
  author: "Your Name"
  license: MIT

name: my-image
version: latest

# Option 1: Dockerfile-based build
dockerfile:
  path: Dockerfile
  context: .
  args:
    VERSION: "1.0.0"

# Option 2: Provisioner-based build
base:
  image: ubuntu:22.04
  platform: linux/amd64

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
    registry: ghcr.io/myorg
    tags:
      - latest
      - v1.0.0
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

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## Troubleshooting

**Common Issues:**

- **"Cannot connect to BuildKit daemon"** - Ensure Docker is running with
  an active buildx builder (`docker buildx ls`)
- **"No active buildx builder found"** - Create one with
  `docker buildx create --use`
- **Templates not found** - Run `warpgate templates update` to refresh cache
- **Registry push fails** - Authenticate with `docker login <registry>`
  before building

See [Troubleshooting Guide](docs/troubleshooting.md) for detailed solutions.

## FAQ

**Q: What's the difference between Warpgate and Packer?**

A: Warpgate offers simpler YAML syntax, faster Go-native builds, native
BuildKit integration, built-in template discovery, first-class
multi-architecture support, and unified workflows for both containers and AMIs.

**Q: Can I use my existing Packer templates?**

A: Yes! Use `warpgate convert packer-template.pkr.hcl`.

**Q: Is Warpgate production-ready?**

A: Yes. Core features are stable and used in production environments.

**See [FAQ](docs/faq.md) for more questions and answers.**

### Built With

Warpgate uses open-source libraries:

- [BuildKit](https://github.com/moby/buildkit)
- [Docker SDK](https://github.com/docker/docker)
- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)
- [go-containerregistry](https://github.com/google/go-containerregistry)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
