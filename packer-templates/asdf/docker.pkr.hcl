source "docker" "amd64" {
  image    = "${var.base_image}:${var.base_image_version}"
  platform = "linux/amd64"
  commit   = true
  pull     = true
  changes = [
    "ENV ASDF_DIR=/home/${var.container_user}/.asdf",
    "ENV ASDF_DATA_DIR=/home/${var.container_user}/.asdf",
    "ENV PATH=/home/${var.container_user}/.asdf/shims:/home/${var.container_user}/.asdf/bin:$PATH",
    "USER ${var.container_user}",
    "WORKDIR /workspace",
    "CMD [\"/bin/bash\"]"
  ]
}

source "docker" "arm64" {
  image    = "${var.base_image}:${var.base_image_version}"
  platform = "linux/arm64"
  commit   = true
  pull     = true
  changes = [
    "ENV ASDF_DIR=/home/${var.container_user}/.asdf",
    "ENV ASDF_DATA_DIR=/home/${var.container_user}/.asdf",
    "ENV PATH=/home/${var.container_user}/.asdf/shims:/home/${var.container_user}/.asdf/bin:$PATH",
    "USER ${var.container_user}",
    "WORKDIR /workspace",
    "CMD [\"/bin/bash\"]"
  ]
}

build {
  name = "asdf-docker"

  sources = [
    "source.docker.amd64",
    "source.docker.arm64"
  ]

  # Install minimal Python dependencies for Ansible and create non-root user
  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y --no-install-recommends python3 python3-pip sudo git curl ca-certificates",
      "groupadd -g ${var.container_gid} ${var.container_user}",
      "useradd -m -u ${var.container_uid} -g ${var.container_gid} -s /bin/bash ${var.container_user}",
      "echo '${var.container_user} ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/${var.container_user}",
      "chmod 0440 /etc/sudoers.d/${var.container_user}",
      "mkdir -p /workspace",
      "chown -R ${var.container_user}:${var.container_user} /workspace",
      "apt-get clean",
      "rm -rf /var/lib/apt/lists/*"
    ]
  }

  # Run Ansible provisioning with asdf role as non-root user
  provisioner "ansible" {
    only          = ["docker.arm64", "docker.amd64"]
    user          = "${var.container_user}"
    playbook_file = "${var.provision_repo_path}/CowDogMoo/ansible-collection-workstation/playbooks/asdf/asdf.yml"
    ansible_env_vars = [
      "PACKER_BUILD_NAME={{ build_name }}",
      "ANSIBLE_ROLES_PATH=${var.provision_repo_path}/CowDogMoo/ansible-collection-workstation/roles"
    ]
    extra_arguments = [
      "-e", "ansible_shell_executable=${var.shell}",
      "-e", "ansible_python_interpreter=/usr/bin/python3",
      "-e", "ansible_user_id=${var.container_user}",
      "-e", "ansible_user_dir=/home/${var.container_user}",
      "-e", "asdf_cleanup=true"
    ]
  }

  # Post-Ansible cleanup using generated script
  # The build_cleanup role generates this script during Ansible provisioning
  provisioner "shell" {
    only = ["docker.arm64", "docker.amd64"]
    inline = [
      "/tmp/post_ansible_cleanup.sh"
    ]
  }

  post-processors {
    post-processor "docker-tag" {
      repository = "${var.container_registry}/${var.registry_namespace}/${var.template_name}"
      tags       = ["latest", "${local.timestamp}"]
    }

    post-processor "manifest" {
      output     = "${var.manifest_path}"
      strip_path = true
    }
  }
}
