variable "REPO" {
    default = "ghcr.io/cowdogmoo/atomic-red"
}

variable "TAG" {
    default = "latest"
}

# Define a function to format tags
function "tag" {
  params = [suffix]
  result = [format("${REPO}%s:${TAG}", notequal("", suffix) ? "-${suffix}" : "")]
}

# Groups
group "default" {
  targets = ["multi"]
}

# Target for AMD64 architecture
target "amd64" {
  context = "."
  dockerfile = "dockerfiles/atomic-red/Dockerfile"
  platforms = ["linux/amd64"]
  args = { BASE_IMAGE_ARCH = "amd64" }
  tags = tag("")
}

# Target for ARM64 architecture
target "arm64" {
  context = "."
  dockerfile = "dockerfiles/atomic-red/Dockerfile"
  platforms = ["linux/arm64"]
  args = { BASE_IMAGE_ARCH = "arm64" }
  tags = tag("arm64")
}

# Target for multi-arch build
target "multi" {
  inherits = ["amd64"]
  tags = tag("")
  platforms = ["linux/amd64", "linux/arm64"]
  output = ["type=registry,push=true"]
}
