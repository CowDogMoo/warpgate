#!/usr/bin/env bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for Odyssey creation.
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
        "${PKR_BUILD_DIR}/playbooks/atomic-red-team/atomic-red-team.yml"

    # Wait for ansible to finish
    while /usr/bin/pgrep ansible > /dev/null; do
        echo "Ansible playbook is running"
        sleep 1
    done
}

cleanup() {
    if [[ "${CLEANUP}" == "true" ]]; then
        # Clean up apt cache
        run_as_root apt-get clean
        run_as_root rm -rf /var/lib/apt/lists/*

        # Clean up pip cache
        python3 -m pip cache purge

        # Remove unused packages and their dependencies
        run_as_root apt-get autoremove -y

        # Check if packages exist before purging them
        for package in git gpg-agent libgmp-dev manpages man-db bsdmainutils; do
            if dpkg -s $package &> /dev/null; then
                run_as_root apt-get purge -y $package
            else
                echo "Package $package is not installed, skipping."
            fi
        done

        # Clean up cloud-init logs if running on EC2
        if [[ -n "${AWS_DEFAULT_REGION}" ]]; then
            run_as_root rm -rf /var/lib/cloud/instances/*
            run_as_root rm -f /var/log/cloud-init.log
            run_as_root rm -f /var/log/cloud-init-output.log
        fi

        # Remove temporary files
        run_as_root rm -rf /tmp/* /var/tmp/*

        # Remove any leftover logs
        run_as_root rm -rf /var/log/*
    fi
}

install_dependencies
run_provision_logic
cleanup
