#!/usr/bin/env bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for container image creation.
set -ex

install_dependencies() {
    # Get latest packages and install aptitude
    apt-get update -y 2> /dev/null | grep packages | cut -d '.' -f 1

    # Install ansible and associated pre-requisites
    apt-get install -y bash gpg-agent python3 python3-pip
    python3 -m pip install --upgrade pip wheel setuptools ansible
}

# Provision logic run by packer
run_provision_logic() {
    mkdir -p "${HOME}/.ansible/collections/ansible_collections/cowdogmoo/workstation"

    # Link the current directory to the expected collection path
    ln -s "${PKR_BUILD_DIR}" "${HOME}/.ansible/collections/ansible_collections/cowdogmoo/workstation"

    pushd "${PKR_BUILD_DIR}"

    # Install galaxy dependencies if they are present
    if [[ -f /provision/requirements.yml ]]; then
        ansible-galaxy install -r /provision/requirements.yml
    fi

    ansible-playbook \
        --connection=local \
        --inventory 127.0.0.1, \
        -e "setup_systemd=${SETUP_SYSTEMD}", \
        --limit 127.0.0.1 "${HOME}/.ansible/collections/ansible_collections/cowdogmoo/workstation/playbooks/attack-box.yml"
    popd

    # Wait for ansible to finish running
    while /usr/bin/pgrep ansible > /dev/null; do
        echo "Ansible playbook is running"
        sleep 1
    done
}

cleanup() {
    # Remove Ansible collections directory
    rm -rf "${HOME}/.ansible/collections"

    # Remove build directory
    rm -rf "${PKR_BUILD_DIR}"
}

install_dependencies
run_provision_logic
cleanup
