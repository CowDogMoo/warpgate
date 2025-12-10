# syntax=docker/dockerfile:1.20

# Build stage - compile the binary
FROM golang:1.25-bookworm AS builder

# Install build dependencies for CGO
RUN apt-get update && apt-get install -y \
    git \
    pkg-config \
    libgpgme-dev \
    libassuan-dev \
    libbtrfs-dev \
    libdevmapper-dev \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Set Go environment
ENV GOTOOLCHAIN=auto \
    CGO_ENABLED=1

WORKDIR /build

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build warpgate binary (stripped)
RUN go build -trimpath -ldflags="-s -w" -o warpgate ./cmd/warpgate

# Runtime stage - minimal image with only runtime dependencies
FROM debian:bookworm-slim

# Metadata labels
LABEL org.opencontainers.image.title="warpgate"
LABEL org.opencontainers.image.description="Container image builder and template manager"
LABEL org.opencontainers.image.vendor="CowDogMoo"
LABEL org.opencontainers.image.source="https://github.com/cowdogmoo/warpgate"

# Install runtime dependencies for BuildKit client, storage libraries, and Ansible
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libgpgme11 \
    libassuan0 \
    libdevmapper1.02.1 \
    ansible \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Copy the compiled binary from builder
COPY --from=builder /build/warpgate /usr/local/bin/warpgate
RUN chmod +x /usr/local/bin/warpgate

# Create warpgate config directory
RUN mkdir -p /root/.warpgate

# Configure containers to use crun as the OCI runtime
RUN mkdir -p /etc/containers && \
    cat <<'EOF' > /etc/containers/containers.conf
[engine]
runtime = "crun"
helper_binaries_dir = ["/usr/libexec/podman", "/usr/lib/podman"]

[engine.runtimes]
crun = [
  "/usr/bin/crun"
]
EOF

# Use vfs storage driver to work around Docker Desktop 4.33+ DinD regression on Mac
# See: https://github.com/docker/for-mac/issues/7413
# On Linux, use overlay for better performance by setting STORAGE_DRIVER=overlay
ARG STORAGE_DRIVER=vfs
RUN cat <<EOF > /etc/containers/storage.conf
[storage]
driver = "${STORAGE_DRIVER}"
EOF

# Set working directory
WORKDIR /workspace

# Health check to verify warpgate binary is accessible
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD warpgate --version || exit 1

ENTRYPOINT ["warpgate"]
CMD ["--help"]
