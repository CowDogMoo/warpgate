# Debugging

Here are a few tips and tricks if you are
running into problems.

## Manual Build

Try building your packer code manually and make sure
you've got everything right. For example:

```bash
packer build -debug \
    -var 'base_image=cisagov/docker-kali-ansible' \
    -var 'base_image_version=latest' \
    -var 'new_image_tag=cowdogmoo/attack-box' \
    -var 'new_image_version=latest' \
    -var 'provision_repo_path=~/cowdogmoo/ansible-collection-workstation' \
    -var "registry_cred=$GITHUB_TOKEN" \
    .
```

---

## Manually Push image

To troubleshoot issues pushing to your container registry,
try pushing an image manually like so:

```bash
GITHUB_USERNAME=cowdogmoo
GITHUB_TOKEN=ghp_......
IMAGE_TAG=ansible-vnc:latest
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
docker push ghcr.io/$GITHUB_USERNAME/$IMAGE_TAG
```

Built images from existing blueprints can be
found [here](https://github.com/orgs/CowDogMoo/packages) and [here](https://github.com/l50?tab=packages).
