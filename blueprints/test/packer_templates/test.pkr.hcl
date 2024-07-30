#########################################################################################
# test packer template with Ansible
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image provisioned with Ansible and an Ubuntu AMI
# provisioned with Ansible and SSM support.
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
    "USER ${var.user}",
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
    "USER ${var.user}",
    "WORKDIR ${var.workdir}",
  ]

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "amazon-ebs" "ubuntu" {
  ami_name      = "${var.blueprint_name}-${local.timestamp}"
  instance_type = "${var.instance_type}"
  region        = "${var.ami_region}"

  source_ami_filter {
    filters = {
      name                 = "${var.os}/images/*${var.os}-${var.os_version}-${var.ami_arch}-server-*"
      root-device-type     = "ebs"
      virtualization-type  = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"] # Canonical's owner ID for Ubuntu images
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
    "source.amazon-ebs.ubuntu",
    "source.docker.amd64",
    "source.docker.arm64",
  ]
}
