variable "REPO" {
    default = "ghcr.io/cowdogmoo/atomic-red"
}

variable "TAG" {
    default = "latest"
}

group "default" {
    targets = ["amd64", "arm64"]
}

target "amd64" {
    dockerfile = "Dockerfile"
    platforms = ["linux/amd64"]
    args = { BASE_IMAGE_ARCH = "amd64" }
    tags = ["${REPO}:${TAG}"]
    output = ["type=image,push=true"]
}

target "arm64" {
    dockerfile = "Dockerfile"
    platforms = ["linux/arm64"]
    args = { BASE_IMAGE_ARCH = "arm64" }
    tags = ["${REPO}:${TAG}"]
    output = ["type=image,push=true"]
}
