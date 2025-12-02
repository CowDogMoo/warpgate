#!/usr/bin/env bash
#
# Run GitHub Actions image-builder workflow using act
#
# This script handles the execution of the image-builder workflow locally,
# including container cleanup, secret file management, and optional template filtering.
#
# Usage:
#   ./scripts/taskfile/run-image-builder.sh [TEMPLATE_NAME]
#
# Arguments:
#   TEMPLATE_NAME    Optional template name to build (if omitted, builds all templates)
#
# Environment Variables:
#   OS               Operating system (default: auto-detected)
#   ARCH             Architecture (default: auto-detected)
#   ACT_SECRETS_FILE Path to secrets file (default: .secrets, falls back to temporary file)

set -euo pipefail

# Color codes for output
readonly RED='\033[0;31m'
readonly YELLOW='\033[1;33m'
readonly GREEN='\033[0;32m'
readonly NC='\033[0m' # No Color

# Configuration
readonly WORKFLOW_FILE=".github/workflows/warpgate-image-builder.yaml"
readonly EVENT_FILE="/tmp/github-event.json"
readonly TEMP_SECRETS_FILE=".secrets.tmp"
TEMPLATE="${1:-}"

# Detect platform and architecture
detect_platform() {
    local os="${OS:-$(uname -s)}"
    local arch="${ARCH:-$(uname -m)}"

    # Check if we're on Mac ARM
    if [[ "$os" == "Darwin" && "$arch" == "arm64" ]]; then
        echo "--container-architecture linux/amd64"
    else
        echo ""
    fi
}

# Clean up existing act containers
cleanup_containers() {
    echo -e "${GREEN}Cleaning up existing act containers...${NC}"
    docker ps -q -f name=act-Image-Builder | xargs -r docker rm -f 2> /dev/null || true
}

# Set up secrets file
setup_secrets() {
    local secrets_file="${ACT_SECRETS_FILE:-.secrets}"

    if [[ ! -f "$secrets_file" ]]; then
        echo -e "${YELLOW}Warning: $secrets_file file not found. Creating temporary one with GITHUB_TOKEN=dummy for testing.${NC}"
        echo "GITHUB_TOKEN=dummy" > "$TEMP_SECRETS_FILE"
        echo "$TEMP_SECRETS_FILE"
    else
        echo "$secrets_file"
    fi
}

# Clean up temporary files
cleanup_temp_files() {
    local secrets_file="$1"

    if [[ "$secrets_file" == "$TEMP_SECRETS_FILE" ]]; then
        rm -f "$TEMP_SECRETS_FILE"
    fi

    rm -f "$EVENT_FILE"
}

# Run act with the appropriate configuration
run_act() {
    local arch_flag="$1"
    local secrets_file="$2"
    local exit_code=0

    if [[ -n "$TEMPLATE" ]]; then
        echo -e "${GREEN}Building template: $TEMPLATE${NC}"
        echo "{\"inputs\":{\"TEMPLATE\":\"$TEMPLATE\"}}" > "$EVENT_FILE"

        # shellcheck disable=SC2086
        act -W "$WORKFLOW_FILE" $arch_flag -e "$EVENT_FILE" --secret-file "$secrets_file" || exit_code=$?
    else
        echo -e "${GREEN}Building all templates${NC}"

        # shellcheck disable=SC2086
        act -W "$WORKFLOW_FILE" $arch_flag --secret-file "$secrets_file" || exit_code=$?
    fi

    return $exit_code
}

# Main execution
main() {
    local arch_flag
    local secrets_file
    local exit_code=0

    # Detect platform-specific flags
    arch_flag=$(detect_platform)

    # Clean up existing containers
    cleanup_containers

    # Set up secrets file
    secrets_file=$(setup_secrets)

    # Run act
    run_act "$arch_flag" "$secrets_file" || exit_code=$?

    # Clean up temporary files
    cleanup_temp_files "$secrets_file"

    if [[ $exit_code -eq 0 ]]; then
        echo -e "${GREEN}✓ Image builder workflow completed successfully${NC}"
    else
        echo -e "${RED}✗ Image builder workflow failed with exit code $exit_code${NC}"
    fi

    return $exit_code
}

# Run main function
main "$@"
