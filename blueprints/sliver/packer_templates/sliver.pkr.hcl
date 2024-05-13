#########################################################################################
# sliver packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image provisioned with
# https://github.com/l50/ansible-collection-arsenal/tree/main/playbooks/sliver
#
#########################################################################################
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
  ami_name      = "${var.blueprint_name}-${formatdate("YYYY-MM-DD-hh-mm-ss", timestamp())}"
  instance_type = "${var.instance_type}"
  region        = "${var.ami_region}"
  source_ami_filter {
    filters = {
      name = "${var.os}/images/*${var.os}-${var.os_version}-${var.ami_arch}-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    owners      = ["099720109477"] // Canonical's owner ID for Ubuntu images
    most_recent = true
  }
  ssh_username = "${var.ssh_username}"
}

build {
  sources = [
    "source.docker.amd64",
    "source.docker.arm64",
    "source.amazon-ebs.ubuntu",
  ]

  provisioner "file" {
    source      = "${var.provision_repo_path}"
    destination = "${var.pkr_build_dir}"
  }

  provisioner "file" {
    source      = "${path.cwd}/scripts/provision.sh"
    destination = "${var.pkr_build_dir}/provision.sh"
  }

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
    ]
    inline = [
      "chmod +x ${var.pkr_build_dir}/provision.sh",
      "${var.pkr_build_dir}/provision.sh"
    ]
  }
}
