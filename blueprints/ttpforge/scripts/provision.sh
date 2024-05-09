#!/usr/bin/env bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for Odyssey creation.
set -ex

export PKR_BUILD_DIR="${1:-ansible-collection-arsenal}"
export CLEANUP="${2:-true}"

install_dependencies() {
    su root -c "bash -c '
        install_packages() {
            if [[ \$EUID -ne 0 ]]; then
                echo \"This script must be run as root.\"
                exit 1
            fi
            apt-get update -y 2> /dev/null
            apt-get install -y bash git gpg-agent python3 python3-pip
            echo debconf debconf/frontend select Noninteractive | debconf-set-selections
        }
        install_packages
    '"

    python3 -m pip install --upgrade \
        ansible-core \
        docker \
        molecule \
        molecule-docker \
        "molecule-plugins[docker]"
}

# Provision logic run by packer
run_provision_logic() {
    if [[ -f "${PKR_BUILD_DIR}/requirements.yml" ]]; then
        ansible-galaxy install -r "${PKR_BUILD_DIR}/requirements.yml"
        ansible-galaxy collection install git+https://github.com/l50/ansible-collection-arsenal.git,main --force
    else
        echo "${PKR_BUILD_DIR}/requirements.yml not found."
    fi

    export ANSIBLE_CONFIG=${HOME}/.ansible.cfg
    if [[ -f "${ANSIBLE_CONFIG}" ]]; then
        cp "${PKR_BUILD_DIR}/ansible.cfg" "${ANSIBLE_CONFIG}"
    fi

    ansible-playbook \
        --connection=local \
        --inventory 127.0.0.1, \
        --limit 127.0.0.1 \
        "${PKR_BUILD_DIR}/playbooks/ttpforge/ttpforge.yml"

    # Wait for ansible to finish
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

install_dependencies
run_provision_logic
cleanup
