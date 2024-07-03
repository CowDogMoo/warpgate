#!/bin/bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# User data script for the attack box Odyssey.
set -ex

# Function to install and configure OpenSSH server
configure_ssh() {
    mkdir -p /home/kali/.ssh
    curl http://169.254.169.254/latest/meta-data/public-keys/0/openssh-key > /home/kali/.ssh/authorized_keys
    chown kali:kali /home/kali/.ssh/authorized_keys
    chmod 600 /home/kali/.ssh/authorized_keys
    usermod -s /bin/zsh kali
}

# Function to install and configure SSM Agent
install_and_configure_ssm() {
    SSM_DIR=/tmp/ssm
    mkdir -p $SSM_DIR
    wget https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/debian_amd64/amazon-ssm-agent.deb -O $SSM_DIR/amazon-ssm-agent.deb
    dpkg -i $SSM_DIR/amazon-ssm-agent.deb
    systemctl enable amazon-ssm-agent
    systemctl start amazon-ssm-agent
    rm -rf $SSM_DIR
}

# Function to install and configure CloudWatch Agent
install_and_configure_cloudwatch() {
    wget https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/amd64/latest/amazon-cloudwatch-agent.deb
    dpkg -i amazon-cloudwatch-agent.deb
    systemctl enable amazon-cloudwatch-agent
    systemctl start amazon-cloudwatch-agent

    cat << EOF > /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json
{
  "agent": {
    "metrics_collection_interval": 60,
    "logfile": "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"
  },
  "metrics": {
    "append_dimensions": {
      "InstanceId": "\${aws:InstanceId}",
      "InstanceType": "\${aws:InstanceType}"
    },
    "metrics_collected": {
      "disk": {
        "measurement": [
          "used_percent"
        ],
        "metrics_collection_interval": 60,
        "resources": [
          "/"
        ]
      },
      "mem": {
        "measurement": [
          "mem_used_percent"
        ],
        "metrics_collection_interval": 60
      }
    }
  }
}
EOF

    /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -s
}

# Main function to run all setups
main() {
    configure_ssh
    install_and_configure_ssm
    install_and_configure_cloudwatch
}

main "$@"
