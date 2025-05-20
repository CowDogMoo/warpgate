# Docker Image Management Guide

This guide provides instructions for building, pushing, and managing Docker
images, including multi-architecture support.

## Table of Contents

- [Pushing Docker Images](#pushing-docker-images)
  - [Automatic Push](#automatic-push)
  - [Manual Push](#manual-push)
- [Finding Image Hashes](#finding-image-hashes)
- [Multi-architecture Images](#multi-architecture-images)
  - [Building Per-Architecture Images](#building-per-architecture-images)
  - [Creating a Multi-architecture Manifest](#creating-a-multi-architecture-manifest)
  - [Benefits of Multi-architecture Images](#benefits-of-multi-architecture-images)
  - [Verifying and Troubleshooting](#verifying-and-troubleshooting)
- [Troubleshooting Permission Issues](#troubleshooting-permission-issues)

## Pushing Docker Images

### Automatic Push

The recommended approach is to use the automatic push process provided by Warp
Gate. This process handles the building and pushing of images for you.

### Manual Push

If you encounter errors during the automatic image push process, follow these
steps for manual pushing:

1. **Authenticate with GitHub Container Registry**:

   ```bash
   echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
   ```

1. **Tag the image** with the appropriate name (replace values as needed):

   ```bash
   docker tag IMAGE_HASH ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:TAG
   ```

1. **Push the tagged image**:

   ```bash
   docker push ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:TAG
   ```

#### Example

```bash
# Login to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u l50 --password-stdin

# Tag the image using the hash extracted from the build logs
docker tag bb12db786d719cfe438fb666b2a9f9788c4add4bb9217fd87e56122b0c7576c5 ghcr.io/l50/sliver:arm64

# Push the image
docker push ghcr.io/l50/sliver:arm64
```

## Finding Image Hashes

You need the image hash to manually tag and push images. Here's how to find it:

### From Warp Gate Build Logs

Look for output similar to:

```bash
--> docker.arm64: Imported Docker image: sha256:bb12db786d719cfe438fb666b2a9f9788c4add4bb9217fd87e56122b0c7576c5
```

or

```bash
Updated ImageHashes: [{amd64 linux } {arm64 linux } {arm64 linux bb12db786d719cfe438fb666b2a9f9788c4add4bb9217fd87e56122b0c7576c5}]
```

### If You've Lost the Build Logs

If you no longer have access to the Warp Gate build logs:

1. **List all Docker images**:

   ```bash
   docker images
   ```

1. **For images without repository/tag names** (shown as `<none>`):

   ```bash
   docker images --all
   ```

1. **Search by specific criteria**:

   ```bash
   # List all images built in the last day
   docker images --all --filter "since=24h"

   # List all images with specific label from your blueprint
   docker images --all --filter "label=org.opencontainers.image.source=github.com/YOUR_GITHUB_USERNAME/YOUR_REPO"
   ```

1. **For detailed inspection of an image**:

   ```bash
   docker inspect IMAGE_ID
   ```

## Multi-architecture Images

Multi-architecture images ensure your container can run seamlessly across
various platforms (like ARM64 and AMD64) without users needing to specify
architecture-specific tags.

### Building Per-Architecture Images

1. **Authenticate with GitHub Container Registry**:

   ```bash
   echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
   ```

1. **Tag architecture-specific images**:

   ```bash
   # For ARM64
   docker tag IMAGE_HASH_ARM64 ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:arm64

   # For AMD64
   docker tag IMAGE_HASH_AMD64 ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:amd64
   ```

1. **Push each architecture-specific image**:

   ```bash
   docker push ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:arm64
   docker push ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:amd64
   ```

### Creating a Multi-architecture Manifest

After pushing individual architecture images:

1. **Create the manifest** that references all architecture-specific images:

   ```bash
   docker manifest create ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:latest \
     ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:arm64 \
     ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:amd64
   ```

1. **(Optional) Add architecture annotations** if needed:

   ```bash
   docker manifest annotate ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:latest \
     ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:arm64 --arch arm64

   docker manifest annotate ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:latest \
     ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:amd64 --arch amd64
   ```

1. **Push the manifest list**:

   ```bash
   docker manifest push ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:latest
   ```

#### Complete Example

```bash
# First authenticate
echo $GITHUB_TOKEN | docker login ghcr.io -u l50 --password-stdin

# Tag the ARM64 image
docker tag bcf70299b531b266c2cca4641872e5303b17f03c746d9fb680e2fe959ce872a4 ghcr.io/l50/ttpforge:arm64

# Push the ARM64 image
docker push ghcr.io/l50/ttpforge:arm64

# Tag the AMD64 image
docker tag 8c83e8128d4b31713c82bb2f7f0d6af2cba230397d7578023ad05f6dd22edb65 ghcr.io/l50/ttpforge:amd64

# Push the AMD64 image
docker push ghcr.io/l50/ttpforge:amd64

# Create and push the multi-architecture manifest
docker manifest create ghcr.io/l50/ttpforge:latest \
  ghcr.io/l50/ttpforge:arm64 \
  ghcr.io/l50/ttpforge:amd64

docker manifest push ghcr.io/l50/ttpforge:latest
```

### Benefits of Multi-architecture Images

- **Simplified user experience**: Users can simply pull `image:latest` without
  worrying about architecture compatibility
- **Automatic platform matching**: Docker automatically pulls the correct image
  for the user's architecture
- **Versioning support**: You can create manifest lists for specific versions
  as well (e.g., `image:v1.0`)
- **CI/CD friendly**: Automated builds can generate and update manifests as
  part of your workflow

### Verifying and Troubleshooting

**Verify your manifest**:

```bash
docker manifest inspect ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:latest
```

**Enable experimental features** if needed:

```bash
export DOCKER_CLI_EXPERIMENTAL=enabled
```

**Verify referenced images exist**:

```bash
docker manifest inspect ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:arm64
docker manifest inspect ghcr.io/YOUR_GITHUB_USERNAME/IMAGE_NAME:amd64
```

For more information on building multi-architecture images, see the [Docker documentation](https://docs.docker.com/build/building/multi-platform/).

## Troubleshooting Permission Issues

If you receive a `403 Forbidden` or `permission_denied` error, ensure your
GitHub token has the proper package permissions:

- `write:packages`
- `read:packages`
- `delete:packages`

Update your token's permissions using:

```bash
gh auth refresh --scopes write:packages,read:packages,delete:packages
```
