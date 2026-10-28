# Run once in an elevated PowerShell. Private networks only.
$ErrorActionPreference = 'Stop'
$program = Join-Path $PSScriptRoot 'MacroLeprechaun.exe'
New-NetFirewallRule -DisplayName 'MacroLeprechaun TCP' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 47640 -Profile Private -Program $program
New-NetFirewallRule -DisplayName 'MacroLeprechaun discovery' -Direction Inbound -Action Allow -Protocol UDP -LocalPort 47641,5353 -Profile Private -Program $program
