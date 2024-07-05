# Disable progress bar
$ProgressPreference = 'SilentlyContinue'
# Stop on all errors
$ErrorActionPreference = 'Stop'

# Function to handle feature enabling
function Enable-Feature {
    param (
        [string]$FeatureName
    )
    Write-Host "Enabling $FeatureName..."
    Enable-WindowsOptionalFeature -Online -FeatureName $FeatureName -All -NoRestart -ErrorAction Stop
}

# Enable Hyper-V and WSL
Enable-Feature -FeatureName "Microsoft-Hyper-V"
Enable-Feature -FeatureName "Microsoft-Windows-Subsystem-Linux"

# Create the restart flag file
$RestartFlagFile = "C:\PersistedData\restart_flag.txt"
if (-not (Test-Path $RestartFlagFile)) {
    New-Item -Path $RestartFlagFile -ItemType File -Force
}

# Schedule the next script to run on startup
$Action = New-ScheduledTaskAction -Execute "PowerShell.exe" -Argument "-ExecutionPolicy Bypass -File 'C:\PersistedData\DownloadAndInstallWSL.ps1'"
$Trigger = New-ScheduledTaskTrigger -AtStartup
$Principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -Hidden -DontStopOnIdleEnd
Register-ScheduledTask -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -TaskName "DownloadAndInstallWSL"

# Restart the computer to complete the installation.
Write-Host "Restarting the computer to complete the installation..."
Restart-Computer -Force
