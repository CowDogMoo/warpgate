# Container Image Creation

Begin by cloning the sliver repo locally and building the container image:

```bash
git clone https://github.com/redcanaryco/invoke-atomicredteam.git
cd invoke-atomicredteam/docker
```

## Pushing the Container Image to Github Container Registry

To push the container image to the `GitHub Container Registry` (`GHCR`), you
will need to create a classic personal access token by following
[these instructions](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).

Once you have the token, assign the value to the `GITHUB_TOKEN` environment variable.

Next you'll want to create a new buildx builder instance and set it as the
active builder. This will allow you to build and push the container image to
`GHCR` for multiple architectures.

```bash
export BUILDX_NO_DEFAULT_ATTESTATIONS=1 # Avoid unknown/unknown images from being pushed
docker buildx create --name mybuilder --bootstrap --use --driver docker-container
```

With that out of the way, you can login to `GHCR` and proceed to build and push
the container image:

```bash
YOUR_GITHUB_USER=cowdogmoo # Replace with your GitHub username

# GITHUB_TOKEN is a personal access token with the `write:packages` scope
echo $GITHUB_TOKEN | docker login ghcr.io -u $YOUR_GITHUB_USER --password-stdin

docker buildx bake --file docker-bake.hcl \
  --push \
  --set "*.tags=ghcr.io/$YOUR_GITHUB_USER/atomic-red:latest"
```

## Testing the Container Image

If everything worked, you should now be able to pull the new container image
from `GHCR`:

```bash
docker pull ghcr.io/$YOUR_GITHUB_USER/atomic-red:latest
```
