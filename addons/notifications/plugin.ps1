# MacroLeprechaun API 2: the puzzle value comes from custom.html/windows.html.
# The UI worker is independent: returning from this action does not close it.
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
if ($request.op -ne 'execute') {
    @{ ok = $true } | ConvertTo-Json -Compress
    return
}
if ($request.block -notin @('custom', 'windows')) { throw 'Unknown notification block' }

$directory = $env:MACROLEPRECHAUN_ADDON_DIR
$workerPath = Join-Path $directory 'window.ps1'
$payload = @{ kind = $request.block; value = $request.value } | ConvertTo-Json -Compress -Depth 20
$encodedPayload = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($payload))
$quotedWorker = "'" + $workerPath.Replace("'", "''") + "'"
$quotedLog = "'" + (Join-Path $directory 'notification-error.log').Replace("'", "''") + "'"
# Only a short UTF-16 launcher goes on the command line. The actual window
# code remains readable in window.ps1 and is loaded again for every notice.
$bootstrap = @"
`$ErrorActionPreference = 'Stop'
try {
    `$notice = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('$encodedPayload')) | ConvertFrom-Json
    & ([scriptblock]::Create([IO.File]::ReadAllText($quotedWorker, [Text.Encoding]::UTF8))) `$notice
} catch {
    [IO.File]::AppendAllText($quotedLog, ([DateTime]::Now.ToString('s') + ' ' + `$_.ToString() + [Environment]::NewLine), [Text.Encoding]::UTF8)
}
"@
$encodedCommand = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($bootstrap))
$interpreter = (Get-Process -Id $PID).Path
Start-Process -FilePath $interpreter -WindowStyle Hidden -ArgumentList @(
    '-NoLogo', '-NoProfile', '-NonInteractive', '-STA', '-ExecutionPolicy', 'Bypass',
    '-EncodedCommand', $encodedCommand
) -ErrorAction Stop
@{ ok = $true } | ConvertTo-Json -Compress
