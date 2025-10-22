FROM python:3.11-slim

LABEL org.opencontainers.image.source="https://github.com/CowDogMoo/Walls-of-Excellence"
LABEL org.opencontainers.image.description="Brother Printer Health Monitor"

WORKDIR /app

# Install dependencies
RUN pip install --no-cache-dir requests urllib3

# Copy monitor script
COPY dockerfiles/printer-monitor/monitor.py /app/monitor.py
RUN chmod +x /app/monitor.py

# Run as non-root user
RUN useradd -m -u 1000 monitor && \
    chown -R monitor:monitor /app
USER monitor

ENTRYPOINT ["python3", "/app/monitor.py"]
