# 一键构建：编译 exe 并自动把配置同步到 exe 同级目录
$root = Split-Path $PSScriptRoot -Parent
Set-Location $root

# wails build -clean 会清空 build/bin，先备份数据目录，构建完恢复
$dataBak = "$root\build\.data-backup"
if (Test-Path "$root\build\bin\data") {
    Remove-Item -Recurse -Force $dataBak -ErrorAction SilentlyContinue
    Copy-Item -Recurse "$root\build\bin\data" $dataBak
}

$env:SCOUT_CONFIG = "$root\configs\config.yaml"
& "$env:USERPROFILE\go\bin\wails.exe" build -clean -webview2 embed
if ($LASTEXITCODE -ne 0) { Write-Host "构建失败"; exit $LASTEXITCODE }

New-Item -ItemType Directory -Force "$root\build\bin\configs" | Out-Null
Copy-Item "$root\configs\config.yaml", "$root\configs\config.example.yaml" "$root\build\bin\configs\" -Force
if (Test-Path $dataBak) {
    Copy-Item -Recurse -Force $dataBak "$root\build\bin\data"
    Remove-Item -Recurse -Force $dataBak
}
Write-Host "构建完成: $root\build\bin\scout.exe"
