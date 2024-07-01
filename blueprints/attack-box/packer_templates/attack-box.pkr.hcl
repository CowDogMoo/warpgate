#########################################################################################
# attack-box packer template
#
# Author: Jayson Grace <Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create container images and AMI provisioned with the
# [attack-box](https://github.com/CowDogMoo/ansible-collection-workstation/tree/main/playbooks/attack-box)
# Ansible playbook.
#########################################################################################
locals { timestamp = formatdate("YYYY-MM-DD-hh-mm-ss", timestamp()) }
source "docker" "amd64" {
  commit      = true
  image   = "${var.base_image}:${var.base_image_version}"
  platform    = "linux/amd64"
  privileged = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.container_user}",
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
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.user}",
    "WORKDIR ${var.workdir}",
  ]

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "amazon-ebs" "kali" {
  ami_name      = "${var.blueprint_name}-${formatdate("YYYY-MM-DD-hh-mm-ss", timestamp())}"
  instance_type = "${var.instance_type}"
  region        = "${var.ami_region}"

  source_ami_filter {
    filters = {
      name                = "${var.os}-${var.os_version}-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["679593333241"]
  }

  communicator   = "ssh"
  run_tags       = "${var.run_tags}"

   launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = "${var.disk_size}"
    volume_type           = "gp2"
    delete_on_termination = true
  }

  #### SSH Configuration ####
  ssh_port                 = "${var.ssh_port}"
  ssh_username             = "${var.ssh_username}"
  ssh_file_transfer_method = "sftp"
  ssh_timeout              = "${var.ssh_timeout}"

  #### SSM and IP Configuration ####
  associate_public_ip_address = "${var.ssh_interface == "session_manager"}"
  ssh_interface = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? "session_manager" : "public_ip"}"
  iam_instance_profile = "${var.ssh_interface == "session_manager" && var.iam_instance_profile != "" ? var.iam_instance_profile : ""}"

  tags = {
    Name      = "${var.blueprint_name}-${local.timestamp}"
    BuildTime = "${local.timestamp}"
  }
}

build {
  sources = [
    # "source.docker.arm64",
    # "source.docker.amd64",
    "source.amazon-ebs.kali",
    ]

  // Transfer the code found at the input provision_repo_path
  // to the pkr_build_dir, which is used by packer during the build process.
  provisioner "file" {
    source = "${var.provision_repo_path}"
    destination = "${var.pkr_build_dir}"
  }

  // The provisioner "file" is used to transfer the provisioning script
  // to the pkr_build_dir. The provisioner "shell" is used to execute the
  // provisioning script within the build environment.
  provisioner "file" {
    source      = "../scripts/provision.sh"
    destination = "${var.pkr_build_dir}/provision.sh"
  }

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
      "DISK_SIZE=${var.disk_size}",
      ]
    inline = [
      "chmod +x ${var.pkr_build_dir}/provision.sh",
      "${var.pkr_build_dir}/provision.sh"
    ]
  }
}
