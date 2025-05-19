# Packer Build for Docker Image

This Packer build script is designed to create a Docker image for an attack box
using the `sliver-docker` builder. The script includes various provisioners
to set up the environment, including installing necessary packages and
configuring the system.

```bash
packer build \
  -only='sliver-docker.docker.*' \
  -var "workstation_repo_path=${HOME}/CowDogMoo/ansible-collection-workstation" \
  -var "arsenal_repo_path=${HOME}/ansible-collection-arsenal" \
  -var 'blueprint_name=sliver' .
```
