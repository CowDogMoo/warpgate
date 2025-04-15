# Manual Image Pushing

If you encounter errors during the automatic image push process, you can
manually push the built images using Docker commands. This is useful for
troubleshooting permission issues or when you need to push images that were
built locally.

## Finding the Image Hash

### From Warp Gate Build Logs

When Warp Gate builds an image, it outputs the image hash in the build logs.
Look for output similar to:

```bash
--> docker.arm64: Imported Docker image: sha256:bb12db786d719cfe438fb666b2a9f9788c4add4bb9217fd87e56122b0c7576c5
```

or

```bash
Updated ImageHashes: [{amd64 linux } {arm64 linux } {arm64 linux bb12db786d719cfe438fb666b2a9f9788c4add4bb9217fd87e56122b0c7576c5}]
```

The hash is the long string starting with `sha256:` or just the alphanumeric
string (depending on the output format).

### If You've Lost the Build Logs

If you no longer have access to the Warp Gate build logs, you can still find
locally built images:

1. List all Docker images to find your recently built images:

   ```bash
   docker images
   ```

1. For images without repository/tag names (shown as `<none>`), use:

   ```bash
   docker images --all
   ```

1. To search by specific criteria (like date or architecture):

   ```bash
   # List all images built in the last day
   docker images --all --filter "since=24h"

   # List all images with specific label from your blueprint
   docker images --all --filter "label=org.opencontainers.image.source=github.com/YOUR_GITHUB_USERNAME/YOUR_REPO"
   ```

1. For detailed inspection of an image, use:

   ```bash
   docker inspect IMAGE_ID
   ```

The image ID shown in these commands is the hash you need for tagging and pushing.

## Manual Push Process

Once you have the image hash, follow these steps to manually push the image:

1. Authenticate with GitHub Container Registry:

   ```bash
   echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
   ```

1. Tag the image with the appropriate name (replace values as needed):

   ```bash
   docker tag IMAGE_HASH ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:TAG
   ```

1. Push the tagged image:

   ```bash
   docker push ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:TAG
   ```

## Example

```bash
# Login to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u l50 --password-stdin

# Tag the image using the hash extracted from the build logs
docker tag bb12db786d719cfe438fb666b2a9f9788c4add4bb9217fd87e56122b0c7576c5 ghcr.io/l50/sliver:arm64

# Push the image
docker push ghcr.io/l50/sliver:arm64
```

## Multi-architecture Images

If you're building multi-architecture images, you'll need to create and push a
manifest list after pushing the individual images. See the
[Docker documentation on multi-architecture images](https://docs.docker.com/build/building/multi-platform/)
for more information.

## Troubleshooting Permission Issues

If you receive a `403 Forbidden` or `permission_denied` error, ensure your
GitHub token has the proper package permissions:

- `write:packages`
- `read:packages`
- `delete:packages`

You can update your token's permissions using:

```bash
gh auth refresh --scopes write:packages,read:packages,delete:packages
```
