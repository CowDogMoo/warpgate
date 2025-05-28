# Packer Build for Sliver Images (Docker & AMI)

This repository contains Packer templates to build **Sliver C2**
**Docker images** (for both `amd64` and `arm64`) and AWS **AMIs** (Ubuntu-based
EC2 images). The build provisions all required packages, sets up tools, and
runs Ansible roles/playbooks to configure the system for robust TTP simulation
and testing workflows.

---

## Requirements

- [Packer](https://www.packer.io/)
- Docker (for building Docker images)
- AWS credentials with permissions to create AMIs
- Provisioning repositories (e.g., `ansible-collection-arsenal`, `workstation` roles)
- Required Packer plugins:

  - `amazon`
  - `docker`
  - `ansible`

---

## Variables

Configurable via `variables.pkr.hcl` or CLI `VARS=` overrides. Key variables:

- `blueprint_name`: Image name prefix (e.g., `sliver`)
- `provision_repo_path`: Path to arsenal provisioning repo
- `workstation_repo_path`: Path to workstation repo
- `ami_region`: AWS region for AMI build (default: `us-east-1`)
- `os_version`: Ubuntu version (default: `jammy-22.04`)

---

## Building Docker Images

This builds **Sliver** Docker images for `amd64` and `arm64`, installs
prerequisites, and provisions using Ansible roles.

**Commands:**

Initialize the template:

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-init \
  TEMPLATE_NAME=sliver \
  ONLY='sliver-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal \
  workstation_repo_path=${HOME}/ansible-workstation \
  template_name=sliver" UPGRADE=true
```

Run the build:

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_NAME=sliver \
  ONLY='sliver-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal \
  workstation_repo_path=${HOME}/ansible-workstation \
  template_name=sliver"
```

After the build, multi-arch Sliver Docker images will be available locally.

---

## Building AWS AMIs

To build an AWS AMI (Ubuntu-based, via `amazon-ebs`):

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_NAME=sliver \
  ONLY='sliver-ami.amazon-ebs.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal \
  template_name=sliver"
```

> 🛡️ Ensure your AWS credentials are configured and your IAM instance profile
> allows SSM usage and AMI creation.

---

## Pushing Docker Images to GitHub Container Registry

After building the Docker image, you can push it to GHCR:

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-push \
  NAMESPACE=l50 \
  IMAGE_NAME=sliver \
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
