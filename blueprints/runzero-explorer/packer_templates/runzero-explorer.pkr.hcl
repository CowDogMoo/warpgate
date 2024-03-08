#########################################################################################
# runZero packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image provisioned with
# https://github.com/CowDogMoo/ansible-collection-workstation/tree/main/playbooks/runzero
#
#########################################################################################
source "docker" "runzero_amd64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/amd64"
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

source "docker" "runzero_arm64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/arm64"
  privileged = true

  changes = [
    "ENTRYPOINT ${var.entrypoint}",
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
    "source.docker.runzero_amd64",
    "source.docker.runzero_arm64"
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

  provisioner "shell-local" {
    inline = [
      "docker login -u ${var.registry_username} -p ${var.registry_cred} ${var.registry_server}",
      "docker manifest create ${var.registry_server}/${var.new_image_tag}:latest --amend ${var.registry_server}/${var.new_image_tag}:amd64-latest --amend ${var.registry_server}/${var.new_image_tag}:arm64-latest",
      "docker manifest push ${var.registry_server}/${var.new_image_tag}:latest"
    ]
    environment_vars = [
      "DOCKER_CLI_EXPERIMENTAL=enabled"
    ]
  }
}
