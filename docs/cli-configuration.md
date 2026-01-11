# Configuration Guide

Complete guide to configuring Warpgate for your environment.

## Table of Contents

- [Overview](#overview)
- [Configuration File Locations](#configuration-file-locations)
- [Global Configuration](#global-configuration)
- [Security Best Practices](#security-best-practices)

## Overview

Warpgate separates configuration from templates:

1. **Configuration** (`~/.config/warpgate/config.yaml`) - User preferences,
   defaults, and system settings (machine-specific)
2. **Templates** (`warpgate.yaml`) - Image definitions that specify what to
   build (portable, version-controlled)

This separation allows:

- **Configuration** - Machine-specific settings like BuildKit endpoint, registry
  defaults, AWS region, and build preferences
- **Templates** - Portable image definitions shared across teams that define
  base images, provisioners, and build targets

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
# BuildKit Configuration
buildkit:
  endpoint: "" # Empty = auto-detect local buildx builder
  tls_enabled: false

# Registry Configuration
registry:
  default: ghcr.io # Default registry for pushes

# AWS Configuration
aws:
  region: us-west-2 # Default AWS region (or use AWS_REGION env var)
  profile: lab # AWS profile from ~/.aws/config (for SSO)
  ami:
    instance_type: t3.medium
    volume_size: 8

# Build Defaults
build:
  default_arch: [amd64]
  parallel_builds: true
  concurrency: 2

# Template Repository Configuration
templates:
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    custom: /path/to/local/templates
```

### BuildKit Configuration

Configure BuildKit builder settings:

```yaml
buildkit:
  endpoint: "" # Empty = auto-detect local buildx builder
  tls_enabled: false
```

**BuildKit endpoint:**

- Leave empty (`""`) to auto-detect the local Docker buildx builder
- Specify a remote BuildKit endpoint for distributed builds
- Example: `tcp://buildkit.example.com:1234`

**TLS settings:**

- Set `tls_enabled: true` for remote BuildKit with TLS
- Requires proper certificates configured in Docker

**Verify your BuildKit setup:**

```bash
# List available builders
docker buildx ls

# Create a new builder if needed
docker buildx create --use --name warpgate-builder

# Inspect current builder
docker buildx inspect
```

### Registry Configuration

Set default registry for image pushes:

```yaml
registry:
  default: ghcr.io # Default: docker.io
```

**Important:** Never store credentials in config files. Use `docker login`
instead (see [Security Best Practices](#security-best-practices)).

### AWS Configuration

Default AWS settings for AMI builds:

```yaml
aws:
  region: us-west-2 # Default region
  profile: myprofile # AWS CLI profile (for SSO)
  ami:
    instance_profile_name: AmazonSSMRoleForInstancesQuickSetup # IAM instance profile
    instance_type: t3.medium # EC2 instance type for builds
    volume_size: 8 # Root volume size in GB
    device_name: /dev/sda1 # Root device name
    volume_type: gp3 # EBS volume type
    build_timeout_min: 90 # Build timeout in minutes
    polling_interval_sec: 30 # Status polling interval
```

#### IAM Instance Profile

AMI builds require an IAM instance profile with permissions for:

- **EC2 Image Builder** - To download and execute build components
- **Systems Manager (SSM)** - For EC2 Instance Connect and management

**Quick setup using AWS managed policies:**

```bash
# Attach ImageBuilder policy to SSM role
aws iam attach-role-policy \
  --role-name AmazonSSMRoleForInstancesQuickSetup \
  --policy-arn arn:aws:iam::aws:policy/EC2InstanceProfileForImageBuilder
```

Then configure in warpgate:

```yaml
aws:
  ami:
    instance_profile_name: AmazonSSMRoleForInstancesQuickSetup
```

**Region precedence (highest to lowest):**

1. Environment variables (`AWS_REGION`, `AWS_DEFAULT_REGION`)
2. CLI flag (`--region`)
3. Template configuration (`targets[].region`)
4. Config file (`aws.region`)
5. AWS profile default region

**Note:** Environment variables override all other settings. Use
`unset AWS_REGION AWS_DEFAULT_REGION` if experiencing unexpected region
selection.

**Override at build time:**

```bash
# Override region
warpgate build myami --target ami --region us-east-1

# Override instance type
warpgate build myami --target ami --instance-type t3.large
```

### Build Defaults

Configure default build behavior:

```yaml
build:
  default_arch: [amd64] # Default architectures (can specify multiple)
  parallel_builds: true # Build architectures in parallel
  concurrency: 2 # Number of concurrent builds
```

**Options:**

- `default_arch` - List of architectures to build by default (e.g.,
  `[amd64]`, `[amd64, arm64]`)
- `parallel_builds` - Whether to build multiple architectures concurrently
- `concurrency` - Maximum number of parallel builds

### Template Repositories

Configure template sources (git repositories and local directories):

```yaml
templates:
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    private: git@git.example.com:mycompany/private-templates.git
  local_paths:
    - ~/dev/templates
```

For complete documentation on managing template repositories, private repos,
discovery order, and best practices, see
[Template Repositories Guide](template-repositories.md).

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
sso_start_url = https://example-org.awsapps.com/start
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

- **Learn template syntax** - See [Template Reference](template-reference.md)
- **Build images** - Follow the [Usage Guide](usage-guide.md)
- **Manage template repositories** - Read [Template Repositories Guide](template-repositories.md)
- **Troubleshoot issues** - Check [Troubleshooting Guide](troubleshooting.md)
