# asdf Base Image Examples

This directory contains example Dockerfiles showing different patterns for
creating custom images based on `ghcr.io/cowdogmoo/asdf:latest`.

## Examples

### 1. Explicit Tool Installation (`Dockerfile.python-golang`)

Install specific tool versions directly in the Dockerfile:

```dockerfile
FROM ghcr.io/cowdogmoo/asdf:latest

USER root
# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    rm -rf /var/lib/apt/lists/*

USER asdf
RUN asdf plugin add python && \
    asdf install python 3.14.0 && \
    asdf global python 3.14.0

RUN asdf plugin add golang && \
    asdf install golang 1.25.4 && \
    asdf global golang 1.25.4
```

**Build:**

```bash
cd examples
docker build -f Dockerfile.python-golang -t myorg/python-golang:latest .
```

**Use:**

```bash
docker run -it myorg/python-golang:latest
python --version  # 3.14.0
go version        # 1.25.4
```

### 2. .tool-versions File (`Dockerfile.tool-versions`)

Use a `.tool-versions` file for declarative version management (recommended):

**Create `.tool-versions` in your project:**

```text
python 3.14.0
golang 1.25.4
```

**Create Dockerfile:**

```dockerfile
FROM ghcr.io/cowdogmoo/asdf:latest

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    rm -rf /var/lib/apt/lists/*

USER asdf
COPY --chown=asdf:asdf .tool-versions /workspace/.tool-versions

RUN while IFS= read -r line; do \
      plugin=$(echo "$line" | awk '{print $1}'); \
      asdf plugin add "$plugin" || true; \
    done < .tool-versions && \
    asdf install
```

**Build:**

```bash
cd examples
docker build -f Dockerfile.tool-versions -t myorg/myapp:latest .
```

## Common Build Dependencies

Different tools require different system packages:

### Python

```dockerfile
RUN apt-get install -y --no-install-recommends \
    build-essential libssl-dev zlib1g-dev \
    libbz2-dev libreadline-dev libsqlite3-dev \
    libffi-dev liblzma-dev
```

### Go

```dockerfile
# Go typically doesn't need extra dependencies
# asdf will download pre-compiled binaries
```

### Node.js

```dockerfile
RUN apt-get install -y --no-install-recommends \
    build-essential
```

### Ruby

```dockerfile
RUN apt-get install -y --no-install-recommends \
    build-essential libssl-dev libreadline-dev \
    zlib1g-dev libyaml-dev
```

## Best Practices

1. **Use .tool-versions**: Keep tool versions in a `.tool-versions` file for
   consistency across local dev and CI/CD
2. **Install build deps as root**: Switch to root for apt-get, then back to
   asdf user
3. **Clean up apt lists**: Always `rm -rf /var/lib/apt/lists/*` after apt-get
4. **Use --no-install-recommends**: Minimize image size
5. **Verify installations**: Run `asdf current` or test tool commands
6. **Layer optimization**: Group related RUN commands to reduce layers

## Multi-Stage Build Pattern

For even smaller images, use multi-stage builds:

```dockerfile
# Stage 1: Build environment
FROM ghcr.io/cowdogmoo/asdf:latest AS builder

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    rm -rf /var/lib/apt/lists/*

USER asdf
COPY --chown=asdf:asdf .tool-versions /workspace/.tool-versions
RUN while IFS= read -r line; do \
      plugin=$(echo "$line" | awk '{print $1}'); \
      asdf plugin add "$plugin" || true; \
    done < .tool-versions && \
    asdf install

# Build your application here
COPY --chown=asdf:asdf . /workspace
RUN make build

# Stage 2: Minimal runtime
FROM ghcr.io/cowdogmoo/asdf:latest

USER asdf
COPY --from=builder /workspace/.tool-versions /workspace/.tool-versions
COPY --from=builder /home/asdf/.asdf /home/asdf/.asdf
COPY --from=builder /workspace/dist /workspace/dist

CMD ["/workspace/dist/app"]
```

## Testing Your Image

```bash
# Build
docker build -t test:latest .

# Run interactively
docker run -it --rm test:latest

# Check installed versions
docker run --rm test:latest bash -c "asdf current"

# Check image size
docker images test:latest

# Test specific tool
docker run --rm test:latest python --version
```
