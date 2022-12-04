# Go Project Template

[![Go Report Card](https://goreportcard.com/badge/github.com/l50/goproject)](https://goreportcard.com/report/github.com/l50/goproject)
[![License](https://img.shields.io/github/license/l50/goproject?label=License&style=flat&color=blue&logo=github)](https://github.com/l50/goproject/blob/master/LICENSE)
[![Tests](https://github.com/l50/goproject/actions/workflows/tests.yaml/badge.svg)](https://github.com/l50/goproject/actions/workflows/tests.yaml)
[![🚨 CodeQL Analysis](https://github.com/l50/goproject/actions/workflows/codeql-analysis.yaml/badge.svg)](https://github.com/l50/goproject/actions/workflows/codeql-analysis.yaml)
[![🚨 Semgrep Analysis](https://github.com/l50/goproject/actions/workflows/semgrep.yaml/badge.svg)](https://github.com/l50/goproject/actions/workflows/semgrep.yaml)
[![Pre-commit](https://github.com/l50/goproject/actions/workflows/pre-commit.yaml/badge.svg)](https://github.com/l50/goproject/actions/workflows/pre-commit.yaml)
[![Coverage Status](https://coveralls.io/repos/github/l50/goproject/badge.svg?branch=main)](https://coveralls.io/github/l50/goproject?branch=main)

This repo provides a base template for a new go project.

It is highly opinionated and may not work for your usecase.
I write a lot of cobra apps and employ magefiles in place of makefiles,
so this template will be very focused around supporting projects of
that nature.

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

- [Install gvm](https://github.com/moovweb/gvm):

  ```bash
  bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
  source "${GVM_BIN}"
  ```

- [Install golang](https://go.dev/):

  ```bash
  gvm install go1.18
  ```

- [Install pre-commit](https://pre-commit.com/):

  ```bash
  brew install pre-commit
  ```

- [Install Mage](https://magefile.org/):

  ```bash
  go install github.com/magefile/mage@latest
  ```

---

## Developer Environment Setup

1. [Fork this project](https://docs.github.com/en/get-started/quickstart/fork-a-repo)

2. (Optional) If you installed gvm, create golang pkgset specifically for this project:

   ```bash
   VERSION='1.18'
   PROJECT=goproject

   gvm install "go${VERSION}"
   gvm use "go${VERSION}"
   gvm pkgset create "${PROJECT}"
   gvm pkgset use "${PROJECT}"
   ```

3. Generate the `magefile` binary:

   ```bash
   mage -d .mage/ -compile ../magefile
   ```

4. Install pre-commit hooks and dependencies:

   ```bash
   ./magefile installPreCommitHooks
   ```

5. Update and run pre-commit hooks locally:

   ```bash
   ./magefile runPreCommit
   ```

---

## Usage

To get started, you will need to:

1. Create a new repo with this template
2. Replace all instances of PROJECT_NAME,
   BIN_NAME, l50, and goproject found throughout the codebase
3. Customize as needed
