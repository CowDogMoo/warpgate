#!/usr/bin/env bash
#
# Prepare Template Push
#
# This script validates prerequisites for pushing multi-architecture Docker images
# and extracts the image hashes from the Packer manifest file.
#
# Usage:
#   ./scripts/taskfile/prepare-template-push.sh <MANIFEST_PATH>
#
# Arguments:
#   MANIFEST_PATH    Path to the Packer manifest JSON file (default: ./manifest.json)
#
# Outputs:
#   Prints extracted ARM64 and AMD64 hashes to stdout
#   Exits with non-zero code on error

set -euo pipefail

# Color codes for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Configuration
MANIFEST_PATH="${1:-./manifest.json}"

# Check if jq is installed
check_dependencies() {
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is not installed. Please install jq to parse JSON files.${NC}" >&2
        echo -e "${YELLOW}Install with: brew install jq (macOS) or apt-get install jq (Linux)${NC}" >&2
        exit 1
    fi
}

# Check if manifest file exists
check_manifest_exists() {
    if [[ ! -f "$MANIFEST_PATH" ]]; then
        echo -e "${RED}Error: Manifest file not found at $MANIFEST_PATH${NC}" >&2
        echo -e "${YELLOW}Please run 'task template-build' first to generate the manifest.${NC}" >&2
        exit 1
    fi
}

# Extract hash for a specific architecture
extract_hash() {
    local arch="$1"
    local hash

    hash=$(jq -r ".builds[] | select(.name == \"$arch\") | .artifact_id" "$MANIFEST_PATH" 2> /dev/null)

    if [[ -z "$hash" || "$hash" == "null" ]]; then
        echo -e "${RED}Error: Could not extract $arch hash from manifest${NC}" >&2
        echo -e "${YELLOW}Manifest may be missing the '$arch' build.${NC}" >&2
        exit 1
    fi

    echo "$hash"
}

# Main execution
main() {
    echo -e "${GREEN}Validating prerequisites for multi-arch push...${NC}"

    # Check dependencies
    check_dependencies

    # Check manifest exists
    check_manifest_exists

    # Extract hashes
    echo -e "${GREEN}Extracting image hashes from manifest:${NC}"

    local arm64_hash
    local amd64_hash

    arm64_hash=$(extract_hash "arm64")
    amd64_hash=$(extract_hash "amd64")

    echo -e "${GREEN}ARM64_HASH:${NC} $arm64_hash"
    echo -e "${GREEN}AMD64_HASH:${NC} $amd64_hash"

    # Export for Taskfile to use
    echo "ARM64_HASH=$arm64_hash"
    echo "AMD64_HASH=$amd64_hash"

    echo -e "${GREEN}✓ Prerequisites validated successfully${NC}"
}

# Run main function
main "$@"
