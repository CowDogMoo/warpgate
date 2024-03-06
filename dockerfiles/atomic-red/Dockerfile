# syntax=docker/dockerfile:1.4

# Define base images for different architectures
FROM --platform=linux/amd64 mcr.microsoft.com/powershell:preview-7.5-mariner-2.0 AS build-amd64
FROM --platform=linux/arm64 mcr.microsoft.com/powershell:preview-7.5-mariner-2.0-arm64 AS build-arm64

# Use the BUILDARCH build argument to select the appropriate base image
ARG BUILDARCH
FROM build-$BUILDARCH AS base

# Final stage: Use the selected base image and run the PowerShell logic
FROM base AS final

# Set PowerShell as the default shell
SHELL ["pwsh", "-Command"]

# Run the specified PowerShell logic
RUN Write-Host "Downloading install-atomicredteam.ps1 script..." ; \
    IEX (IWR 'https://raw.githubusercontent.com/redcanaryco/invoke-atomicredteam/master/install-atomicredteam.ps1' -UseBasicParsing) ; \
    Write-Host "Script downloaded. Installing Atomic Red Team..." ; \
    Install-AtomicRedTeam -getAtomics -Verbose -Force; \
    New-Item $PROFILE -Force ; \
    Write-Host "Installation complete."

COPY ./setup.ps1 .
RUN ./setup.ps1
