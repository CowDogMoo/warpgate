#######################################################
#                  Warpgate variables                 #
#######################################################
variable "blueprint_name" {
  type        = string
  description = "Name of the blueprint."
}

variable "provision_script_path" {
  type        = string
  description = "Path on disk to the provisioning script."
  default = "../scripts/provision.ps1"
}

variable "provision_repo_path" {
  type        = string
  description = "Path on disk to repo that contains the provisioning code to build an odyssey."
}

#######################################################
#                  AWS variables                      #
#######################################################
variable "ami_arch" {
  type        = string
  description = "The architecture of the AMI to create."
  default     = "64Bit"
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

variable "base_image" {
  type        = string
  description = "Base image."
  default    = "Windows_Server"
}

variable "base_image_version" {
  type        = string
  description = "Version of the base image."
  default   = "2022-English-Full-Base"
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
  default     = "PackerInstanceProfile"
}

variable "instance_type" {
  type        = string
  description = "The type of instance to use for AMI creation."
  default     = "t3.micro"
}

variable "os" {
  type        = string
  description = "Operating system to use for the AMI."
  default     = "Windows_Server"
}

variable "os_version" {
  type        = string
  description = "OS version to use for the AMI."
  default   = "2022-English-Full-Base"
}

variable "run_tags" {
  type        = map(string)
  description = "Tags to apply to the instance."
  default     = {
    Name = "packer-windows"
  }
}

variable "ssh_interface" {
  type        = string
  description = "The interface to use for SSH connections."
  default     = "session_manager"
}

variable "ssh_username" {
  type        = string
  description = "Default user for a blueprint."
  default     = "Administrator"
}

variable "ssh_timeout" {
  type        = string
  description = "Timeout for SSH connections."
  default     = "20m"
}

variable "user" {
  type        = string
  description = "Default odyssey user."
  default     = "Administrator"
}

variable "user_data_file" {
  type        = string
  description = "Path to the user data file for instance initialization."
  default     = "../scripts/bootstrap_win.txt"
}

variable "winrm_username" {
  type        = string
  description = "Username for WinRM connection."
  default     = "Administrator"
}

variable "winrm_password" {
  type        = string
  description = "Password for WinRM connection."
  default     = "NotARealPassword"
}

variable "winrm_port" {
  type        = number
  description = "WinRM port to use for the instance."
  default     = 5986
}

variable "winrm_timeout" {
  type        = string
  description = "Timeout for WinRM connections."
  default     = "20m"
}
