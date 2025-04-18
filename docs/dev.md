# Developer Environment Setup

To get involved with this project,
[create a fork](https://docs.github.com/en/get-started/quickstart/fork-a-repo)
and follow along.

---

## Dependencies

- [Install homebrew](https://brew.sh/):

  ```bash
  # Linux
  sudo apt-get update
  sudo apt-get install -y build-essential procps curl file git
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"

  # macOS
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  ```

- [Install dependencies with brew](https://brew.sh/):

  ```bash
  brew install pre-commit packer
  ```

- [Install asdf](https://asdf-vm.com/):

  ```bash
  git clone https://github.com/asdf-vm/asdf.git ~/.asdf
  ```

- Use asdf to install go:

  ```bash
  asdf install golang 1.24.0
  ```

- [Install Mage](https://magefile.org/):

  ```bash
  go install github.com/magefile/mage@latest
  ```

- [Install Docker](https://docs.docker.com/get-docker/)

- [Create a classic GitHub Personal Access Token](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token)
  (fine-grained isn't supported yet) with the following permissions
  taken from [here](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry):

  - `read:packages`
  - `write:packages`
  - `delete:packages`

---

## Configure environment

1. Install pre-commit hooks:

   ```bash
   mage installPreCommitHooks
   ```

1. Run pre-commit hooks locally:

   ```bash
   mage runPreCommit
   ```

1. Compile warpgate:

   ```bash
   go build -o wg
   ```
