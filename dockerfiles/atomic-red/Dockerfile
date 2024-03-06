# Base images used for this Dockerfile
# mcr.microsoft.com/powershell:mariner-2.0
# mcr.microsoft.com/powershell:mariner-2.0-arm64

# Use build arguments to define the base image for the current architecture
ARG IMG
FROM ${IMG} AS base

# Final stage: Use the selected base image and run the PowerShell logic
FROM base AS final

# Set PowerShell as the default shell
SHELL ["pwsh", "-Command"]

# Install PowerShellGet and the powershell-yaml module
RUN Install-Module -Name PowerShellGet -Repository PSGallery -Force -Verbose; \
    Install-Module -Name powershell-yaml -Repository PSGallery -Force -Verbose

# Run the specified PowerShell logic
RUN Write-Host "Downloading install-atomicredteam.ps1 script..." ; \
    IEX (IWR 'https://raw.githubusercontent.com/redcanaryco/invoke-atomicredteam/master/install-atomicredteam.ps1' -UseBasicParsing) ; \
    Write-Host "Script downloaded. Installing Atomic Red Team..." ; \
    Install-AtomicRedTeam -getAtomics -Verbose ; \
    New-Item $PROFILE -Force ; \
    Write-Host "Installation complete."

COPY ./setup.ps1 .
RUN ./setup.ps1
