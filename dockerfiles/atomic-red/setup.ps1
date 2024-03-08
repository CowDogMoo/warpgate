if ($isWindows) {
    Set-Variable -Name "ARTPath" -Value "C:\AtomicRedTeam"
}
else {
    Set-Variable -Name "ARTPath" -Value "$HOME/AtomicRedTeam"
}

Write-Output @"
Import-Module "$ARTPath/invoke-atomicredteam/Invoke-AtomicRedTeam.psd1" -Force;
`$PSDefaultParameterValues`["Invoke-AtomicTest:PathToAtomicsFolder"] = "$ARTPath/atomics";
`$PSDefaultParameterValues`["Invoke-AtomicTest:ExecutionLogPath"]="1.csv";
"@ > $PROFILE
