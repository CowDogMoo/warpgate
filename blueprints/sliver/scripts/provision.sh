#!/usr/bin/env bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for container image creation.
set -ex

export PKR_BUILD_DIR="${1:-ansible-collection-arsenal}"
export CLEANUP="${2:-true}"

install_sudo_if_needed() {
    # Check if we are root and sudo is not available
    if [[ $EUID -eq 0 ]] && ! command -v sudo &> /dev/null; then
        apt-get update && apt-get install -y sudo
    fi
}

run_as_root() {
    if command -v sudo &> /dev/null; then
        sudo "$@"
    else
        "$@"
    fi
}

add_py_deps_to_path() {
    # Add .local/bin to PATH if it's not already there
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        export PATH="$PATH:$HOME/.local/bin"
    fi
}

install_dependencies() {
    install_sudo_if_needed

    run_as_root apt-get update -y 2> /dev/null
    run_as_root apt-get install -y bash git gpg-agent python3 python3-pip
    echo 'debconf debconf/frontend select Noninteractive' | run_as_root debconf-set-selections

    # Install Python packages globally to avoid PATH issues
    python3 -m pip install --upgrade pip
    python3 -m pip install --upgrade \
        ansible-core \
        docker \
        molecule \
        molecule-docker \
        "molecule-plugins[docker]"

    add_py_deps_to_path
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
        "${PKR_BUILD_DIR}/playbooks/sliver/sliver.yml"

    # Wait for ansible to finish running
    while /usr/bin/pgrep ansible > /dev/null; do
        echo "Ansible playbook is running"
        sleep 1
    done
}

cleanup() {
    if [[ "${CLEANUP}" == "true" ]]; then
        # Remove build directory
        rm -rf "${PKR_BUILD_DIR}"

        # Remove unnecessary packages and files
        run_as_root apt-get autoremove -y
        run_as_root apt-get clean
        run_as_root rm -rf /var/lib/apt/lists/*
        run_as_root rm -rf /tmp/*
        run_as_root rm -rf /var/tmp/*

        # Remove pip cache
        python3 -m pip cache purge

        # Remove Python bytecode files
        run_as_root find / -type f -name "*.py[co]" -delete
        run_as_root find / -type d -name "__pycache__" -exec rm -rf {} +

        # Remove logs and other unnecessary files
        run_as_root find /var/log -type f -exec truncate -s 0 {} \;

        # Clean up cloud-init logs if running on EC2
        if [[ -n "${AWS_DEFAULT_REGION}" ]]; then
            run_as_root rm -rf /var/lib/cloud/instances/*
            run_as_root rm -f /var/log/cloud-init.log
            run_as_root rm -f /var/log/cloud-init-output.log
        fi

        # Remove bash history
        unset HISTFILE
        rm -f ~/.bash_history
        history -c
    fi
}

install_dependencies
run_provision_logic
cleanup
