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
    "LABEL architecture=amd64"
  ]

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "docker" "runzero_arm64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/arm64"
  privileged = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "ENTRYPOINT ${var.entrypoint}",
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

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
      "RUNZERO_DOWNLOAD_TOKEN=${var.runzero_download_token}",
    ]
    inline = [
      "chmod +x ${var.pkr_build_dir}/provision.sh",
      "${var.pkr_build_dir}/provision.sh"
    ]
  }

  post-processors {
    post-processor "docker-tag" {
      repository = "${var.registry_server}/${var.image_name}"
      tags       = ["amd64-latest"]
      only       = ["source.docker.runzero_amd64"]
    }

    post-processor "docker-tag" {
      repository = "${var.registry_server}/${var.image_name}"
      tags       = ["arm64-latest"]
      only       = ["source.docker.runzero_arm64"]
    }

    post-processor "docker-push" {
      login          = true
      login_server   = "${var.registry_server}"
      login_username = "${var.registry_username}"
      login_password = "${var.registry_cred}"
    }
  }
}
