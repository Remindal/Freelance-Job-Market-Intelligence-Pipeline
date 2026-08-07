# 一键启动：先确保采集浏览器在跑（带调试端口），再启动桌面 app。
# 注意：app 每次都直接启动——它自己有单实例锁，
# 若已有实例在托盘运行，会唤起其窗口并自动退出，不会开第二个进程。
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

Start-Process "$root\build\bin\scout.exe"
