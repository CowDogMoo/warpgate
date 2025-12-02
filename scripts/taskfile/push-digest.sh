#!/usr/bin/env bash
#
# Push Architecture-Specific Docker Image by Digest
#
# This script pushes a Docker image for a specific architecture to a registry,
# extracts the digest, and saves it to a file for later manifest creation.
#
# Usage:
#   ./scripts/taskfile/push-digest.sh <REGISTRY> <NAMESPACE> <IMAGE_NAME> <ARCH> <MANIFEST_PATH> <GITHUB_USER> <GITHUB_TOKEN>
#
# Arguments:
#   REGISTRY       Container registry (e.g., ghcr.io)
#   NAMESPACE      Registry namespace (e.g., organization or user)
#   IMAGE_NAME     Name of the image
#   ARCH           Architecture (amd64 or arm64)
#   MANIFEST_PATH  Path to Packer manifest.json file
#   GITHUB_USER    GitHub username for authentication
#   GITHUB_TOKEN   GitHub token for authentication
#
# Outputs:
#   Creates digest-<IMAGE_NAME>-<ARCH>.txt with the pushed image digest

set -euo pipefail

# Color codes for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Parse arguments
if [[ $# -ne 7 ]]; then
    echo -e "${RED}Error: Invalid number of arguments${NC}" >&2
    echo "Usage: $0 <REGISTRY> <NAMESPACE> <IMAGE_NAME> <ARCH> <MANIFEST_PATH> <GITHUB_USER> <GITHUB_TOKEN>" >&2
    exit 1
fi

readonly REGISTRY="$1"
readonly NAMESPACE="$2"
readonly IMAGE_NAME="$3"
readonly ARCH="$4"
readonly MANIFEST_PATH="$5"
readonly GITHUB_USER="$6"
readonly GITHUB_TOKEN="$7"

readonly DIGEST_FILE="digest-${IMAGE_NAME}-${ARCH}.txt"

# Check if manifest file exists
check_manifest_exists() {
    if [[ ! -f "$MANIFEST_PATH" ]]; then
        echo -e "${RED}Error: Manifest file not found at $MANIFEST_PATH${NC}" >&2
        exit 1
    fi
}

# Check if jq is installed
check_dependencies() {
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is not installed. Please install jq to parse JSON files.${NC}" >&2
        exit 1
    fi
}

# Extract image hash from manifest
extract_image_hash() {
    local hash

    hash=$(jq -r ".builds[] | select(.name == \"$ARCH\") | .artifact_id" "$MANIFEST_PATH" 2> /dev/null)

    if [[ -z "$hash" || "$hash" == "null" ]]; then
        echo -e "${RED}Error: Could not extract $ARCH hash from manifest${NC}" >&2
        exit 1
    fi

    echo "$hash"
}

# Login to Docker registry
docker_login() {
    echo -e "${GREEN}Logging in to $REGISTRY...${NC}"
    echo "$GITHUB_TOKEN" | docker login "$REGISTRY" -u "$GITHUB_USER" --password-stdin
}

# Tag and push the image
push_image() {
    local image_hash="$1"
    local target_tag="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}:${ARCH}"

    echo -e "${GREEN}Tagging image: $image_hash → $target_tag${NC}"
    docker tag "$image_hash" "$target_tag"

    echo -e "${GREEN}Pushing $ARCH image...${NC}"
    local push_output
    push_output=$(docker push "$target_tag" 2>&1)
    echo "$push_output"

    echo "$push_output"
}

# Extract digest from push output
extract_digest() {
    local push_output="$1"
    local digest

    # Extract just the sha256 digest without size info
    digest=$(echo "$push_output" | grep -oE 'digest: sha256:[a-f0-9]{64}' | cut -d' ' -f2)

    if [[ -z "$digest" ]]; then
        echo -e "${RED}Error: Failed to extract digest from push output${NC}" >&2
        echo -e "${YELLOW}Push output:${NC}" >&2
        echo "$push_output" >&2
        exit 1
    fi

    echo "$digest"
}

# Save digest to file
save_digest() {
    local digest="$1"

    echo "$digest" > "$DIGEST_FILE"
    echo -e "${GREEN}Saved digest to $DIGEST_FILE${NC}"
}

# Main execution
main() {
    echo -e "${GREEN}Pushing $ARCH image for $IMAGE_NAME...${NC}"

    # Validate prerequisites
    check_dependencies
    check_manifest_exists

    # Extract image hash from manifest
    local image_hash
    image_hash=$(extract_image_hash)
    echo -e "${GREEN}Image hash: $image_hash${NC}"

    # Login to registry
    docker_login

    # Push the image
    local push_output
    push_output=$(push_image "$image_hash")

    # Extract and save digest
    local digest
    digest=$(extract_digest "$push_output")
    echo -e "${GREEN}Pushed $ARCH image with digest: $digest${NC}"

    save_digest "$digest"

    echo -e "${GREEN}✓ Successfully pushed $ARCH image${NC}"
}

# Run main function
main "$@"
