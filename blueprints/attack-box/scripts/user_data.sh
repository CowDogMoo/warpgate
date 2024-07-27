#!/bin/bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Combined script for Odyssey attack box configuration and provisioning
set -ex

export PKR_BUILD_DIR="${1:-/tmp}"
export CLEANUP="${2:-true}"
packages='gpg-agent python3 python3-pip'

# Function to install and configure OpenSSH server
configure_ssh() {
    run_as_root mkdir -p /home/kali/.ssh
    run_as_root curl -s http://169.254.169.254/latest/meta-data/public-keys/0/openssh-key > /home/kali/.ssh/authorized_keys
    run_as_root chown kali:kali /home/kali/.ssh/authorized_keys
    run_as_root chmod 600 /home/kali/.ssh/authorized_keys
    run_as_root usermod -s /bin/bash kali
}

# Function to install and configure SSM Agent
install_and_configure_ssm() {
    if ! snap list amazon-ssm-agent &> /dev/null; then
        SSM_DIR=/tmp/ssm
        mkdir -p $SSM_DIR
        run_as_root wget https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/debian_amd64/amazon-ssm-agent.deb -O $SSM_DIR/amazon-ssm-agent.deb
        run_as_root dpkg -i $SSM_DIR/amazon-ssm-agent.deb
        run_as_root systemctl enable amazon-ssm-agent
        run_as_root systemctl start amazon-ssm-agent
        rm -rf $SSM_DIR
    else
        echo "amazon-ssm-agent is already installed via snap. Skipping installation."
    fi
}

# Function to install and configure CloudWatch Agent
install_and_configure_cloudwatch() {
    # Download and install the Amazon CloudWatch Agent
    curl -O https://s3.amazonaws.com/amazoncloudwatch-agent/debian/amd64/latest/amazon-cloudwatch-agent.deb
    sudo dpkg -i amazon-cloudwatch-agent.deb
    # Enable and start the Amazon CloudWatch Agent service
    sudo systemctl enable amazon-cloudwatch-agent
    sudo systemctl start amazon-cloudwatch-agent
    # Write the CloudWatch Agent configuration file
    sudo tee /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json > /dev/null << EOF
{
  "agent": {
    "metrics_collection_interval": 60,
    "run_as_user": "cwagent"
  },
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/syslog",
            "log_group_name": "syslog",
            "log_stream_name": "{instance_id}"
          }
        ]
      }
    }
  }
}
EOF
    # Restart the Amazon CloudWatch Agent service to apply the new configuration
    sudo systemctl restart amazon-cloudwatch-agent
}

wait_for_apt_lock() {
    while sudo fuser /var/lib/apt/lists/lock > /dev/null 2>&1; do
        echo "Waiting for apt lock to be released..."
        sleep 1
    done
}

install_sudo_if_needed() {
    # Check if we are root and sudo is not available
    if [[ $EUID -eq 0 ]] && ! command -v sudo &> /dev/null; then
        run_as_root apt-get update && run_as_root apt-get install -y sudo
    fi
}

run_as_root() {
    echo "Running command as root:" "$@"
    if command -v sudo &> /dev/null; then
        sudo "$@"
    else
        "$@"
    fi
    local status=$?
    if [ $status -ne 0 ]; then
        echo "Error: Command failed with status $status"
        exit $status
    fi
}

add_py_deps_to_path() {
    echo "Adding .local/bin to PATH if not already present..."
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        export PATH="$PATH:$HOME/.local/bin"
    fi
}

install_dependencies() {
    echo "Installing dependencies..."
    install_sudo_if_needed

    wait_for_apt_lock
    run_as_root apt-get update -y
    run_as_root apt-get install -y "${packages}"
    echo 'debconf debconf/frontend select Noninteractive' | run_as_root debconf-set-selections

    # Install Python packages globally to avoid PATH issues
    python3 -m pip install --upgrade \
        ansible-core \
        docker \
        molecule \
        molecule-docker \
        "molecule-plugins[docker]"

    add_py_deps_to_path
}

run_provision_logic() {
    echo "Running provision logic..."
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
        "${PKR_BUILD_DIR}/playbooks/attack_box/attack_box.yml"

    # Wait for ansible to finish
    while /usr/bin/pgrep ansible > /dev/null; do
        echo "Ansible playbook is running"
        sleep 1
    done
}

cleanup() {
    if [[ "${CLEANUP}" == "true" ]]; then
        echo "Cleaning up the system..."
        run_as_root apt-get clean
        run_as_root rm -rf /var/lib/apt/lists/*

        # Clean up pip cache
        if command -v pip3 &> /dev/null; then
            python3 -m pip cache purge
        fi

        # Remove unused packages and their dependencies
        run_as_root apt-get autoremove -y

        # Clean up cloud-init logs if running on EC2
        if [[ -n "${AWS_DEFAULT_REGION}" ]]; then
            run_as_root rm -rf /var/lib/cloud/instances/*
            run_as_root rm -f /var/log/cloud-init.log
            run_as_root rm -f /var/log/cloud-init-output.log
        fi

        # Remove temporary files
        run_as_root rm -rf /tmp/* /var/tmp/*

        # Remove build directory
        run_as_root rm -rf "${PKR_BUILD_DIR}"

        # Remove any leftover logs
        run_as_root rm -rf /var/log/*
    fi
}

# Main function to run all setups
main() {
    configure_ssh
    install_and_configure_ssm
    install_and_configure_cloudwatch
    #install_dependencies
    #run_provision_logic
    # cleanup
}

main "$@"
