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
  ssh_username            = var.ssh_username
  ssh_password            = "S3cr3tP@ssw0rd"
  communicator            = "ssh"
  ssh_timeout             = var.ssh_timeout

  user_data_file = "${var.user_data_file}"
  associate_public_ip_address = true
}

build {
  sources = [
    "source.amazon-ebs.windows",
  ]

  provisioner "powershell" {
    inline = [
      "mkdir ${var.pkr_build_dir}",
    ]
  }

  provisioner "powershell" {
    script = "${var.provision_repo_path}/provision.ps1"
  }
}
