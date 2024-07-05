#########################################################################################
# windows packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a Windows AMI provisioned with a basic PowerShell script.
#########################################################################################
locals {
  timestamp = formatdate("YYYY-MM-DD-hh-mm-ss", timestamp())
}

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

  #### SSH Configuration ####
  ssh_port                 = "${var.communicator == "ssh" ? var.ssh_port : null}"
  ssh_username             = "${var.communicator == "ssh" ? var.ssh_username : null}"
  ssh_file_transfer_method = "${var.communicator == "ssh" ? "sftp" : null}"
  ssh_timeout              = "${var.communicator == "ssh" ? var.ssh_timeout : null}"

  #### WinRM Configuration ####
  winrm_username = "${var.communicator == "winrm" ? var.winrm_username : null}"
  winrm_password = "${var.communicator == "winrm" ? var.winrm_password : null}"
  winrm_port     = "${var.communicator == "winrm" ? var.winrm_port : null}"
  winrm_timeout  = "${var.communicator == "winrm" ? var.winrm_timeout : null}"

  #### SSM and IP Configuration ####
  associate_public_ip_address = "${var.ssh_interface == "session_manager"}"
  ssh_interface               = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? "session_manager" : "public_ip"}"
  iam_instance_profile        = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  name    = "windows"
  sources = ["source.amazon-ebs.windows"]

  # Upload the Ansible playbooks and other required files
  provisioner "file" {
    source      = "${var.provision_repo_path}"
    destination = "${var.pkr_build_dir}"
  }

  provisioner "ansible" {
    only = ["amazon-ebs.windows"]
    playbook_file = "${var.provision_repo_path}/playbooks/windows-scenarios/windows-scenarios.yml"
    user = "Administrator"
    extra_arguments = [
      "--extra-vars",
      "ansible_shell_type=powershell",
      "--extra-vars",
      "ansible_shell_executable=None",
      "--extra-vars",
      "ansible_user=Administrator",
    ]
  }

  provisioner "powershell" {
    inline = [
      "C:\\ProgramData\\Amazon\\EC2-Windows\\Launch\\Scripts\\InitializeInstance.ps1 -Schedule",
      "C:\\ProgramData\\Amazon\\EC2-Windows\\Launch\\Scripts\\SysprepInstance.ps1 -NoShutdown"
    ]
  }
}
