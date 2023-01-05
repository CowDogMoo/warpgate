# Debugging

Here are a few tips and tricks if you are
running into problems.

## Manual Build

Try building your packer code manually and make sure
you've got everything right. For example:

```bash
packer build -debug \
    -var 'base_image=ubuntu' \
    -var 'base_image_version=latest' \
    -var 'new_image_tag=cowdogmoo/ansible-vnc-zsh' \
    -var 'new_image_version=latest' \
    -var 'provision_repo_path=~/cowdogmoo/ubuntu-vnc-zsh' \
    -var 'setup_systemd=false' \
    -var "registry_cred=$PAT" \
    .
```

---

## Manually Push image

To troubleshoot issues pushing to your container registry,
try pushing an image manually like so:

```bash
GITHUB_USERNAME=CowDogMoo
PAT=ghp_......
IMAGE_TAG=ansible-vnc:latest
docker login ghcr.io -u $GITHUB_USERNAME -p $PAT
docker push ghcr.io/$GITHUB_USERNAME/$IMAGE_TAG
```

Built images from existing blueprints can be
found [here](https://github.com/orgs/CowDogMoo/packages) and [here](https://github.com/l50?tab=packages).
