# Guacamole Connection Provisioner

Optimized sidecar container for dynamically provisioning Apache Guacamole connections from Kubernetes secrets.

## Features

### Performance Optimizations

1. **Pre-installed Dependencies**: All Python packages are installed at build time, reducing startup from ~40-60s to ~5-10s
2. **Connection Pooling**: Reuses database connections instead of creating new ones every cycle
3. **Batch Operations**: Uses `psycopg2.extras.execute_batch` for efficient bulk inserts
4. **State Caching**: Computes file hashes to detect changes and skip unnecessary updates
5. **Smart Updates**: Compares database state with new config to avoid redundant writes
6. **Configurable**: All settings can be tuned via environment variables
7. **Graceful Shutdown**: Properly handles SIGTERM and SIGINT signals
8. **Non-root**: Runs as unprivileged user for security

## Building

```bash
# Build the image
docker build -t guacamole-provisioner:latest .

# Build with custom tag
docker build -t ghcr.io/cowdogmoo/guacamole-provisioner:v1.0.0 .

# Multi-platform build
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ghcr.io/cowdogmoo/guacamole-provisioner:latest .
```

## Configuration

Environment variables for tuning performance:

| Variable | Default | Description |
|----------|---------|-------------|
| `CONNECTIONS_DIR` | `/connections` | Directory containing connection YAML files |
| `CONFIG_DIR` | `/config` | Directory containing guacamole.properties |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_NAME` | `guacamole_db` | Database name |
| `DB_USER` | `guacamole` | Database user |
| `STARTUP_DELAY` | `10` | Seconds to wait for Guacamole startup |
| `POLL_INTERVAL` | `300` | Seconds between file checks (5 minutes) |
| `BATCH_SIZE` | `100` | Number of records per batch insert |

## Connection File Format

Place YAML files in subdirectories under `/connections`:

```yaml
name: "my-connection"
protocol: "rdp"
parameters:
  hostname: "192.168.1.100"
  port: "3389"
  username: "admin"
  password: "secret"
  security: "any"
  ignore-cert: "true"
users:
  - "user1"
  - "user2"
permissions:
  - "READ"
```

## Performance Comparison

### Before Optimization
- Startup time: 40-60 seconds
- Poll interval: Fixed 5 minutes
- Database operations: 1 query per parameter/permission
- Change detection: None (always updates)
- Connection handling: New connection every cycle
- Memory: ~50-100MB during pip install

### After Optimization
- Startup time: 5-10 seconds
- Poll interval: Configurable
- Database operations: Batched (up to 100 per query)
- Change detection: SHA256 file hashing
- Connection handling: Pooled (1-5 connections)
- Memory: ~30-50MB steady state

## Usage in Kubernetes

```yaml
containers:
  - name: connection-provisioner
    image: ghcr.io/cowdogmoo/guacamole-provisioner:latest
    env:
      - name: STARTUP_DELAY
        value: "15"
      - name: POLL_INTERVAL
        value: "180"  # Check every 3 minutes
    volumeMounts:
      - name: connection-secrets
        mountPath: /connections
        readOnly: true
      - name: guacamole-config
        mountPath: /config
        readOnly: true
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        memory: 128Mi
```

## Security

- Runs as non-root user (UID 1000)
- Read-only volume mounts recommended
- Database password read from guacamole.properties
- No secrets in environment variables or logs
