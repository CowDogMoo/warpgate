#!/bin/bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
ARCH="arm64"
TARGET="container"
VERBOSE="--verbose"

usage() {
    cat <<EOF
Usage: $0 <template> [options]

Build a Warpgate template using Docker.

TEMPLATES:
    asdf        Build the asdf template with workstation collection
    sliver      Build the sliver template with arsenal collection

OPTIONS:
    -a, --arch ARCH         Architecture (default: arm64)
    -t, --target TARGET     Build target (default: container)
    -v, --verbose           Enable verbose output (default: enabled)
    -q, --quiet             Disable verbose output
    -h, --help              Show this help message

EXAMPLES:
    $0 asdf
    $0 sliver --arch amd64
    $0 asdf --target container --quiet

EOF
    exit 1
}

# Parse arguments
if [ $# -eq 0 ]; then
    echo -e "${RED}Error: No template specified${NC}"
    usage
fi

# Check for help flag first
if [ "$1" == "-h" ] || [ "$1" == "--help" ]; then
    usage
fi

TEMPLATE="$1"
shift

# Parse optional arguments
while [ $# -gt 0 ]; do
    case "$1" in
        -a|--arch)
            ARCH="$2"
            shift 2
            ;;
        -t|--target)
            TARGET="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE="--verbose"
            shift
            ;;
        -q|--quiet)
            VERBOSE=""
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo -e "${RED}Error: Unknown option $1${NC}"
            usage
            ;;
    esac
done

# Configure template-specific settings
case "$TEMPLATE" in
    asdf)
        TEMPLATE_PATH="$HOME/cowdogmoo/warpgate-templates/templates/asdf/warpgate.yaml"
        COLLECTION_PATH="$HOME/cowdogmoo/ansible-collection-workstation"
        COLLECTION_NAME="CowDogMoo/ansible-collection-workstation"
        ;;
    sliver)
        TEMPLATE_PATH="$HOME/cowdogmoo/warpgate-templates/templates/sliver/warpgate.yaml"
        COLLECTION_PATH="$HOME/ansible-collection-arsenal"
        COLLECTION_NAME="CowDogMoo/ansible-collection-arsenal"
        ;;
    *)
        echo -e "${RED}Error: Unknown template '$TEMPLATE'${NC}"
        echo "Valid templates: asdf, sliver"
        usage
        ;;
esac

# Check if warpgate:latest image exists, build if not
if ! docker images | grep -q "^warpgate.*latest"; then
    echo -e "${YELLOW}warpgate:latest image not found locally${NC}"
    echo -e "${YELLOW}Building warpgate:latest image using task...${NC}"
    echo ""
    task -y images:build IMAGE=warpgate
    echo ""
    echo -e "${GREEN}warpgate:latest image built successfully${NC}"
    echo ""
fi

# Build the docker command
echo -e "${GREEN}Building $TEMPLATE template...${NC}"
echo -e "${YELLOW}Template: $TEMPLATE_PATH${NC}"
echo -e "${YELLOW}Collection: $COLLECTION_NAME${NC}"
echo -e "${YELLOW}Architecture: $ARCH${NC}"
echo -e "${YELLOW}Target: $TARGET${NC}"
echo ""

docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$TEMPLATE_PATH:/workspace/warpgate.yaml:ro" \
    -v "$COLLECTION_PATH:/provision:ro" \
    -e ARSENAL_REPO_PATH=/provision \
    -e WORKSTATION_REPO_PATH=/provision \
    --privileged \
    --security-opt seccomp=unconfined \
    --security-opt apparmor=unconfined \
    warpgate:latest build /workspace/warpgate.yaml --arch "$ARCH" --target "$TARGET" $VERBOSE
