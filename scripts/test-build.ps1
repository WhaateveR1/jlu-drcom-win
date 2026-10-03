$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$temp = Join-Path ([IO.Path]::GetTempPath()) ('drcom-build-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path (Join-Path $temp 'scripts') -Force | Out-Null
try {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'build.ps1') -Destination (Join-Path $temp 'scripts')
    foreach ($name in @('config.example.toml', 'README.md', 'USER_GUIDE.md', 'LICENSE', 'NOTICE.md', 'CHANGELOG.md')) {
        Copy-Item -LiteralPath (Join-Path $repo $name) -Destination $temp
    }
    $runtime = Join-Path $temp 'dist/jlu-drcom-win'
    New-Item -ItemType Directory -Path $runtime -Force | Out-Null
    $sentinel = Join-Path $runtime 'config.toml'
    $zip = Join-Path $temp 'dist/jlu-drcom-win.zip'
    [IO.File]::WriteAllText($sentinel, 'synthetic account sentinel')
    [IO.File]::WriteAllText((Join-Path $temp 'config.toml'), 'synthetic source account')
    function go {
        $global:LASTEXITCODE = 0
        if ($args[0] -eq $global:drcomBuildTestFailure) { $global:LASTEXITCODE = 23; return }
        if ($args[0] -eq 'build') {
            $out = $args[[array]::IndexOf($args, '-o') + 1]
            [IO.File]::WriteAllText($out, 'synthetic executable')
        }
    }
    foreach ($phase in @('test', 'vet', 'build')) {
        $global:drcomBuildTestFailure = $phase
        [IO.File]::WriteAllText($zip, 'last known good release')
        $failed = $false
        try { & (Join-Path $temp 'scripts/build.ps1') } catch {
            if ($_.Exception.Message -notlike "go $phase failed*") { throw }
            $failed = $true
        }
        if (-not $failed) { throw "$phase failure was swallowed" }
        if ([IO.File]::ReadAllText($zip) -ne 'last known good release') { throw 'Old release changed on failure' }
        if ([IO.File]::ReadAllText($sentinel) -ne 'synthetic account sentinel') { throw 'User configuration changed' }
    }
    $global:drcomBuildTestFailure = ''
    & (Join-Path $temp 'scripts/build.ps1')
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [IO.Compression.ZipFile]::OpenRead($zip)
    try {
        $names = @($archive.Entries | ForEach-Object { $_.Name })
        if ($names -contains 'config.toml' -or $names -contains 'drcom-win.exe' -or $names -notcontains 'drcom-tray.exe') { throw 'Unexpected package contents' }
    } finally { $archive.Dispose() }
    if ([IO.File]::ReadAllText($sentinel) -ne 'synthetic account sentinel') { throw 'User configuration changed during successful build' }
    Write-Output 'Build failure handling and configuration preservation passed.'
}
finally {
    Remove-Variable -Name drcomBuildTestFailure -Scope Global -ErrorAction SilentlyContinue
    $resolved = [IO.Path]::GetFullPath($temp)
    $parent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
    if ([IO.Path]::GetDirectoryName($resolved) -ne $parent -or [IO.Path]::GetFileName($resolved) -notmatch '^drcom-build-test-[a-f0-9]{32}$') { throw 'Unexpected test cleanup path' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
