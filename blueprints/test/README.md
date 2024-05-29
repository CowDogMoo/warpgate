# Test Blueprint

**Test Blueprint** builds a container image provisioned with
a test script.

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
- `./packer_templates/test.pkr.hcl`: Main Packer template file for the
  Test blueprint.

## Getting Started

1. Ensure you have the necessary [prerequisites](#prerequisites) installed.

1. Clone the repository containing the Test blueprint:

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

Use warpgate to build local container images based on the `test`
blueprint:

```bash
wg imageBuilder \
  -b "test" \
  -p "$PWD/blueprints/test/scripts/provision.sh" \
  -t "$(op item get 'CowDogMoo/warpgate BOT TOKEN' --fields token)"
```

### Additional Notes

- Pull the Docker image:

  ```bash
  docker pull ghcr.io/l50/test:latest
  ```

- Run the Docker container:

  ```bash
  docker run -it --rm \
    --privileged \
    --entrypoint pwsh \
    --user root \
    ghcr.io/l50/test:latest
  ```
