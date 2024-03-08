#########################################################################################
# runzero packer template
#
# Author: Jayson Grace <Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image provisioned with
# https://github.com/CowDogMoo/ansible-collection-workstation/tree/main/playbooks/runzero
#
#########################################################################################
source "docker" "runzero_amd64" {
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/amd64"
  commit     = true
  privileged = true
  changes = [
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
  ]
  run_command = ["-d", "-i", "-t", "--privileged", "--tmpfs", "/tmp", "--tmpfs", "/run", "--tmpfs", "/run/lock", "--volume", "/sys/fs/cgroup:/sys/fs/cgroup:ro", "{{ .Image }}", "/lib/systemd/systemd"]
}

source "docker" "runzero_arm64" {
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/arm64"
  commit     = true
  privileged = true
  changes = [
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
  ]
  run_command = ["-d", "-i", "-t", "--privileged", "--tmpfs", "/tmp", "--tmpfs", "/run", "--tmpfs", "/run/lock", "--volume", "/sys/fs/cgroup:/sys/fs/cgroup:ro", "{{ .Image }}", "/lib/systemd/systemd"]
}

build {
  sources = [
    "source.docker.runzero_amd64",
    "source.docker.runzero_arm64"
  ]

  provisioner "file" {
    source      = "${var.provision_repo_path}/scripts"
    destination = "/ansible-collection-workstation/scripts"
  }

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
    ]
    script = "/ansible-collection-workstation/scripts/provision.sh"
  }

  post-processors {
    post-processor "docker-tag" {
      repository = "${var.registry_server}/${var.new_image_tag}"
      tags       = ["latest"]
    }

    post-processor "docker-push" {
      login          = true
      login_server   = "${var.registry_server}"
      login_username = "${var.registry_username}"
      login_password = "${var.registry_cred}"
    }
  }
}
