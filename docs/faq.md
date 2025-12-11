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

Warpgate is an alternative to Packer with several key advantages:

**Syntax:**

- YAML configuration instead of HCL/JSON
- More intuitive structure for defining build steps
- Easier to learn and maintain

**Performance:**

- Direct BuildKit library integration (no plugin overhead)
- Efficient container builds
- Faster build times (30-50% faster in benchmarks)

**Features:**

- Built-in template discovery and repository management
- Multi-architecture builds (amd64, arm64)
- Native BuildKit integration for container images
- Single binary with no external plugin dependencies

**Use cases:**

- Building container images using the same provision logic you'd use for
  VM builds and not having to create a separate Dockerfile
- Building AWS AMIs
- Sharing build configurations across teams

### Can I convert my existing Packer templates?

Yes! Run the conversion tool:

```bash
warpgate convert packer-template.pkr.hcl
```

**Note:** This feature is experimental. The converter handles:

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

## Platform Support

### Does Warpgate work on macOS and Windows?

**Yes!** Warpgate supports macOS, Windows, and Linux.

Warpgate uses [BuildKit](https://github.com/moby/buildkit) for container
builds, which provides cross-platform support through Docker Desktop
(macOS/Windows) or native Docker (Linux).

### Can I build Windows containers?

**Current state:** Warpgate currently supports only Linux containers
(`linux/amd64`, `linux/arm64`).

**Windows AMI support:** Warpgate can build Windows AWS AMIs using the
PowerShell provisioner, but Windows container image builds are not currently
supported.

**Technical context:** While warpgate uses BuildKit (which can theoretically
build Windows containers), the current implementation and template system are
designed exclusively for Linux-based container images. Windows templates were
removed from the project during the first major refactor in May 2025 (commit 14a1abb).

**Future support:** There is no current roadmap for Windows container support.
If this is important to you, please [open a GitHub issue](https://github.com/CowDogMoo/warpgate/issues)
to discuss your use case.

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

## Templates

### Where can I find existing templates?

**Official templates:**

[warpgate-templates repository](https://github.com/cowdogmoo/warpgate-templates)

**Template guides:**

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

## Security

### How does Warpgate handle credentials?

Warpgate never stores credentials in config files.

- **Container registries:** Uses Docker's credential helpers
- **AWS:** Uses AWS SDK credential chain (SSO, environment variables, IAM roles)
- **Secrets in templates:** Pass via variables at build time, never hardcode

See [Configuration Guide - Security Best Practices](configuration.md#security-best-practices)
for detailed setup instructions.

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

**Responsible use:**

Only use for:

- Authorized security testing
- Educational purposes
- Defensive security operations
- Your own systems

Never use for:

- Unauthorized access
- Malicious purposes

## Performance

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

### My template isn't found, what's wrong?

Templates must be in a configured repository and the cache must be updated. Run
`warpgate templates update` to refresh the cache.

See [Troubleshooting - Templates Not Found](troubleshooting.md#templates-not-found)
for detailed diagnostics.

### Builds work but pushes fail?

This usually means you're not authenticated to the registry. Authenticate
with `docker login` and try again.

See [Troubleshooting - Registry Issues](troubleshooting.md#registry-issues) for
complete solutions.

## Still Have Questions?

**Documentation:**

- [Usage Guide](usage-guide.md) - Practical examples
- [Configuration Guide](configuration.md) - Setup and config
- [Troubleshooting](troubleshooting.md) - Common problems
- [Template Format](template-format.md) - Template syntax

**Community:**

- [GitHub Issues](https://github.com/CowDogMoo/warpgate/issues) - Report bugs
- [Contributing Guide](../CONTRIBUTING.md) - Contribute to Warpgate
