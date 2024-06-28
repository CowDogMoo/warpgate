############################################
#              AWS variables               #
############################################
variable "ami_arch" {
  type        = string
  description = "The architecture of the AMI to create."
  default     = "64Bit"
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

variable "base_image" {
  type    = string
  description = "Base image."
}

variable "base_image_version" {
  type    = string
  description = "Version of the base image."
}

variable "instance_type" {
  type        = string
  description = "The type of instance to use for the initial AMI creation."
  default     = "t3.micro"
}

variable "user_data_file" {
  type        = string
  description = "Path to the user data file for instance initialization."
  default     = "./scripts/bootstrap_win.txt"
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
  description = "Directory that packer will execute the transferred provisioning logic from within the container."
  default     = "C:\\provision-scripts"
}

variable "provision_repo_path" {
  type        = string
  description = "Path on disk to the repo that contains the provisioning code to build the container image."
}

variable "os" {
  type        = string
  description = "Operating system to use for the AMI."
  default     = "Windows_Server"
}

variable "os_version" {
  type        = string
  description = "OS version to use for the AMI."
  default     = "2022-English-Full-Base*"
}

variable "ssh_username" {
  type    = string
  default = "Administrator"
}

variable "ssh_timeout" {
  type    = string
  default = "10m"
}

variable "user" {
  type        = string
  description = "Default user for a blueprint."
  default     = "Administrator"
}
