#########################################################################################
# sliver packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create a docker image provisioned with
# https://github.com/l50/ansible-collection-arsenal/tree/main/playbooks/sliver
# Ansible playbook.
#########################################################################################
# Docker AMD64 source configuration
source "docker" "amd64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/amd64"
  privileged = true

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  changes = [
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.user}",
    "WORKDIR ${var.workdir}",
    "ENV PATH=/opt/sliver:/home/sliver/.sliver/go/bin:$PATH",
  ]

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

# Docker ARM64 source configuration
source "docker" "arm64" {
  commit     = true
  image      = "${var.base_image}:${var.base_image_version}"
  platform   = "linux/arm64"
  privileged = true

  changes = [
    "ENTRYPOINT ${var.entrypoint}",
    "USER ${var.user}",
    "WORKDIR ${var.workdir}",
    "ENV PATH=/opt/sliver:/home/sliver/.sliver/go/bin:$PATH",
  ]

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

build {
  name = "sliver-docker"
  sources = [
    "source.docker.amd64",
    "source.docker.arm64",
  ]

  # Pre-provisioner for ansible
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
    galaxy_file   = "${var.arsenal_repo_path}/requirements.yml"
    playbook_file = "${var.arsenal_repo_path}/playbooks/sliver/sliver.yml"
    ansible_env_vars = [
      "PACKER_BUILD_NAME={{ build_name }}",
      "ANSIBLE_REMOTE_TMP=/tmp/ansible-tmp-$USER"
    ]
    extra_arguments = [
      "-e", "ansible_shell_executable=${var.shell}",
      "-e", "sliver_cleanup=true",
      "-e", "sliver_unpack_at_build=false"
    ]
  }

  # Final cleanup after Ansible
  provisioner "shell" {
    only = ["docker.arm64", "docker.amd64"]
    inline = [
      # Clean up Ansible/Packer runtime artifacts
      "rm -rf /home/${var.user}/.ansible",
      "rm -rf /tmp/ansible*",
      "rm -rf /tmp/packer*",

      # Ensure no running processes are holding files open
      "sync",

      # Final permission fixes if needed
      "chmod 755 /opt/sliver/sliver-server 2>/dev/null || true",
      "chmod 755 /opt/sliver/sliver-client 2>/dev/null || true",

      # Clear bash history and any other shell artifacts
      "rm -f /home/${var.user}/.bash_history",
      "rm -f /home/${var.user}/.wget-hsts",
      "history -c 2>/dev/null || true",

      # Ensure clean exit
      "exit 0"
    ]
  }

  post-processors {
    post-processor "docker-tag" {
      repository = "sliver"
      tags       = ["latest"]
    }
  }

  # Create manifest with the necessary information to tag and push the created image(s)
  post-processor "manifest" {
    output     = "${var.manifest_path}"
    strip_path = true
  }
}
