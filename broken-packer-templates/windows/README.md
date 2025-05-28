# Windows Blueprint

**Windows Blueprint** builds an Amazon Machine Image (AMI) for Windows Server 2019 using Packer and SSM.

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
- `./scripts/provision.ps1`: Script containing the provisioning logic.
- `./scripts/choco.ps1`: Script for installing Chocolatey.
- `./scripts/bootstrap_win.txt`: Script for bootstrapping the Windows instance.
- `./packer_templates/plugins.pkr.hcl`: Packer configuration file for required plugins.
- `./packer_templates/windows.pkr.hcl`: Main Packer template file for the Windows blueprint.

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

### Building the AMI

Use warpgate to build a Windows AMI based on the `windows` blueprint:

```bash
wg imageBuilder \
  -b "windows" \
  -p "$HOME/security/ansible-collection-arsenal" \
  -t "$(op item get 'CowDogMoo/warpgate BOT TOKEN' --fields token)"
```

### Additional Notes

- Pull the Docker image:

  ```bash
  docker pull ghcr.io/l50/windows:latest
  ```

- Run the Docker container:

  ```bash
  docker run -it --rm \
    --privileged \
    --volume /sys/fs/cgroup:/sys/fs/cgroup:rw \
    --cgroupns host \
    --entrypoint /bin/bash \
    --user windows \
    --workdir /home/windows \
    ghcr.io/l50/windows:latest
  ```
