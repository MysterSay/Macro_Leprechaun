$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force build | Out-Null
Push-Location desktop
try {
    go test ./...; if ($LASTEXITCODE) { throw 'Tests failed' }
    go build -buildvcs=false -trimpath -ldflags '-s -w -H windowsgui' -o ../build/MacroLeprechaun.exe .
    if ($LASTEXITCODE) { throw 'Build failed' }
} finally { Pop-Location }
# Addons are source files. No compiler is used for them. Existing edited addon
# files in build/data are preserved; copy updated examples explicitly if needed.
foreach ($addon in @('pcstats', 'notifications')) {
    $target = Join-Path 'build/data/plagin' $addon
    New-Item -ItemType Directory -Force $target | Out-Null
    Get-ChildItem -File (Join-Path 'addons' $addon) | ForEach-Object {
        $destination = Join-Path $target $_.Name
        if (-not (Test-Path -LiteralPath $destination)) { Copy-Item -LiteralPath $_.FullName -Destination $destination }
    }
}
Write-Host 'Windows: build/MacroLeprechaun.exe'
Write-Host 'Addons: edit data/plagin/<id>/plugin.ps1 directly; no build step.'
Write-Host 'Android: gradle assembleDebug with JDK 17 and Android SDK 35.'
