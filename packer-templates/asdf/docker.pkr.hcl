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
    playbook_file = "${path.root}/playbook.yml"
    ansible_env_vars = [
      "PACKER_BUILD_NAME={{ build_name }}",
      "ANSIBLE_ROLES_PATH=${var.provision_repo_path}/CowDogMoo/ansible-collection-workstation/roles"
    ]
    extra_arguments = [
      "-e", "ansible_shell_executable=${var.shell}",
      "-e", "ansible_python_interpreter=/usr/bin/python3",
      "-e", "ansible_user_id=${var.container_user}",
      "-e", "ansible_user_dir=/home/${var.container_user}"
    ]
  }

  # Aggressive cleanup to minimize image size (similar to sliver cleanup)
  provisioner "shell" {
    inline = [
      # Remove Ansible artifacts
      "rm -rf /home/${var.container_user}/.ansible",
      "rm -rf /root/.ansible",
      "rm -rf /tmp/ansible* /tmp/packer*",

      # Remove Python development packages (keep core Python)
      "apt-get remove -y --purge python3-dev python3-pip python3-setuptools python3-wheel python3-distutils python3-lib2to3 || true",

      # Remove development tools
      "apt-get remove -y --purge gcc* g++* cpp* build-essential libtool cmake dpkg-dev *-dev *-doc manpages manpages-dev || true",

      # Remove Perl
      "apt-get remove -y --purge perl perl-base perl-modules-* libperl* || true",

      # Remove unnecessary graphics/system libraries
      "apt-get remove -y --purge libllvm* llvm* libgl* mesa* || true",

      # Clean Python artifacts but keep core
      "find /usr/lib/python3* -type d -name '__pycache__' -exec rm -rf {} + 2>/dev/null || true",
      "find /usr/lib/python3* -type f \\( -name '*.pyc' -o -name '*.pyo' \\) -delete 2>/dev/null || true",
      "find /usr/lib/python3* -type d \\( -name 'test' -o -name 'tests' \\) -exec rm -rf {} + 2>/dev/null || true",

      # Remove static libraries and headers
      "find /usr -name '*.a' -delete 2>/dev/null || true",
      "find /usr -name '*.la' -delete 2>/dev/null || true",
      "rm -rf /usr/include",

      # Remove documentation and locales
      "rm -rf /usr/share/doc /usr/share/man /usr/share/info",
      "find /usr/share/locale -mindepth 1 -maxdepth 1 ! -name 'en*' -exec rm -rf {} + 2>/dev/null || true",

      # Clean caches
      "rm -rf /home/${var.container_user}/.cache /root/.cache",
      "rm -rf /home/${var.container_user}/.local /root/.local",
      "rm -rf /var/cache/* /var/tmp/* /tmp/*",

      # Truncate logs
      "find /var/log -type f -exec truncate -s 0 {} \\; 2>/dev/null || true",

      # Final APT cleanup
      "apt-get autoremove -y --purge",
      "apt-get autoclean -y",
      "apt-get clean",
      "rm -rf /var/lib/apt/lists/*",
      "rm -rf /var/cache/apt/*"
    ]
  }

  post-processors {
    post-processor "docker-tag" {
      repository = "${var.container_registry}/${var.registry_namespace}/${var.template_name}"
      tags       = ["latest", "${local.timestamp}"]
    }

    post-processor "manifest" {
      output     = "packer-manifest-${source.name}.json"
      strip_path = true
    }
  }
}
