#!/usr/bin/env bash
# Author: Jayson Grace <Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for docker image creation.
set -ex

install_dependencies()
                       {
    # Get latest packages and install aptitude
    apt update -y 2> /dev/null | grep packages | cut -d '.' -f 1
    apt install -y aptitude 2> /dev/null | grep packages | cut -d '.' -f 1

    # Install ansible and associated pre-requisites
    aptitude install -y bash gpg-agent python3 python3-pip
    python3 -m pip install --upgrade pip wheel setuptools ansible
}

# Provision logic run by packer
run_provision_logic()
                      {
    mkdir -p "${HOME}/.ansible/roles"
    ln -s "${PKR_BUILD_DIR}" "${HOME}/.ansible/roles/cowdogmoo.vnc_zsh"

    pushd "${PKR_BUILD_DIR}"

    ansible-playbook \
        --connection=local \
        --inventory 127.0.0.1, \
        -e "setup_systemd=${SETUP_SYSTEMD}", \
        --limit 127.0.0.1 examples/playbook.yaml
    popd

    # Wait for ansible to finish running
    while /usr/bin/pgrep ansible > /dev/null; do
        echo "Ansible playbook is running"
        sleep 1
    done
}

install_dependencies
run_provision_logic
