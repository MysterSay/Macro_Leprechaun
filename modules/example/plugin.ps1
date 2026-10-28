# Copy this folder to data/plagin/example, reload addons, enable it and save.
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$message = [string]$request.variables.example__message
if ($request.op -eq 'execute') {
    switch ($request.block) {
        'text'   { $message = [string]$request.value }
        'choice' { $message = [string]$request.value }
        'dialog' { $message = [string]$request.value.text }
        default  { throw 'Unknown puzzle' }
    }
}
# Return short IDs only; the host creates example__clock/example__message.
@{ ok = $true; variables = @{
    clock = (Get-Date -Format 'HH:mm:ss')
    message = $message
} } | ConvertTo-Json -Compress -Depth 10
