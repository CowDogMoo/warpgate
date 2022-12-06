# ubuntu-systemd-vnc packer template
#
# Author: Jayson Grace <Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a systemd-based docker image
# that installs vnc and xfce on Ubuntu.
#
# Expected build time: ~10 minutes

# Define the plugin(s) used by Packer.
packer {
  required_plugins {
    docker = {
      version = ">= 1.0.1"
      source  = "github.com/hashicorp/docker"
    }
  }
}

source "docker" "systemd-vnc" {
  commit      = true
  image   = "${var.base_image}:${var.base_image_version}"
  privileged = true
  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }
  changes = [
    "WORKDIR ${var.workdir}",
  ]
  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{.Image}}"]
}

build {
  sources = ["source.docker.systemd-vnc"]

  // Transfer the code found at the input provision_repo_path
  // to the pkr_build_dir, which is used by packer
  // during the build process.
  provisioner "file" {
    source      = "${var.provision_repo_path}/"
    destination = "${var.pkr_build_dir}/"
  }

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
      "SETUP_SYSTEMD=${var.setup_systemd}",
      ]
    script           = "scripts/provision.sh"
  }

  post-processors {
    post-processor "docker-tag" {
      repository = "${var.new_image_tag}"
      tags = ["${var.new_image_version}"]
    }
    post-processor "docker-push" {}
  }
}
