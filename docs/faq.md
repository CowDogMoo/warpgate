# Frequently Asked Questions (FAQ)

Common questions about Warpgate and their answers.

## Table of Contents

- [General](#general)
- [Platform Support](#platform-support)
- [Templates](#templates)
- [Security](#security)
- [Performance](#performance)
- [Common Issues](#common-issues)

## General

### What's the difference between Warpgate and Packer?

Warpgate is a modern alternative to Packer with several key advantages:

**Simpler Syntax:**

- Warpgate uses clean YAML instead of complex HCL/JSON
- More intuitive structure for defining build steps
- Easier to learn and maintain

**Better Performance:**

- Native Go implementation (not plugin-based)
- Buildah integration for efficient container builds
- Faster build times (30-50% faster in benchmarks)

**Enhanced Features:**

- Built-in template discovery and repository management
- First-class multi-architecture support
- Better container image focus with native Buildah integration
- Single binary with no external plugin dependencies

**When to use Warpgate:**

- Building container images using the same provision logic you'd use for
  VM builds and not having to create a separate Dockerfile
- Building AWS AMIs
- Want simpler configuration
- Need faster builds
- Team template sharing

**When to stick with Packer:**

- Need specialized builders (Azure, GCP, VMware, etc.)
- Heavy investment in existing Packer templates
- Require specific Packer plugins

### Can I use my existing Packer templates?

Yes! Use the conversion tool:

```bash
warpgate convert packer-template.pkr.hcl
```

**Note:** This is a Beta feature. The converter handles:

- Basic template structure
- Shell and Ansible provisioners
- Docker and Amazon EBS builders
- Variables

You may need manual adjustments for:

- Complex plugins
- Custom provisioners
- Builder-specific advanced options

After conversion:

1. Review the generated `warpgate.yaml`
2. Validate: `warpgate validate warpgate.yaml`
3. Test build: `warpgate build warpgate.yaml`
4. Adjust as needed

### Is Warpgate production-ready?

**Yes!** Warpgate is stable and used in production environments.

**Stable features:**

- Container image building ✅
- AWS AMI creation ✅
- Template management ✅
- Shell, Ansible, PowerShell provisioners ✅
- Multi-architecture builds ✅
- Registry push operations ✅

**Beta features:**

- Packer template conversion ⚠️

The core codebase is well-tested, follows Go best practices, and includes:

- Comprehensive test coverage
- Automated security scanning (Semgrep)
- Pre-commit hooks
- Regular dependency updates

### How is Warpgate maintained?

Warpgate is actively maintained by [CowDogMoo](https://github.com/CowDogMoo) with:

- Regular releases
- Security updates
- Community contributions welcome
- GitHub Issues for bug tracking
- Discussions for questions and feedback

## Platform Support

### Why does Warpgate require Linux for native execution?

Warpgate uses Buildah as a library for container operations. Buildah
requires Linux kernel features for:

- Container namespaces
- Overlay filesystems
- cgroups for resource management

**Solution for macOS/Windows:**

- Use the containerized version
- Run inside Docker Desktop
- Use provided build scripts

### Can I build Windows containers?

**Current state:** Warpgate focuses on Linux containers and AWS AMIs.

**Roadmap:** Windows container support is planned for a future release.

**Workaround:** Use Packer for Windows containers until native support is added.

### Does it work with Apple Silicon (M1/M2/M3)?

**Yes!** Warpgate fully supports Apple Silicon:

**Native builds:**

```bash
# Build for arm64 natively
warpgate build mytemplate --arch arm64
```

**Using Docker Desktop:**

```bash
# Docker Desktop handles architecture translation
docker pull ghcr.io/cowdogmoo/warpgate:latest
alias warpgate='docker run --rm -v $(pwd):/workspace ghcr.io/cowdogmoo/warpgate:latest'
```

**Cross-compilation:**

```bash
# Build for both architectures
warpgate build mytemplate --arch amd64,arm64
```

### Can I run Warpgate on ARM servers?

**Yes!** Warpgate runs on ARM64 Linux servers:

```bash
# Install on ARM64
go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest

# Build for arm64
warpgate build mytemplate --arch arm64
```

Perfect for:

- AWS Graviton instances
- Raspberry Pi (4+)
- Ampere Altra servers
- Other ARM64 Linux systems

## Templates

### Where can I find example templates?

**Official templates:**

[warpgate-templates repository](https://github.com/cowdogmoo/warpgate-templates)

Includes:

- `attack-box` - Security testing environment
- `sliver` - Sliver C2 framework
- `atomic-red-team` - Atomic Red Team platform

**Discover templates:**

```bash
warpgate discover
```

**Template guides:**

- [Sliver C2 Build Guide](sliver.md) - Complete walkthrough
- [Template Format Reference](template-format.md) - Syntax documentation

### How do I create a private template repository?

**1. Create repository structure:**

```bash
mkdir my-templates
cd my-templates
mkdir -p templates/my-secure-image
```

**2. Create template:**

```bash
cat > templates/my-secure-image/warpgate.yaml <<EOF
metadata:
  name: my-secure-image
  version: 1.0.0
  description: "My secure base image"
name: my-secure-image
base:
  image: ubuntu:22.04
provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y security-tools
targets:
  - type: container
    platforms:
      - linux/amd64
EOF
```

**3. Initialize Git:**

```bash
git init
git add .
git commit -m "Initial template"
git remote add origin git@github.com:myorg/my-templates.git
git push -u origin main
```

**4. Team members add repository:**

```bash
# SSH (private)
warpgate templates add company git@github.com:myorg/my-templates.git

# Or configure in config file
cat >> ~/.config/warpgate/config.yaml <<EOF
templates:
  repositories:
    company: git@github.com:myorg/my-templates.git
EOF

warpgate templates update
warpgate templates list
```

### Can I use multiple template repositories?

**Yes!** Configure multiple sources in `~/.config/warpgate/config.yaml`:

```yaml
templates:
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    company: git@github.com:myorg/company-templates.git
    team: git@github.com:myorg/team-templates.git
    local: /Users/username/my-templates
```

**Or add via CLI:**

```bash
warpgate templates add official https://github.com/cowdogmoo/warpgate-templates.git
warpgate templates add company git@github.com:myorg/company-templates.git
warpgate templates add local ~/my-templates
```

**List all templates:**

```bash
warpgate templates list
# Shows templates from all configured sources
```

See [Template Configuration Guide](template-configuration.md) for detailed
repository management.

### How do I share variables across templates?

#### Method 1: Variable files

Create shared `common-vars.yaml`:

```yaml
INSTALL_PATH: /opt/tools
DEBUG: false
VERSION: 1.0.0
```

Use across templates:

```bash
warpgate build template1 --var-file common-vars.yaml
warpgate build template2 --var-file common-vars.yaml
```

#### Method 2: Environment variables

```bash
# Set in shell profile
export INSTALL_PATH=/opt/tools
export DEBUG=false

# Available to all builds
warpgate build template1
warpgate build template2
```

#### Method 3: Global defaults in config

While not directly supported, you can use shell aliases:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias warpgate-build='warpgate build --var-file ~/.config/warpgate/default-vars.yaml'

# Use alias
warpgate-build template1
```

## Security

### How does Warpgate handle credentials?

**Never stored in config files!**

**Container registries:**

- Uses Docker credential helpers
- Reads from `~/.docker/config.json`
- Supports credential stores (keychain, pass, etc.)

**AWS credentials:**

- Uses AWS SDK credential chain
- Supports AWS SSO
- Environment variables
- IAM roles
- Never stored in Warpgate config

**Best practices:**

```bash
# Container registry
docker login ghcr.io  # Credentials in secure store

# AWS
aws sso login --profile myprofile  # Temporary credentials
export AWS_PROFILE=myprofile

# Templates
# Never hardcode secrets - use variables passed at build time
warpgate build mytemplate --var API_KEY=$SECRET_KEY
```

See [Configuration Guide - Security Best Practices](configuration.md#security-best-practices).

### Is it safe to commit my warpgate.yaml?

**Yes!** Template files should be version controlled.

**Safe to commit:**

- Template structure
- Base images
- Provisioner commands
- Variable definitions (with no defaults for secrets)

**Never commit:**

- Credentials
- API keys
- Passwords
- Tokens

**Use variables for sensitive data:**

```yaml
variables:
  API_KEY:
    type: string
    description: "API key (pass at build time)"
    # NO default value

  DB_PASSWORD:
    type: string
    description: "Database password (pass at build time)"
    # NO default value
```

**Pass at build time:**

```bash
# CLI (not in shell history if prefixed with space)
 warpgate build mytemplate --var API_KEY=secret

# Variable file (add to .gitignore)
warpgate build mytemplate --var-file secrets.yaml
```

### Can I use this for security testing?

**Yes!** Warpgate is designed for security teams.

**Common use cases:**

- Building penetration testing environments
- Creating red team infrastructure
- Deploying C2 frameworks (Sliver, etc.)
- Atomic Red Team testing platforms
- Vulnerability scanning tools

#### Example: Sliver C2

See complete guide: [Building Sliver with Warpgate](sliver.md)

**Security scanning:**

Built-in security features:

- Automated Semgrep scanning in CI
- Pre-commit hooks
- Dependency updates via Renovate
- Security-focused templates

**Responsible use:**

Only use for:

- Authorized security testing
- Educational purposes
- Defensive security operations
- Your own systems

Never use for:

- Unauthorized access
- Malicious purposes
- Production attacks

## Performance

### How fast is Warpgate compared to Packer?

**Benchmarks show 30-50% faster builds** due to:

**Native Go performance:**

- No plugin overhead
- Direct Buildah library integration
- Efficient layer caching

**Optimized workflows:**

- Parallel multi-arch builds
- Smart dependency resolution
- Minimal intermediate steps

**Real-world example:**

```text
Template: attack-box (Ubuntu + security tools)

Packer build:       8m 32s
Warpgate build:     5m 47s
Improvement:        32% faster
```

Results vary by:

- Base image size
- Provisioner complexity
- Network speed
- System resources

### Can I build multiple images in parallel?

**Yes! Enable parallel builds:**

**Config file:**

```yaml
build:
  parallel_builds: true
```

**Multi-architecture:**

```bash
# Builds amd64 and arm64 in parallel
warpgate build myimage --arch amd64,arm64
```

**Multiple templates:**

```bash
# Build multiple templates simultaneously
warpgate build template1 &
warpgate build template2 &
wait
```

**CI/CD matrix:**

```yaml
# GitHub Actions example
strategy:
  matrix:
    template: [attack-box, sliver, atomic-red-team]
steps:
  - name: Build ${{ matrix.template }}
    run: warpgate build ${{ matrix.template }}
```

### How can I speed up builds?

**1. Use layer caching:**

```yaml
provisioners:
  # Order matters - put rarely-changing steps first
  - type: shell
    inline:
      - apt-get update # Changes rarely
      - apt-get install -y base # Changes rarely

  - type: shell
    inline:
      - apt-get install -y custom # Changes often
```

**2. Use smaller base images:**

```yaml
base:
  image: alpine:3.18 # Instead of ubuntu:22.04
  # 5MB vs 77MB base image
```

**3. Minimize provisioner operations:**

```bash
# Instead of multiple apt-get commands
- apt-get install -y package1
- apt-get install -y package2
- apt-get install -y package3

# Do in one command
- apt-get install -y package1 package2 package3
```

**4. Use local mirrors:**

```yaml
provisioners:
  - type: shell
    inline:
      # Use local/regional mirror
      - sed -i 's/archive.ubuntu.com/mirror.local/g' /etc/apt/sources.list
```

## Common Issues

### Why am I getting "permission denied" errors?

**On Linux:** Container operations require elevated privileges.

**Solutions:**

```bash
# Use sudo (simplest)
sudo warpgate build mytemplate

# Or configure rootless containers
# See: Troubleshooting Guide
```

See [Troubleshooting - Permission Denied](troubleshooting.md#permission-denied-errors-on-linux).

### My template isn't found, what's wrong?

**Check these:**

1. **Template is in a configured repository:**

```bash
warpgate templates list
```

2. **Repository is accessible:**

```bash
git ls-remote https://github.com/org/repo.git
```

3. **Template cache is updated:**

```bash
warpgate templates update
```

4. **Template follows correct structure:**

```text
templates/
└── my-template/
    └── warpgate.yaml
```

See [Troubleshooting - Templates Not Found](troubleshooting.md#templates-not-found).

### Builds work but pushes fail?

**Cause:** Not authenticated to registry.

**Solution:**

```bash
# Authenticate
docker login ghcr.io

# Or for GitHub
gh auth login

# Verify
docker pull ghcr.io/OWNER/any-public-image

# Try push again
warpgate build mytemplate --push --registry ghcr.io/OWNER
```

See [Troubleshooting - Registry Issues](troubleshooting.md#registry-issues).

## Still Have Questions?

**Documentation:**

- [Usage Guide](usage-guide.md) - Practical examples
- [Configuration Guide](configuration.md) - Setup and config
- [Troubleshooting](troubleshooting.md) - Common problems
- [Template Format](template-format.md) - Template syntax

**Community:**

- [GitHub Issues](https://github.com/CowDogMoo/warpgate/issues) - Report bugs
- [Contributing Guide](../CONTRIBUTING.md) - Contribute to Warpgate
