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
  image      = "ubuntu:latest"
  platform   = "linux/amd64"
  privileged = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
    "LABEL architecture=amd64"
  ]

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "docker" "runzero_arm64" {
  commit     = true
  image      = "ubuntu:latest"
  platform   = "linux/arm64"
  privileged = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
    "LABEL architecture=arm64"
  ]

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

  post-processor "docker-tag" {
    repository = "ghcr.io/cowdogmoo/runzero-explorer"
    tags       = ["amd64-latest"]
    only       = ["source.docker.runzero_amd64"]
  }

  post-processor "docker-tag" {
    repository = "ghcr.io/cowdogmoo/runzero-explorer"
    tags       = ["arm64-latest"]
    only       = ["source.docker.runzero_arm64"]
  }

  post-processor "docker-push" {
    login          = true
    login_server   = "ghcr.io"
    login_username = "cowdogmoo"
    login_password = "${var.registry_cred}"
  }
}
