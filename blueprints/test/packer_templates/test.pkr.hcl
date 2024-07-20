#########################################################################################
# test packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image provisioned with a basic bash script and an Ubuntu AMI 
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
  # "${var.ssh_interface != "session_manager"}"
  ssh_interface = "session_manager"
  # ssh_interface = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? "session_manager" : "public_ip"}"
  iam_instance_profile = "${var.iam_instance_profile}"
  # iam_instance_profile = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  # ssh_interface = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? "session_manager" : "public_ip"}"
  # iam_instance_profile = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  sources = [
    # "source.docker.amd64",
    # "source.docker.arm64",
    "source.amazon-ebs.ubuntu",
  ]

  provisioner "shell" {
    inline = [
      "mkdir -p ${var.pkr_build_dir}",
    ]
  }

  provisioner "file" {
    source      = "../scripts/provision.sh"
    destination = "${var.pkr_build_dir}/provision.sh"
  }

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    ]
    inline = [
      "chmod +x ${var.pkr_build_dir}/provision.sh",
      "${var.pkr_build_dir}/provision.sh"
    ]
  }
}

#   provisioner "ansible" {
#     playbook_file  = "${var.provision_repo_path}/playbooks/workstation/workstation.yml"
#     # inventory_file_template =  "{{ .HostAlias }} ansible_host={{ .ID }} ansible_user={{ .User }} ansible_ssh_common_args='-o StrictHostKeyChecking=no -o ProxyCommand=\"sh -c \\\"aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p\\\"\"'\n"
#     # inventory_file = "${var.pkr_build_dir}/playbooks/workstation/windows-inventory_aws_ec2.yml"
#     inventory_file_template =  "{{ .HostAlias }} ansible_host={{ .ID }} ansible_user={{ .User }} ansible_ssh_common_args='-o StrictHostKeyChecking=no -o ProxyCommand=\"sh -c \\\"aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p\\\"\"'\n"
#     user           = "${var.ssh_username}"
#     galaxy_file    = "${var.provision_repo_path}/requirements.yml"
#     # use_proxy      = false
#     use_proxy               =  false
#     ansible_env_vars = [
#       "PACKER_BUILD_NAME={{ build_name }}",
#       "AWS_DEFAULT_REGION=${var.ami_region}",
#       ]
#     extra_arguments = [
#       "--connection", "packer",
#       "-e", "ansible_aws_ssm_bucket_name=${var.ansible_aws_ssm_bucket_name}",
#       # "-e", "ansible_shell_executable=None",
#       "-e", "ansible_connection=amazon.aws.aws_ssm",
#       "-vvv",
#     ]
#   }
# }
