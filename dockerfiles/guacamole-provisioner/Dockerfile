FROM python:3.14-alpine

# Install dependencies at build time for faster startup
RUN apk add --no-cache \
    postgresql-dev \
    gcc \
    musl-dev \
    inotify-tools && \
    pip install --no-cache-dir \
    pyyaml==6.0.1 \
    psycopg2-binary==2.9.9 \
    pyinotify==0.9.6 && \
    apk del gcc musl-dev

# Copy the provisioner script
COPY provision_connections.py /usr/local/bin/provision_connections.py
RUN chmod +x /usr/local/bin/provision_connections.py

# Run as non-root user
RUN adduser -D -u 1000 provisioner
USER provisioner

ENTRYPOINT ["python", "-u", "/usr/local/bin/provision_connections.py"]
