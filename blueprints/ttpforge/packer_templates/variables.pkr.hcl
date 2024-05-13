############################################
#              AWS variables               #
############################################
variable "ami_arch" {
  type        = string
  description = "The architecture of the AMI to create."
  default     = "amd64"
}

variable "ami_instance_type" {
  type        = string
  description = "The type of instance to use for the initial AMI creation."
  default     = "t3.small"
}

variable "ami_region" {
  type        = string
  description = "AWS region to launch the instance and create AMI."
  default     = "us-east-1"
}

variable "instance_type" {
  type        = string
  description = "The type of instance to use for the initial AMI creation."
  default     = "t3.medium"
}

############################################
#           Container variables            #
############################################
variable "base_image" {
  type        = string
  description = "Base image."
}

variable "base_image_version" {
  type        = string
  description = "Version of the base image."
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

############################################
#           Global variables               #
############################################
variable "blueprint_name" {
  type        = string
  description = "Name of the blueprint."
}

variable "pkr_build_dir" {
  type        = string
  description = "Directory that packer will execute the transferred provisioning logic from within the build environment."
  default     = "ansible-collection-arsenal"
}

variable "provision_repo_path" {
  type        = string
  description = "Path on disk to the repo that contains the provisioning code to build the odyssey."
}

variable "os" {
  type        = string
  description = "Operating system to use for the AMI."
  default     = "ubuntu"
}

variable "os_version" {
  type        = string
  description = "OS version to use for the AMI."
  default     = "jammy-22.04"
}

variable "user" {
  type        = string
  description = "Default user for a blueprint."
}
