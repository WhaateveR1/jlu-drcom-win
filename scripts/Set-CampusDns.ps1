param(
    [int]$InterfaceIndex,
    [string]$InterfaceGuid,
    [Parameter(Mandatory)][string]$BackupPath,
    [switch]$Restore,
    [Parameter(Mandatory)][string]$ResultPath
)
$ErrorActionPreference = 'Stop'
try {
    if ($Restore) {
        $backup = Get-Content -LiteralPath $BackupPath -Raw | ConvertFrom-Json
        $InterfaceGuid = $backup.InterfaceGuid
    }
    $adapter = Get-NetAdapter | Where-Object { $_.InterfaceGuid.ToString() -eq $InterfaceGuid }
    if (@($adapter).Count -ne 1) { throw 'The saved physical adapter was not found.' }
    $ipv4Dns = Get-DnsClientServerAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4
    if (-not $Restore -and $adapter.ifIndex -ne $InterfaceIndex) { throw 'Interface changed; run diagnosis again.' }
    if ($Restore) {
        if ($backup.Automatic) {
            Set-DnsClientServerAddress -InputObject $ipv4Dns -ResetServerAddresses
        } else {
            Set-DnsClientServerAddress -InputObject $ipv4Dns -ServerAddresses @($backup.Servers)
        }
    } else {
        if (Test-Path -LiteralPath $BackupPath) { throw 'Backup already exists; refusing to overwrite it.' }
        $servers = (Get-DnsClientServerAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4).ServerAddresses
        $key = 'HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\' + $adapter.InterfaceGuid
        $manual = (Get-ItemProperty -LiteralPath $key).NameServer
        @{ InterfaceGuid=$adapter.InterfaceGuid.ToString(); Servers=@($servers); Automatic=[string]::IsNullOrWhiteSpace($manual) } |
            ConvertTo-Json | Set-Content -LiteralPath $BackupPath -Encoding UTF8
        Set-DnsClientServerAddress -InputObject $ipv4Dns -ServerAddresses @('10.10.10.10','10.10.10.11')
    }
    Clear-DnsClientCache
    @{ Success=$true; Interface=$adapter.Name; Servers=@((Get-DnsClientServerAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4).ServerAddresses) } |
        ConvertTo-Json | Set-Content -LiteralPath $ResultPath -Encoding UTF8
} catch {
    @{ Success=$false; Error=$_.Exception.Message } | ConvertTo-Json | Set-Content -LiteralPath $ResultPath -Encoding UTF8
    exit 1
}
