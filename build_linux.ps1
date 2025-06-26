# Script de build pour Linux
Write-Host "Construction de Bashcord pour Linux..." -ForegroundColor Green

# Obtenir le hash git
$gitHash = "dev"
$gitTag = "dev-build"

try {
    $gitHash = (git rev-parse --short HEAD 2>$null) -replace "`n", "" -replace "`r", ""
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Attention: Impossible d'obtenir le hash git, utilisation de 'dev'" -ForegroundColor Yellow
        $gitHash = "dev"
    }
} catch {
    Write-Host "Attention: Git non disponible, utilisation de 'dev'" -ForegroundColor Yellow
    $gitHash = "dev"
}

Write-Host "Hash Git: $gitHash" -ForegroundColor Cyan
Write-Host "Tag: $gitTag" -ForegroundColor Cyan

$ldflags = "-s -w -X 'vencord/buildinfo.InstallerGitHash=$gitHash' -X 'vencord/buildinfo.InstallerTag=$gitTag'"

# Compiler la version CLI pour Linux (sans CGO)
Write-Host "`nCompilation de la version CLI pour Linux..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"

try {
    go build -v -tags "static cli" -ldflags $ldflags -o "Bashcord-Linux-cli"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Succès! Version CLI Linux créée: Bashcord-Linux-cli" -ForegroundColor Green
        $fileInfo = Get-Item "Bashcord-Linux-cli"
        Write-Host "Taille: $([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Cyan
    } else {
        Write-Host "Échec de la compilation CLI Linux" -ForegroundColor Red
    }
} catch {
    Write-Host "Erreur lors de la compilation CLI Linux: $_" -ForegroundColor Red
}

# Essayer de compiler la version GUI pour Linux
Write-Host "`nTentative de compilation de la version GUI pour Linux..." -ForegroundColor Yellow
Write-Host "Attention: Nécessite les dépendances de développement Linux" -ForegroundColor Yellow
$env:CGO_ENABLED = "1"

try {
    go build -v -tags "static gui" -ldflags $ldflags -o "Bashcord-Linux-gui"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Succès! Version GUI Linux créée: Bashcord-Linux-gui" -ForegroundColor Green
        $fileInfo = Get-Item "Bashcord-Linux-gui"
        Write-Host "Taille: $([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Cyan
    } else {
        Write-Host "Échec de la compilation GUI Linux (attendu sur Windows)" -ForegroundColor Red
    }
} catch {
    Write-Host "Erreur lors de la compilation GUI Linux: $_" -ForegroundColor Red
}

Write-Host "`nPour compiler la version GUI Linux, vous avez besoin des dépendances suivantes:" -ForegroundColor Yellow
Write-Host "Sur Ubuntu/Debian:" -ForegroundColor Cyan
Write-Host "sudo apt install -y pkg-config libsdl2-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libglx-dev libgl1-mesa-dev libxxf86vm-dev libwayland-dev libxkbcommon-dev wayland-protocols extra-cmake-modules" -ForegroundColor Gray

Write-Host "`nSur Arch Linux:" -ForegroundColor Cyan
Write-Host "sudo pacman -S pkg-config sdl2 libx11 libxcursor libxrandr libxinerama libxi mesa wayland wayland-protocols" -ForegroundColor Gray

Write-Host "`nRésumé des fichiers créés:" -ForegroundColor Green
Get-ChildItem "Bashcord-Linux*" | ForEach-Object {
    $size = [math]::Round($_.Length / 1MB, 2)
    Write-Host "$($_.Name) - $size MB" -ForegroundColor White
} 