#########################################################################################
# atomic-red-team packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image provisioned with
# https://github.com/l50/ansible-collection-arsenal/tree/main/playbooks/atomic_red_team
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
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
  ]

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

build {
  sources = [
    "source.docker.amd64",
    "source.docker.arm64"
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
