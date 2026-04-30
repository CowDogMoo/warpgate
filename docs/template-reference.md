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

Warpgate supports two template modes:

### Provisioner-Based Templates

Traditional templates with provisioners (Ansible, shell, etc.):

```yaml
metadata: # Template information
name: # Image name
base: # Base image configuration
provisioners: # Build steps (Ansible/shell/etc.)
targets: # Build targets
variables: # Template variables (optional)
```

### Dockerfile-Based Templates

Simple templates that use existing Dockerfiles:

```yaml
metadata: # Template information
name: # Image name
dockerfile: # Dockerfile configuration
  path: Dockerfile # Path to Dockerfile
  context: . # Build context
  args: # Build arguments (optional)
    KEY: value
targets: # Build targets
```

**When to use each:**

- **Dockerfile mode**: Simple images, existing Dockerfiles, standard Docker
  workflows
- **Provisioner mode**: Complex provisioning, Ansible playbooks,
  cross-platform builds (containers + AMIs)

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

Run PowerShell scripts (for Windows images or containers with PowerShell Core):

```yaml
provisioners:
  - type: powershell
    ps_scripts:
      - scripts/configure.ps1
      - scripts/install-features.ps1
```

**Multiple scripts:**

```yaml
provisioners:
  - type: powershell
    ps_scripts:
      - scripts/setup.ps1
      - scripts/configure.ps1
      - scripts/finalize.ps1
```

**Notes:**

- Scripts execute via `pwsh` (PowerShell Core) in containers
- For AMI builds, uses EC2 Image Builder's `ExecutePowerShell` action
- Scripts run in order specified

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

Build AWS AMIs using EC2 Image Builder service:

```yaml
targets:
  - type: ami
    region: us-west-2
    instance_type: t3.medium
    volume_size: 20
    ami_name: "my-image-{{imagebuilder:buildDate}}"
    ami_description: "Custom AMI built with warpgate"
    ami_tags:
      Name: my-image
      ManagedBy: warpgate
      Environment: production
```

**How it works:**

Warpgate uses AWS EC2 Image Builder as its backend for AMI builds:

1. Creates Image Builder components from your provisioners
2. Creates an Image Builder pipeline
3. Launches an EC2 instance
4. Executes provisioners (shell scripts, PowerShell scripts, Ansible playbooks)
5. Creates and tags the AMI
6. Cleans up resources

**Prerequisites:**

- AWS credentials configured (see [CLI Configuration](cli-configuration.md))
- IAM instance profile with `EC2InstanceProfileForImageBuilder` policy
- Sufficient EC2 and Image Builder permissions

**Required Options:**

- `region` - AWS region for AMI creation (or use `AWS_REGION` env var)
- `instance_type` - EC2 instance type for build (default: `t3.medium`)
- `volume_size` - Root volume size in GB
- `ami_name` - Name for resulting AMI

**Optional:**

- `ami_description` - Description for the AMI
- `ami_tags` - Map of tags to apply to the AMI (key: value pairs)
- `subnet_id` - VPC subnet ID for the build instance
- `instance_profile_name` - IAM instance profile (overrides config file setting)
- `security_group_ids` - List of security group IDs for the build instance
- `device_name` - Root device name
  (default: `/dev/sda1` for Linux, `/dev/xvda` for Windows)
- `volume_type` - EBS volume type: `gp2`, `gp3`, `io1`, `io2` (default: `gp3`)

**AMI naming:**

Use ImageBuilder macros for dynamic AMI names:

- `{{imagebuilder:buildDate}}` - Timestamp (e.g., `2026-01-11T22-23-07.730Z`)

```yaml
ami_name: "my-app-{{imagebuilder:buildDate}}"
# Results in: my-app-2026-01-11T22-23-07.730Z
```

You can also use template variables:

```yaml
variables:
  VERSION: "1.0.0"

targets:
  - type: ami
    ami_name: "my-app-v${VERSION}-{{imagebuilder:buildDate}}"
# Results in: my-app-v1.0.0-2026-01-11T22-23-07.730Z
```

**Build time:**

- Linux AMIs: 10-20 minutes
- Windows AMIs: 30-60 minutes (depends on Windows Updates and software)

### Windows AMI with Fast Launch

For Windows AMIs, enable Fast Launch to reduce instance launch times by up to 65%:

