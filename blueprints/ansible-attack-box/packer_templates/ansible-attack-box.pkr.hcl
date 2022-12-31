# Define the plugin(s) used by Packer.
packer {
  required_plugins {
    docker = {
      version = ">= 1.0.1"
      source  = "github.com/hashicorp/docker"
    }
  }
}

source "docker" "ansible-attack-box" {
  commit      = true
  image   = "${var.base_image}:${var.base_image_version}"
  changes = [
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.container_user}",
    "WORKDIR ${var.workdir}",
  ]
  run_command = ["-d", "-i", "-t", "{{ .Image }}"]
}

build {
  sources = ["source.docker.ansible-attack-box"]

  provisioner "file" {
    source = "${var.provision_repo_path}"
    destination = "${var.pkr_build_dir}"
  }

  provisioner "shell" {
    environment_vars = [
      "PKR_BUILD_DIR=${var.pkr_build_dir}",
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
