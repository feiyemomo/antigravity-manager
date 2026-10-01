@echo off
cd /d "%~dp0"
title Stop Antigravity Manager

taskkill /f /im Antigravity-Manager.exe >nul 2>&1
taskkill /f /im agy-tools.exe >nul 2>&1
echo Service stopped.
timeout /t 1 >nul
exit
