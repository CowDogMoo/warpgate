# Disable progress bar
$ProgressPreference = 'SilentlyContinue'
# Stop on all errors
$ErrorActionPreference = 'Stop'

# Parameters
param (
    [string]$DistroPath = 'C:\\Program Files\\WindowsApps',
    [string]$DistroName = 'ubuntu',
    [string]$DistroVersion = '2404',
    [string]$DistroAppx = 'CanonicalGroupLimited.Ubuntu24.04onWindows'
)

# Check if WSL distribution is installed
try {
    $DistroPackage = Get-AppxPackage -Name $DistroAppx -ErrorAction Stop
} catch {
    throw 'Failed to get AppxPackage for $DistroAppx. Ensure the WSL distribution is installed correctly.'
}

if ($DistroPackage) {
    # Configure WSL distribution
    Write-Host 'Configuring $DistroName ($DistroVersion)...'
    try {
        Invoke-Expression "& '$DistroPath\\$DistroAppx\\$DistroName$DistroVersion.exe' install --root"
        Write-Host 'Launching $DistroName ($DistroVersion)...'
        Start-Process -FilePath "$DistroPath\\$DistroAppx\\$DistroName$DistroVersion.exe" -ErrorAction Stop
    } catch {
        throw 'Failed to configure or launch WSL distribution.'
    }
}

# Schedule the next script to run on startup
$Action = New-ScheduledTaskAction -Execute 'PowerShell.exe' -Argument '-ExecutionPolicy Bypass -File \"C:\\PersistedData\\InstallDependencies.ps1\"'
$Trigger = New-ScheduledTaskTrigger -AtStartup
$Principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -Hidden -DontStopOnIdleEnd
Register-ScheduledTask -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -TaskName 'InstallDependencies'

# Restart the computer to complete the installation.
Write-Host 'Restarting the computer to complete the installation...'
Restart-Computer -Force
"@

Set-Content -Path "C:\PersistedData\ConfigureWSL.ps1" -Value $content3 -Encoding utf8
