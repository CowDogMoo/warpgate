# Usage Guide

Practical examples and common workflows for building images with Warpgate.

## Table of Contents

- [Quick Start](#quick-start)
- [Container Images](#container-images)
- [AWS AMIs](#aws-amis)
- [Template Management](#template-management)
- [Multi-Architecture Builds](#multi-architecture-builds)
- [Common Workflows](#common-workflows)

## Quick Start

Get started in under 60 seconds:

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

## Container Images

### Build from Template

Build container images using discovered templates:

```bash
# Build from discovered template
warpgate build attack-box

# Build specific architecture
warpgate build attack-box --arch amd64

# Build multiple architectures
warpgate build attack-box --arch amd64,arm64

# Build and push to registry
warpgate build attack-box --push --registry ghcr.io/myorg
```

### Build from Local File

Build from a template file in your current directory:

```bash
# Validate template first (recommended)
warpgate validate warpgate.yaml

# Build the image
warpgate build warpgate.yaml --arch amd64

# Build with custom tag
warpgate build warpgate.yaml --arch amd64 --var VERSION=1.2.3
```

### Build from Git Repository

Build directly from a Git repository:

```bash
# Build from GitHub repo
warpgate build --from-git https://github.com/cowdogmoo/warpgate-templates.git//templates/attack-box

# Build from specific branch or tag
warpgate build --from-git https://github.com/myorg/templates.git?ref=develop//path/to/template

# Build from private repo (requires SSH key or credentials)
warpgate build --from-git git@github.com:myorg/private-templates.git//path/to/template
```

**Git URL format:**

- `https://github.com/org/repo.git//path/to/template` - Public HTTPS
- `git@github.com:org/repo.git//path/to/template` - Private SSH
- Add `?ref=branch` or `?ref=v1.0.0` for specific refs

### Build with Variable Overrides

Customize builds using variables:

**Via CLI flags:**

```bash
# Single variable
warpgate build sliver --var PROVISION_REPO_PATH=/path/to/arsenal

# Multiple variables
warpgate build sliver \
  --var PROVISION_REPO_PATH=/path/to/arsenal \
  --var VERSION=1.0.0 \
  --var DEBUG=true
```

**Via variable file:**

Create `vars.yaml`:

```yaml
PROVISION_REPO_PATH: /path/to/ansible-collection-arsenal
VERSION: 1.0.0
DEBUG: true
ENABLE_GUI: false
```

Use it:

```bash
warpgate build sliver --var-file vars.yaml
```

**Via environment variables:**

```bash
export PROVISION_REPO_PATH=/path/to/arsenal
export VERSION=1.0.0
warpgate build sliver
```

**Variable precedence:** CLI flags (`--var`) > Variable files
(`--var-file`) > Environment variables > Template defaults

### Push to Registry

Build and push images to a container registry:

```bash
# Authenticate first
docker login ghcr.io

# Build and push
warpgate build myimage --push --registry ghcr.io/myorg

# Custom tags
warpgate build myimage \
  --push \
  --registry ghcr.io/myorg \
  --var VERSION=1.2.3
```

**Supported registries:**

- GitHub Container Registry (ghcr.io)
- Docker Hub (docker.io)
- Google Container Registry (gcr.io)
- Amazon ECR (_.dkr.ecr._.amazonaws.com)
- Any OCI-compliant registry

## AWS AMIs

### Build an AMI

Create AWS AMIs for EC2:

**Prerequisites:**

```bash
# Configure AWS credentials (choose one method)

# Option 1: AWS SSO (recommended)
aws configure sso
aws sso login --profile myawsprofile
export AWS_PROFILE=myawsprofile

# Option 2: Environment variables
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-west-2

# Option 3: IAM role (if running on EC2)
# No configuration needed
```

**Build AMI:**

```bash
# Build with default settings
warpgate build my-ami-template --target ami

# Specify region and instance type
warpgate build my-ami-template \
  --target ami \
  --var AWS_REGION=us-west-2 \
  --var INSTANCE_TYPE=t3.large

# Name the AMI
warpgate build my-ami-template \
  --target ami \
  --var AMI_NAME="my-custom-ami-$(date +%Y%m%d)"
```

**Template configuration:**

```yaml
targets:
  - type: ami
    region: us-west-2
    instance_type: t3.medium
    volume_size: 20
    ami_name: "my-image-${VERSION}"
```

### Multi-Target Builds

Build both containers and AMIs:

```yaml
targets:
  # Container target
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64

  # AMI target
  - type: ami
    region: us-west-2
    instance_type: t3.medium
    volume_size: 20
```

Build both:

```bash
# Build all targets
warpgate build mytemplate

# Build specific target only
warpgate build mytemplate --target container
warpgate build mytemplate --target ami
```

## Template Management

### List Templates

Find available templates from configured sources:

```bash
# List all templates
warpgate templates list

# Output example:
# NAME              VERSION   SOURCE      DESCRIPTION
# attack-box        1.0.0     official    Security testing environment
# sliver            1.0.0     official    Sliver C2 framework
# atomic-red-team   1.0.0     official    Atomic Red Team test platform
```

### Get Template Information

View detailed information about a template:

```bash
warpgate templates info attack-box

# Shows:
# - Template metadata
# - Base image
# - Provisioners
# - Build targets
# - Variables and defaults
```

### Add Template Sources

Add Git repositories or local directories:

```bash
# Add Git repository (auto-generates name)
warpgate templates add https://github.com/myorg/security-templates.git

# Add with custom name
warpgate templates add my-templates https://github.com/myorg/templates.git

# Add local directory
warpgate templates add ~/my-warpgate-templates

# Add private repository (SSH)
warpgate templates add private-templates git@github.com:myorg/private-templates.git
```

### Remove Template Sources

Remove a template source:

```bash
# Remove by name
warpgate templates remove my-templates

# List sources to find names
warpgate templates list
```

### Update Template Cache

Refresh templates from all configured sources:

```bash
# Update all template sources
warpgate templates update

# Discovers new templates
# Pulls latest changes from Git repos
# Rebuilds template index
```

**When to update:**

- After adding a new template source
- To get latest template changes
- When templates appear missing

See [Template Configuration Guide](template-configuration.md) for
comprehensive repository management.

## Multi-Architecture Builds

### Build for Multiple Architectures

Create images that run on different CPU architectures:

```bash
# Build for amd64 only
warpgate build myimage --arch amd64

# Build for arm64 only
warpgate build myimage --arch arm64

# Build for both (creates two separate images)
warpgate build myimage --arch amd64,arm64

# Build and push both
warpgate build myimage --arch amd64,arm64 --push --registry ghcr.io/myorg
```

**Automatic tagging:**

- `myimage:latest-amd64` - AMD64 image
- `myimage:latest-arm64` - ARM64 image

### Create Multi-Arch Manifests

Create a manifest that references both architectures:

```bash
# Build for multiple architectures first
warpgate build myimage --arch amd64,arm64 --push --registry ghcr.io/myorg

# Create multi-arch manifest
warpgate manifest create \
  --name ghcr.io/myorg/myimage:latest \
  --images ghcr.io/myorg/myimage:latest-amd64,ghcr.io/myorg/myimage:latest-arm64

# Push manifest to registry
warpgate manifest push ghcr.io/myorg/myimage:latest
```

**Result:** Users can pull `ghcr.io/myorg/myimage:latest` and automatically
get the correct architecture.

### Inspect Manifests

View manifest details:

```bash
# Inspect multi-arch manifest
warpgate manifest inspect ghcr.io/myorg/myimage:latest

# Shows:
# - Supported platforms
# - Image digests
# - Sizes
```

### Save Build Digests

Save image digests for signing with cosign or other tools:

```bash
# Build and save digests
warpgate build myimage \
  --arch amd64,arm64 \
  --push \
  --save-digests \
  --digest-dir ./digests

# Digests saved to:
# ./digests/myimage-amd64.digest
# ./digests/myimage-arm64.digest

# Use with cosign
cosign sign $(cat ./digests/myimage-amd64.digest)
cosign sign $(cat ./digests/myimage-arm64.digest)
```

## Common Workflows

### Development Workflow

Iterative template development:

```bash
# 1. Create template
cat > warpgate.yaml <<EOF
metadata:
  name: dev-image
  version: 0.1.0
name: dev-image
base:
  image: ubuntu:22.04
provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y curl
targets:
  - type: container
    platforms:
      - linux/amd64
EOF

# 2. Validate template
warpgate validate warpgate.yaml

# 3. Build and test locally
warpgate build warpgate.yaml --arch amd64

# 4. Test the image
docker run --rm dev-image:latest curl --version

# 5. Iterate (edit template, repeat 2-4)

# 6. Push when ready
warpgate build warpgate.yaml --push --registry ghcr.io/myorg
```

### CI/CD Pipeline

Automated builds in CI/CD:

**GitHub Actions example:**

```yaml
name: Build Images

on:
  push:
    branches: [main]
    tags: ["v*"]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install Warpgate
        run: go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

      - name: Login to GitHub Container Registry
        run: echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin

      - name: Build and push
        run: |
          warpgate build warpgate.yaml \
            --arch amd64,arm64 \
            --push \
            --registry ghcr.io/${{ github.repository_owner }} \
            --var VERSION=${{ github.ref_name }}
```

### Team Template Repository

Share templates across your team:

```bash
# 1. Create template repository
mkdir my-team-templates
cd my-team-templates

# 2. Create templates/ directory
mkdir -p templates/security-base

# 3. Create template
cat > templates/security-base/warpgate.yaml <<EOF
metadata:
  name: security-base
  version: 1.0.0
  description: "Team security base image"
name: security-base
base:
  image: ubuntu:22.04
# ... rest of template
EOF

# 4. Initialize git and push
git init
git add .
git commit -m "Add security-base template"
git remote add origin git@github.com:myorg/team-templates.git
git push -u origin main

# 5. Team members add repository
warpgate templates add team https://github.com/myorg/team-templates.git

# 6. Use templates
warpgate templates list
warpgate build security-base
```

### Security Scanning Workflow

Build, scan, and push images:

```bash
# 1. Build image
warpgate build myimage --arch amd64

# 2. Scan with Trivy
trivy image myimage:latest

# 3. If scan passes, push
if trivy image --exit-code 1 --severity HIGH,CRITICAL myimage:latest; then
  warpgate build myimage --push --registry ghcr.io/myorg
else
  echo "Security scan failed!"
  exit 1
fi
```

## Next Steps

- **Learn template syntax** - See [Template Format](template-format.md)
- **Configure Warpgate** - Read [Configuration Guide](configuration.md)
- **View all commands** - Check [Commands Reference](commands.md)
- **Troubleshoot issues** - Visit [Troubleshooting Guide](troubleshooting.md)
