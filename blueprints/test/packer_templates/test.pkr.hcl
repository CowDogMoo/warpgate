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
    owners      = ["099720109477"] // Canonical's owner ID for Ubuntu images
  }

  communicator   = "${var.communicator}"
  run_tags       = "${var.run_tags}"
  user_data_file = "${var.user_data_file}"

  #### SSH Configuration ####
  ssh_username   = "${var.ssh_username}"
  ssh_file_transfer_method = "${var.communicator == "ssh" ? "sftp" : null}"
  ssh_timeout              = "${var.communicator == "ssh" ? var.ssh_timeout : null}"

  #### SSM and IP Configuration ####
  associate_public_ip_address = true
  ssh_interface = "session_manager"
  iam_instance_profile = "${var.iam_instance_profile}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  sources = ["source.amazon-ebs.ubuntu"]

  provisioner "file" {
    source      = "ansible.cfg"
    destination = "/tmp/ansible.cfg"
  }

  provisioner "ansible" {
    playbook_file  = "${var.provision_repo_path}/playbooks/workstation/workstation.yml"
    inventory_file_template =  "{{ .HostAlias }} ansible_host={{ .ID }} ansible_user={{ .User }} ansible_ssh_common_args='-o StrictHostKeyChecking=no -o ProxyCommand=\"sh -c \\\"aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p\\\"\"'\n"
    user           = "${var.ssh_username}"
    galaxy_file    = "${var.provision_repo_path}/requirements.yml"
    ansible_env_vars = [
      "ANSIBLE_CONFIG=/tmp/ansible.cfg",
      "AWS_DEFAULT_REGION=${var.ami_region}",
      "PACKER_BUILD_NAME={{ build_name }}",
    ]
    extra_arguments = [
      "--connection", "packer",
      "-e", "ansible_aws_ssm_bucket_name=${var.ansible_aws_ssm_bucket_name}",
      "-e", "ansible_connection=aws_ssm",
      "-vvv",
    ]
  }
}
