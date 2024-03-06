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
    args = {
        BASE_IMAGE = "mcr.microsoft.com/powershell:mariner-2.0"
    }
    platforms = ["linux/amd64"]
    tags = ["${REPO}:${TAG}"]
    output = ["type=image,push=true"]
}

target "arm64" {
    dockerfile = "Dockerfile"
    args = {
        BASE_IMAGE = "mcr.microsoft.com/powershell:mariner-2.0-arm64"
    }
    platforms = ["linux/arm64"]
    tags = ["${REPO}:${TAG}"]
    output = ["type=image,push=true"]
}