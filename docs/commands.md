<!-- markdownlint-disable MD024 -->
# Commands Reference

Complete CLI command reference for Warpgate.

## Table of Contents

- [Global Flags](#global-flags)
- [build](#build)
- [validate](#validate)
- [templates](#templates)
- [discover](#discover)
- [manifest](#manifest)
- [convert](#convert)
- [version](#version)

## Global Flags

Available for all commands:

```bash
--config string     Config file path (default: ~/.config/warpgate/config.yaml)
--verbose          Enable verbose logging
--quiet            Suppress non-error output
--help             Show help for command
```

## build

Build container images or AWS AMIs from templates.

### Synopsis

```bash
warpgate build [template-name|file-path] [flags]
```

### Description

The `build` command creates container images or AWS AMIs from Warpgate
templates. Templates can be specified by name (from discovered templates),
file path, or Git URL.

### Examples

```bash
# Build from template name
warpgate build attack-box

# Build from local file
warpgate build ./warpgate.yaml

# Build from Git repository
warpgate build --from-git https://github.com/cowdogmoo/warpgate-templates.git//templates/attack-box

# Build specific architecture
warpgate build attack-box --arch amd64

# Build multiple architectures
warpgate build attack-box --arch amd64,arm64

# Build with variables
warpgate build sliver --var ARSENAL_PATH=/opt/arsenal --var VERSION=1.0.0

# Build from variable file
warpgate build sliver --var-file vars.yaml

# Build and push to registry
warpgate build myimage --push --registry ghcr.io/myorg

# Build AMI
warpgate build my-ami-template --target ami

# Save build digests for signing
warpgate build myimage --save-digests --digest-dir ./digests
```

### Flags

| Flag | Type | Description |
| ------ | ------ | ------------- |
| `--arch` | string | Architectures to build: `amd64`, `arm64` |
| `--var` | string | Override template variable (format: `KEY=value`) |
| `--var-file` | string | Load variables from YAML file |
| `--push` | bool | Push image to registry after build |
| `--registry` | string | Container registry URL (e.g., `ghcr.io/myorg`) |
| `--save-digests` | bool | Save image digests to files |
| `--digest-dir` | string | Directory to save digests (default: `./digests`) |
| `--target` | string | Build target: `container` (default), `ami` |
| `--from-git` | string | Build from Git repository URL |

### Variable Precedence

When the same variable is defined in multiple places:

1. **CLI flags (`--var`)** - Highest precedence
2. **Variable files (`--var-file`)**
3. **Environment variables**
4. **Template defaults** - Lowest precedence

### Exit Codes

- `0` - Build succeeded
- `1` - Build failed
- `2` - Template validation failed

## validate

Validate template syntax and configuration.

### Synopsis

```bash
warpgate validate [template-name|file-path] [flags]
```

### Description

The `validate` command checks template syntax, required fields, and
configuration validity without performing a build.

### Examples

```bash
# Validate template file
warpgate validate warpgate.yaml

# Validate template from repository
warpgate validate attack-box

# Validate with variable substitution
warpgate validate sliver --var ARSENAL_PATH=/opt/arsenal
```

### Flags

| Flag | Type | Description |
| ------ | ------ | ------------- |
| `--var` | string | Override variable for validation (format: `KEY=value`) |
| `--var-file` | string | Load variables from YAML file |
| `--syntax-only` | bool | Check syntax only, skip semantic validation |

### Exit Codes

- `0` - Template is valid
- `1` - Template has errors
- `2` - File not found or parse error

## templates

Manage template repositories and discovery.

### Synopsis

```bash
warpgate templates [subcommand] [flags]
```

### Subcommands

#### list

List all available templates from configured sources.

```bash
warpgate templates list
```

**Output format:**

```text
NAME              VERSION   SOURCE      DESCRIPTION
attack-box        1.0.0     official    Security testing environment
sliver            1.0.0     official    Sliver C2 framework
atomic-red-team   1.0.0     official    Atomic Red Team test platform
```

#### info

Show detailed information about a specific template.

```bash
warpgate templates info [template-name]
```

**Example:**

```bash
warpgate templates info attack-box
```

**Output includes:**

- Template metadata
- Base image
- Provisioners
- Build targets
- Variables with defaults

#### add

Add a new template source (Git repository or local directory).

```bash
warpgate templates add [name] [url|path]
```

**Examples:**

```bash
# Add Git repository (auto-generates name)
warpgate templates add https://github.com/myorg/templates.git

# Add with custom name
warpgate templates add my-templates https://github.com/myorg/templates.git

# Add local directory
warpgate templates add ~/my-warpgate-templates

# Add private repository
warpgate templates add private git@github.com:myorg/private-templates.git
```

#### remove

Remove a template source by name.

```bash
warpgate templates remove [name]
```

**Example:**

```bash
warpgate templates remove my-templates
```

#### update

Update template cache from all configured sources.

```bash
warpgate templates update
```

**What it does:**

- Pulls latest changes from Git repositories
- Scans local directories for new templates
- Rebuilds template index
- Removes stale templates

**When to use:**

- After adding a new template source
- To get latest template changes
- When templates appear missing

### Configuration

Template sources can also be configured in `~/.config/warpgate/config.yaml`:

```yaml
templates:
  repositories:
    official: https://github.com/cowdogmoo/warpgate-templates.git
    custom: /path/to/local/templates
    private: git@github.com:myorg/private.git
```

See [Template Configuration Guide](template-configuration.md) for details.

## discover

Discover templates from configured sources.

### Synopsis

```bash
warpgate discover [flags]
```

### Description

The `discover` command scans configured template sources and displays
available templates. It's an alias for `warpgate templates list`.

### Examples

```bash
# Discover all templates
warpgate discover

# Same as:
warpgate templates list
```

## manifest

Manage multi-architecture image manifests.

### Synopsis

```bash
warpgate manifest [subcommand] [flags]
```

### Description

The `manifest` command creates, pushes, and inspects multi-architecture image
manifests that reference images for different CPU architectures.

### Subcommands

#### create

Create a multi-architecture manifest.

```bash
warpgate manifest create --name [registry/image:tag] --images [image1,image2,...]
```

**Example:**

```bash
warpgate manifest create \
  --name ghcr.io/myorg/myimage:latest \
  --images ghcr.io/myorg/myimage:latest-amd64,ghcr.io/myorg/myimage:latest-arm64
```

**Flags:**

| Flag      | Type   | Description                                  |
| --------- | ------ | -------------------------------------------- |
| `--name`  | string | Manifest name (fully qualified registry path)|
| `--images`| string | Comma-separated list of images to include    |

#### push

Push a manifest to a container registry.

```bash
warpgate manifest push [registry/image:tag]
```

**Example:**

```bash
warpgate manifest push ghcr.io/myorg/myimage:latest
```

**Prerequisites:**

- Manifest must be created first (`manifest create`)
- Must be authenticated to registry (`docker login`)

#### inspect

Inspect a multi-architecture manifest.

```bash
warpgate manifest inspect [registry/image:tag]
```

**Example:**

```bash
warpgate manifest inspect ghcr.io/myorg/myimage:latest
```

**Output includes:**

- Supported platforms (OS/architecture)
- Image digests
- Image sizes
- Created dates

### Complete Workflow Example

```bash
# 1. Build images for multiple architectures
warpgate build myimage --arch amd64,arm64 --push --registry ghcr.io/myorg

# 2. Create manifest referencing both images
warpgate manifest create \
  --name ghcr.io/myorg/myimage:latest \
  --images ghcr.io/myorg/myimage:latest-amd64,ghcr.io/myorg/myimage:latest-arm64

# 3. Push manifest to registry
warpgate manifest push ghcr.io/myorg/myimage:latest

# 4. Verify manifest
warpgate manifest inspect ghcr.io/myorg/myimage:latest

# 5. Users can now pull and get the correct architecture automatically
docker pull ghcr.io/myorg/myimage:latest
```

## convert

Convert Packer templates to Warpgate format.

### Synopsis

```bash
warpgate convert [packer-template.pkr.hcl] [flags]
```

### Description

The `convert` command converts HashiCorp Packer HCL templates to Warpgate YAML
format. This is a Beta feature and may require manual adjustments.

### Examples

```bash
# Convert Packer template
warpgate convert packer-template.pkr.hcl

# Specify output file
warpgate convert packer-template.pkr.hcl --output my-warpgate.yaml
```

### Flags

| Flag | Type | Description |
| --- | --- | --- |
| `--output` | string | Output file path (default: `warpgate.yaml`) |

### Limitations

**Supported:**

- Basic Packer structure
- Shell provisioners
- Ansible provisioners
- Docker and Amazon EBS builders
- Variables

**Not supported:**

- Complex Packer plugins
- Custom provisioners
- Some builder-specific options

**After conversion:**

- Review and test the generated template
- Validate with `warpgate validate`
- Adjust provisioners and variables as needed

## version

Display version information.

### Synopsis

```bash
warpgate version [flags]
```

### Examples

```bash
# Show version
warpgate version

# Show detailed version info
warpgate version --verbose
```

### Output

```text
warpgate version v1.2.3
```

With `--verbose`:

```text
warpgate version v1.2.3
Go version: go1.21.0
Git commit: abc1234
Build date: 2024-01-15T10:30:00Z
Platform: linux/amd64
```

## Environment Variables

Warpgate respects these environment variables:

| Variable | Description |
| --- | --- |
| `WARPGATE_CONFIG` | Config file path (overrides `--config`) |
| `WARPGATE_CACHE_DIR` | Cache directory path |
| `WARPGATE_REGISTRY_USERNAME` | Registry username for authentication |
| `WARPGATE_REGISTRY_TOKEN` | Registry token/password |
| `AWS_PROFILE` | AWS CLI profile to use |
| `AWS_REGION` | AWS region for AMI builds |
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |

## Exit Codes

Warpgate uses these exit codes:

| Code | Meaning                           |
| ---- | --------------------------------- |
| `0`  | Success                           |
| `1`  | General error                     |
| `2`  | Invalid template or configuration |
| `3`  | Network error                     |
| `4`  | Authentication error              |

## Getting Help

```bash
# Show general help
warpgate --help

# Show help for specific command
warpgate build --help

# Show version
warpgate version
```

## See Also

- [Usage Guide](usage-guide.md) - Practical examples and workflows
- [Configuration Guide](configuration.md) - Configuration reference
- [Template Format](template-format.md) - Template syntax reference
- [Troubleshooting](troubleshooting.md) - Common issues and solutions

---

**Need help?** [Open an issue](https://github.com/CowDogMoo/warpgate/issues)
or ask in [Discussions](https://github.com/CowDogMoo/warpgate/discussions).
