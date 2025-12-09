# Configuration Guide

Complete guide to configuring Warpgate for your environment.

## Table of Contents

- [Overview](#overview)
- [Configuration File Locations](#configuration-file-locations)
- [Global Configuration](#global-configuration)
- [Template Configuration](#template-configuration)
- [Security Best Practices](#security-best-practices)

## Overview

Warpgate uses a two-tier configuration system:

1. **Global config** (`~/.config/warpgate/config.yaml`) - User preferences,
   defaults, and system settings
2. **Template config** (`warpgate.yaml`) - Image definitions (portable, version-controlled)

This separation allows:

- **Global config** - Machine-specific settings (storage, runtime, AWS region)
- **Template config** - Portable definitions shared across teams

## Configuration File Locations

Warpgate follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html).

### Config Files

Warpgate searches for configuration files in this order:

1. `$XDG_CONFIG_HOME/warpgate/config.yaml` (typically `~/.config/warpgate/config.yaml`)
2. `~/.warpgate/config.yaml` (legacy, still supported)
3. `./config.yaml` (current directory)

**First match wins.** Settings in earlier files take precedence.

### Cache Directory

Template cache and build artifacts:

- `$XDG_CACHE_HOME/warpgate/` (typically `~/.cache/warpgate/`)

### Creating Config Directory

```bash
# Create config directory
mkdir -p ~/.config/warpgate

# Create initial config file
touch ~/.config/warpgate/config.yaml
```

## Global Configuration

The global config file (`~/.config/warpgate/config.yaml`) controls system-wide settings.

### Complete Example

```yaml
# Storage and Runtime Configuration
storage:
  driver: vfs           # Storage driver: overlay, vfs, etc.
  root: ""              # Optional: custom storage root path

container:
  runtime: runc         # Container runtime: runc, crun, etc.

# Registry Configuration
registry:
  default: ghcr.io      # Default registry for pushes
  # NOTE: Use docker login for authentication, NOT config files

# AWS Configuration
aws:
  region: us-west-2     # Default AWS region (or use AWS_REGION env var)
  profile: lab          # AWS profile from ~/.aws/config (for SSO)
  ami:
    instance_type: t3.medium
    volume_size: 8

# Build Defaults
build:
  default_arch: amd64
  parallel_builds: true

# Template Repository Configuration
templates:
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    custom: /path/to/local/templates
```

### Storage Configuration

Controls how container images are stored.

```yaml
storage:
  driver: vfs           # Storage driver
  root: ""              # Custom storage path (optional)
```

**Supported drivers:**

- **vfs** - Portable, no kernel requirements (recommended for most users)
- **overlay** - Better performance, requires kernel support

**When to use vfs:**

- Default choice for most installations
- Rootless containers
- Systems without overlay support
- Maximum compatibility

**When to use overlay:**

- Native Linux with root access
- Maximum performance needed
- Kernel supports overlay2

### Container Runtime

Specify the container runtime:

```yaml
container:
  runtime: runc         # or: crun, kata, gvisor
```

**Common runtimes:**

- **runc** - Default OCI runtime (recommended)
- **crun** - Faster alternative to runc
- **kata** - VM-based isolation
- **gvisor** - sandboxed runtime

### Registry Configuration

Set default registry for image pushes:

```yaml
registry:
  default: ghcr.io      # Default: docker.io
```

**Important:** Never store credentials in config files. Use `docker login`
instead (see [Security Best Practices](#security-best-practices)).

### AWS Configuration

Default AWS settings for AMI builds:

```yaml
aws:
  region: us-west-2     # Default region
  profile: myprofile    # AWS CLI profile
  ami:
    instance_type: t3.medium
    volume_size: 8      # GB
```

**Override at build time:**

```bash
warpgate build myami --target ami --var AWS_REGION=us-east-1
```

### Build Defaults

Configure default build behavior:

```yaml
build:
  default_arch: amd64        # Default architecture
  parallel_builds: true      # Build architectures in parallel
```

### Template Repositories

Configure multiple template sources:

```yaml
templates:
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    private: git@github.com:myorg/private-templates.git
    local: /Users/username/my-templates
```

See [Template Configuration Guide](template-configuration.md) for detailed
repository management.

## Template Configuration

Templates define **what** to build. They are portable and should be version-controlled.

### Minimal Template

```yaml
# Template metadata
metadata:
  name: my-image
  version: 1.0.0
  description: "My custom security image"

# Image name (used for tagging)
name: my-image

# Base image
base:
  image: ubuntu:22.04

# Provisioners run in order
provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y curl

# Build targets
targets:
  - type: container
    platforms:
      - linux/amd64
```

### Template Variables

Make templates customizable with variables:

```yaml
metadata:
  name: customizable-image
  version: 1.0.0

name: customizable-image

base:
  image: ubuntu:22.04

# Define variables with defaults
variables:
  ARSENAL_PATH:
    type: string
    default: "/opt/arsenal"
    description: "Path to arsenal installation"

  ENABLE_DEBUG:
    type: bool
    default: false
    description: "Enable debug logging"

  WORKER_COUNT:
    type: int
    default: 4
    description: "Number of worker processes"

# Use variables in provisioners
provisioners:
  - type: shell
    inline:
      - echo "Installing to ${ARSENAL_PATH}"
      - mkdir -p ${ARSENAL_PATH}
      - echo "Workers: ${WORKER_COUNT}"

targets:
  - type: container
    platforms:
      - linux/amd64
```

### Variable Substitution

Variables use `${VAR_NAME}` syntax:

```yaml
provisioners:
  - type: shell
    inline:
      - mkdir -p ${INSTALL_PATH}
      - echo "Version: ${VERSION}"
      - cp files/* ${INSTALL_PATH}/
```

### Overriding Variables

Variables can be overridden at build time:

**1. CLI flags (highest precedence):**

```bash
warpgate build mytemplate --var ARSENAL_PATH=/custom/path --var VERSION=2.0.0
```

**2. Variable files:**

Create `vars.yaml`:

```yaml
ARSENAL_PATH: /custom/path
VERSION: 2.0.0
ENABLE_DEBUG: true
```

Use it:

```bash
warpgate build mytemplate --var-file vars.yaml
```

**3. Environment variables (lowest precedence):**

```bash
export ARSENAL_PATH=/custom/path
export VERSION=2.0.0
warpgate build mytemplate
```

**Precedence order:** CLI flags > Variable files > Environment variables >
Template defaults

## Security Best Practices

### Container Registry Authentication

**Never store tokens in config files!** Use secure authentication methods:

#### 1. Docker Login (Recommended)

Authenticate using Docker's credential system:

```bash
# Authenticate to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Or authenticate interactively
docker login ghcr.io

# Warpgate automatically uses ~/.docker/config.json
warpgate build mytemplate --push --registry ghcr.io
```

**How it works:**

- Credentials stored in `~/.docker/config.json`
- Can use credential helpers (keychain, pass, etc.)
- Warpgate reads credentials automatically

#### 2. GitHub CLI (for GHCR)

For GitHub Container Registry specifically:

```bash
# Authenticate with GitHub
gh auth login

# Token is automatically available to warpgate
warpgate build mytemplate --push --registry ghcr.io
```

#### 3. Environment Variables (CI/CD)

For automated pipelines:

```bash
export WARPGATE_REGISTRY_USERNAME=myusername
export WARPGATE_REGISTRY_TOKEN=$GITHUB_TOKEN

warpgate build mytemplate --push
```

**CI/CD examples:**

GitHub Actions:

```yaml
- name: Build and push
  env:
    WARPGATE_REGISTRY_USERNAME: ${{ github.actor }}
    WARPGATE_REGISTRY_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: warpgate build mytemplate --push --registry ghcr.io
```

GitLab CI:

```yaml
build:
  script:
    - export WARPGATE_REGISTRY_USERNAME=$CI_REGISTRY_USER
    - export WARPGATE_REGISTRY_TOKEN=$CI_REGISTRY_PASSWORD
    - warpgate build mytemplate --push
```

### AWS Credentials

**Never store AWS credentials in config files!** Use the AWS SDK credential chain:

#### 1. AWS SSO (Recommended for Organizations)

Configure AWS SSO for secure, temporary credentials:

```bash
# Configure SSO
aws configure sso

# Login with your profile
aws sso login --profile myawsprofile

# Set profile in environment
export AWS_PROFILE=myawsprofile

# Build AMI (automatically uses SSO credentials)
warpgate build --template my-ami --target ami
```

**~/.aws/config example:**

```ini
[profile myawsprofile]
sso_start_url = https://myorg.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = PowerUserAccess
region = us-west-2
```

#### 2. Environment Variables (Ephemeral Credentials)

For temporary access or CI/CD:

```bash
# Set AWS credentials
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_SESSION_TOKEN=IQoJb3...  # Required for SSO/STS
export AWS_REGION=us-west-2

# Build AMI
warpgate build --template my-ami --target ami
```

#### 3. IAM Roles (EC2/ECS/Lambda)

For builds running on AWS infrastructure:

```bash
# No configuration needed - automatically detected
warpgate build --template my-ami --target ami
```

**IAM role examples:**

EC2 instance profile:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateImage",
        "ec2:DescribeImages",
        "ec2:RegisterImage",
        "ec2:CreateTags"
      ],
      "Resource": "*"
    }
  ]
}
```

#### AWS SDK Credential Chain

Warpgate uses the standard AWS SDK credential chain (in order):

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. AWS SSO configuration (`~/.aws/config`)
3. Shared credentials file (`~/.aws/credentials`)
4. IAM roles (ECS tasks, EC2 instances, Lambda)
5. ECS container credentials

**Best practice:** Use SSO for human users, IAM roles for services.

### Variable Security

**For sensitive data:**

1. **Never hardcode secrets in templates**
2. **Define variables with no defaults**
3. **Pass values at build time**

Example:

```yaml
variables:
  API_KEY:
    type: string
    description: "API key for service (pass at build time)"
    # NO default value

  DB_PASSWORD:
    type: string
    description: "Database password (pass at build time)"
    # NO default value
```

Build with secrets:

```bash
# Pass via CLI (not saved in history if prefixed with space)
 warpgate build mytemplate --var API_KEY=secret123 --var DB_PASSWORD=pass456

# Or use variable file (add to .gitignore)
warpgate build mytemplate --var-file secrets.yaml
```

**secrets.yaml** (add to `.gitignore`):

```yaml
API_KEY: secret123
DB_PASSWORD: pass456
```

## Next Steps

- **Create templates** - See [Template Format](template-format.md)
- **Build images** - Follow the [Usage Guide](usage-guide.md)
- **Manage templates** - Read [Template Configuration](template-configuration.md)
- **Troubleshoot issues** - Check [Troubleshooting Guide](troubleshooting.md)

---

**Need help?** [Open an issue](https://github.com/CowDogMoo/warpgate/issues) or
ask in [Discussions](https://github.com/CowDogMoo/warpgate/discussions).
