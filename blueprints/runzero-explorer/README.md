# RunZero-Explorer Blueprint

**RunZero-Explorer Blueprint** builds a container image to run the
[runZero explorer](https://console.runzero.com/deploy/download/explorers).

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Blueprint Structure](#blueprint-structure)
- [Getting Started](#getting-started)
- [Usage](#usage)

---

## Prerequisites

Before you start, ensure you have the following installed:

- [Packer](https://www.packer.io/)
- [gh CLI tool](https://cli.github.com/)
- [Docker](https://www.docker.com/)

## Blueprint Structure

- `./config.yaml`: Configuration file defining the blueprint's basic settings.
- `./variables.pkr.hcl`: Variable definitions for the Packer build.
- `./scripts/provision.sh`: Script containing the provisioning logic.
- `./packer_templates/plugins.pkr.hcl`: Packer configuration file for required
  plugins.
- `./packer_templates/runzero-explorer.pkr.hcl`: Main Packer template file for
  the RunZero-Explorer blueprint.

## Getting Started

1. Ensure you have the necessary [prerequisites](#prerequisites) installed.

1. Clone the Warp Gate repository:

   ```bash
   gh repo clone CowDogMoo/warpgate
   cd warpgate
   ```

1. Compile Warp Gate and add it to `$PATH`:

   ```bash
   go build -o wg && cp ~/.local/bin/wg
   ```

## Usage

### Building the Container Image

1. Set the RUNZERO_DOWNLOAD_TOKEN environment variable:

   ```bash
   export RUNZERO_DOWNLOAD_TOKEN=YOUR_DOWNLOAD_TOKEN_HERE

   # 1password: RunZero Explorer Download Token
   export RUNZERO_DOWNLOAD_TOKEN=$(op item get 'runzero' --fields RUNZERO_DOWNLOAD_TOKEN)
   ```

1. Use warpgate to build local container images based on the `runzero-explorer`
   blueprint:

   ```bash

   wg imageBuilder \
     -b "runzero-explorer" \
     -p "$HOME/cowdogmoo/ansible-collection-workstation"
   ```

### Additional Notes

- Pull the Docker image:

  ```bash
  docker pull ghcr.io/cowdogmoo/runzero-explorer
  ```

- Run the Docker container:

  ```bash
  docker run -it --rm \
   --privileged \
   --volume /sys/fs/cgroup:/sys/fs/cgroup:rw \
   --cgroupns host \
   --entrypoint /bin/bash \
   --user root \
   --workdir /root \
   ghcr.io/cowdogmoo/runzero-explorer:latest
  ```

- Verify the RunZero Explorer service is running:

  ```bash
  systemctl list-unit-files | grep enabled | grep 'runzero*'
  ```
