$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
Set-Location "d:\DEV\EVA-Mind"
& "C:\Program Files\Go\bin\go.exe" build -ldflags="-s -w" -o eva-mind-linux .
if ($LASTEXITCODE -eq 0) {
    Write-Host "BUILD OK - Linux amd64"
    $f = Get-Item "eva-mind-linux"
    Write-Host "Size: $([math]::Round($f.Length/1MB, 1)) MB"
} else {
    Write-Host "BUILD FAILED"
}
