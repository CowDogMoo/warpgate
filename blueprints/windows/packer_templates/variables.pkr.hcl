############################################
#           Global variables               #
############################################
variable "blueprint_name" {
  type        = string
  description = "Name of the blueprint."
}

variable "provision_repo_path" {
  type        = string
  description = "Path on disk to the repo that contains the provisioning code to build the container image."
}

variable "restart_check_command" {
  type        = string
  description = "Command to check for system restarts."
  default     = "powershell -command \"& { Write-Output 'Restarting...'; exit 1 }\""
}

############################################
#               SSH variables              #
############################################
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

############################################
#              WinRM variables             #
############################################
variable "winrm_username" {
  type        = string
  description = "Username for WinRM connection."
  default     = "Administrator"
}

variable "winrm_password" {
  type        = string
  description = "Password for WinRM connection."
  default     = "Sup3rS3c,;)r3t"
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
  type        = string
  description = "Base image."
}

variable "base_image_version" {
  type        = string
  description = "Version of the base image."
}

variable "communicator" {
  type        = string
  description = "The communicator to use for the instance - ssh or winrm."
  default     = "ssh"
}

variable "iam_instance_profile" {
  type        = string
  description = "IAM instance profile to use for the instance."
  default     = "AmazonSSMInstanceProfileForInstances"
}

variable "instance_type" {
  type        = string
  description = "The type of instance to use for the initial AMI creation."
  default     = "t3.micro"
}

variable "max_retries" {
  type        = number
  description = "Maximum number of retries for the restart provisioner."
  default     = 5
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

# variable "pkr_build_dir" {
#   type        = string
#   description = "Directory that packer will execute the transferred provisioning logic from within the container."
#   default     = "scripts"
# }

variable "run_tags" {
  type        = map(string)
  description = "Tags to apply to the instance."
  default     = {
    Name = "packer-windows"
  }
}

variable "user_data_file" {
  type        = string
  description = "Path to the user data file for instance initialization."
  default     = "./scripts/bootstrap_win.txt"
}
