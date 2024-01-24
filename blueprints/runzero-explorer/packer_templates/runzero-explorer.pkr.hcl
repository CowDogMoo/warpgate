# runzero packer template
#
# Author: Jayson Grace <Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image
# provisioned with
# https://github.com/CowDogMoo/ansible-collection-workstation/tree/main/playbooks/runzero
# on Kali.
source "docker" "runzero" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
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


build {
  dynamic "source" {
    for_each = var.architectures
    iterator = arch
    labels   = ["source.docker.runzero.${arch.key}"]
    content {
      name      = "runzero.${arch.key}"
      platform  = arch.value.platform
  }
}

  // Transfer the code found at the input provision_repo_path
  // to the pkr_build_dir, which is used by packer
  // during the build process.
  provisioner "file" {
    source      = "${var.provision_repo_path}"
    destination = "${var.pkr_build_dir}"
  }

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
    ]
    script = "scripts/provision.sh"
  }

// Define a post-processor for each architecture
  post-processor "docker-tag" {
    only       = ["runzero.amd64"]
    repository = "${var.registry_server}/${var.new_image_tag}-amd64"
    tags       = ["${var.new_image_version}"]
  }

  post-processor "docker-tag" {
    only       = ["runzero.arm64"]
    repository = "${var.registry_server}/${var.new_image_tag}-arm64"
    tags       = ["${var.new_image_version}"]
  }
}
