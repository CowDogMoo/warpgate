# Packer Build for Docker Image

This Packer build script is designed to create a Docker image for an attack box
using the `ttpforge-docker` builder. The script includes various provisioners
to set up the environment, including installing necessary packages and
configuring the system.

```bash
task -y template-build TEMPLATE_DIR=blueprints/ttpforge/packer_templates \
  TEMPLATE_NAME=ttpforge \
  ONLY='ttpforge-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-arsenal \
  blueprint_name=ttpforge"
```
