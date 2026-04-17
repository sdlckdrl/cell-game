@echo off
setlocal
cd /d "%~dp0"

set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0

go build -o game_server .
if errorlevel 1 (
  echo Build failed.
  exit /b 1
)

echo Built game_server for linux/amd64
