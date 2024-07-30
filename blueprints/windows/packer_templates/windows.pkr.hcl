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
      name                = "${var.os}-${var.os_version}*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["amazon"]
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

  #### WinRM Configuration ####
  winrm_username = "${var.communicator == "winrm" ? var.winrm_username : null}"
  winrm_password = "${var.communicator == "winrm" ? var.winrm_password : null}"
  winrm_port     = "${var.communicator == "winrm" ? var.winrm_port : null}"
  winrm_timeout  = "${var.communicator == "winrm" ? var.winrm_timeout : null}"

  #### SSM and IP Configuration ####
  associate_public_ip_address = true
  iam_instance_profile        = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  sources = ["source.amazon-ebs.windows"]

  provisioner "ansible" {
    playbook_file  = "${var.provision_repo_path}/playbooks/vulnerable_windows_scenarios/windows_scenarios.yml"
    inventory_file = "${var.provision_repo_path}/playbooks/vulnerable_windows_scenarios/windows_inventory_aws_ec2.yml"
    galaxy_file    = "${var.provision_repo_path}/requirements.yml"
    use_proxy      = false
    ansible_env_vars = [
      "AWS_DEFAULT_REGION=${var.ami_region}",
      "PACKER_BUILD_NAME={{ build_name }}",
      "SSH_INTERFACE=${var.ssh_interface}",
      "no_proxy='*'",
    ]
    extra_arguments = [
      "--connection", "packer",
      "-e", "ansible_aws_ssm_bucket_name=${var.ansible_aws_ssm_bucket_name}",
      "-e", "ansible_connection=aws_ssm",
      "-e", "ansible_aws_ssm_region=${var.ami_region}",
      "-e", "ansible_shell_type=${var.shell}",
      "-e", "ansible_shell_executable=None",
      "-e", "ansible_aws_ssm_timeout=${var.ansible_aws_ssm_timeout}",
      "-e", "ansible_aws_ssm_s3_addressing_style=virtual",
      "-vvvv",
    ]
  }
}
