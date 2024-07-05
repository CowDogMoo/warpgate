# Disable progress bar
$ProgressPreference = 'SilentlyContinue'
# Stop on all errors
$ErrorActionPreference = 'Stop'

function Remove-SSHKeys {
    Write-Output "Cleaning up SSH keys..."
    $openSSHAuthorizedKeys = Join-Path -Path $env:ProgramData -ChildPath "ssh/administrators_authorized_keys"
    if (Test-Path $openSSHAuthorizedKeys) {
        Remove-Item -Path $openSSHAuthorizedKeys -Recurse -Force -ErrorAction Stop
    } else {
        Write-Output "SSH keys path does not exist, skipping cleanup."
    }
}

function Enable-DownloadKeyTask {
    Write-Output "Checking for DownloadKey task..."
    if (Get-ScheduledTask -TaskName "DownloadKey" -ErrorAction SilentlyContinue) {
        Enable-ScheduledTask -TaskName "DownloadKey" -ErrorAction Stop
        Write-Output "DownloadKey task enabled."
    } else {
        if ($env:SSH_INTERFACE -eq "session_manager") {
            Write-Output "Scheduled task 'DownloadKey' not found. Expected behavior when using Session Manager."
        } else {
            Write-Output "Scheduled task 'DownloadKey' not found."
        }
    }
}

function Initialize-Sysprep {
    Write-Output "Running Sysprep..."
    & "$Env:ProgramFiles\Amazon\EC2Launch\ec2launch.exe" sysprep -ErrorAction Stop
}

Remove-SSHKeys
Enable-DownloadKeyTask
# Initialize-Sysprep (Uncomment if needed)
