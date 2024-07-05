# Disable progress bar
$ProgressPreference = 'SilentlyContinue'
# Stop on all errors
$ErrorActionPreference = 'Stop'

function Install-Dependencies {
    Write-Output "Installing dependencies..."
    choco install -y git wsl2 -ErrorAction Stop

    Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Windows-Subsystem-Linux

    # # Ensure WSL and Virtual Machine Platform features are enabled
    # dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart
    # dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart

    # Restart to apply the changes
    # Restart-Computer -Force
    # Start-Sleep -Seconds 60  # Give some time for the restart

    # Install Ubuntu with wsl
    wsl --install -d Ubuntu

    if (wsl -l -q | Select-String -Pattern 'Ubuntu') {
        Write-Output "Ubuntu WSL distribution installed successfully."
        wsl -d Ubuntu -- bash -c "sudo apt update && sudo apt upgrade -y" -ErrorAction Stop
        wsl -d Ubuntu -- bash -c "sudo apt install -y python3-pip git libffi-dev libssl-dev" -ErrorAction Stop
        wsl -d Ubuntu -- bash -c "pip3 install --user ansible pywinrm" -ErrorAction Stop
    } else {
        Write-Error "Failed to install Ubuntu WSL distribution."
    }
}

function Set-AnsibleConfiguration {
    Write-Output "Configuring Ansible..."

    if (wsl -l -q | Select-String -Pattern 'Ubuntu') {
        wsl -d Ubuntu -- bash -c "{
            echo '[defaults]' > ~/.ansible.cfg
            echo 'inventory = ~/.ansible-hosts' >> ~/.ansible.cfg
            echo 'localhost ansible_connection=local' > ~/.ansible-hosts
            echo '---' > ~/test.yml
            echo '- name: talk to localhost' >> ~/test.yml
            echo '  hosts: localhost' >> ~/test.yml
            echo '  connection: local' >> ~/test.yml
            echo '  gather_facts: no' >> ~/test.yml
            echo '  tasks:' >> ~/test.yml
            echo '    - name: Print Hello from Ansible' >> ~/test.yml
            echo '      debug: msg=""Hello from Ansible""' >> ~/test.yml
        }" -ErrorAction Stop
    } else {
        Write-Error "Ubuntu WSL distribution not found."
    }
}

function Invoke-AnsiblePlaybook {
    Write-Output "Running Ansible playbook..."

    if (wsl -l -q | Select-String -Pattern 'Ubuntu') {
        wsl -d Ubuntu -- bash -c "ansible-playbook ~/test.yml" -ErrorAction Stop
    } else {
        Write-Error "Ubuntu WSL distribution not found."
    }
}

# Run the installation function
Install-Dependencies