```yaml
targets:
  - type: ami
    region: us-west-2
    instance_type: t3.large
    volume_size: 50
    ami_name: "windows-server-${VERSION}"
    # Enable Windows Fast Launch for faster instance starts
    fast_launch_enabled: true
    fast_launch_max_parallel_launches: 6
    fast_launch_target_resource_count: 5
```

**Windows Fast Launch Options:**

- `fast_launch_enabled` - Enable EC2 Fast Launch (creates pre-provisioned
  snapshots)
- `fast_launch_max_parallel_launches` - Max parallel instances for snapshot
  creation (default: 6, max: 40/region)
- `fast_launch_target_resource_count` - Number of pre-provisioned snapshots
  to maintain (default: 5)

**How Fast Launch Works:**

When enabled, EC2 creates pre-provisioned snapshots after the AMI is built.
When you launch instances from the AMI, they start from these pre-provisioned
snapshots instead of going through the full Windows initialization process.
This reduces launch times from ~10 minutes to ~3 minutes.

**Notes:**

- Fast Launch incurs additional costs for snapshot storage and temporary t3 instances
- The default quota is 40 max parallel launches per region across all AMIs
- Only applicable to Windows AMIs (Linux AMIs already launch quickly)

### Azure Target

Build Azure VM images using Azure VM Image Builder (AIB) and publish them to
an Azure Compute Gallery:

```yaml
targets:
  - type: azure
    subscription_id: 00000000-0000-0000-0000-000000000000
    resource_group: my-build-rg
    location: eastus
    gallery: myGallery
    gallery_image_definition: ubuntu-22-04
    os_type: Linux
    vm_size: Standard_D2s_v3
    identity_id: /subscriptions/.../userAssignedIdentities/aib-uami
    source_image:
      marketplace:
        publisher: Canonical
        offer: 0001-com-ubuntu-server-jammy
        sku: 22_04-lts-gen2
        version: latest
    target_regions:
      - westus2
      - westeurope
    image_tags:
      Owner: redteam
      Environment: prod
```

**How it works:**

Warpgate uses Azure VM Image Builder as the build backend (no Packer):

1. Generates an AIB ImageTemplate that references the source image and your
   provisioners (mapped to AIB shell, PowerShell, and File customizers).
2. Submits the template and runs it. AIB launches a build VM, runs the
   customizers, and captures the result.
3. Publishes the captured image as a new gallery image version
   (`YYYY.MMDD.HHMMSS` UTC) into the configured Compute Gallery.
4. Optionally replicates to additional regions via `target_regions`.
5. Deletes the AIB ImageTemplate resource on success by default (pass
   `--cleanup=false` to keep it around for debugging — Azure cleanup defaults
   to on; AMI cleanup defaults to off).

**Prerequisites:**

- Azure credentials reachable by `DefaultAzureCredential` (env vars, `az login`,
  managed identity).
- A user-assigned managed identity with the AIB role on the build resource
  group and Contributor on the gallery resource group.
- The Compute Gallery and gallery image definition already exist.

**Required options:**

- `subscription_id` - Azure subscription (or set `AZURE_SUBSCRIPTION_ID`).
- `resource_group` - Resource group hosting the build resources.
- `location` - Azure region for the build (e.g., `eastus`).
- `gallery` - Compute Gallery name where the image is published.
- `gallery_image_definition` - Image definition (parent of versions).
- `os_type` - `Linux` or `Windows`.
- `identity_id` - Resource ID of the user-assigned managed identity used by AIB.
- `source_image` - Either `marketplace` (publisher/offer/sku/version) or
  `gallery_image_version_id` (full resource ID).
- `vm_size` - Build VM size (e.g., `Standard_D2s_v3`). SKU availability
  depends on subscription quota and region capacity, so we don't guess this.

Azure builds do not auto-discover these fields during `warpgate build`; set
them explicitly in the template or override them with CLI flags.

**Optional:**

- `staging_resource_group` - Resource group AIB uses for ephemeral build
  resources. AIB creates one automatically when omitted.
- `target_regions` - Additional regions for replication (CLI flag
  `--target-regions` overrides).
- `share_with` - List of Azure AD principal object IDs (users, groups, or
  service principals) that should receive the Reader role on the published
  gallery image version after a successful build. Sharing is idempotent;
  re-running with the same list is a safe no-op. Requires the build credential
  to hold User Access Administrator (or higher) on the gallery's resource
  group scope.
