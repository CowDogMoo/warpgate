#######################################################
#                  Warpgate variables                 #
#######################################################
variable "blueprint_name" {
  type        = string
  description = "Name of the blueprint."
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

variable "shell" {
  type        = string
  description = "Shell to use."
  default     = "/bin/zsh"
}

variable "user" {
  type        = string
  description = "Default user for the container."
}

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
}

variable "ansible_aws_ssm_bucket_name" {
  type        = string
  description = "Name of the S3 bucket to store ansible artifacts."
}

variable "ansible_aws_ssm_timeout" {
  type        = number
  description = "Timeout for ansible SSM connections - 30 minutes by default."
  default     = 1800
}

variable "communicator" {
  type        = string
  description = "The communicator to use for the instance - ssh or winrm."
  default     = "ssh"
}


variable "disk_device_name" {
  type        = string
  description = "Disk device to use for the instance."
  default     = "/dev/sda1"
}

variable "disk_size" {
  type        = number
  description = "Disk size in GB for building the AMI."
  default     = 50
}

variable "iam_instance_profile" {
  type        = string
  description = "IAM instance profile to use for the instance."
}

variable "instance_type" {
  type        = string
  description = "The type of instance to use for AMI creation."
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

variable "run_tags" {
  type        = map(string)
  description = "Tags to apply to the instance."
  default = {
    Name = "packer-sliver"
  }
}

variable "ssh_interface" {
  type        = string
  description = "The interface to use for SSH connections."
  default     = "session_manager"
}

variable "ssh_username" {
  type        = string
  description = "The SSH username for the AMI."
  default     = "ubuntu"
}

variable "ssh_timeout" {
  type        = string
  description = "Timeout for SSH connections."
  default     = "20m"
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

# TODO: Add the following variables to the sliver blueprint
# and remove the user so we can have an ssh_username and a container_username
# variable "container_username" {
#   type        = string
#   description = "Default user for the container."
# }

variable "workdir" {
  type        = string
  description = "Working directory for a new container."
}
