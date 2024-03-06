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

With that out of the way, you can build and push the container image to `GHCR`:

```bash
# Because each architecture we want to support has a different base image
# use build instead of buildx.

# Subsequently, you'll want to run these commands on systems that have the
# appropriate architecture.

export BUILDX_NO_DEFAULT_ATTESTATIONS=1 # Avoid unknown/unknown images from being pushed
echo $GITHUB_TOKEN | docker login ghcr.io -u cowdogmoo --password-stdin

# Build and push ARM64 container image
docker build \
     --build-arg IMG=mcr.microsoft.com/powershell:mariner-2.0-arm64 \
    -t ghcr.io/$YOUR_GITHUB_USER/atomic-red:latest \
    --platform linux/arm64 \
    --push .

# Build and push AMD64 container image
docker build \
     --build-arg IMG=mcr.microsoft.com/powershell:latest \
    -t ghcr.io/$YOUR_GITHUB_USER/atomic-red:latest \
    --platform linux/amd64 \
    --push .
```

## Testing the Container Image

If everything worked, you should now be able to pull the new container image
from `GHCR`:

```bash
docker pull ghcr.io/$YOUR_GITHUB_USER/atomic-red:latest
```
