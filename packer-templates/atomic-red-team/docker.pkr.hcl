#########################################################################################
# atomic-red-team packer template
#
# Author: Jayson Grace <jayson.e.grace@gmail.com>
#
# Description: Create container images and AMI provisioned with the
# [atomic-red-team](https://github.com/redcanaryco/atomic-red-team)
# Ansible playbook for security testing and adversary emulation.
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
  ]

  volumes = {
    "/sys/fs/cgroup" = "/sys/fs/cgroup:rw"
  }

  run_command = ["-d", "-i", "-t", "--cgroupns=host", "{{ .Image }}"]
}

build {
  name = "atomic-red-team-docker"
  sources = [
    "source.docker.amd64",
    "source.docker.arm64"
  ]

  # Pre-provisioner for ansible
  provisioner "shell" {
    only = ["docker.arm64", "docker.amd64"]
    inline = [
      "echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections",
      "apt-get update",
      "apt-get install -y python3 python3-pip sudo"
    ]
  }

  provisioner "ansible" {
    only          = ["docker.arm64", "docker.amd64"]
    galaxy_file   = "${var.provision_repo_path}/requirements.yml"
    playbook_file = "${var.provision_repo_path}/playbooks/atomic-red-team/atomic-red-team.yml"
    ansible_env_vars = [
      "PACKER_BUILD_NAME={{ build_name }}"
    ]
    extra_arguments = [
      "-e", "ansible_shell_executable=${var.shell}"
    ]
  }

  # Clean up to reduce image size
  provisioner "shell" {
    only = ["docker.arm64", "docker.amd64"]
    inline = [
      "apt-get clean",
      "rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*"
    ]
  }

  # Create manifest with the necessary information to tag and push the created image(s)
  post-processor "manifest" {
    output     = "${var.manifest_path}"
    strip_path = true
  }
}
