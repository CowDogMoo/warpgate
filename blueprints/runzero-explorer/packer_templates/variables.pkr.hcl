variable "base_image" {
  type        = string
  description = "Base image."
}

variable "base_image_version" {
  type        = string
  description = "Version of the base image."
}

variable "blueprint_name" {
  type        = string
  description = "Name of the blueprint."
}

variable "container_user" {
  type        = string
  description = "Default user for a new container."
}

variable "pkr_build_dir" {
  type        = string
  description = "Directory that packer will execute the transferred provisioning logic from within the container."
  default     = "/ansible-collection-workstation"
}

variable "provision_repo_path" {
  type        = string
  description = "Path on disk to the repo that contains the provisioning code to build the container image."
}

variable "runzero_download_token" {
  type        = string
  description = "Token to download runzero."
  default     = env("RUNZERO_DOWNLOAD_TOKEN")
  validation {
    condition     = length(var.runzero_download_token) > 0
    error_message = <<EOF
The RUNZERO_DOWNLOAD_TOKEN is not set: this is required to download runZero explorer.
EOF
  }
}

variable "setup_systemd" {
  type        = bool
  description = "Create systemd service for container."
  default     = false
}

variable "workdir" {
  type        = string
  description = "Working directory for a new container."
}
