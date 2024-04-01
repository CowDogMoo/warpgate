# Sliver Blueprint

**Sliver Blueprint** builds a container image to run the
[Sliver framework](https://github.com/BishopFox/sliver), a post-exploitation framework.

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
- `./packer_templates/plugins.pkr.hcl`: Packer configuration file for required plugins.
- `./packer_templates/sliver.pkr.hcl`: Main Packer template file for the Sliver blueprint.

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

Use warpgate to build local container images based on the `attack-box`
blueprint:

```bash
wg imageBuilder \
  -b "sliver" \
  -p "$HOME/security/ansible-collection-arsenal" \
  -t "$(op item get 'CowDogMoo/warpgate BOT TOKEN' --fields token)"
```

### Additional Notes

- Pull the Docker image:

  ```bash
  docker pull ghcr.io/l50/sliver:latest
  ```

- Run the Docker container:

  ```bash
  docker run -it --rm \
    --privileged \
    --volume /sys/fs/cgroup:/sys/fs/cgroup:rw \
    --cgroupns host \
    --entrypoint /bin/bash \
    --user sliver \
    --workdir /home/sliver \
    ghcr.io/l50/sliver:latest
  ```
