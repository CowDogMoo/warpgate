# Warp Gate

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/cowdogmoo/warpgate)](https://goreportcard.com/report/github.com/cowdogmoo/warpgate)
[![🚨 CodeQL Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/codeql-analysis.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/codeql-analysis.yaml)
[![🚨 Semgrep Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-Commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)
[![Renovate](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml)

<img src="docs/images/wg-logo.jpeg" alt="Warp Gate Logo" width="100%">

Warp Gate facilitates the creation of container images using [Packer](https://www.packer.io/)
and various forms of provisioning logic, such as [Ansible](https://github.com/ansible/ansible),
or even just a good old Bash script.

This project is for folks who don't want to spend time converting
the logic they use to provision a VM into a Dockerfile and then have
to maintain that logic in two places.

Virtually everything that is required to build a container image
is abstracted into a blueprint, which Warp Gate consumes.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Usage](#usage)
- [Developer Environment Setup](docs/dev.md)
- [Debugging](docs/debug.md)

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

This next example will create a container image using the existing
`runzero-explorer` blueprint and the `runzero-explorrer` ansible playbook
playbook found in the `cowdogmoo.workstation` collection.

Additionally, a `$GITHUB_TOKEN` is provided for the commit and push operations.

```bash
wg imageBuilder \
  -b runzero-explorer \
  -p ~/cowdogmoo/ansible-collection-workstation \
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
