#######################################################
#                  Warpgate variables                 #
#######################################################
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

variable "disk_size" {
  type        = number
  description = "Disk size in GB for building the AMI."
  default     = 50
}

variable "iam_instance_profile" {
  type        = string
  description = "IAM instance profile to use for the instance."
  default     = "AmazonSSMInstanceProfileForInstances"
}

variable "instance_type" {
  type        = string
  description = "The type of instance to use for the initial AMI creation."
}

variable "os" {
  type        = string
  description = "Operating system to use for the AMI."
  default     = "kali"
}

variable "os_version" {
  type        = string
  description = "OS version to use for the AMI."
  default     = "last-snapshot"
}

variable "run_tags" {
  type        = map(string)
  description = "Tags to apply to the instance."
  default     = {
    Name = "packer-attack-box"
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
  default     = "kali"
}

variable "ssh_port" {
  type        = number
  description = "SSH port to use for the instance."
  default     = 22
}

variable "ssh_timeout" {
  type        = string
  description = "Timeout for SSH connections."
  default     = "20m"
}

variable "user" {
  type        = string
  description = "Default odyssey user."
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

variable "entrypoint" {
  type    = string
  description = "Optional entrypoint script."
}

variable "workdir" {
  type        = string
  description = "Working directory for a new container."
}
