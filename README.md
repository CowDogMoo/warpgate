# Warp Gate

[![License](https://img.shields.io/github/license/CowDogMoo/warpgate?label=License&style=flat&color=blue&logo=github)](https://github.com/CowDogMoo/warpgate/blob/main/LICENSE)
[![🚨 Semgrep Analysis](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/semgrep.yaml)
[![Pre-Commit](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/pre-commit.yaml)
[![Renovate](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml/badge.svg)](https://github.com/CowDogMoo/warpgate/actions/workflows/renovate.yaml)

<img src="docs/images/wg-logo.jpeg" alt="Warp Gate Logo" width="100%">

**Warp Gate** is a robust, automatable engine for building security labs,
golden images, and multi-architecture containers using modular Packer templates
and Taskfile-driven workflows. Warp Gate images spin up rapidly for use in
security labs, cyber ranges, DevOps CI, and immutable infrastructure.

---

## Refactor Notice

**With the latest release, Warp Gate has migrated to a
[Taskfile](https://taskfile.dev)-centric build system**
using flexible Packer templates.
_All legacy Go commands and variables are replaced!_
If updating from an older version, **read this README and follow the new workflow**.

---

## Key Features & Recent Changes

- **Modular, directory-based Packer templates**—each configuration is self-contained.
- **Modern Packer support for Docker and AMI builds** (atomic-red-team,
  attack-box, sliver, ttpforge, runzero-explorer and more).
- **Multi-architecture (amd64/arm64) image support.**
- **Unified Taskfile workflows:** templating, validation, building, digest
  handling, registry pushes, secrets, upgrades, and more.
- **Local testing in CI with [act](https://github.com/nektos/act).**
- **Automated secrets/credential usage for container registry pushes.**
- **Namespace/image customizations per template.**
- **Thorough, up-to-date docs for all build/test/push flows.**

---

## Getting Started

### Prerequisites

- [go-task](https://taskfile.dev/installation/)
  (`brew install go-task/tap/go-task` or see docs)
- [Packer](https://www.packer.io/downloads)
- [Docker](https://www.docker.com/)
- [jq](https://stedolan.github.io/jq/)
- [act](https://github.com/nektos/act) (for local GitHub Actions runs)
- (Optional) [GitHub CLI](https://cli.github.com/)
- (Optional) [Ansible](https://www.ansible.com/)

### Clone the Repo

```bash
gh repo clone CowDogMoo/warpgate
cd warpgate
```

---

## Usage

### Typical Build/Pipeline Flow

Templates are managed under `packer-templates/`. Each template provides Packer
files for Docker/AMI builds, variable definitions, and scripts.

1. Initialize a Template

   Creates lockfiles, initializes submodules, and gets everything ready:

   ```bash
   export TASK_X_REMOTE_TASKFILES=1   # (Usually a no-op, good habit for portability)
   task template-init -- TEMPLATE_NAME=attack-box
   ```

1. Validate a Template

   Checks all template variables, format, and Packer syntax:

   ```bash
   task template-validate -- TEMPLATE_NAME=attack-box
   ```

1. Build an Image (Docker or AMI)

   **Docker Example (multi-arch, with custom VARS):**

   ```bash
   task template-build \
     -- TEMPLATE_NAME=atomic-red-team \
     ONLY='atomic-red-team-docker.docker.*' \
     VARS="provision_repo_path=${HOME}/ansible-collection-arsenal template_name=atomic-red-team"
   ```

   **AMI Example (Ubuntu-based):**

   ```bash
   task template-build \
     -- TEMPLATE_NAME=atomic-red-team \
     ONLY='atomic-red-team-ami.amazon-ebs.*' \
     VARS="provision_repo_path=${HOME}/ansible-collection-arsenal template_name=atomic-red-team"
   ```

   > For any image, set `TEMPLATE_NAME=my-template` and add custom variables
   > with VARS (see below).

1. Push Docker Images to GitHub Container Registry (GHCR)

   After a multi-arch build succeeds:

   ```bash
   task template-push \
     -- NAMESPACE=l50 \
       IMAGE_NAME=atomic-red-team \
       GITHUB_TOKEN=$(gh auth token) \
       GITHUB_USER=l50
   ```

   You can also push per-arch images by digest, and/or create/update a manifest:

   ```bash
   # Push just arm64 by digest:
   task template-push-digest \
     -- NAMESPACE=l50 \
       IMAGE_NAME=atomic-red-team \
       ARCH=arm64 \
       GITHUB_TOKEN=$(gh auth token) \
       GITHUB_USER=l50

   # Merge/push the multi-arch manifest:
   task template-create-manifest \
     -- NAMESPACE=l50 \
       IMAGE_NAME=atomic-red-team \
       GITHUB_TOKEN=$(gh auth token) \
       GITHUB_USER=l50
   ```

1. Run Everything in CI, or Simulate CI Locally

   You can use [GitHub Actions](.github/workflows/image-builder.yaml) for all of
   the above.
   Want a local dry run of your workflows? Use [act](https://github.com/nektos/act):

   ```bash
   task run-image-builder-action -- TEMPLATE=attack-box
   ```

---

## Template Structure & Customization

- All image templates live under `packer-templates/*` (ex: `packer-templates/attack-box`).
- Each template typically contains:
  - `docker.pkr.hcl` and/or `ami.pkr.hcl` (platform-specific builds)
  - `locals.pkr.hcl` (variables)
  - Provisioning scripts/playbooks

**To create a new image template:**

```bash
cp -R packer-templates/attack-box/ packer-templates/my-test-image/
# Edit docker.pkr.hcl, ami.pkr.hcl, and locals.pkr.hcl as needed
# Add provisioning logic (scripts or ansible) to packer-templates/my-test-image/
```

---

## Custom Variables

All template variables can be overridden from the CLI:

- `template_name`: Main image/tag prefix (`attack-box`, `atomic-red-team`, etc)
- `provision_repo_path`: Path to extra provisioning repo/playbooks/scripts
- `ami_region`: AWS region (for AMI builds; default: `us-east-1`)
- `os_version`: OS base/version (default: Ubuntu, specific version per template)

Example to build with custom vars:

```bash
task template-build \
  -- TEMPLATE_NAME=sliver \
     VARS="provision_repo_path=${HOME}/ansible-collection-arsenal template_name=sliver"
```

See each template's `locals.pkr.hcl` and the generated Taskfile for all
supported vars.

---

## Registry Authentication

You must have a **Classic GitHub Personal Access Token** with `write:packages`,
`read:packages`, and `delete:packages` scopes. If you are only pushing to a
single namespace in CI, you can use a fine-grained token.

**Web:**

- Go to GitHub > Settings > Developer Settings > Personal Access Tokens
- Create a token with the required scopes.

**CLI:**

```bash
gh auth refresh --scopes write:packages,read:packages,delete:packages
gh auth status --show-token
```

**Login for Docker:**

```bash
echo "<your_token>" | docker login ghcr.io -u yourusername --password-stdin
```

**Set secrets for Taskfile push:**

- `NAMESPACE`, `IMAGE_NAME`, `GITHUB_TOKEN`, `GITHUB_USER`
  (pass as ENV or CLI args to `task`)

**Secrets troubleshooting:**

```bash
task secrets
```

---

## Migrating from the Go CLI

- **The old `wg` CLI is obsolete—use `task ...` commands as shown above.**
- All image builds, pushes, and CI/CD are done via the Taskfile.
- Variables, secrets, and build steps are now standardized.

---

## Troubleshooting, Advanced Options & Best Practice

- All supported commands/vars are discoverable in `Taskfile.yaml` and in the
  help for `task`.
- Use `VAR_FILES` and per-template overrides for advanced customization.

---

## Contributing

Open Issues for template improvements, workflows, or Taskfile features!
