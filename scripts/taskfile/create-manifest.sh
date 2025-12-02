#!/usr/bin/env bash
#
# Create and Push Multi-Architecture Manifest
#
# This script creates and pushes a multi-architecture Docker manifest by combining
# architecture-specific images. It also creates a timestamped tag.
#
# Usage:
#   ./scripts/taskfile/create-manifest.sh <REGISTRY> <NAMESPACE> <IMAGE_NAME> <TAG> <GITHUB_USER> <GITHUB_TOKEN>
#
# Arguments:
#   REGISTRY       Container registry (e.g., ghcr.io)
#   NAMESPACE      Registry namespace (e.g., organization or user)
#   IMAGE_NAME     Name of the image
#   TAG            Tag for the manifest (e.g., latest)
#   GITHUB_USER    GitHub username for authentication
#   GITHUB_TOKEN   GitHub token for authentication
#
# Requires:
#   - digest-<IMAGE_NAME>-amd64.txt
#   - digest-<IMAGE_NAME>-arm64.txt
#
# Outputs:
#   Creates and pushes multi-arch manifest with both the specified tag and a timestamp tag

set -euo pipefail

# Color codes for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Parse arguments
if [[ $# -ne 6 ]]; then
    echo -e "${RED}Error: Invalid number of arguments${NC}" >&2
    echo "Usage: $0 <REGISTRY> <NAMESPACE> <IMAGE_NAME> <TAG> <GITHUB_USER> <GITHUB_TOKEN>" >&2
    exit 1
fi

readonly REGISTRY="$1"
readonly NAMESPACE="$2"
readonly IMAGE_NAME="$3"
readonly TAG="$4"
readonly GITHUB_USER="$5"
readonly GITHUB_TOKEN="$6"

readonly AMD64_DIGEST_FILE="digest-${IMAGE_NAME}-amd64.txt"
readonly ARM64_DIGEST_FILE="digest-${IMAGE_NAME}-arm64.txt"

# Check if digest files exist
check_digest_files() {
    local missing_files=()

    if [[ ! -f "$AMD64_DIGEST_FILE" ]]; then
        missing_files+=("$AMD64_DIGEST_FILE")
    fi

    if [[ ! -f "$ARM64_DIGEST_FILE" ]]; then
        missing_files+=("$ARM64_DIGEST_FILE")
    fi

    if [[ ${#missing_files[@]} -gt 0 ]]; then
        echo -e "${RED}Error: Required digest files not found:${NC}" >&2
        for file in "${missing_files[@]}"; do
            echo -e "  ${YELLOW}$file${NC}" >&2
        done
        echo -e "${YELLOW}Please run 'task template-push-digest' first for both architectures.${NC}" >&2
        exit 1
    fi
}

# Login to Docker registry
docker_login() {
    echo -e "${GREEN}Logging in to $REGISTRY...${NC}"
    echo "$GITHUB_TOKEN" | docker login "$REGISTRY" -u "$GITHUB_USER" --password-stdin
}

# Read digest from file
read_digest() {
    local file="$1"
    local digest

    digest=$(cat "$file" | tr -d '[:space:]')

    if [[ -z "$digest" ]]; then
        echo -e "${RED}Error: Empty digest in $file${NC}" >&2
        exit 1
    fi

    echo "$digest"
}

# Create and push manifest
create_manifest() {
    local tag="$1"
    local amd64_digest="$2"
    local arm64_digest="$3"

    local manifest_tag="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}:${tag}"
    local amd64_image="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}@${amd64_digest}"
    local arm64_image="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}@${arm64_digest}"

    echo -e "${GREEN}Creating manifest: $manifest_tag${NC}"
    echo -e "  AMD64: $amd64_image"
    echo -e "  ARM64: $arm64_image"

    docker buildx imagetools create -t "$manifest_tag" \
        "$amd64_image" \
        "$arm64_image"

    echo -e "${GREEN}✓ Successfully created and pushed manifest: $manifest_tag${NC}"
}

# Generate timestamp tag
generate_timestamp_tag() {
    date +%Y%m%d-%H%M%S
}

# Main execution
main() {
    echo -e "${GREEN}Creating multi-architecture manifest for $IMAGE_NAME...${NC}"

    # Check prerequisites
    check_digest_files

    # Login to registry
    docker_login

    # Read digests
    local amd64_digest
    local arm64_digest

    amd64_digest=$(read_digest "$AMD64_DIGEST_FILE")
    arm64_digest=$(read_digest "$ARM64_DIGEST_FILE")

    echo -e "${GREEN}AMD64 digest: $amd64_digest${NC}"
    echo -e "${GREEN}ARM64 digest: $arm64_digest${NC}"

    # Create manifest with specified tag
    create_manifest "$TAG" "$amd64_digest" "$arm64_digest"

    # Create manifest with timestamp tag
    local timestamp
    timestamp=$(generate_timestamp_tag)
    echo -e "${GREEN}Also creating timestamped manifest...${NC}"
    create_manifest "$timestamp" "$amd64_digest" "$arm64_digest"

    echo -e "${GREEN}✓ Multi-architecture manifest creation complete${NC}"
    echo -e "${GREEN}Available tags:${NC}"
    echo -e "  - $TAG"
    echo -e "  - $timestamp"
}

# Run main function
main "$@"
