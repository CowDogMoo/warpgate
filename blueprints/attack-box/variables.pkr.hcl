variable "architectures" {
  type = map(object({
    platform = string
  }))
  default = {
    "amd64" = {
      platform = "linux/amd64"
    },
    "arm64" = {
      platform = "linux/arm64/v8"
    }
  }
}

variable "base_image" {
  type    = string
  description = "Base image."
}

variable "base_image_version" {
  type    = string
  description = "Version of the base image."
}

variable "container_user" {
  type    = string
  description = "Default user for a new container."
}

variable "entrypoint" {
  type    = string
  description = "Optional entrypoint script."
}

variable "new_image_tag" {
  type    = string
  description = "Tag for the created image."
}

variable "new_image_version" {
  type = string
  description = "Version for the created image."
}

variable "pkr_build_dir" {
  type    = string
  description = "Directory that packer will execute the transferred provisioning logic from within the container."
  default = "/ansible-collection-workstation"
}

variable "provision_repo_path" {
  type    = string
  description = "Path on disk to the repo that contains the provisioning code to build the container image."
}

variable "registry_server" {
  type    = string
  description = "Container registry to push to."
}

variable "registry_username" {
  type    = string
  description = "Username to connect to registry with."
}

variable "registry_cred" {
  type    = string
  description = "Token or credential to authenticate to registry with."
}

variable "setup_systemd" {
  type    = bool
  description = "Setup vnc service with systemd."
  default = true
}

variable "workdir" {
  type    = string
  description = "Working directory for a new container."
}
