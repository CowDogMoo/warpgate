# Define the plugin(s) used by Packer.
packer {
  required_plugins {
    docker = {
      version = ">= 1.0.9"
      source  = "github.com/hashicorp/docker"
    }
    amazon = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}
