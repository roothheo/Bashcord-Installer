@echo off
echo Construction de Bashcord pour Linux...

REM Obtenir le hash git
for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_HASH=%%i
if "%GIT_HASH%"=="" set GIT_HASH=dev

echo Hash Git: %GIT_HASH%

REM Compiler la version CLI pour Linux (sans CGO)
echo.
echo Compilation de la version CLI pour Linux...
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -v -tags "static cli" -ldflags "-s -w -X 'vencord/buildinfo.InstallerGitHash=%GIT_HASH%' -X 'vencord/buildinfo.InstallerTag=dev-build'" -o Bashcord-Linux-cli

if %errorlevel% equ 0 (
    echo.
    echo Succès! Version CLI Linux créée: Bashcord-Linux-cli
) else (
    echo.
    echo Échec de la compilation CLI Linux
)

REM Essayer de compiler la version GUI pour Linux (nécessite des dépendances système)
echo.
echo Tentative de compilation de la version GUI pour Linux...
echo Attention: Ceci nécessite les dépendances de développement Linux installées
set CGO_ENABLED=1
go build -v -tags "static gui" -ldflags "-s -w -X 'vencord/buildinfo.InstallerGitHash=%GIT_HASH%' -X 'vencord/buildinfo.InstallerTag=dev-build'" -o Bashcord-Linux-gui

if %errorlevel% equ 0 (
    echo.
    echo Succès! Version GUI Linux créée: Bashcord-Linux-gui
) else (
    echo.
    echo Échec de la compilation GUI Linux
    echo.
    echo Pour compiler la version GUI Linux, vous avez besoin des dépendances suivantes:
    echo - pkg-config
    echo - libsdl2-dev
    echo - libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev
    echo - libglx-dev libgl1-mesa-dev libxxf86vm-dev
    echo - libwayland-dev libxkbcommon-dev wayland-protocols
    echo.
    echo Sur Ubuntu/Debian:
    echo sudo apt install -y pkg-config libsdl2-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libglx-dev libgl1-mesa-dev libxxf86vm-dev libwayland-dev libxkbcommon-dev wayland-protocols extra-cmake-modules
)

echo.
echo Compilation terminée!
pause 