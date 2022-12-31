# Warp Gate

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![🚨 Semgrep Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)

Warp Gate facilitates the creation of docker images
using [Packer](https://www.packer.io/) and provided forms of provisioning logic,
such as [Ansible](https://github.com/ansible/ansible),
or even just a good old Bash script. This facilitates the creation of container
images without having to write a `Dockerfile`.
Instead, you can simply use the same code that you would to provision a VM.

Everything required to built a container image is represented as a blueprint, which
Warp Gate consumes.

---

## Table of Contents

- [Usage](#usage)
- [Developer Environment Setup](docs/dev.md)
- [Debugging](docs/debug.md)

---

## Usage

### Warp image from existing blueprint

This example will create a container image using
the existing `ansible-attack-box` blueprint and
the ansible playbook at `~/.ansible/Workspace/ansible-attack-box`.

```bash
wg imageBuilder -b ansible-attack-box -p ~/.ansible/Workspace/ansible-attack-box
```

This next example will create a container image using
the existing `ansible-vnc-zsh` blueprint and
the ansible playbook at `~/cowdogmoo/ansible-vnc-zsh`.

```bash
wg imageBuilder -b ansible-vnc-zsh -p ~/cowdogmoo/ansible-vnc-zsh
```

### Create new blueprint skeleton

Create a new blueprint called `new-blueprint` that builds a regular
and a systemd-based container using `kalilinux/kali-rolling:latest`
and `cisagov/docker-kali-ansible:latest` as the base images.

```bash
wg blueprint -c new-blueprint \
    --systemd \
    --base kalilinux/kali-rolling:latest,cisagov/docker-kali-ansible:latest \
    --tag yourname/your-container-image:latest
```

Be sure to add provisioning logic to `blueprints/new-blueprint/scripts/provision.sh`
and address any relevant TODOs in `config.yaml`.
