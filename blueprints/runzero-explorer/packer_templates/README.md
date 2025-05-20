# Packer Build for Docker Image

This Packer build script is designed to create a Docker image for an attack box
using the `runzero-explorer-docker` builder. The script includes various provisioners
to set up the environment, including installing necessary packages and
configuring the system.

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build TEMPLATE_DIR=blueprints/runzero-explorer/packer_templates \
  TEMPLATE_NAME=runzero-explorer \
  ONLY='runzero-explorer-docker.docker.*' \
  VARS="provision_repo_path=${HOME}/ansible-collection-bulwark \
  blueprint_name=runzero-explorer"
```
