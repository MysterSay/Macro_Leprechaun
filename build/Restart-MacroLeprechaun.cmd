@echo off
rem Close old MacroLeprechaun instances before launching the updated executable.
taskkill /IM MacroLeprechaun.exe /F >nul 2>&1
if not exist "%~dp0MacroLeprechaun.exe" (
  echo MacroLeprechaun.exe was not found next to this file.
  pause
  exit /b 1
)
start "" /D "%~dp0" "%~dp0MacroLeprechaun.exe"
