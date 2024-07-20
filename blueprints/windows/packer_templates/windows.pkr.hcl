#########################################################################################
# windows packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a windows AMI provisioned with a basic powershell script.
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
  # ssh_port                 = "${var.communicator == "ssh" ? var.ssh_port : null}"
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
  ssh_interface = "${var.ssh_interface}"
  iam_instance_profile = "${var.iam_instance_profile}"
  # ssh_interface = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? "session_manager" : "public_ip"}"
  # iam_instance_profile = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  name    = var.blueprint_name
  sources = ["source.amazon-ebs.windows"]

  provisioner "ansible" {
    playbook_file  = "${var.provision_repo_path}/playbooks/vulnerable-windows-scenarios/windows-scenarios.yml"
    inventory_file_template =  "{{ .HostAlias }} ansible_host={{ .ID }} ansible_user={{ .User }} ansible_ssh_common_args='-o StrictHostKeyChecking=no -o ProxyCommand=\"sh -c \\\"aws ssm start-session --target %h --document-name AWS-StartSSHSession -parameters portNumber=%p\\\"\"'\n"
    user           = "${var.user}"
    galaxy_file    = "${var.provision_repo_path}/requirements.yml"
    extra_arguments = [
      "-e", "ansible_aws_ssm_bucket_name=${var.ansible_aws_ssm_bucket_name}",
      "-e", "ansible_shell_type=powershell",
      "-e", "ansible_shell_executable=None",
      "-vvv",
    ]
  }
}

