$ErrorActionPreference = 'Stop'

# Install Chocolatey
# See https://chocolatey.org/install#individual
Set-ExecutionPolicy Bypass -Scope Process -Force
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Globally Auto confirm every action
# See: https://docs.chocolatey.org/en-us/faqs#why-do-i-have-to-confirm-packages-now-is-there-a-way-to-remove-this
choco feature enable -n allowGlobalConfirmation
