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

- **[CLI Configuration Guide](docs/cli-configuration.md)** - Global
  configuration and security best practices
- **[Template Reference](docs/template-reference.md)** - Complete YAML syntax
  reference
- **[Template Repositories](docs/template-repositories.md)** - Repository
  management and discovery

### Reference

- **[Commands Reference](docs/commands.md)** - Complete CLI documentation
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions
- **[FAQ](docs/faq.md)** - Frequently asked questions

### Templates

- **[Official Templates](https://github.com/CowDogMoo/warpgate-templates)** -
  Ready-to-use templates

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
| **PowerShell Provisioner** | Run PowerShell (Windows AMIs)    |
| **Variable Substitution**  | CLI flags/files/env vars         |
| **Packer Conversion**      | Convert Packer to Warpgate       |
| **Registry Push**          | Push images to registries        |
| **Multi-arch Manifests**   | Create/push multi-arch images    |

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
