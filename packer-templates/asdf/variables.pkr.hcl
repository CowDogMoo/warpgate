variable "base_image" {
  type        = string
  description = "Base image for the container"
  default     = "ubuntu"
}

variable "base_image_version" {
  type        = string
  description = "Version tag for the base image"
  default     = "22.04"
}

variable "provision_repo_path" {
  type        = string
  description = "Path to the ansible-collection-workstation repository"
  default     = env("HOME")
}

variable "template_name" {
  type        = string
  description = "Name of the template"
  default     = "asdf"
}

variable "shell" {
  type        = string
  description = "Shell to use for ansible provisioning"
  default     = "/bin/bash"
}

variable "container_registry" {
  type        = string
  description = "Container registry to push images to"
  default     = "ghcr.io"
}

variable "registry_namespace" {
  type        = string
  description = "Namespace in the container registry"
  default     = "cowdogmoo"
}

variable "container_user" {
  type        = string
  description = "Non-root user for container"
  default     = "asdf"
}

variable "container_uid" {
  type        = string
  description = "UID for container user"
  default     = "1000"
}

variable "container_gid" {
  type        = string
  description = "GID for container user"
  default     = "1000"
}

variable "manifest_path" {
  type        = string
  description = "Path to output the manifest file"
  default     = "./manifest.json"
}
