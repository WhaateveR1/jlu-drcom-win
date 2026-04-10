$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
$package = Join-Path $dist "jlu-drcom-win"
$staging = Join-Path $dist ("jlu-drcom-win-staging-" + [guid]::NewGuid().ToString("N"))
$cliExe = Join-Path $staging "drcom-win.exe"
$trayExe = Join-Path $staging "drcom-tray.exe"
$zip = Join-Path $dist "jlu-drcom-win.zip"

New-Item -ItemType Directory -Force -Path $dist | Out-Null
New-Item -ItemType Directory -Force -Path $staging | Out-Null
if (Test-Path $zip) {
    Remove-Item -LiteralPath $zip -Force
}

Push-Location $root
try {
    go test ./...
    go build -trimpath -ldflags "-s -w" -o $cliExe .\cmd\drcom-win
    go build -trimpath -ldflags "-H=windowsgui -s -w" -o $trayExe .\cmd\drcom-tray
}
finally {
    Pop-Location
}

Copy-Item -LiteralPath (Join-Path $root "config.example.toml") -Destination (Join-Path $staging "config.example.toml")
Copy-Item -LiteralPath (Join-Path $root "README.md") -Destination (Join-Path $staging "README.md")
Copy-Item -LiteralPath (Join-Path $root "USER_GUIDE.md") -Destination (Join-Path $staging "USER_GUIDE.md")
Copy-Item -LiteralPath (Join-Path $root "LICENSE") -Destination (Join-Path $staging "LICENSE")
Copy-Item -LiteralPath (Join-Path $root "NOTICE.md") -Destination (Join-Path $staging "NOTICE.md")
Copy-Item -LiteralPath (Join-Path $root "CHANGELOG.md") -Destination (Join-Path $staging "CHANGELOG.md")

Compress-Archive -Path (Join-Path $staging "*") -DestinationPath $zip -Force

New-Item -ItemType Directory -Force -Path $package | Out-Null
$allowedPackageFiles = @(
    "drcom-win.exe",
    "drcom-tray.exe",
    "config.example.toml",
    "README.md",
    "USER_GUIDE.md",
    "LICENSE",
    "NOTICE.md",
    "CHANGELOG.md"
)
foreach ($existing in Get-ChildItem -LiteralPath $package -File) {
    if ($allowedPackageFiles -notcontains $existing.Name) {
        try {
            Remove-Item -LiteralPath $existing.FullName -Force
        }
        catch {
            Write-Warning "Could not remove stale file $($existing.Name) from $package. $($_.Exception.Message)"
        }
    }
}
foreach ($item in Get-ChildItem -LiteralPath $staging) {
    try {
        Copy-Item -LiteralPath $item.FullName -Destination (Join-Path $package $item.Name) -Force
    }
    catch {
        Write-Warning "Could not refresh $($item.Name) in $package. The file may be running. Zip package is still up to date. $($_.Exception.Message)"
    }
}

Remove-Item -LiteralPath $staging -Recurse -Force

Write-Output "Refreshed $package when files were not in use"
Write-Output "Packed $zip"
