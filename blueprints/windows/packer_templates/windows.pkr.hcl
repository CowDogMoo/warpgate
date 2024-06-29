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

  user_data_file = "${var.user_data_file}"
  communicator   = "${var.communicator}"
  run_tags       = "${var.run_tags}"

  # SSH Configuration
  ssh_port                 = "${var.communicator == "ssh" ? var.ssh_port : null}"
  ssh_username             = "${var.communicator == "ssh" ? var.ssh_username : null}"
  ssh_file_transfer_method = "${var.communicator == "ssh" ? "sftp" : null}"
  ssh_timeout              = "${var.communicator == "ssh" ? var.ssh_timeout : null}"

  # WinRM Configuration
  winrm_username = "${var.communicator == "winrm" ? var.winrm_username : null}"
  winrm_password = "${var.communicator == "winrm" ? var.winrm_password : null}"
  winrm_port     = "${var.communicator == "winrm" ? var.winrm_port : null}"
  winrm_timeout  = "${var.communicator == "winrm" ? var.winrm_timeout : null}"

  # SSM and IP Configuration
  associate_public_ip_address = "${var.ssh_interface != "session_manager"}"
  ssh_interface = "session_manager"
  #ssh_interface               = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.ssh_interface : "public_ip"}"
  iam_instance_profile        = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  name    = "windows"
  sources = ["source.amazon-ebs.windows"]

  provisioner "powershell" {
    environment_vars = [
      "SSH_INTERFACE=${var.ssh_interface}"
    ]
    # TODO: script = "${pkr_build_dir}/provision.ps1"
    script = "scripts/provision.ps1"
  }

  # Restart the instance to ensure the AMI is in a clean state
  provisioner "windows-restart" {
    restart_check_command = "${var.restart_check_command}"
    max_retries           = "${var.max_retries}"
  }
}
