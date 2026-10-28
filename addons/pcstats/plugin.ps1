# MacroLeprechaun API 2. Edit this UTF-8 file directly; no build step.
# stdin: one JSON request. stdout: one JSON reply. Diagnostics belong on stderr.
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$values = @{}
function Requested($name) {
    if ($null -eq $request.requestedVariables) { return $true }
    return $request.requestedVariables -contains $name
}

# Each flag is false unless enabled in settings.html and saved in the host.
# Windows performance classes are queried only for enabled metrics.
if ($request.settings.cpu -and (Requested 'cpu')) {
    try {
        $row = Get-CimInstance Win32_PerfFormattedData_PerfOS_Processor -Filter "Name='_Total'" -ErrorAction Stop
        if ($null -eq $row) { throw 'CPU counter unavailable' }
        $values.cpu = ([Math]::Round([double]$row.PercentProcessorTime, 1)).ToString([Globalization.CultureInfo]::InvariantCulture)
    } catch { $values.cpu = 'unavailable' }
}
if ($request.settings.ram -and (Requested 'ram')) {
    try {
        $memory = Get-CimInstance Win32_OperatingSystem -ErrorAction Stop
        if ([double]$memory.TotalVisibleMemorySize -le 0) { throw 'Memory information unavailable' }
        $percent = 100 * (1 - [double]$memory.FreePhysicalMemory / [double]$memory.TotalVisibleMemorySize)
        $values.ram = ([Math]::Round($percent, 1)).ToString([Globalization.CultureInfo]::InvariantCulture)
    } catch { $values.ram = 'unavailable' }
}
if ($request.settings.gpu -and (Requested 'gpu')) {
    try {
        $rows = @(Get-CimInstance Win32_PerfFormattedData_GPUPerformanceCounters_GPUEngine -ErrorAction Stop)
        if ($rows.Count -eq 0) { throw 'GPU counters unavailable' }
        # Aggregate the busiest engine type. Drivers without these counters
        # return unavailable; do not replace missing information with zero.
        $maximum = 0.0
        foreach ($group in ($rows | Group-Object { ($_.Name -split '_engtype_')[-1] })) {
            $sum = ($group.Group | Measure-Object UtilizationPercentage -Sum).Sum
            if ($sum -gt $maximum) { $maximum = [double]$sum }
        }
        $values.gpu = ([Math]::Round([Math]::Min(100, $maximum), 1)).ToString([Globalization.CultureInfo]::InvariantCulture)
    } catch { $values.gpu = 'unavailable' }
}
if ($request.settings.keyboard -and (Requested 'keyboard')) {
    # The host supplies the layout of the foreground window, not this hidden
    # PowerShell process. No C# compiler or native addon DLL is required.
    $values.keyboard = [string]$request.host.keyboardLayout
    if (-not $values.keyboard) { $values.keyboard = 'unavailable' }
}
@{ ok = $true; variables = $values } | ConvertTo-Json -Compress -Depth 10
