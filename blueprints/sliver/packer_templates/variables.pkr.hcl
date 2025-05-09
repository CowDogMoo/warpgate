############################################
#              AWS variables               #
############################################
variable "ami_arch" {
  type        = string
  description = "The architecture of the AMI to create."
  default     = "amd64"
}

variable "ami_region" {
  type        = string
  description = "AWS region to launch the instance and create AMI."
  default     = "us-east-1"
}

variable "ansible_aws_ssm_bucket_name" {
  type        = string
  description = "Name of the S3 bucket to store ansible artifacts."
}

variable "instance_type" {
  type        = string
  description = "The type of instance to use for AMI creation."
}

variable "ssh_username" {
  type        = string
  description = "The SSH username for the AMI."
  default     = "ubuntu"
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

variable "disk_size" {
  type        = number
  description = "Disk size in GB for building the AMI."
  default     = 50
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

variable "pkr_build_dir" {
  type        = string
  description = "Directory that packer will execute the transferred provisioning logic from within the container."
  default     = "ansible-collection-arsenal"
}

variable "provision_repo_path" {
  type        = string
  description = "Path on disk to the repo that contains the provisioning code to build the container image."
}

variable "user" {
  type        = string
  description = "Default odyssey user."
}
