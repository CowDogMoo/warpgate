#!/usr/bin/env bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for container image creation.
set -ex

export PKR_BUILD_DIR="${1:-/ansible-collection-workstation}"
export CLEANUP="${2:-true}"

install_dependencies() {
    # Get latest packages and install aptitude
    apt-get update -y 2> /dev/null | grep packages | cut -d '.' -f 1

    # Install ansible and associated pre-requisites
    apt-get install -y bash git gpg-agent python3 python3-pip
    python3 -m pip install --upgrade pip wheel setuptools ansible
}

# Provision logic run by packer
run_provision_logic() {
    cowdogmoo_collections_path="${HOME}/.ansible/collections/ansible_collections/cowdogmoo"
    mkdir -p "$cowdogmoo_collections_path"

    # Link PKR_BUILD_DIR to the expected collection path
    ln -s "${PKR_BUILD_DIR}" "$cowdogmoo_collections_path/workstation"

    # Install galaxy dependencies if they are present
    if [[ -f "${PKR_BUILD_DIR}/requirements.yml" ]]; then
        ansible-galaxy install -r "${PKR_BUILD_DIR}/requirements.yml"
    fi

    ANSIBLE_CONFIG=${HOME}/.ansible.cfg
    if [[ -f "${ANSIBLE_CONFIG}" ]]; then
        cp "${PKR_BUILD_DIR}/ansible.cfg" "${ANSIBLE_CONFIG}"
    fi

    ansible-playbook \
        --connection=local \
        --inventory 127.0.0.1, \
        --limit 127.0.0.1 "${PKR_BUILD_DIR}/playbooks/runzero-explorer/runzero-explorer.yml"

    # Wait for ansible to finish running
    while /usr/bin/pgrep ansible > /dev/null; do
        echo "Ansible playbook is running"
        sleep 1
    done
}

cleanup() {
    # Remove build directory
    if [[ "${CLEANUP}" == "true" ]]; then
        rm -rf "${PKR_BUILD_DIR}"
    fi
}

# Ensure RUNZERO_DOWNLOAD_TOKEN is set
if [[ -z "${RUNZERO_DOWNLOAD_TOKEN}" ]]; then
    echo "RUNZERO_DOWNLOAD_TOKEN is not set. Exiting."
    exit 1
fi

install_dependencies
run_provision_logic
cleanup
