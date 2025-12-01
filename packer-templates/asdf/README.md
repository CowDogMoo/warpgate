# asdf Container Template

A minimal container image template with [asdf](https://asdf-vm.com/) version
manager pre-installed. This template uses Packer and the
ansible-collection-workstation asdf role to create a very small,
multi-architecture container image.

## Features

- **Minimal Base**: Built on Ubuntu 22.04 with minimal dependencies
- **Multi-Architecture**: Supports both amd64 and arm64 platforms
- **asdf Pre-installed**: Ready to install and manage multiple runtime versions
- **Aggressive Cleanup**: Optimized for small image size
- **CI/CD Ready**: Integrates with GitHub Actions for automated builds

## Prerequisites

- [Packer](https://www.packer.io/) >= 1.7.0
- [Task](https://taskfile.dev/) (for simplified build commands)
- [Ansible](https://www.ansible.com/) >= 2.9
- [Docker](https://www.docker.com/)
- [ansible-collection-workstation](https://github.com/CowDogMoo/ansible-collection-workstation)
  cloned to `$HOME/CowDogMoo/ansible-collection-workstation`

## Quick Start

### Using Task (Recommended)

```bash
# Initialize the template
task template-init TEMPLATE_NAME=asdf

# Validate the configuration
task template-validate TEMPLATE_NAME=asdf

# Build for both architectures
task template-build \
  TEMPLATE_NAME=asdf \
  ONLY='asdf-docker.docker.*' \
  VARS="provision_repo_path=${HOME} template_name=asdf"

# Build for specific architecture only
task template-build \
  TEMPLATE_NAME=asdf \
  ONLY='asdf-docker.docker.amd64' \
  VARS="provision_repo_path=${HOME} template_name=asdf"

# Push to registry
task template-push TEMPLATE_NAME=asdf
```

### Using Packer Directly

```bash
cd packer-templates/asdf

# Initialize Packer plugins
packer init .

# Validate the template
packer validate \
  -var "provision_repo_path=${HOME}" \
  -var "template_name=asdf" \
  .

# Build the image
packer build \
  -var "provision_repo_path=${HOME}" \
  -var "template_name=asdf" \
  .
```

## Configuration Variables

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `base_image` | Base container image | `ubuntu` |
| `base_image_version` | Base image version tag | `22.04` |
| `provision_repo_path` | Path to collection | `$HOME` |
| `template_name` | Name for container image | `asdf` |
| `shell` | Shell for provisioning | `/bin/bash` |
| `container_registry` | Container registry URL | `ghcr.io` |
| `registry_namespace` | Registry namespace | `cowdogmoo` |
| `container_user` | Non-root user | `asdf` |
| `container_uid` | UID for container user | `1000` |
| `container_gid` | GID for container user | `1000` |

## Customization

### Using a Different Base Image

```bash
task template-build \
  TEMPLATE_NAME=asdf \
  VARS="base_image=debian base_image_version=bookworm-slim provision_repo_path=${HOME} template_name=asdf"
```

### Custom Registry

```bash
task template-build \
  TEMPLATE_NAME=asdf \
  VARS="container_registry=docker.io registry_namespace=myuser provision_repo_path=${HOME} template_name=asdf"
```

## Usage

### Option 1: Use as a Base Image (Recommended)

Create a `Dockerfile` in your project:

```dockerfile
FROM ghcr.io/cowdogmoo/asdf:latest

# Copy your .tool-versions file
COPY .tool-versions /workspace/.tool-versions

# Install all tools defined in .tool-versions
RUN asdf plugin add nodejs && \
    asdf plugin add python && \
    asdf plugin add golang && \
    asdf install && \
    asdf reshim

# Copy your application code
COPY . /workspace

# Your application commands
CMD ["node", "app.js"]
```

Create a `.tool-versions` file in your project root:

```text
nodejs 20.10.0
python 3.11.7
golang 1.21.5
```

Build your application image:

```bash
docker build -t myapp:latest .
```

### Option 2: Interactive Development

```bash
# Pull the image
docker pull ghcr.io/cowdogmoo/asdf:latest

# Run interactively with volume mount for your code
docker run -it -v $(pwd):/workspace ghcr.io/cowdogmoo/asdf:latest

# Inside the container, install and use tools
asdf plugin add nodejs
asdf install nodejs latest
asdf global nodejs latest
node --version
```

### Option 3: Use with Docker Compose

```yaml
version: '3.8'
services:
  dev:
    image: ghcr.io/cowdogmoo/asdf:latest
    volumes:
      - .:/workspace
      - asdf_cache:/home/asdf/.asdf
    working_dir: /workspace
    command: tail -f /dev/null

volumes:
  asdf_cache:
```

### Option 4: CI/CD Pipeline

```yaml
# GitHub Actions example
jobs:
  test:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/cowdogmoo/asdf:latest
    steps:
      - uses: actions/checkout@v4
      - name: Install tools from .tool-versions
        run: |
          for plugin in $(awk '{print $1}' .tool-versions); do
            asdf plugin add $plugin
          done
          asdf install
      - name: Run tests
        run: npm test
```

### Security: Running with Your UID/GID

To avoid permission issues with mounted volumes, run as your user:

```bash
docker run -it \
  --user $(id -u):$(id -g) \
  -v $(pwd):/workspace \
  ghcr.io/cowdogmoo/asdf:latest
```

Or create a custom Dockerfile:

```dockerfile
FROM ghcr.io/cowdogmoo/asdf:latest

# Match your host user (pass as build args)
ARG USER_UID=1000
ARG USER_GID=1000

USER root
RUN usermod -u ${USER_UID} asdf && \
    groupmod -g ${USER_GID} asdf && \
    chown -R asdf:asdf /home/asdf /workspace

USER asdf
```

## Image Size Optimization

This template includes several optimizations to minimize image size:

1. **Minimal Dependencies**: Only installs Python, Git, Curl, and CA
   certificates
2. **No Recommends**: Uses `--no-install-recommends` flag for apt
3. **Aggressive Cleanup**: Removes Ansible artifacts, package lists, logs, and
   caches
4. **Base Image Choice**: Ubuntu 22.04 provides a good balance of compatibility
   and size

To further reduce size, consider:

- Using `ubuntu:22.04-minimal` or Alpine Linux as the base
- Installing only specific asdf plugins you need in the playbook
- Using multi-stage builds if additional build tools are needed

## CI/CD Integration

To add this template to the GitHub Actions workflow, update `.github/workflows/warpgate-image-builder.yaml`:

```yaml
matrix:
  template:
    - name: asdf
      namespace: cowdogmoo
      vars: "provision_repo_path=${HOME} template_name=asdf"
    # ... other templates
```

## Troubleshooting

### Ansible Galaxy Collection Not Found

Ensure ansible-collection-workstation is cloned:

```bash
git clone https://github.com/CowDogMoo/ansible-collection-workstation \
  $HOME/CowDogMoo/ansible-collection-workstation
cd $HOME/CowDogMoo/ansible-collection-workstation
ansible-galaxy collection build --force
ansible-galaxy collection install cowdogmoo-workstation-*.tar.gz -p $HOME/.ansible/collections
```

### Build Fails on Architecture

If building for arm64 fails, ensure Docker has multi-platform support:

```bash
docker run --privileged --rm tonistiigi/binfmt --install all
```

## Related Documentation

- [asdf Documentation](https://asdf-vm.com/guide/getting-started.html)
- [Packer Docker Builder](https://www.packer.io/plugins/builders/docker)
- [ansible-collection-workstation](https://github.com/CowDogMoo/ansible-collection-workstation)
