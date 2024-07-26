#########################################################################################
# attack-box packer template
#
# Author: Jayson Grace <Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create container images and AMI provisioned with the
# [attack-box](https://github.com/CowDogMoo/ansible-collection-workstation/tree/main/playbooks/attack-box)
# Ansible playbook.
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
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
  ]

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "docker" "arm64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/arm64"
  privileged = true

  changes = [
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.user}",
    "WORKDIR ${var.workdir}",
  ]

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "amazon-ebs" "kali" {
  ami_name      = "${var.blueprint_name}-${local.timestamp}"
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
    volume_type           = "gp2"
    delete_on_termination = true
  }

  ami_block_device_mappings {
    device_name           = "${var.disk_device_name}"
    volume_size           = "${var.disk_size}"
    volume_type           = "gp2"
    delete_on_termination = true
  }

  communicator   = "${var.communicator}"
  run_tags       = "${var.run_tags}"
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
    # "source.docker.arm64",
    # "source.docker.amd64",
    "source.amazon-ebs.kali",
  ]

  provisioner "ansible" {
    playbook_file           = "${var.provision_repo_path}/playbooks/attack-box/attack-box.yml"
    inventory_file          = "${var.provision_repo_path}/playbooks/attack-box/attack-box_aws_ec2.yml"
    user                    = var.ssh_username
    galaxy_file             = "${var.provision_repo_path}/requirements.yml"
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
      "-e", "ansible_aws_ssm_s3_addressing_style=virtual",
      "-vvvv",
    ]
  }
}