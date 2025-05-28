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
  name = "runzero-explorer-docker"
  sources = [
    "source.docker.amd64",
    "source.docker.arm64"
  ]

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
    playbook_file = "${var.provision_repo_path}/playbooks/runzero_explorer/runzero_explorer.yml"
    ansible_env_vars = [
      "PACKER_BUILD_NAME={{ build_name }}",
      "RUNZERO_DOWNLOAD_TOKEN=${var.runzero_download_token}"
    ]
    extra_arguments = [
      "-e", "ansible_shell_executable=${var.shell}",
      "-e", "runzero_download_token=${var.runzero_download_token}"
    ]
  }

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
