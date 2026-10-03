@echo off
if "%~1"=="" (
    echo Usage: .\scripts\generate-domain.bat DomainName
    echo Example: .\scripts\generate-domain.bat Product
    exit /b 1
)
powershell -ExecutionPolicy Bypass -File "%~dp0generate-domain.ps1" %1