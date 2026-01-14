# Warp Gate

**Build container images and AWS AMIs with speed, simplicity, and security.**

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/CowDogMoo/warpgate?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/CowDogMoo/warpgate?label=Release&logo=github)](https://github.com/CowDogMoo/warpgate/releases)

[![🚨 Semgrep](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-Commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)
[![Renovate](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml)

<p align="center">
  <img src="docs/images/wg-logo.jpeg" alt="Warp Gate Logo" width="100%">
</p>

---

## Overview

Warp Gate creates standardized, reproducible environments. Build container
images and AWS AMIs from YAML templates, then reproduce them anywhere
with a single command. It handles everything from straightforward Dockerfiles
to complex multi-step provisioning with Ansible or shell scripts, and supports
building for multiple architectures simultaneously.

**Why Warp Gate?**

- [Declarative YAML templates](https://github.com/CowDogMoo/warpgate-templates)
- One tool for containers and cloud images
- Extensible provisioning (Ansible, shell, PowerShell)
- Multiarch support

**Useful for:**

- Security teams building attack/defense infrastructure
- DevOps engineers creating base images
- Platform teams standardizing environments
- Collaboration on infrastructure deployments across teams

## Prerequisites

Before using Warp Gate, ensure you have the following installed:

| Requirement        | Version | Notes                                     |
| ------------------ | ------- | ----------------------------------------- |
| **Go**             | 1.25+   | Required for `go install`                 |
| **Docker**         | 20.10+  | Required for container builds             |
| **Docker Buildx**  | 0.8+    | Required for multi-arch builds            |
| **AWS CLI** (opt.) | 2.x     | Required for AMI builds                   |
| **Ansible** (opt.) | 2.9+    | Required for Ansible provisioner          |

```bash
# Verify Docker Buildx is available
docker buildx version

# Create a buildx builder (if not exists)
docker buildx create --use --name warpgate-builder
```

## Quick Start

```bash
# Install warpgate
go install github.com/cowdogmoo/warpgate/v3/cmd/warpgate@latest

# Initialize configuration (optional)
warpgate config init

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

### Configuration Guides

- **[CLI Configuration Guide](docs/cli-configuration.md)** - Global
  configuration and security best practices
- **[Template Reference](docs/template-reference.md)** - Complete YAML syntax
  reference
- **[Template Repositories](docs/template-repositories.md)** - Repository
  management and discovery

### Reference

- **[Commands Reference](docs/commands.md)** - Complete CLI documentation
- **[Release Guide](docs/release-guide.md)** - Version management and release workflows
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions
- **[FAQ](docs/faq.md)** - Frequently asked questions

### Templates

- **[Official Templates](https://github.com/CowDogMoo/warpgate-templates)** -
  Ready-to-use templates

## Features

### Core Capabilities

| Feature                      | Description                            |
| ---------------------------- | -------------------------------------- |
| **Container Images**         | Build OCI images with BuildKit         |
| **Dockerfile Support**       | Native Dockerfile builds               |
| **AWS AMIs**                 | Create EC2 AMIs                        |
| **Multi-arch Builds**        | Build amd64/arm64 simultaneously       |
| **Multi-region AMI Builds**  | Build AMIs across multiple AWS regions |
| **Parallel Region Builds**   | Build in all regions simultaneously    |
| **Cross-region AMI Copy**    | Copy AMIs to additional regions        |
| **Build Monitoring**         | Stream logs and EC2 status             |
| **Template Discovery**       | Git/local template repo mgmt           |
| **Ansible Provisioner**      | Run Ansible playbooks                  |
| **Shell Provisioner**        | Execute shell scripts                  |
| **PowerShell Provisioner**   | Run PowerShell (Windows AMIs)          |
| **Variable Substitution**    | CLI flags/files/env vars               |
| **Packer Conversion**        | Convert Packer to Warpgate             |
| **Registry Push**            | Push images to registries              |
| **Multi-arch Manifests**     | Create/push multi-arch images          |
| **Context-aware Logging**    | Thread-safe, structured logging        |
| **Sensitive Data Redaction** | Auto-redact credentials in logs        |

### AMI Build Features

Warp Gate provides advanced AMI building capabilities:

```bash
# Build AMI in a specific region
warpgate build --template attack-box --target ami --region us-west-2

# Build AMI in multiple regions (sequential)
warpgate build --template attack-box --target ami --regions us-east-1,us-west-2,eu-west-1

# Build AMI in multiple regions (parallel)
warpgate build --template attack-box --target ami --regions us-east-1,us-west-2 --parallel-regions

# Build AMI and copy to additional regions
warpgate build --template attack-box --target ami --region us-east-1 --copy-to-regions us-west-2,eu-west-1

# Stream build logs in real-time
warpgate build --template attack-box --target ami --stream-logs

# Show EC2 instance status during build
warpgate build --template attack-box --target ami --show-ec2-status

# Dry-run validation without creating resources
warpgate build --template attack-box --target ami --dry-run

# Force recreation of existing AWS resources
warpgate build --template attack-box --target ami --force
```

### Resource Cleanup

Clean up AWS Image Builder resources:

```bash
# Clean up resources for a specific build
warpgate cleanup my-template

# Dry-run to see what would be deleted
warpgate cleanup my-template --dry-run

# Non-interactive mode (skip confirmation prompts)
warpgate cleanup my-template --yes

# Clean up old component versions, keeping 3 most recent
warpgate cleanup my-template --versions --keep 3

# Clean up all warpgate-created resources
warpgate cleanup --all --dry-run
```

## Configuration

### Configuration Precedence

Warp Gate uses a layered configuration system. Settings are applied in the
following order (highest to lowest priority):

1. **CLI flags** - Command-line arguments (e.g., `--log-level debug`)
2. **Environment variables** - `WARPGATE_*` prefixed variables
3. **Configuration file** - YAML config file
4. **Built-in defaults** - Sensible default values

This means CLI flags always take precedence over environment variables, which
take precedence over config file settings.

### Configuration File Locations

Warp Gate searches for configuration files in the following locations:

1. `$XDG_CONFIG_HOME/warpgate/config.yaml` (typically `~/.config/warpgate/`)
2. `~/.warpgate/config.yaml` (legacy, for backward compatibility)
3. `./config.yaml` (current directory)

Initialize a config file with defaults:

```bash
warpgate config init
```

View current effective configuration:

```bash
warpgate config show
```

### Environment Variables

Warp Gate supports the following environment variables:

| Variable | Description | Default |
| --- | --- | --- |
| `WARPGATE_LOG_LEVEL` | Log verbosity (debug/info/etc) | `info` |
| `WARPGATE_LOG_FORMAT` | Log format (text, json, color) | `color` |
| `WARPGATE_REGISTRY_DEFAULT` | Default container registry | `ghcr.io` |
| `WARPGATE_BUILD_DEFAULT_ARCH` | Default build architectures | `amd64` |
| `AWS_REGION` | AWS region for AMI builds | - |
| `AWS_PROFILE` | AWS credentials profile | - |

### Example Configuration

Create `~/.config/warpgate/config.yaml`:

```yaml
# Logging configuration
log:
  level: info
  format: text

# Default registry for container images
registry:
  default: ghcr.io/myorg

# Build settings
build:
  default_arch:
    - amd64
    - arm64
  parallelism_limit: 2

# AWS settings for AMI builds
aws:
  region: us-west-2

# Template sources
templates:
  repositories:
    official: https://github.com/CowDogMoo/warpgate-templates
  local_paths:
    - ~/my-templates
```

## Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./builder/...

# Run with verbose output
go test -v ./...
```

## How to Contribute

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for ways to
contribute and development guidelines.

### Built With

Warpgate uses open-source libraries:

- [BuildKit](https://github.com/moby/buildkit)
- [Docker SDK](https://github.com/docker/docker)
- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)
- [go-containerregistry](https://github.com/google/go-containerregistry)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
