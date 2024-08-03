#########################################################################################
# TTPForge Packer Template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create TTPForge Odyssey provisioned with
# https://github.com/l50/ansible-collection-arsenal/tree/main/playbooks/ttpforge
#########################################################################################
locals {
  timestamp = formatdate("YYYY-MM-DD-hh-mm-ss", timestamp())
}

source "docker" "amd64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/amd64"
  privileged = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "WORKDIR ${var.workdir}",
  ]

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "docker" "arm64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/arm64"
  privileged = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "WORKDIR ${var.workdir}",
  ]

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "amazon-ebs" "ubuntu" {
  ami_name      = "${var.blueprint_name}-${local.timestamp}"
  instance_type = "${var.instance_type}"
  region        = "${var.ami_region}"

  source_ami_filter {
    filters = {
      name                = "${var.os}/images/*${var.os}-${var.os_version}-${var.ami_arch}-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"] // Canonical's owner ID for Ubuntu images
  }

  ami_block_device_mappings {
    device_name           = "${var.disk_device_name}"
    volume_size           = "${var.disk_size}"
    volume_type           = "gp2"
    delete_on_termination = true
  }

  launch_block_device_mappings {
    device_name           = "${var.disk_device_name}"
    volume_size           = "${var.disk_size}"
    volume_type           = "gp2"
    delete_on_termination = true
  }

  communicator = "${var.communicator}"
  run_tags     = "${var.run_tags}"
  user_data_file = "${var.user_data_file}"

  #### SSH Configuration ####
  ssh_file_transfer_method = "${var.communicator == "ssh" ? "sftp" : null}"
  ssh_interface            = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? "session_manager" : "public_ip"}"
  ssh_timeout              = "${var.communicator == "ssh" ? var.ssh_timeout : null}"
  ssh_username             = "${var.ssh_username}"

  #### SSM and IP Configuration ####
  associate_public_ip_address = true
  iam_instance_profile        = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  sources = [
    "source.docker.amd64",
    "source.docker.arm64",
    "source.amazon-ebs.ubuntu",
  ]

  # Packer build for container images
  provisioner "shell" {
    only = ["docker.arm64", "docker.amd64"]
    inline = [
      "apt-get update -y 2> /dev/null",
      "apt-get install -y bash git gpg-agent python3 python3-pip sudo",
      "echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections",
    ]
  }

  provisioner "ansible" {
    only          = ["docker.arm64", "docker.amd64"]
    playbook_file = "${var.provision_repo_path}/playbooks/ttpforge/ttpforge.yml"
    galaxy_file   = "${var.provision_repo_path}/requirements.yml"
    ansible_env_vars = [
      "PACKER_BUILD_NAME={{ build_name }}",
    ]
    extra_arguments = [
      "-vvvv",
    ]
  }

  # Packer build for Amazon AMI
  provisioner "shell-local" {
    only = ["amazon-ebs.ubuntu"]
    inline = [
      "cat > ${var.provision_repo_path}/playbooks/ttpforge/ttpforge_inventory_aws_ec2.yml <<EOF",
      "---",
      "all:",
      "  hosts:",
      "    localhost:",
      "      ansible_connection: local",
      "      ansible_python_interpreter: /usr/bin/python3",
      "EOF",
    ]
  }

  provisioner "ansible" {
    only           = ["amazon-ebs.ubuntu"]
    playbook_file  = "${var.provision_repo_path}/playbooks/ttpforge/ttpforge.yml"
    inventory_file = "${var.provision_repo_path}/playbooks/ttpforge/ttpforge_inventory_aws_ec2.yml"
    galaxy_file    = "${var.provision_repo_path}/requirements.yml"
    ansible_env_vars = [
      "AWS_DEFAULT_REGION=${var.ami_region}",
      "PACKER_BUILD_NAME={{ build_name }}",
    ]
    extra_arguments = [
      "--connection", "packer",
      "-e", "ansible_aws_ssm_bucket_name=${var.ansible_aws_ssm_bucket_name}",
      "-e", "ansible_connection=aws_ssm",
      "-e", "ansible_aws_ssm_region=${var.ami_region}",
      "-e", "ansible_shell_executable=${var.shell}",
      "-e", "ansible_aws_ssm_timeout=${var.ansible_aws_ssm_timeout}",
      "-e", "ansible_aws_ssm_s3_addressing_style=virtual",
      "-vvvv",
    ]
  }
}
