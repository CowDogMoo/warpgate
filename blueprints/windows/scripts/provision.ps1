<#
.SYNOPSIS
    Installs the SMB server feature on a Windows machine.

.DESCRIPTION
    This script installs the SMB server feature using PowerShell, which allows the machine to share folders and files using the SMB protocol.

.EXAMPLE
    ./Install-SMBServer.ps1
#>

# Function to install SMB Server
function Install-SMBServer {
    # Check if SMB server feature is already installed
    $feature = Get-WindowsFeature -Name FS-SMB1

    if ($feature.Installed -eq $false) {
        Write-Output "SMB Server feature not found. Installing SMB Server feature..."

        # Install SMB Server feature
        Install-WindowsFeature -Name FS-SMB1 -IncludeAllSubFeature -IncludeManagementTools

        if ($? -eq $true) {
            Write-Output "SMB Server feature installed successfully."
        } else {
            Write-Error "Failed to install SMB Server feature."
        }
    } else {
        Write-Output "SMB Server feature is already installed."
    }
}

# Execute the function
Install-SMBServer
