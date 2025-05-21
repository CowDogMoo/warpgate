# Packer Build for TTPForge Images (Docker & AMI)

This repository contains Packer templates to build **TTPForge**
**Docker images** (for both `amd64` and `arm64`) and AWS **AMIs** (Ubuntu-based
EC2 images). The build provisions all required packages, sets up tools, and
runs Ansible roles/playbooks to configure the system for robust TTP simulation
and testing workflows.

---

## Requirements

- [Packer](https://www.packer.io/)
- AWS account & credentials (for AMI builds)
- Docker (for building Docker images)
- Provisioning repo with Ansible roles/playbooks (see `provision_repo_path`)
- Required Packer plugins:

  - `amazon`
  - `docker`
  - `ansible`

---

## Variables

Many parameters are configurable via the command line or environment
(see `variables.pkr.hcl`).

The most important are:

- `blueprint_name`: Image name prefix (default: `ttpforge`)
- `provision_repo_path`: Path to provisioning repo (e.g., `${HOME}/ansible-collection-arsenal`)
- `ami_region`: AWS region for AMI (default: `us-east-1`)
- `os_version`: OS version (default: `jammy-22.04`)

---

## Building Docker Images

This builds TTPForge Docker images for `amd64` and `arm64`, installs
prerequisites, and provisions using Ansible roles.

**Command:**

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_DIR=blueprints/ttpforge/packer_templates \
  TEMPLATE_NAME=ttpforge \
  ONLY='ttpforge-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal blueprint_name=ttpforge"
```

After the build, multi-arch TTPForge Docker images will be available locally.

---

## Building AWS AMIs

To build an AWS AMI (Ubuntu-based, via `amazon-ebs`):

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_DIR=blueprints/ttpforge/packer_templates \
  TEMPLATE_NAME=ttpforge \
  ONLY='ttpforge-ami.amazon-ebs.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal blueprint_name=ttpforge"
```

> 🛡️ Ensure your AWS credentials are configured and your IAM instance profile
> allows SSM usage and AMI creation.

---

## Pushing Docker Images to GitHub Container Registry

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-push \
  NAMESPACE=l50 \
  IMAGE_NAME=ttpforge \
  GITHUB_TOKEN=$(gh auth token) \
  GITHUB_USER=l50
```

---

## Notes

- The build uses both **shell and Ansible provisioners**. Ensure your
  provisioning playbooks and requirement files are available at the path
  specified by `provision_repo_path`.
- **AMI build:**
  - Creates and tags an AMI in your AWS account.
  - Designed to use SSM (Session Manager) for connections where possible.
- **Docker build:**
  - Multi-arch (`amd64` + `arm64`) and privileged for full testbed support.
  - Images are suitable for CI, local testing, or even deployment in a
    kubernetes cluster.
- Customizations such as default user, disk size, and instance type can be
  controlled via `variables.pkr.hcl` or VARS CLI argument.
