Write-Host "=== Entangled VPN Client Build ===" -ForegroundColor Cyan

# Check dependencies
$goVer = go version 2>$null
if (-not $goVer) {
    Write-Host "Go is not installed!" -ForegroundColor Red
    Write-Host "Install Go from: https://go.dev/dl/" -ForegroundColor Yellow
    exit 1
}
Write-Host "Found: $goVer"

$nodeVer = node --version
if (-not $nodeVer) {
    Write-Host "Node.js is not installed!" -ForegroundColor Red
    exit 1
}
Write-Host "Found: Node.js $nodeVer"

# Check Wails CLI
$wailsVer = wails version 2>$null
if (-not $wailsVer) {
    Write-Host "Installing Wails CLI..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
}
Write-Host "Wails: $(wails version)"

# Install frontend dependencies
Push-Location client/frontend
Write-Host "`nInstalling frontend dependencies..." -ForegroundColor Cyan
npm install
Pop-Location

# Restore tracked packaging (icon + requireAdministrator). client/build/ is gitignored.
New-Item -ItemType Directory -Force -Path "client\build\windows" | Out-Null
if (Test-Path "client\packaging\appicon.png") {
    Copy-Item "client\packaging\appicon.png" "client\build\appicon.png" -Force
}
if (Test-Path "client\packaging\windows") {
    Copy-Item "client\packaging\windows\*" "client\build\windows\" -Force
}
# Download Wintun DLL
$wintunDir = "$env:TEMP\wintun"
if (-not (Test-Path "$wintunDir\wintun\bin\amd64\wintun.dll")) {
    Write-Host "`nDownloading Wintun DLL..." -ForegroundColor Cyan
    $url = "https://www.wintun.net/builds/wintun-0.14.1.zip"
    $zip = "$env:TEMP\wintun.zip"
    Invoke-WebRequest -Uri $url -OutFile $zip
    Expand-Archive -Path $zip -DestinationPath $wintunDir -Force
}

$wintunDll = "$wintunDir\wintun\bin\amd64\wintun.dll"
if (Test-Path $wintunDll) {
    Copy-Item $wintunDll -Destination "client\build\windows\wintun.dll" -Force
    Write-Host "Wintun DLL copied" -ForegroundColor Green
}

# Download Wintun Go module dependencies
Push-Location client
Write-Host "`nTidying Go modules..." -ForegroundColor Cyan
go mod tidy
Pop-Location

# Build
Push-Location client
Write-Host "`nBuilding client..." -ForegroundColor Cyan
wails build -o "Entangled.exe"
Pop-Location

# Copy wintun.dll next to the binary
if (Test-Path $wintunDll) {
    Copy-Item $wintunDll -Destination "client\build\bin\wintun.dll" -Force
    Write-Host "Wintun DLL copied to output" -ForegroundColor Green
}

Write-Host "`n=== Build complete! ===" -ForegroundColor Green
Write-Host "Output: client/build/bin/Entangled.exe"
Write-Host "Wintun: client/build/bin/wintun.dll"
