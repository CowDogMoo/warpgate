# ubuntu-vnc packer template
#
# Author: Jayson Grace <Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image
# provisioned with https://github.com/CowDogMoo/ansible-vnc-zsh on Ubuntu.
#

# Define the plugin(s) used by Packer.
packer {
  required_plugins {
    docker = {
      version = ">= 1.0.1"
      source  = "github.com/hashicorp/docker"
    }
  }
}

source "docker" "vnc" {
  commit      = true
  image   = "${var.base_image}:${var.base_image_version}"
  changes = [
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
    "ENTRYPOINT /run/docker-entrypoint.sh ; zsh",
  ]
  run_command = ["-d", "-i", "-t", "{{.Image}}"]
}

build {
  sources = ["source.docker.vnc"]

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
      repository = "${var.registry_server}/${var.new_image_tag}"
      tags = ["${var.new_image_version}"]
    }

    post-processor "docker-push" {
      login_username = "${var.registry_username}"
      login_password = "${var.registry_cred}"
    }
  }
}
