$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root 'dist'
$staging = Join-Path $dist ('staging-' + [guid]::NewGuid().ToString('N'))
$pendingZip = $staging + '.zip'
$previousZip = $staging + '.previous.zip'
$zip = Join-Path $dist 'jlu-drcom-win.zip'

New-Item -ItemType Directory -Path $staging -Force | Out-Null
Push-Location $root
try {
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed ($LASTEXITCODE)" }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed ($LASTEXITCODE)" }
    & go build -trimpath -ldflags '-H=windowsgui -s -w' -o (Join-Path $staging 'drcom-tray.exe') ./cmd/drcom-tray
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }

    # Explicit allowlist: never include a local account configuration or logs.
    foreach ($name in @('config.example.toml', 'README.md', 'USER_GUIDE.md', 'LICENSE', 'NOTICE.md', 'CHANGELOG.md')) {
        Copy-Item -LiteralPath (Join-Path $root $name) -Destination $staging
    }
    Compress-Archive -Path (Join-Path $staging '*') -DestinationPath $pendingZip
    if (Test-Path -LiteralPath $zip) {
        [IO.File]::Replace($pendingZip, $zip, $previousZip)
    } else {
        [IO.File]::Move($pendingZip, $zip)
    }
    Write-Output "Packed $zip"
    # Do not refresh or clean an extracted runtime directory: it may hold accounts.
}
finally {
    Pop-Location
    $expectedParent = [IO.Path]::GetFullPath($dist).TrimEnd('\')
    $resolved = [IO.Path]::GetFullPath($staging)
    if ([IO.Path]::GetDirectoryName($resolved) -ne $expectedParent -or [IO.Path]::GetFileName($resolved) -notmatch '^staging-[a-f0-9]{32}$') {
        throw 'Refusing to clean an unexpected staging path'
    }
    if (Test-Path -LiteralPath $staging) { Remove-Item -LiteralPath $staging -Recurse -Force }
    if (Test-Path -LiteralPath $pendingZip) { Remove-Item -LiteralPath $pendingZip -Force }
    if (Test-Path -LiteralPath $previousZip) { Remove-Item -LiteralPath $previousZip -Force }
}
