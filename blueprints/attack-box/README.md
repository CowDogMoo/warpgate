# Attack-Box Blueprint

**Attack-Box Blueprint** is designed to build a container image tailored for
security testing and penetration testing tasks, leveraging tools and
environments typically used in the field.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Blueprint Structure](#blueprint-structure)
- [Getting Started](#getting-started)
- [Usage](#usage)

---

## Prerequisites

Before you begin, ensure you have the following installed:

- [Packer](https://www.packer.io/)
- [gh CLI tool](https://cli.github.com/)
- [Docker](https://www.docker.com/)

## Blueprint Structure

- `./config.yaml`: Configuration file that outlines the blueprint's fundamental settings.
- `./variables.pkr.hcl`: Variable definitions for the Packer build process.
- `./scripts/provision.sh`: Script that encapsulates the logic for provisioning.
- `./packer_templates/plugins.pkr.hcl`: Packer configuration file for the
  plugins required.
- `./packer_templates/attack-box.pkr.hcl`: Principal Packer template file for
  the Attack-Box blueprint.

## Getting Started

1. Ensure all [prerequisites](#prerequisites) are installed on your system.

2. Clone the repository containing the blueprint:

   ```bash
   gh repo clone CowDogMoo/attack-box
   cd attack-box
   ```

3. Compile Warp Gate and add it to `$PATH`:

   ```bash
   go build -o wg && cp ~/.local/bin/wg
   ```

## Usage

### Building the Container Image

Use warpgate to build local container images based on the `attack-box`
blueprint:

```bash
wg imageBuilder \
  -b "attack-box" \
  -p "$HOME/cowdogmoo/ansible-collection-workstation"
```

### Additional Notes

- To pull the Docker image:

  ```bash
  docker pull ghcr.io/cowdogmoo/attack-box
  ```

- To run the Docker container:

  ```bash
  docker run -it --rm \
   --privileged \
   --volume /sys/fs/cgroup:/sys/fs/cgroup:rw \
   --cgroupns host \
   --entrypoint /run/docker-entrypoint.sh ; zsh \
   --user kali \
   --workdir /home/kali \
   ghcr.io/cowdogmoo/attack-box:latest
  ```

- Verify the container is set up correctly and all necessary services are running:

  ```bash
  docker ps
  ```

  ```bash
  systemctl list-unit-files | grep enabled | grep 'services*'
  ```
