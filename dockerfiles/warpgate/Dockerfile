# syntax=docker/dockerfile:1.20

# Build stage - compile the binary
FROM quay.io/buildah/stable:latest AS builder

# Install ONLY build dependencies
RUN dnf -y install \
    golang \
    git \
    gpgme-devel \
    libassuan-devel \
    device-mapper-devel \
    btrfs-progs-devel \
    pkg-config \
    gcc \
    && dnf -y clean all

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
FROM quay.io/buildah/stable:latest

# Metadata labels
LABEL org.opencontainers.image.title="warpgate"
LABEL org.opencontainers.image.description="Container image builder and template manager"
LABEL org.opencontainers.image.vendor="CowDogMoo"
LABEL org.opencontainers.image.source="https://github.com/cowdogmoo/warpgate"

# Install ONLY runtime dependencies (ansible, openssh-clients)
RUN dnf -y install \
    ansible \
    openssh-clients \
    && dnf -y clean all

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
