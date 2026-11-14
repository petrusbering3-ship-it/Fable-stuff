@echo off
rem Builds FableBuddy.exe next to this file. Needs Go (https://go.dev/dl).
cd /d "%~dp0"
where go >nul 2>nul
if errorlevel 1 (
  echo Go is not installed. Install it with:  winget install GoLang.Go
  echo Then close this window and run build.bat again.
  pause
  exit /b 1
)
set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags "-s -w -H windowsgui" -o FableBuddy.exe .
if errorlevel 1 (
  echo Build failed.
  pause
  exit /b 1
)
echo Built FableBuddy.exe - starting it now.
start "" FableBuddy.exe
