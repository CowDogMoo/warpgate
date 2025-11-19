source "docker" "amd64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/amd64"
  privileged = true
  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }
  changes = [
    "USER ${var.user}",
    "WORKDIR ${var.workdir}",
  ]
  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

source "docker" "arm64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/arm64"
  privileged = true
  changes = [
    "USER ${var.user}",
    "WORKDIR ${var.workdir}",
  ]
  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }
  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

build {
  name = "ttpforge-docker"
  sources = [
    "source.docker.amd64",
    "source.docker.arm64"
  ]
  provisioner "shell" {
    only = ["docker.arm64", "docker.amd64"]
    inline = [
      "echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections",
      "apt-get update",
      "apt-get install -y --no-install-recommends python3 python3-pip sudo",
      "rm -rf /var/lib/apt/lists/*"
    ]
  }

  provisioner "ansible" {
    only          = ["docker.arm64", "docker.amd64"]
    galaxy_file   = "${var.provision_repo_path}/requirements.yml"
    playbook_file = "${var.provision_repo_path}/playbooks/ttpforge/ttpforge.yml"
    ansible_env_vars = [
      "PACKER_BUILD_NAME={{ build_name }}",
      "ANSIBLE_REMOTE_TMP=/tmp/ansible-tmp-$USER",
      "ANSIBLE_COLLECTIONS_PATH=$HOME/.ansible/collections:${var.provision_repo_path}"
    ]
    extra_arguments = [
      "-e", "ansible_shell_executable=${var.shell}",
      "-e", "ttpforge_cleanup=true"
    ]
  }

  # Final cleanup after Ansible
  provisioner "shell" {
    only = ["docker.arm64", "docker.amd64"]
    inline = [
      # Clean up Ansible/Packer runtime artifacts
      "rm -rf /home/${var.user}/.ansible || true",
      "rm -rf /tmp/ansible* || true",
      "rm -rf /tmp/packer* || true",

      # Ensure no running processes are holding files open
      "sync",

      # Clear bash history and any other shell artifacts
      "rm -f /home/${var.user}/.bash_history || true",
      "rm -f /home/${var.user}/.wget-hsts || true",
      "history -c 2>/dev/null || true",

      # Final apt cleanup
      "apt-get clean || true",
      "rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* || true",

      # Ensure clean exit
      "exit 0"
    ]
  }

  # Create manifest with the necessary information to tag and push the created image(s)
  post-processor "manifest" {
    output     = "${var.manifest_path}"
    strip_path = true
  }
}
