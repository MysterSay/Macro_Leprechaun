# Runs in a hidden STA Windows PowerShell process; no clipboard access.
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = New-Object System.Text.UTF8Encoding($false)
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
try {
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    $q = [Console]::In.ReadToEnd() | ConvertFrom-Json
    $element = [System.Windows.Automation.AutomationElement]::FocusedElement
    if ($null -eq $element) { throw 'No focused UI Automation element' }
    if ($element.Current.IsPassword) { throw 'Password fields are not supported' }
    $pattern = $null
    if (-not $element.TryGetCurrentPattern([System.Windows.Automation.TextPattern]::Pattern, [ref]$pattern)) {
        throw 'Focused field does not expose UI Automation TextPattern'
    }
    if ($pattern.SupportedTextSelection -eq [System.Windows.Automation.SupportedTextSelection]::None) { throw 'Field does not support text selection' }
    $ranges = $pattern.GetSelection()
    if ($ranges.Count -ne 1) { throw 'A single caret or selection is required' }
    $selection = $ranges[0]
    $start = [System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start
    $end = [System.Windows.Automation.Text.TextPatternRangeEndpoint]::End
    $doc = $pattern.DocumentRange
    $result = @{}
    switch ($q.action) {
        'read' {
            $value = $selection.GetText(16385)
            if ($value.Length -eq 0) { throw 'Select text before transforming it' }
            if ($value.Length -gt 16384) { throw 'Selection exceeds 16384 UTF-16 characters' }
            $result.text = $value
        }
        'replace' {
            $request = $q.value | ConvertFrom-Json
            $original = $selection.GetText(16385)
            if ($original -cne [string]$request.expected) { throw 'Selection changed before replacement' }
            if ($original -ceq [string]$request.replacement) { $result.text = 'done'; break }
            # Prefer a native value update when the provider exposes the exact field value.
            # Prefix and suffix are retained verbatim; only the selection changes.
            $prefix = $doc.Clone()
            $prefix.MoveEndpointByRange($end, $selection, $start)
            $suffix = $doc.Clone()
            $suffix.MoveEndpointByRange($start, $selection, $end)
            $before = $prefix.GetText(60001)
            $after = $suffix.GetText(60001)
            $valuePattern = $null
            $canSetValue = $element.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$valuePattern)
            if ($canSetValue -and -not $valuePattern.Current.IsReadOnly -and ($before.Length + $original.Length + $after.Length) -le 60000 -and $valuePattern.Current.Value -ceq ($before + $original + $after)) {
                $valuePattern.SetValue($before + [string]$request.replacement + $after)
                $result.text = 'done'
            } else {
                # Re-select the complete original range immediately before keyboard replacement.
                $selection.Select()
                $result.text = 'keyboard'
            }
        }
        'find' {
            if ([string]::IsNullOrEmpty($q.value)) { throw 'Search text is empty' }
            $caret = $selection.Clone()
            $caret.MoveEndpointByRange($start, $caret, $end)
            $scope = $doc.Clone()
            $found = $null
            $previous = $null
            $covering = $null
            # Walk in document order until the caret is reached. This also handles
            # a match containing the caret and overlapping occurrences.
            for ($count = 0; $count -lt 10000; $count++) {
                $candidate = $scope.FindText([string]$q.value, $false, $false)
                if ($null -eq $candidate) { break }
                if ($candidate.CompareEndpoints($start, $caret, $start) -ge 0) {
                    $found = $candidate
                    break
                }
                if ($candidate.CompareEndpoints($end, $caret, $start) -gt 0) {
                    if ($null -eq $covering) { $covering = $candidate }
                } else { $previous = $candidate }
                $scope.MoveEndpointByRange($start, $candidate, $start)
                if ($scope.MoveEndpointByUnit($start, [System.Windows.Automation.Text.TextUnit]::Character, 1) -eq 0) { break }
            }
            if ($count -eq 10000) { throw 'Search exceeds 10000 occurrences' }
            if ($null -ne $covering -and ($null -eq $found -or $found.CompareEndpoints($start, $caret, $start) -gt 0)) { $found = $covering }
            if ($null -eq $found) { $found = $previous }
            elseif ($null -ne $previous -and $found.CompareEndpoints($start, $caret, $start) -gt 0) {
                $leftGap = $doc.Clone()
                $leftGap.MoveEndpointByRange($start, $previous, $end)
                $leftGap.MoveEndpointByRange($end, $caret, $start)
                $rightGap = $doc.Clone()
                $rightGap.MoveEndpointByRange($start, $caret, $start)
                $rightGap.MoveEndpointByRange($end, $found, $start)
                if ($leftGap.GetText(-1).Length -lt $rightGap.GetText(-1).Length) { $found = $previous }
            }
            if ($null -eq $found) { throw 'Text not found in focused field' }
            $found.Select()
        }
        'select' {
            $range = $selection.Clone()
            switch ($q.value) {
                { $_ -eq '' -or $_ -eq 'all' } { $range = $doc.Clone() }
                'before' { $range = $doc.Clone(); $range.MoveEndpointByRange($end, $selection, $end) }
                'after' { $range = $doc.Clone(); $range.MoveEndpointByRange($start, $selection, $end) }
                'line' {
                    # Select logical line, bounded by actual CR/LF rather than screen wrapping.
                    $range.MoveEndpointByRange($start, $range, $end)
                    foreach ($direction in @(-1, 1)) {
                        $endpoint = $start
                        if ($direction -eq 1) { $endpoint = $end }
                        for ($i = 0; $i -lt 16384; $i++) {
                            $probe = $range.Clone()
                            $probe.MoveEndpointByRange($start, $range, $endpoint)
                            $probe.MoveEndpointByRange($end, $range, $endpoint)
                            if ($direction -eq -1) { $moved = $probe.MoveEndpointByUnit($start, [System.Windows.Automation.Text.TextUnit]::Character, -1) }
                            else { $moved = $probe.MoveEndpointByUnit($end, [System.Windows.Automation.Text.TextUnit]::Character, 1) }
                            if ($moved -eq 0) { break }
                            if ($probe.GetText(-1) -match '[\r\n]') { break }
                            $range.MoveEndpointByRange($endpoint, $probe, $endpoint)
                        }
                        if ($i -eq 16384) { throw 'Line exceeds scan limit' }
                    }
                }
                'sentence' {
                    $range.MoveEndpointByRange($start, $range, $end)
                    foreach ($direction in @(-1, 1)) {
                        $endpoint = $start
                        if ($direction -eq 1) { $endpoint = $end }
                        for ($i = 0; $i -lt 16384; $i++) {
                            $probe = $range.Clone()
                            $probe.MoveEndpointByRange($start, $range, $endpoint)
                            $probe.MoveEndpointByRange($end, $range, $endpoint)
                            if ($direction -eq -1) { $moved = $probe.MoveEndpointByUnit($start, [System.Windows.Automation.Text.TextUnit]::Character, -1) }
                            else { $moved = $probe.MoveEndpointByUnit($end, [System.Windows.Automation.Text.TextUnit]::Character, 1) }
                            if ($moved -eq 0) { break }
                            $char = $probe.GetText(-1)
                            if ($char -match '[.!?\r\n]') {
                                if ($direction -eq 1 -and $char -notmatch '[\r\n]') { $range.MoveEndpointByRange($end, $probe, $end) }
                                break
                            }
                            $range.MoveEndpointByRange($endpoint, $probe, $endpoint)
                        }
                        if ($i -eq 16384) { throw 'Sentence exceeds scan limit' }
                    }
                }
                default { throw 'Unknown selection mode' }
            }
            $range.Select()
        }
        default { throw 'Unknown UI Automation operation' }
    }
    # Do not replace a selection in a field that lost focus during the operation.
    if (-not [System.Windows.Automation.Automation]::Compare($element, [System.Windows.Automation.AutomationElement]::FocusedElement)) { throw 'Focused field changed during operation' }
    $result.ok = $true
    [Console]::WriteLine(($result | ConvertTo-Json -Compress))
} catch {
    [Console]::WriteLine((@{ok=$false; error=$_.Exception.Message} | ConvertTo-Json -Compress))
    exit 1
}
