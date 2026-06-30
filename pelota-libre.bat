@echo off
setlocal

set "APP_DIR=%LOCALAPPDATA%\pelota-libre"
set "BIN=%APP_DIR%\pelota.exe"

if not exist "%BIN%" (
    echo Downloading pelota-libre to %APP_DIR%...
    if not exist "%APP_DIR%" mkdir "%APP_DIR%"
    powershell -Command "
        $url = 'https://github.com/rodrigo-sys/pelota-libre/releases/latest/download/pelota-windows-amd64.exe';
        Invoke-WebRequest -Uri $url -OutFile '%BIN%'
    "
    if %errorlevel% neq 0 (
        echo Download failed. Check your internet connection.
        pause
        exit /b 1
    )
)

"%BIN%" %*
pause
