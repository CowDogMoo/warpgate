#########################################################################################
# attack-box packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create container images and AMI provisioned with the
# [attack-box](https://github.com/CowDogMoo/ansible-collection-workstation/tree/main/playbooks/attack-box)
# Ansible playbook.
#########################################################################################
# Amazon EBS source configuration for Kali
source "amazon-ebs" "kali" {
  ami_name      = "${var.template_name}-${local.timestamp}"
  instance_type = "${var.instance_type}"
  region        = "${var.ami_region}"

  source_ami_filter {
    filters = {
      name                = "${var.os}-${var.os_version}-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["679593333241"] # Offensive Security's owner ID for Kali images
  }

  launch_block_device_mappings {
    device_name           = "${var.disk_device_name}"
    volume_size           = "${var.disk_size}"
    volume_type           = "gp3"
    delete_on_termination = true
  }

  ami_block_device_mappings {
    device_name           = "${var.disk_device_name}"
    volume_size           = "${var.disk_size}"
    volume_type           = "gp3"
    delete_on_termination = true
  }

  communicator   = "${var.communicator}"
  run_tags       = "${var.run_tags}"
  user_data_file = "${var.user_data_file}"

  ssh_file_transfer_method = "${var.communicator == "ssh" ? "sftp" : null}"
  ssh_interface            = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? "session_manager" : "public_ip"}"
  ssh_timeout              = "${var.communicator == "ssh" ? var.ssh_timeout : null}"
  ssh_username             = "${var.ssh_username}"

  associate_public_ip_address = true
  iam_instance_profile        = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.template_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  name = "attack-box-ami"
  sources = [
    "source.amazon-ebs.kali"
  ]

  # Pre-provisioner for ansible
  provisioner "shell" {
    only = ["amazon-ebs.kali"]
    inline = [
      "echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections",
      "apt-get update",
      "apt-get install -y python3 python3-pip sudo",
      "wget https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/debian_amd64/amazon-ssm-agent.deb",
      "dpkg -i amazon-ssm-agent.deb",
      "systemctl enable amazon-ssm-agent",
      "systemctl start amazon-ssm-agent",
    ]
  }

  provisioner "ansible" {
    only           = ["amazon-ebs.kali"]
    playbook_file  = "${var.provision_repo_path}/playbooks/attack_box/attack_box.yml"
    inventory_file = "${var.provision_repo_path}/playbooks/attack_box/attack_box_inventory_aws_ec2.yml"
    galaxy_file    = "${var.provision_repo_path}/requirements.yml"

    extra_arguments = [
      "--connection", "packer",
      "-e", "ansible_aws_ssm_bucket_name=${var.ansible_aws_ssm_bucket_name}",
      "-e", "ansible_connection=aws_ssm",
      "-e", "ansible_aws_ssm_region=${var.ami_region}",
      "-e", "ansible_shell_executable=${var.shell}",
      "-e", "ansible_aws_ssm_timeout=${var.ansible_aws_ssm_timeout}",
      "-e", "ansible_aws_ssm_s3_addressing_style=virtual",
      "-vvvv"
    ]
  }
}
