#########################################################################################
# windows packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a windows AMI provisioned with a basic powershell script.
#########################################################################################
source "amazon-ebs" "windows" {
  ami_name      = "${var.blueprint_name}-${formatdate("YYYY-MM-DD-hh-mm-ss", timestamp())}"
  instance_type = "${var.instance_type}"
  region        = "${var.ami_region}"
  source_ami_filter {
    filters = {
      name                = "${var.os}-${var.os_version}"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["amazon"]
  }
  user_data_file = "${var.user_data_file}"
  communicator = "ssh"
  ssh_password = "SuperS3cr3t!!!!"
  ssh_username = var.ssh_username
  ssh_timeout = "10m"
}

build {
  name = "windows"
  sources = ["source.amazon-ebs.windows"]

  provisioner "powershell" {
    script = "scripts/choco.ps1"
  }

  provisioner "windows-restart" {
    max_retries = 3
  }

  # provisioner "powershell" {
  #   script = "scripts/imagePrep.ps1"
  # }
}
