# Packer Build for Atomic Red Team Images (Docker & AMI)

This repository contains Packer templates to build Atomic Red Team
**Docker images** (for both `amd64` and `arm64`) or AWS **AMIs** (Ubuntu-based
EC2 images). The build will provision required packages, tools, and run Ansible
roles to configure the system.

---

## Requirements

- [Packer](https://www.packer.io/)
- Access to AWS (for AMI build)
- Docker (if building Docker images)
- Ansible roles/playbooks (see `provision_repo_path`)
- Required Packer plugins: `docker`, `amazon`, and `ansible`

---

## Variables

Many build-time variables are configurable via the command line or environment
(see `variables.pkr.hcl`).

The most important are:

- `provision_repo_path`: Path to provisioning repo (e.g., `${HOME}/ansible-collection-arsenal`)
- `blueprint_name`: Image name prefix, e.g. `atomic-red-team`
- `ami_region`: AWS region for the AMI (default: `us-east-1`)
- `os_version`: OS version (`ubuntu:jammy` by default)

---

## Building Docker Images

This will build Docker images for both `amd64` and `arm64` platforms.

**Command:**

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_DIR=blueprints/atomic-red-team/packer_templates \
  TEMPLATE_NAME=atomic-red-team \
  ONLY='atomic-red-team-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal blueprint_name=atomic-red-team"
```

---

## Building AWS AMIs

To build an AWS AMI (for Ubuntu, via `amazon-ebs`):

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_DIR=blueprints/atomic-red-team/packer_templates \
  TEMPLATE_NAME=atomic-red-team \
  ONLY='atomic-red-team-ami.amazon-ebs.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal blueprint_name=atomic-red-team"
```

_Ensure your AWS credentials and permissions are set up for AMI creation._

---

## Pushing Docker Images to GitHub Container Registry

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-push \
  NAMESPACE=l50 \
  IMAGE_NAME=atomic-red-team \
  GITHUB_TOKEN=$(gh auth token) \
  GITHUB_USER=l50
```

---

## Notes

- The build uses both **shell and Ansible provisioners**. Ensure your
  provisioning playbooks and requirement files are available at the path
  specified by `provision_repo_path`.
- For the **AMI build**, the template creates and tags an AMI in your AWS account.
- For **Docker**, the images will be locally available after the build for both architectures.
