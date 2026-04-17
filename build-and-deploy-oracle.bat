@echo off
setlocal
cd /d "%~dp0"

call "%~dp0build-linux-amd64.bat"
if errorlevel 1 (
  echo Build step failed.
  exit /b 1
)

call "%~dp0deploy-oracle.bat"
if errorlevel 1 (
  echo Deploy step failed.
  exit /b 1
)

echo Build and deploy completed.
