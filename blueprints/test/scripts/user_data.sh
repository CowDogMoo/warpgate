#!/bin/bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Combined script for Odyssey attack box configuration and provisioning
set -ex

export PKR_BUILD_DIR="${1:-/tmp}"
export CLEANUP="${2:-true}"

# Function to install and configure CloudWatch Agent
install_and_configure_cloudwatch() {
    # Download and install the Amazon CloudWatch Agent
    curl -O https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/amd64/latest/amazon-cloudwatch-agent.deb
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

cleanup() {
    if [[ "${CLEANUP}" == "true" ]]; then
        echo "Cleaning up the system..."
        run_as_root apt-get clean
        run_as_root rm -rf /var/lib/apt/lists/*

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
    install_and_configure_cloudwatch
    cleanup
}

main "$@"
