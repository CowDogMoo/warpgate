#######################################################
#                  Warpgate variables                 #
#######################################################
variable "template_name" {
  type        = string
  description = "Name of the packer template."
}

variable "pkr_build_dir" {
  type        = string
  description = "Directory that packer will execute the transferred provisioning logic from within the build environment."
  default     = "ansible-collection-arsenal"
}

variable "arsenal_repo_path" {
  type        = string
  description = "Path on disk to the repo that contains the arsenal code."
}

variable "workstation_repo_path" {
  type        = string
  description = "Path on disk to the repo that contains the workstation code."
}

variable "provision_script_path" {
  type        = string
  description = "Path on disk to the provisioning script."
  default     = "../scripts/provision.sh"
}

variable "shell" {
  type        = string
  description = "Shell to use."
  default     = "/bin/bash"
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

variable "ansible_aws_ssm_bucket_name" {
  type        = string
  description = "Name of the S3 bucket to store ansible artifacts."
  default     = "dummy-bucket"
}

variable "ansible_aws_ssm_timeout" {
  type        = number
  description = "Timeout for ansible SSM connections - 30 minutes by default."
  default     = 1800
}

variable "communicator" {
  type        = string
  description = "The communicator to use for the instance - ssh or winrm."
  default     = "none"
}

variable "disk_device_name" {
  type        = string
  description = "Disk device to use for the instance."
  default     = "/dev/xvda"
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
  description = "The type of instance to use for the initial AMI creation."
  default     = "t3.micro"
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

variable "user" {
  type        = string
  description = "Default odyssey user."
  default     = "root"
}

variable "user_data_file" {
  type        = string
  description = "Path to the user data file for instance initialization."
  default     = "../scripts/user_data.sh"
}

############################################
#           Container variables            #
############################################
variable "base_image" {
  type        = string
  description = "Base image."
  default     = "ubuntu"
}

variable "base_image_version" {
  type        = string
  description = "Version of the base image."
  default     = "latest"
}

variable "entrypoint" {
  type        = string
  description = "Optional entrypoint script."
  default     = ""
}

variable "manifest_path" {
  type        = string
  description = "Path to the generated manifest file."
  default     = "manifest.json"
}

variable "workdir" {
  type        = string
  description = "Working directory for a new container."
  default     = "/root"
}
