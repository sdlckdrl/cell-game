@echo off
setlocal
cd /d "%~dp0"

set KEY=D:\project\ssh-key-2026-04-16.key
set HOST=ubuntu@168.107.2.23
set TARGET=/home/ubuntu/

scp -i "%KEY%" -r game_server index.html styles.css super.html src "%HOST%:%TARGET%"
if errorlevel 1 (
  echo Upload failed.
  exit /b 1
)

ssh -i "%KEY%" %HOST% "chmod +x /home/ubuntu/game_server"
if errorlevel 1 (
  echo Upload succeeded, but chmod failed.
  exit /b 1
)

echo Deploy completed.
