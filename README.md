# Warp Gate

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![🚨 Semgrep Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)

Warp Gate facilitates the creation of docker images using Packer and
provided forms of provisioning logic, such as Ansible. This allows
for the creation of container images without having to write a
`Dockerfile`. Instead, you can simply use the same code that
you would to provision a VM.

An image is represented as a blueprint, which Warp Gate consumes
to create it.

Feel free to peruse existing `blueprints/` and modify one to fit
your needs.

---

## Table of Contents

- [Developer Environment Setup](docs/dev.md)
- [Usage](#usage)

---

## Usage

Warp in container image from blueprint:

```bash
# Path to the blueprint configuration from the repo root
BLUEPRINT_CFG=blueprints/ansible-vnc-zsh/config.yaml
# Path on disk to the provisioning repo
PROVISIONING_REPO="${HOME}/cowdogmoo/ansible-vnc"
./wg --config "${BLUEPRINT_CFG}" imageBuilder -p "${PROVISIONING_REPO}"
```
