# Define base images for different architectures
ARG BASE_IMAGE_AMD64=mcr.microsoft.com/powershell:mariner-2.0
ARG BASE_IMAGE_ARM64=mcr.microsoft.com/powershell:mariner-2.0-arm64

# Use build arguments to define the base image for the current architecture
ARG ARCH
FROM ${ARCH} AS base

# Final stge: Use the selected base image and run the Powershell logic
FROM base AS final

# Set PowerShell as the default shell
SHELL ["pwsh", "-Command"]

# Run the specified PowerShell logic
RUN IEX (IWR 'https://raw.githubusercontent.com/redcanaryco/invoke-atomicredteam/master/install-atomicredteam.ps1' -UseBasicParsing); \
    Install-AtomicRedTeam -getAtomics
RUN New-Item $PROFILE -Force
COPY ./setup.ps1 .
RUN ./setup.ps1
