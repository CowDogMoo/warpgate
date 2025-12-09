# Warpgate Template Format Reference

Complete YAML syntax reference for Warpgate templates.

## Table of Contents

- [Overview](#overview)
- [Template Structure](#template-structure)
- [Metadata Section](#metadata-section)
- [Base Image](#base-image)
- [Variables](#variables)
- [Provisioners](#provisioners)
- [Targets](#targets)
- [Complete Examples](#complete-examples)

## Overview

Warpgate templates use YAML for simple, readable configuration. Templates define:

- Base image to build from
- Provisioning steps (shell scripts, Ansible playbooks, etc.)
- Build targets (container images, AWS AMIs)
- Variables for customization

## Template Structure

Every template requires these top-level keys:

```yaml
metadata: # Template information
name: # Image name
base: # Base image configuration
provisioners: # Build steps (optional)
targets: # Build targets
variables: # Template variables (optional)
```

## Metadata Section

Template metadata for documentation and versioning:

```yaml
metadata:
  name: my-image # Required: template name
  version: 1.0.0 # Required: semantic version
  description: "Description" # Recommended: what this builds
  author: "Your Name" # Optional: author/team
  tags: # Optional: categorization
    - security
    - tools
```

**Best practices:**

- Use semantic versioning (MAJOR.MINOR.PATCH)
- Write clear, concise descriptions
- Tag for discoverability

## Base Image

Specify the starting image:

```yaml
base:
  image: ubuntu:22.04 # Required: base image with tag
```

**Supported base images:**

- **Linux:** ubuntu, debian, alpine, fedora, centos, etc.
- **Versions:** Always specify tag (avoid `latest`)
- **Format:** `repository:tag` or `registry/repository:tag`

**Examples:**

```yaml
base:
  image: ubuntu:22.04        # Ubuntu 22.04 LTS

base:
  image: alpine:3.18         # Alpine Linux 3.18

base:
  image: debian:12-slim      # Debian 12 slim variant

base:
  image: ghcr.io/myorg/base:v1.0  # Custom registry
```

## Variables

Define customizable parameters:

```yaml
variables:
  VAR_NAME:
    type: string|bool|int # Variable type
    default: "value" # Default value
    description: "What this does" # User-facing description
```

**Variable types:**

- `string` - Text values
- `bool` - true/false
- `int` - Numeric values

**Example:**

```yaml
variables:
  INSTALL_PATH:
    type: string
    default: "/opt/tools"
    description: "Installation directory for tools"

  ENABLE_DEBUG:
    type: bool
    default: false
    description: "Enable debug logging"

  WORKER_COUNT:
    type: int
    default: 4
    description: "Number of worker processes"
```

**Variable substitution:**

Use `${VAR_NAME}` syntax in provisioners:

```yaml
provisioners:
  - type: shell
    inline:
      - mkdir -p ${INSTALL_PATH}
      - echo "Installing to ${INSTALL_PATH}"
```

**Override at build time:**

```bash
# CLI flags (highest precedence)
warpgate build mytemplate --var INSTALL_PATH=/custom/path

# Variable file
warpgate build mytemplate --var-file vars.yaml

# Environment variables (lowest precedence)
export INSTALL_PATH=/custom/path
warpgate build mytemplate
```

## Provisioners

Provisioners execute in order to configure the image.

### Shell Provisioner

Execute shell commands or scripts:

**Inline commands:**

```yaml
provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y curl wget
      - echo "Setup complete"
```

**Script file:**

```yaml
provisioners:
  - type: shell
    script_path: scripts/install.sh
    environment:
      VAR: value
```

**With environment variables:**

```yaml
provisioners:
  - type: shell
    inline:
      - echo "Path is $CUSTOM_PATH"
    environment:
      CUSTOM_PATH: /opt/myapp
```

### Ansible Provisioner

Run Ansible playbooks:

```yaml
provisioners:
  - type: ansible
    playbook_path: playbook.yml # Required: playbook file
    galaxy_file: requirements.yml # Optional: Galaxy requirements
    extra_vars: # Optional: extra variables
      var1: value1
      var2: ${TEMPLATE_VAR}
```

**Example:**

```yaml
provisioners:
  - type: ansible
    playbook_path: playbooks/security-tools.yml
    galaxy_file: requirements.yml
    extra_vars:
      tools_path: "${TOOLS_PATH}"
      enable_gui: "${ENABLE_GUI}"
```

**Requirements:**

- Ansible must be available in base image or installed first
- Playbook path is relative to template directory
- Galaxy collections are installed before playbook runs

### PowerShell Provisioner

Run PowerShell scripts (for Windows images):

**Inline commands:**

```yaml
provisioners:
  - type: pwsh
    inline:
      - Write-Host "Configuring..."
      - Install-WindowsFeature -Name Web-Server
```

**Script file:**

```yaml
provisioners:
  - type: pwsh
    script_path: scripts/configure.ps1
```

### Provisioner Order

Provisioners run sequentially in the order defined:

```yaml
provisioners:
  # 1. Update system
  - type: shell
    inline:
      - apt-get update

  # 2. Install Ansible
  - type: shell
    inline:
      - apt-get install -y ansible

  # 3. Run Ansible playbook (requires Ansible from step 2)
  - type: ansible
    playbook_path: setup.yml

  # 4. Final configuration
  - type: shell
    script_path: scripts/finalize.sh
```

## Targets

Define what to build:

### Container Target

Build container images:

```yaml
targets:
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64
    tags:
      - latest
      - ${VERSION}
```

**Platform options:**

- `linux/amd64` - x86_64 architecture
- `linux/arm64` - ARM64/aarch64 architecture

**Tagging:**

- Static tags: `latest`, `v1.0.0`
- Variable tags: `${VERSION}`, `${BUILD_NUMBER}`

### AMI Target

Build AWS AMIs:

```yaml
targets:
  - type: ami
    region: us-west-2
    instance_type: t3.medium
    volume_size: 20
    ami_name: "my-image-${VERSION}"
```

**Options:**

- `region` - AWS region (or use AWS_REGION env var)
- `instance_type` - EC2 instance type for build
- `volume_size` - Root volume size in GB
- `ami_name` - Name for resulting AMI (variables supported)

### Multiple Targets

Build both containers and AMIs:

```yaml
targets:
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64

  - type: ami
    region: us-west-2
    instance_type: t3.medium
    volume_size: 20
```

## Complete Examples

### Minimal Template

```yaml
metadata:
  name: hello-world
  version: 1.0.0

name: hello-world

base:
  image: ubuntu:22.04

provisioners:
  - type: shell
    inline:
      - echo "Hello from Warpgate!"

targets:
  - type: container
    platforms:
      - linux/amd64
```

### Security Tools Template

```yaml
metadata:
  name: security-workstation
  version: 2.1.0
  description: "Comprehensive security testing environment"
  author: "Security Team"
  tags:
    - security
    - pentesting
    - red-team

name: security-workstation

base:
  image: ubuntu:22.04

variables:
  TOOLS_PATH:
    type: string
    default: "/opt/tools"
    description: "Installation path for security tools"

  ENABLE_GUI:
    type: bool
    default: false
    description: "Install GUI tools"

  VERSION:
    type: string
    default: "latest"

provisioners:
  # Update system
  - type: shell
    inline:
      - apt-get update
      - apt-get upgrade -y
      - apt-get install -y curl wget git python3 python3-pip ansible

  # Install security tools via Ansible
  - type: ansible
    playbook_path: playbooks/security-tools.yml
    galaxy_file: requirements.yml
    extra_vars:
      tools_path: "${TOOLS_PATH}"
      enable_gui: "${ENABLE_GUI}"

  # Final configuration
  - type: shell
    script_path: scripts/configure.sh
    environment:
      TOOLS_PATH: "${TOOLS_PATH}"

targets:
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64
    tags:
      - latest
      - ${VERSION}

  - type: ami
    region: us-west-2
    instance_type: t3.medium
    volume_size: 20
    ami_name: "security-workstation-${VERSION}"
```

### Web Server Template

```yaml
metadata:
  name: nginx-server
  version: 1.0.0
  description: "Nginx web server with SSL"
  author: "DevOps Team"

name: nginx-server

base:
  image: ubuntu:22.04

variables:
  NGINX_VERSION:
    type: string
    default: "1.24.0"
    description: "Nginx version to install"

  ENABLE_SSL:
    type: bool
    default: true
    description: "Enable SSL/TLS support"

provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y nginx=${NGINX_VERSION}*

  - type: ansible
    playbook_path: nginx-config.yml
    extra_vars:
      enable_ssl: "${ENABLE_SSL}"

targets:
  - type: container
    platforms:
      - linux/amd64
    tags:
      - nginx-${NGINX_VERSION}
      - latest
```

### Multi-Stage Development Template

```yaml
metadata:
  name: dev-environment
  version: 1.0.0
  description: "Complete development environment"

name: dev-env

base:
  image: ubuntu:22.04

variables:
  INSTALL_NODE:
    type: bool
    default: true

  INSTALL_PYTHON:
    type: bool
    default: true

  INSTALL_GO:
    type: bool
    default: false

provisioners:
  # Base tools
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y build-essential git curl

  # Conditional Node.js install
  - type: shell
    inline:
      - curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
      - apt-get install -y nodejs

  # Conditional Python install
  - type: shell
    inline:
      - apt-get install -y python3.11 python3-pip

  # Conditional Go install
  - type: shell
    inline:
      - wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
      - tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

targets:
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64
```

## Best Practices

**Metadata:**

- Always include name, version, and description
- Use semantic versioning
- Tag templates for easy discovery

**Base Images:**

- Use specific tags, not `latest`
- Prefer official images
- Consider image size (alpine for smaller footprints)

**Variables:**

- Provide sensible defaults
- Write clear descriptions
- Use appropriate types

**Provisioners:**

- Order matters - dependencies first
- Keep provisioners focused
- Use scripts for complex logic
- Leverage Ansible for configuration management

**Targets:**

- Build for needed architectures only
- Use meaningful tags
- Consider multi-platform builds

**Security:**

- Don't hardcode secrets
- Use variables for sensitive data
- Pass secrets at build time via `--var`

**Maintainability:**

- Keep templates modular
- Document custom variables
- Version control your templates
- Test templates before committing

## Validation

Validate templates before building:

```bash
warpgate validate warpgate.yaml
```

Common validation errors:

- Missing required fields
- Invalid YAML syntax
- Unsupported provisioner types
- Invalid platform specifications

## See Also

- [Template Configuration Guide](template-configuration.md) - Repository management
- [Main README](../README.md) - Getting started
- [Sliver Guide](sliver.md) - Example: Building Sliver C2
