# Troubleshooting Guide

Solutions to common issues when using Warpgate.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Build Issues](#build-issues)
- [Registry Issues](#registry-issues)
- [Template Issues](#template-issues)
- [AWS AMI Issues](#aws-ami-issues)
- [Platform-Specific Issues](#platform-specific-issues)
- [Getting More Help](#getting-more-help)

## Installation Issues

### Binary not found after installation

**Symptoms:** Running `warpgate` results in "command not found".

**Cause:** `$GOPATH/bin` is not in your PATH.

**Solution:**

```bash
# Check if binary was installed
ls $(go env GOPATH)/bin/warpgate

# Add GOPATH/bin to PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Make permanent by adding to shell profile
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc  # or ~/.zshrc
source ~/.bashrc
```

### Go install fails with "module not found"

**Symptoms:** `go install` fails with module resolution errors.

**Solution:**

```bash
# Ensure Go is properly installed
go version

# Clear module cache and retry
go clean -modcache
go install github.com/cowdogmoo/warpgate/v3/cmd/warpgate@latest

# Or install specific version
go install github.com/cowdogmoo/warpgate/v3/cmd/warpgate@v1.2.3
```

### Container image pull fails

**Symptoms:** `docker pull ghcr.io/cowdogmoo/warpgate:latest` fails.

**Solution:**

```bash
# Check Docker is running
docker ps

# Check network connectivity
ping -c 3 ghcr.io

# Try explicit version
docker pull ghcr.io/cowdogmoo/warpgate:v1.2.3

# Check for rate limiting (Docker Hub)
docker login
```

## Build Issues

### "Permission denied" errors on Linux

**Symptoms:** Build fails with permission errors accessing Docker daemon
socket (`/var/run/docker.sock`).

**Cause:** User is not in the docker group and cannot access Docker daemon.

#### Solution: Add user to docker group

```bash
# Add current user to docker group
sudo usermod -aG docker $USER

# Log out and back in for changes to take effect
# Or activate new group membership immediately:
newgrp docker

# Verify access
docker ps

# Build templates
warpgate build mytemplate
```

**Alternative: Configure rootless Docker (advanced)

```bash
# Install rootless Docker
# See: https://docs.docker.com/engine/security/rootless/

# Install rootless Docker
curl -fsSL https://get.docker.com/rootless | sh

# Configure environment
systemctl --user start docker
export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/docker.sock

# Build templates
warpgate build mytemplate
```

### Build fails with "cannot connect to Docker daemon"

**Symptoms:** Build fails with errors like "Cannot connect to the Docker
daemon" or "Is the docker daemon running?".

**Cause:** Docker daemon is not running or warpgate cannot connect to it.

**Solution:**

**1. Check Docker status:**

```bash
# Check if Docker is running
docker ps

# Start Docker daemon (Linux)
sudo systemctl start docker

# Or use Docker Desktop (macOS/Windows)
# Start Docker Desktop application
```

**2. Verify Docker socket permissions:**

```bash
# Check socket exists and is accessible
ls -l /var/run/docker.sock

# If permission denied, add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

**3. Configure custom Docker endpoint (if needed):**

Edit `~/.config/warpgate/config.yaml`:

```yaml
buildkit:
  endpoint: "unix:///var/run/docker.sock"  # Default
  # Or for remote Docker: "tcp://remote-host:2376"
  tls_enabled: false
```

### Variable substitution not working

**Symptoms:** Variables show as `${VAR_NAME}` in built images instead of
actual values.

**Cause:** Variables not properly defined or passed.

**Solution:**

```bash
# Check variable is defined in template
warpgate validate mytemplate.yaml

# Use CLI flags (highest precedence)
warpgate build mytemplate --var VAR_NAME=value

# Or use variable file
cat > vars.yaml <<EOF
VAR_NAME: value
OTHER_VAR: other_value
EOF

warpgate build mytemplate --var-file vars.yaml

# Verify variable substitution in template
cat warpgate.yaml  # Check ${VAR_NAME} syntax
```

**Variable precedence:** CLI flags > Variable files > Environment variables >
Template defaults

### Build hangs or is very slow

**Symptoms:** Build appears stuck or takes much longer than expected.

**Possible causes and solutions:**

**1. Network issues downloading base image:**

```bash
# Check network connectivity
docker pull ubuntu:22.04

# Use local base image
warpgate build mytemplate --var BASE_IMAGE=local-ubuntu:22.04
```

**2. Large provisioner operations:**

```bash
# Enable verbose logging to see progress
warpgate build mytemplate --verbose

# Check what provisioner is running
docker ps  # If containerized
ps aux | grep warpgate  # Native
```

**3. Parallel builds consuming resources:**

```yaml
# Disable parallel builds in config
build:
  parallel_builds: false
```

## Registry Issues

### "Connection refused" when pushing to registry

**Symptoms:** Build succeeds but push fails with connection error.

**Cause:** Not authenticated to registry or incorrect registry URL.

**Solution:**

```bash
# Authenticate to registry
docker login ghcr.io

# Verify credentials are saved
cat ~/.docker/config.json

# Try push again with explicit registry
warpgate build mytemplate --push --registry ghcr.io/myorg
```

### "Unauthorized" or "403 Forbidden" errors

**Symptoms:** Push fails with authentication or authorization errors.

**Cause:** Invalid credentials or insufficient permissions.

**Solution:**

```bash
# For GitHub Container Registry (GHCR)
# Ensure token has write:packages scope
gh auth login --scopes write:packages

# Or create new token at https://github.com/settings/tokens
# with write:packages permission

# Login with token
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Verify authentication
docker pull ghcr.io/OWNER/any-public-image
```

### Image pushed but not visible in registry

**Symptoms:** Push succeeds but image doesn't appear in registry UI.

**Possible causes:**

**1. Package visibility settings (GHCR):**

```bash
# Make package public via GitHub UI:
# https://github.com/orgs/ORGNAME/packages/IMAGE_NAME/settings

# Or via API
gh api -X PATCH /user/packages/container/IMAGE_NAME \
  -f visibility=public
```

**2. Incorrect registry/image name:**

```bash
# Check exact image name
docker images | grep myimage

# Push with fully qualified name
warpgate build myimage --push --registry ghcr.io/USERNAME/myimage
```

## Template Issues

### Templates not found

**Symptoms:** `warpgate templates list` shows no templates or missing expected templates.

**Cause:** Template sources not configured or cache not updated.

**Solution:**

```bash
# Check configuration
cat ~/.config/warpgate/config.yaml

# Add default template repository
warpgate templates add https://github.com/cowdogmoo/warpgate-templates.git

# Update template cache
warpgate templates update

# Verify templates are discoverable
warpgate templates list

# Check individual template
warpgate templates info attack-box
```

### Template validation fails

**Symptoms:** `warpgate validate` reports errors in template.

**Common errors and solutions:**

**1. Missing required fields:**

```yaml
# ❌ Invalid - missing metadata
name: myimage
base:
  image: ubuntu:22.04

# ✅ Valid - has metadata
metadata:
  name: myimage
  version: 1.0.0
name: myimage
base:
  image: ubuntu:22.04
```

**2. Invalid YAML syntax:**

```bash
# Use YAML linter
yamllint warpgate.yaml

# Common issues:
# - Incorrect indentation
# - Missing colons
# - Unquoted special characters
```

**3. Invalid provisioner configuration:**

```yaml
# ❌ Invalid - missing inline or script_path
provisioners:
  - type: shell

# ✅ Valid - has inline commands
provisioners:
  - type: shell
    inline:
      - apt-get update
```

### Template from Git fails to clone

**Symptoms:** Build with `--from-git` fails with clone errors.

**Cause:** Authentication issues or incorrect URL format.

**Solution:**

```bash
# For public repositories
warpgate build --from-git https://github.com/org/repo.git//templates/mytemplate

# For private repositories (SSH)
# Ensure SSH key is configured
ssh -T git@github.com
warpgate build --from-git git@github.com:org/repo.git//templates/mytemplate

# For private repositories (HTTPS with token)
warpgate build --from-git https://TOKEN@github.com/org/repo.git//templates/mytemplate

# Verify repository is accessible
git ls-remote https://github.com/org/repo.git
```

## AWS AMI Issues

### AWS AMI builds fail with credentials error

**Symptoms:** AMI build fails with "unable to locate credentials" or "access denied".

**Cause:** AWS credentials not configured or invalid.

**Solution:**

**Method 1: AWS SSO (recommended):**

```bash
# Configure SSO
aws configure sso

# Login
aws sso login --profile myprofile

# Set profile
export AWS_PROFILE=myprofile

# Verify credentials
aws sts get-caller-identity

# Build AMI
warpgate build my-ami-template --target ami
```

**Method 2: Environment variables:**

```bash
# Set credentials
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_REGION=us-west-2

# Verify
aws sts get-caller-identity

# Build
warpgate build my-ami-template --target ami
```

### AMI build fails with permission errors

**Symptoms:** Build fails with "not authorized" errors for EC2 operations.

**Cause:** IAM user/role lacks required permissions.

**Solution:**

Ensure IAM user/role has these permissions:

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
        "ec2:CreateTags",
        "ec2:DescribeTags",
        "ec2:RunInstances",
        "ec2:TerminateInstances",
        "ec2:DescribeInstances",
        "ec2:CreateSecurityGroup",
        "ec2:DeleteSecurityGroup"
      ],
      "Resource": "*"
    }
  ]
}
```

### AMI build times out

**Symptoms:** AMI build hangs or times out during provisioning.

**Possible causes:**

**1. Provisioner script hangs:**

```bash
# Test provisioner scripts locally first
docker run -it ubuntu:22.04 bash
# Run your provisioner commands manually
```

**2. Instance type too small:**

```yaml
# Use larger instance type
targets:
  - type: ami
    instance_type: t3.large # Instead of t3.micro
```

**3. Network connectivity issues:**

Check security group allows outbound traffic for:

- Package downloads
- Git clone operations
- External APIs

### Instance profile permission errors

**Symptoms:**

```text
Error: pipeline failed with reason: failed to download the EC2 Image
Builder Component Error - User:
arn:aws:sts::123456789012:assumed-role/MyRole/i-1234567890abcdef0
is not authorized to perform: imagebuilder:GetComponent
```

**Cause:** The IAM instance profile used by the build EC2 instance lacks
ImageBuilder permissions.

**Solution:**

The instance profile needs both SSM and ImageBuilder permissions. Attach the
AWS managed `EC2InstanceProfileForImageBuilder` policy:

```bash
# Find your instance profile role name
aws iam get-instance-profile --instance-profile-name YourInstanceProfile \
  --query 'InstanceProfile.Roles[0].RoleName' --output text

# Attach ImageBuilder policy
aws iam attach-role-policy \
  --role-name YourRoleName \
  --policy-arn arn:aws:iam::aws:policy/EC2InstanceProfileForImageBuilder

# Verify policies are attached
aws iam list-attached-role-policies --role-name YourRoleName
```

**Quick fix using AWS managed profile:**

```bash
# Use AmazonSSMRoleForInstancesQuickSetup and add ImageBuilder permissions
aws iam attach-role-policy \
  --role-name AmazonSSMRoleForInstancesQuickSetup \
  --policy-arn arn:aws:iam::aws:policy/EC2InstanceProfileForImageBuilder
```

Then update your config:

```yaml
# ~/.config/warpgate/config.yaml
aws:
  ami:
    instance_profile_name: AmazonSSMRoleForInstancesQuickSetup
```

### AMI created in wrong region

**Symptoms:** AMI appears in a different region than specified in template or config.

**Cause:** AWS region configuration precedence. Environment variables
override all other settings.

**Solution:**

Check your environment:

```bash
# Check environment variables (these override everything)
echo $AWS_REGION
echo $AWS_DEFAULT_REGION

# If set, unset them to use config file settings
unset AWS_REGION
unset AWS_DEFAULT_REGION

# Or explicitly override with CLI flag
warpgate build myami --target ami --region us-west-2
```

**Region precedence (highest to lowest):**

1. Environment variables (`AWS_REGION`, `AWS_DEFAULT_REGION`)
2. CLI flag (`--region`)
3. Template configuration (`targets[].region`)
4. Config file (`aws.region`)
5. AWS profile default region

### ResourceAlreadyExistsException errors

**Symptoms:**

```text
Error: The following resource 'InfrastructureConfiguration' already exists
Error: The following resource 'ImageRecipe' already exists
```

**Cause:** Previous build failed and left ImageBuilder resources behind.

**Solution:**

Clean up leftover ImageBuilder resources:

```bash
# List and delete image pipelines
aws imagebuilder list-image-pipelines \
  --query 'imagePipelineList[?contains(name, `mytemplate`)].arn' \
  --output text | xargs -I {} aws imagebuilder delete-image-pipeline --image-pipeline-arn {}

# List and delete image recipes
aws imagebuilder list-image-recipes \
  --query 'imageRecipeSummaryList[?contains(name, `mytemplate`)].arn' \
  --output text | xargs -I {} aws imagebuilder delete-image-recipe --image-recipe-arn {}

# List and delete infrastructure configurations
aws imagebuilder list-infrastructure-configurations \
  --query 'infrastructureConfigurationSummaryList[?contains(name, `mytemplate`)].arn' \
  --output text | xargs -I {} aws imagebuilder delete-infrastructure-configuration --infrastructure-configuration-arn {}

# Then retry the build
warpgate build mytemplate --target ami
```

**Or clean up specific resources:**

```bash
# Find pipeline ARN from error message or list command
PIPELINE_ARN="arn:aws:imagebuilder:us-west-1:123456789012:image-pipeline/mytemplate-pipeline"

# Delete in order: pipeline -> recipe -> infrastructure config
aws imagebuilder delete-image-pipeline --image-pipeline-arn $PIPELINE_ARN
aws imagebuilder delete-image-recipe --image-recipe-arn <recipe-arn>
aws imagebuilder delete-infrastructure-configuration --infrastructure-configuration-arn <infra-arn>
```

### Windows AMI builds take very long

**Symptoms:** Windows AMI builds taking 45-90 minutes.

**Cause:** This is expected. Windows Updates and software installation
(MSSQL, IIS) are time-consuming.

**Typical build times:**

- Linux AMI: 10-20 minutes
- Windows Server base: 30-45 minutes
- Windows Server with MSSQL: 45-90 minutes

**Optimization tips:**

1. **Pre-bake base AMIs** - Create layered AMIs:
   - Base: Windows + Updates (build monthly)
   - App: Base AMI + Application (build as needed)

2. **Reduce update scope** - Only install critical updates:

   ```powershell
   # In PowerShell provisioner script
   Install-WindowsUpdate -NotCategory "Drivers" -AcceptAll -AutoReboot
   ```

3. **Use larger instance types** for builds:

   ```yaml
   targets:
     - type: ami
       instance_type: t3.large  # More CPU = faster builds
   ```

4. **Parallel provisioning** - Split independent tasks into separate provisioners

## Platform-Specific Issues

### macOS: Build fails with BuildKit errors

**Symptoms:** Native build attempts fail with "BuildKit not supported on darwin".

**Cause:** BuildKit requires Linux kernel features.

**Solution:**

Use containerized version:

```bash
# Pull container image
docker pull ghcr.io/cowdogmoo/warpgate:latest

# Create alias
alias warpgate='docker run --rm -v $(pwd):/workspace ghcr.io/cowdogmoo/warpgate:latest'

# Or use build scripts
bash scripts/build-template.sh mytemplate
```

### Windows: Volume mount issues

**Symptoms:** Container fails to access files on Windows host.

**Cause:** Windows path format incompatible with Docker.

**Solution:**

```powershell
# Use ${PWD} in PowerShell
docker run --rm -v ${PWD}:/workspace ghcr.io/cowdogmoo/warpgate:latest build mytemplate

# Or use absolute path with forward slashes
docker run --rm -v //c/Users/username/warpgate:/workspace ghcr.io/cowdogmoo/warpgate:latest build mytemplate
```

## Getting More Help

If your issue isn't covered here:

### 1. Check Existing Issues

Search [GitHub Issues](https://github.com/CowDogMoo/warpgate/issues) for
similar problems.

### 2. Enable Verbose Logging

Get detailed output:

```bash
warpgate build mytemplate --verbose
```

### 3. Gather Information

When reporting issues, include:

- Warpgate version: `warpgate version`
- Operating system: `uname -a` (Linux/macOS) or `systeminfo` (Windows)
- Go version: `go version`
- Template file (if applicable)
- Complete error message
- Steps to reproduce

### 4. Ask for Help

- [Open an issue](https://github.com/CowDogMoo/warpgate/issues/new)
- Check the [FAQ](faq.md)

### 5. Community Resources

- [Template Repositories Guide](template-repositories.md)
- [CLI Configuration Guide](cli-configuration.md)
- [Usage Guide](usage-guide.md)
- [Commands Reference](commands.md)

---

**Found a bug?**
[Report it](https://github.com/CowDogMoo/warpgate/issues/new) so we can fix
it for everyone!
