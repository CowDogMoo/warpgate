# Disable progress bar
$ProgressPreference = 'SilentlyContinue'
# Stop on all errors
$ErrorActionPreference = 'Stop'

# Parameters
param(
    [string]$DistroPath = "C:\Program Files\WindowsApps",
    [string]$DistroCachePath = "C:\PersistedData",
    [string]$DistroName = "ubuntu",
    [string]$DistroVersion = "2404",
    [string]$DistroAppx = "CanonicalGroupLimited.Ubuntu24.04onWindows"
)

# Ensure required parameters are not null or empty
if ([string]::IsNullOrEmpty($DistroPath) -or
    [string]::IsNullOrEmpty($DistroCachePath) -or
    [string]::IsNullOrEmpty($DistroName) -or
    [string]::IsNullOrEmpty($DistroVersion) -or
    [string]::IsNullOrEmpty($DistroAppx)) {
    throw "One or more required parameters are null or empty."
}

# Create the directory if it does not exist
if (-not (Test-Path -Path $DistroCachePath)) {
    New-Item -Path $DistroCachePath -ItemType Directory -Force
}

$DistroUrl = "https://aka.ms/wsl-$DistroName-$DistroVersion"
$DistroFilename = Join-Path -Path $DistroCachePath -ChildPath "$DistroName-$DistroVersion.appx"

# Download WSL distribution
Write-Host "Downloading $DistroName ($DistroVersion)..."
Invoke-WebRequest -Uri $DistroUrl -OutFile $DistroFilename -UseBasicParsing -ErrorAction Stop

# Install WSL distribution
Write-Host "Installing $DistroName ($DistroVersion)..."
Add-AppxPackage -Path $DistroFilename -ErrorAction Stop

# Schedule the next script to run on startup
$Action = New-ScheduledTaskAction -Execute "PowerShell.exe" -Argument "-ExecutionPolicy Bypass -File 'C:\PersistedData\ConfigureWSL.ps1'"
$Trigger = New-ScheduledTaskTrigger -AtStartup
$Principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -Hidden -DontStopOnIdleEnd
Register-ScheduledTask -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -TaskName "ConfigureWSL"

# Restart the computer to complete the installation.
Write-Host "Restarting the computer to complete the installation..."
Restart-Computer -Force
