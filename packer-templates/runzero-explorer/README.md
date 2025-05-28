# Packer Build for runZero Explorer (Docker & AMI)

This repository contains Packer templates to build **runZero Explorer**
**Docker images** (for both `amd64` and `arm64`) and AWS **AMIs** (Ubuntu-based
EC2 images). The build provisions required packages and uses Ansible playbooks
to configure the system for network discovery and security monitoring.

---

## Requirements

- [Packer](https://www.packer.io/)
- Docker (for building Docker images)
- AWS credentials with permissions to create AMIs
- runZero download token (environment variable `RUNZERO_DOWNLOAD_TOKEN`)
- Provisioning repositories (e.g., `ansible-collection-bulwark`)
- Required Packer plugins:

  - `amazon`
  - `docker`
  - `ansible`

---

## Variables

Many parameters are configurable via the command line or environment
(see `variables.pkr.hcl`).

The most important are:

- `blueprint_name`: Image name prefix, e.g. `runzero-explorer`
- `provision_repo_path`: Path to provisioning repo (e.g., `${HOME}/ansible-collection-bulwark`)
- `runzero_download_token`: Token to download runZero Explorer (required)
- `ami_region`: AWS region for AMI build (default: `us-east-1`)
- `instance_type`: EC2 instance type (default: `t3.medium`)
- `os_version`: Ubuntu version (default: `jammy-22.04`)

---

## Building Docker Images

This builds **runZero Explorer Docker images** for `amd64` and `arm64`,
installs prerequisites, and provisions using Ansible roles.

**Commands:**

Set your required variables and initialize the template (if needed):

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-init \
  TEMPLATE_NAME=runzero-explorer \
  ONLY='runzero-explorer-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-bulwark \
  blueprint_name=runzero-explorer" UPGRADE=true
```

Run the build:

```bash
export RUNZERO_DOWNLOAD_TOKEN=your-token-here
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_NAME=runzero-explorer \
  ONLY='runzero-explorer-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-bulwark \
  blueprint_name=runzero-explorer"
```

After the build, **multi-arch runZero Explorer Docker images** will be
available locally.

---

## Building AWS AMIs

To build an **AWS AMI** (Ubuntu-based, via `amazon-ebs`):

```bash
export RUNZERO_DOWNLOAD_TOKEN=your-token-here
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_NAME=runzero-explorer \
  ONLY='runzero-explorer-ami.amazon-ebs.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-bulwark \
  blueprint_name=runzero-explorer"
```

> 🛡️ Ensure your AWS credentials are configured, and your IAM instance profile
> allows SSM usage and AMI creation.

---

## Pushing Docker Images to GitHub Container Registry

After building the Docker image, you can push it to GHCR:

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-push \
  NAMESPACE=cowdogmoo \
  IMAGE_NAME=runzero-explorer \
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
