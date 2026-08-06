# 一键启动：先确保采集 Chrome 在跑（带调试端口），再启动桌面 app
$root = Split-Path $PSScriptRoot -Parent

$cdpAlive = $false
try {
    Invoke-RestMethod "http://127.0.0.1:9222/json/version" -TimeoutSec 2 | Out-Null
    $cdpAlive = $true
} catch {}

if (-not $cdpAlive) {
    $chrome = "$env:LOCALAPPDATA\Google\Chrome\Application\chrome.exe"
    Start-Process $chrome -ArgumentList '--remote-debugging-port=9222', "--user-data-dir=`"$root\data\browser-profile`""
    Start-Sleep -Seconds 3
}

if (-not (Get-Process upwork-scout -ErrorAction SilentlyContinue)) {
    Start-Process "$root\build\bin\upwork-scout.exe"
}
