# Warp Gate

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![🚨 Semgrep Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![🚨 CodeQL Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/codeql-analysis.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/codeql-analysis.yaml)
[![Pre-commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)

<img src="docs/images/wg-logo.jpeg" alt="Warp Gate Logo" width="100%">

Warp Gate facilitates the creation of container images using [Packer](https://www.packer.io/)
and various forms of provisioning logic, such as [Ansible](https://github.com/ansible/ansible),
or even just a good old Bash script.

This project is for folks who don't want to spend time converting
the logic they use to provision a VM into a `Dockerfile`.

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

This example will create a container image using
the existing `ansible-attack-box` blueprint and
the ansible playbook at `~/.ansible/Workspace/ansible-attack-box`.

```bash
./wg imageBuilder -b ansible-attack-box -p ~/.ansible/Workspace/ansible-attack-box
```

This next example will create a container image using
the existing `ansible-vnc-zsh` blueprint and
the ansible playbook at `~/cowdogmoo/ansible-vnc-zsh`.

```bash
./wg imageBuilder -b ansible-vnc-zsh -p ~/cowdogmoo/ansible-vnc-zsh
```

### Create new blueprint skeleton

Create a new blueprint called `new-blueprint` that builds a regular
and a systemd-based container using `kalilinux/kali-rolling:latest`
and `cisagov/docker-kali-ansible:latest` as the base images.

```bash
./wg blueprint -c new-blueprint \
    --systemd \
    --base kalilinux/kali-rolling:latest,cisagov/docker-kali-ansible:latest \
    --tag yourname/your-container-image:latest
```

Be sure to add provisioning logic to `blueprints/new-blueprint/scripts/provision.sh`
and address any relevant TODOs in `config.yaml`.
