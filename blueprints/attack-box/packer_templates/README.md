# Packer Build for Attack-Box Images (Docker & AMI)

This repository contains Packer templates to build **Attack-Box**
Docker images (for both `amd64` and `arm64`) or AWS **AMIs** (Kali Linux-based
EC2 images). The build provisions required packages, tools, and runs Ansible
roles to configure the system for security testing and red teaming.

---

## Requirements

- [Packer](https://www.packer.io/)
- Access to AWS (for AMI build)
- Docker (if building Docker images)
- Ansible roles/playbooks (`attack_box` playbooks in your `provision_repo_path`)
- Required Packer plugins: `docker`, `amazon`, and `ansible`

---

## Variables

Many build-time variables are configurable via the command line or environment
(see `variables.pkr.hcl`).

Key variables include:

- `provision_repo_path`: Path to provisioning repo (e.g., `${HOME}/ansible-collection-arsenal`)
- `blueprint_name`: Image name prefix, e.g. `attack-box`
- `ami_region`: AWS region for the AMI (default: `us-east-1`)
- `os_version`: Kali image version (`last-snapshot` by default)

---

## Building Docker Images

This will build Docker images for both `amd64` and `arm64` platforms.

**Command:**

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_DIR=blueprints/attack-box/packer_templates \
  TEMPLATE_NAME=attack-box \
  ONLY='attack-box-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal blueprint_name=attack-box"
```

---

## Building AWS AMIs

To build an AWS AMI (Kali-based, via `amazon-ebs`):

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build \
  TEMPLATE_DIR=blueprints/attack-box/packer_templates \
  TEMPLATE_NAME=attack-box \
  ONLY='attack-box-ami.amazon-ebs.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal blueprint_name=attack-box"
```

_Ensure your AWS credentials and permissions are set up for AMI creation._

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

- The build uses **shell and Ansible provisioners**. Ensure your
  provisioning playbooks and requirements files for `attack_box` are available
  at the path specified by `provision_repo_path`.
- The **AMI build** creates and tags a Kali-based AMI in your AWS account with
  EC2 Session Manager (SSM) support enabled.
- For **Docker**, images for both architectures will be available locally after
  the build.
- Example Ansible playbook is expected at:
  - `${provision_repo_path}/playbooks/attack_box/attack_box.yml`
- Customizations such as default user, disk size, and instance type can be
  controlled via `variables.pkr.hcl` or VARS CLI argument.
