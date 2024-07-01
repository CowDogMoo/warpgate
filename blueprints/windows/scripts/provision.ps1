$ErrorActionPreference = 'Stop'

function Install-Chocolatey {
    Write-Output "Installing Chocolatey..."
    Set-ExecutionPolicy Bypass -Scope Process -Force
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
    Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
    choco feature enable -n allowGlobalConfirmation
}

function Cleanup-SSHKeys {
    Write-Output "Cleaning up SSH keys..."
    $openSSHAuthorizedKeys = Join-Path $env:ProgramData 'ssh\administrators_authorized_keys'
    Remove-Item -Recurse -Force -Path $openSSHAuthorizedKeys
}

function Enable-DownloadKeyTask {
    Write-Output "Checking for DownloadKey task..."
    if (Get-ScheduledTask -TaskName "DownloadKey" -ErrorAction SilentlyContinue) {
        Enable-ScheduledTask "DownloadKey"
        Write-Output "DownloadKey task enabled."
    } else {
        if ($env:SSH_INTERFACE -eq "session_manager") {
            Write-Output "Scheduled task 'DownloadKey' not found. Expected behavior when using Session Manager."
        } else {
            Write-Output "Scheduled task 'DownloadKey' not found."
        }
    }
}

function Run-Sysprep {
    Write-Output "Running Sysprep..."
    & "$Env:Programfiles\Amazon\EC2Launch\ec2launch.exe" sysprep
}

# Main script execution
Install-Chocolatey
Cleanup-SSHKeys
Enable-DownloadKeyTask
Run-Sysprep
