<#
.SYNOPSIS
    Configures the machine for SSH access and installs the SMB server feature.

.DESCRIPTION
    This script installs the SMB server feature using PowerShell, which allows the machine to share folders and files using the SMB protocol.
    Additionally, it sets up SSH for provisioning, including generating host keys, fetching public keys from instance metadata, and configuring the SSH service.

.EXAMPLE
    ./provision.ps1
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

# SSH Setup for Provisioning
$ProgressPreference = 'SilentlyContinue'
$ErrorActionPreference = 'Stop'

Start-Transcript -path ("C:\{0}.log" -f $MyInvocation.MyCommand.Name) -append

Push-Location C:\OpenSSH-Win64
.\ssh-keygen -A
.\ssh-add ssh_host_dsa_key
.\ssh-add ssh_host_rsa_key
.\ssh-add ssh_host_ecdsa_key
.\ssh-add ssh_host_ed25519_key
del *_key
Pop-Location

$keyPath = "C:\Users\Administrator\.ssh\authorized_keys"
$keyUrl = "http://169.254.169.254/latest/meta-data/public-keys/0/openssh-key"

New-Item -ErrorAction Ignore -Type Directory C:\Users\Administrator\.ssh > $null

$ErrorActionPreference = 'SilentlyContinue'
Do {
    Start-Sleep 1
    Write-Output ("{0:u}: Trying to fetch key from metadata service" -f (Get-Date))
    Invoke-WebRequest $keyUrl -UseBasicParsing -OutFile $keyPath
    Write-Output $Error[0]
} While ( -Not (Test-Path $keyPath) )
$ErrorActionPreference = 'Stop'
Write-Output ("{0:u}: Key successfully retrieved" -f (Get-Date))

Add-Type -AssemblyName System.Web
$password = [System.Web.Security.Membership]::GeneratePassword(19, 10).replace("&", "a").replace("<", "b").replace(">", "c")

$unattendPath = "$Env:ProgramData\Amazon\EC2-Windows\Launch\Sysprep\Unattend.xml"
$xml = [xml](Get-Content $unattendPath)
$targetElememt = $xml.unattend.settings.Where{($_.Pass -eq 'oobeSystem')}.component.Where{($_.name -eq 'Microsoft-Windows-Shell-Setup')}

$autoLogonElement = [xml]('<AutoLogon>
    <Password>
        <Value>{0}</Value>
        <PlainText>true</PlainText>
    </Password>
    <Enabled>true</Enabled>
    <Username>Administrator</Username>
</AutoLogon>' -f $password)
$targetElememt.appendchild($xml.ImportNode($autoLogonElement.DocumentElement, $true))

$userAccountElement = [xml]('<UserAccounts xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
    <AdministratorPassword>
        <Value>{0}</Value>
        <PlainText>true</PlainText>
    </AdministratorPassword>
    <LocalAccounts>
        <LocalAccount wcm:action="add">
            <Password>
                <Value>{0}</Value>
                <PlainText>true</PlainText>
            </Password>
            <Group>administrators</Group>
            <DisplayName>Administrator</DisplayName>
            <Name>Administrator</Name>
            <Description>Administrator User</Description>
        </LocalAccount>
    </LocalAccounts>
</UserAccounts>' -f $password)
$targetElememt.appendchild($xml.ImportNode($userAccountElement.DocumentElement, $true))

$xml.Save($unattendPath)

Add-Content $Env:ProgramData\Amazon\EC2-Windows\Launch\Sysprep\BeforeSysprep.cmd 'del "C:\Program Files\OpenSSH-Win64\*_key*"'
Add-Content $Env:ProgramData\Amazon\EC2-Windows\Launch\Sysprep\BeforeSysprep.cmd 'del C:\Users\Administrator\.ssh\authorized_keys'
Add-Content $Env:ProgramData\Amazon\EC2-Windows\Launch\Sysprep\BeforeSysprep.cmd 'del C:\provision.ps1'

Add-Content $Env:ProgramData\Amazon\EC2-Windows\Launch\Sysprep\SysprepSpecialize.cmd 'powershell -ExecutionPolicy Bypass -NoProfile -c "& C:\specialize-script.ps1"'

& $Env:ProgramData\Amazon\EC2-Windows\Launch\Scripts\InitializeInstance.ps1 -Schedule
& $Env:ProgramData\Amazon\EC2-Windows\Launch\Scripts\SysprepInstance.ps1

Stop-Transcript

# Execute the function
Install-SMBServer