- `subnet_id` - Resource ID of a pre-existing subnet
  (`/subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Network/virtualNetworks/<vnet>/subnets/<subnet>`).
  When set, AIB places the build VM on this subnet without a public IP and
  provisions a small proxy VM in the same subnet to relay control traffic.
  The subnet must allow outbound internet (NAT gateway, Azure Firewall, or
  service endpoints to storage/keyvault/ARM) so apt/pip and AIB callbacks can
  reach their destinations. CLI flag `--subnet-id` overrides.
- `proxy_vm_size` - VM size for the AIB proxy VM (defaults to
  `Standard_A1_v2`). Has no effect unless `subnet_id` is set. CLI flag
  `--proxy-vm-size` overrides.
- `image_tags` - Map of tags applied to the published gallery image version.

**Provisioner support:**

- `shell` (Linux) - inline commands.
- `script` (Linux) - one or more local shell scripts from `scripts`; each is
  embedded into the build, written to disk, made executable, and run in order.
- `powershell` (Windows) - `ps_scripts` (preferred) or inline commands; runs elevated.
- `file` - Source can be an HTTPS/SAS URL, or a local path when
  `azure.image.file_staging_storage_account` and
  `azure.image.file_staging_container` are configured (the local file is
  uploaded to blob storage before the build, and cleaned up afterwards).
- `ansible` - Runs an Ansible playbook on the build VM. The playbook (and an
  optional `galaxy_file`) are embedded into the build via base64 inline
  commands, so no extra storage is required. Linux targets emit a Shell
  customizer that auto-installs `ansible` via apt/dnf/yum; Windows targets
  emit a PowerShell customizer that installs Python via Chocolatey and pip
  installs `ansible`/`pywinrm`. Recognised provisioner fields:
  `playbook_path`, `galaxy_file`, `inventory`, `extra_vars`. Connection-flavor
  vars (`ansible_connection`, `ansible_aws_ssm_*`, and on Windows also
  `ansible_shell_type`) are stripped — the playbook is run with
  `--connection=local`.
Windows targets are detected from `os_type: Windows` on the target, or from
`ansible_shell_type: powershell` (or `cmd`) in an ansible provisioner's
`extra_vars`.

**CLI flags:**

```bash
warpgate build my-template.yaml \
  --target azure \
  --subscription <sub> \
  --location eastus \
  --resource-group my-build-rg \
  --gallery myGallery \
  --image-definition ubuntu-22-04 \
  --identity-id /subscriptions/.../userAssignedIdentities/aib-uami \
  --target-regions westus2,westeurope
```

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

### Dockerfile-Based Template

For simple images with existing Dockerfiles:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/cowdogmoo/warpgate/main/schema/warpgate-template.json
metadata:
  name: printer-monitor
  version: 1.0.0
  description: Brother printer health monitoring utility
  author: Your Name <your.email@example.com>
  license: MIT
  tags:
    - monitoring
    - utility
  requires:
    warpgate: ">=1.0.0"

name: printer-monitor
version: latest

dockerfile:
  path: Dockerfile # Relative to this warpgate.yaml
  context: . # Build context directory
  args: # Optional build arguments
    PYTHON_VERSION: "3.12"

targets:
  - type: container
    platforms:
      - linux/amd64
      - linux/arm64
    registry: ghcr.io/myorg
    tags:
      - latest
      - v1.0.0
    push: false
```

**Usage:**

```bash
# Build for amd64
warpgate build printer-monitor/warpgate.yaml --arch amd64

# Build for arm64 with custom registry
warpgate build printer-monitor/warpgate.yaml --arch arm64 --registry ghcr.io/myorg

# Build and push
warpgate build printer-monitor/warpgate.yaml --arch amd64 --push

# Build with custom build args
warpgate build printer-monitor/warpgate.yaml --arch amd64 --build-arg PYTHON_VERSION=3.11
```

### Minimal Template (Provisioner Mode)

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
      - wget https://go.dev/dl/go1.25.4.linux-amd64.tar.gz
      - tar -C /usr/local -xzf go1.25.4.linux-amd64.tar.gz

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

- [Template Repositories Guide](template-repositories.md) - Repository management
- [Main README](../README.md) - Getting started
