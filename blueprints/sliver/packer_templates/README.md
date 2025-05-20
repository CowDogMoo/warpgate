# Packer Build for Docker Image

This Packer build script is designed to create a Docker image for an attack box
using the `sliver-docker` builder. The script includes various provisioners
to set up the environment, including installing necessary packages and
configuring the system.

```bash
export TASK_X_REMOTE_TASKFILES=1
task -y template-build TEMPLATE_DIR=blueprints/sliver/packer_templates \
  TEMPLATE_NAME=sliver \
  ONLY='sliver-docker.docker.*' \
  VARS="workstation_repo_path=${HOME}/CowDogMoo/ansible-collection-workstation \
  arsenal_repo_path=${HOME}/ansible-collection-arsenal \
  blueprint_name=sliver"
```
