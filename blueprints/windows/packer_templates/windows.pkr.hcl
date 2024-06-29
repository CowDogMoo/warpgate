#########################################################################################
# windows packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a windows AMI provisioned with a basic powershell script.
#########################################################################################
locals { timestamp = formatdate("YYYY-MM-DD-hh-mm-ss", timestamp()) }

source "amazon-ebs" "windows" {
  ami_name      = "${var.blueprint_name}-${local.timestamp}"
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
  iam_instance_profile = "AmazonSSMInstanceProfileForInstances"
  user_data_file = "${var.user_data_file}"
  associate_public_ip_address = true
  communicator = "ssh"
  ssh_port = 22
  ssh_username = var.ssh_username
  ssh_file_transfer_method    = "sftp"
  ssh_timeout = "20m"
  ssh_interface = "session_manager"

  snapshot_tags = {
    Name      =  "${var.blueprint_name}"
    BuildTime = "${local.timestamp}"
  }

  tags = {
    Name      =  "${var.blueprint_name}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  name = "windows"
  sources = ["source.amazon-ebs.windows"]

  provisioner "powershell" {
    script = "scripts/choco.ps1"
  }

  provisioner "windows-restart" {
    restart_check_command = "powershell -command \"& { Write-Output 'Restarting...'; exit 1 }\""
    max_retries = 3
  }

  # provisioner "powershell" {
  #   script = "scripts/imagePrep.ps1"
  # }
}
