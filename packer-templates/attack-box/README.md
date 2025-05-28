# Packer Build for Attack-Box Images (Docker & AMI)

This repository contains Packer templates to build **Attack-Box**
Docker images (for both `amd64` and `arm64`) or AWS **AMIs** (Kali Linux-based
EC2 images). The build provisions required packages, tools, and runs Ansible
roles to configure the system for security testing and red teaming.

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

- `blueprint_name`: Image name prefix, e.g. `attack-box`
- `provision_repo_path`: Path to provisioning repo (e.g., `${HOME}/ansible-collection-arsenal`)
- `ami_region`: AWS region for the AMI (default: `us-east-1`)
- `os_version`: Kali image version (`last-snapshot` by default)

---

## Building Docker Images

This builds Attack Box Docker images for `amd64` and `arm64`, installs
prerequisites, and provisions using Ansible roles.

**Commands:**

Initialize the template:

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-init \
  TEMPLATE_NAME=attack-box \
  ONLY='attack-box-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal \
  template_name=attack-box" UPGRADE=true
```

Run the build:

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_NAME=attack-box \
  ONLY='attack-box-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal template_name=attack-box"
```

After the build, multi-arch Attack Box Docker images will be available locally.

---

## Building AWS AMIs

To build an AWS AMI (Kali-based, via `amazon-ebs`):

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_NAME=attack-box \
  ONLY='attack-box-ami.amazon-ebs.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal template_name=attack-box"
```

> 🛡️ Ensure your AWS credentials are configured and your IAM instance profile
> allows SSM usage and AMI creation.

---

## Pushing Docker Images to GitHub Container Registry

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-push \
  NAMESPACE=l50 \
  IMAGE_NAME=attack-box \
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
