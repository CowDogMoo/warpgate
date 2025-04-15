# Warp Gate

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/cowdogmoo/warpgate)](https://goreportcard.com/report/github.com/cowdogmoo/warpgate)
[![🚨 CodeQL Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/codeql-analysis.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/codeql-analysis.yaml)
[![🚨 Semgrep Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-Commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)
[![Renovate](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml)

<img src="docs/images/wg-logo.jpeg" alt="Warp Gate Logo" width="100%">

Warp Gate employs **Blueprints**, YAML configurations that define the
provisioning logic for creating **Odysseys**. These can be either
multi-architecture container images or AWS Golden Images, serving a variety of
use cases from security simulations to rapid deployment. Odysseys offer a broad
spectrum of applications, including, but not limited to:

1. **Security Simulations**:

   - Golden images pre-configured with vulnerabilities for penetration testing
     or cyber range exercises.
   - Container images with specific configurations for simulating attack
     scenarios.

1. **Development and Testing**:

   - Container images ensure consistent environments across development,
     staging, and production, reducing compatibility issues.
   - Golden images provide a standardized base for development and testing,
     ensuring uniformity.

1. **Rapid Deployment and Scaling**:

   - Container images facilitate quick deployment and scaling in microservices
     architectures.
   - Golden images allow for rapid VM deployments with pre-installed
     configurations.

1. **Disaster Recovery**:

   - Golden images enable quick service restoration through pre-configured VMs.
   - Container images ensure minimal downtime by facilitating rapid
     redeployment.

1. **Immutable Infrastructure**:

   - Container images support deploying immutable infrastructures where updates
     are made by replacing containers.
   - Golden images help in setting up immutable servers that are frequently
     recycled and redeployed.

---

## Getting Started

1. Download and install the [gh cli tool](https://cli.github.com/).

1. Clone the repo:

   ```bash
   gh repo clone CowDogMoo/warpgate
   cd warpgate
   ```

1. Get latest warpgate release:

   ```bash
   OS="$(uname | python3 -c 'print(open(0).read().lower().strip())')"
   ARCH="$(uname -a | awk '{ print $NF }')"
   gh release download -p "*${OS}_${ARCH}.tar.gz"
   tar -xvf *tar.gz
   ```

---

## Usage

### Warp image from existing blueprint

This example will create a container image using the existing
`attack-box` blueprint and the `attack-box`
playbook found in the `cowdogmoo.workstation` collection.

```bash
wg imageBuilder \
  -b attack-box \
  -p ~/cowdogmoo/ansible-collection-workstation
```

### Container Image Creation with GitHub Container Registry

This example will create a container image using the existing
`runzero-explorer` blueprint and the `runzero-explorrer` ansible playbook
found in the `cowdogmoo.workstation` collection.

```bash
wg imageBuilder \
  -b runzero-explorer \
  -p ~/ansible-collection-bulwark \
  -t $GITHUB_TOKEN
```

### Create new blueprint skeleton

Create a new blueprint called `new-blueprint` that builds a regular
and a systemd-based container using `kalilinux/kali-rolling:latest`
and `cisagov/docker-kali-ansible:latest` as the base images.

```bash
NAME=yourusername
IMG_NAME=yourcontainerimagename

wg blueprint create new-blueprint \
    --systemd \
    --base kalilinux/kali-rolling:latest,cisagov/docker-kali-ansible:latest \
    --tag $NAME/$IMG_NAME:latest
```

Be sure to add provisioning logic to
`blueprints/new-blueprint/scripts/provision.sh` and address any relevant
TODOs in `config.yaml`.

---

## Authentication & Image Pushing

### Token Setup for GitHub Container Registry

For pushing images to GitHub Container Registry (ghcr.io), you need a token
with the proper package permissions. There are two methods to create this token:

**Option 1: GitHub Web Interface (recommended):**

1. Go to GitHub Settings → Developer Settings → Personal Access Tokens → Tokens (classic)
2. Generate a new token with these permissions:
   - `write:packages`
   - `read:packages`
   - `delete:packages`
   - Add `repo` scope if your repository is private

**Option 2: Using GitHub CLI:**

Use the refresh command to add package scopes to your existing token:

```bash
gh auth refresh --scopes write:packages,read:packages,delete:packages
```

Then set the token as an environment variable:

```bash
export GITHUB_TOKEN=$(gh auth token)
```

### GitHub Actions Authentication

When using GitHub Actions, the workflow logs in to `ghcr.io` using:

```bash
echo "${{ secrets.BOT_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin
```

Note: Warp Gate uses a **Classic Personal Access Token** (`BOT_TOKEN`)
instead of the default `GITHUB_TOKEN`, ensuring that the workflow has the
correct package write permissions.

---

## Additional Documentation

- [Developer Environment Setup](docs/dev.md)
- [Debugging](docs/debug.md)
- [Local Github Action Testing](docs/act.md)
