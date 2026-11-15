@echo off
setlocal EnableExtensions EnableDelayedExpansion
set "REPO=%PAN_VLESSBANK_REPO%"
if "%REPO%"=="" set "REPO=jhopan/Pan-VlessBank"
set "RELEASE=%PAN_VLESSBANK_RELEASE%"
if "%RELEASE%"=="" set "RELEASE=latest"
set "APP_DIR=%PAN_VLESSBANK_DIR%"
if "%APP_DIR%"=="" set "APP_DIR=%USERPROFILE%\pan-vlessbank"
set "ASSET=pan-vlessbank_windows_amd64.exe"
set "TMP=%TEMP%\pan-vlessbank-install-%RANDOM%%RANDOM%"
mkdir "%TMP%" || exit /b 1
curl.exe -fsSL --retry 3 -o "%TMP%\%ASSET%" "https://github.com/%REPO%/releases/download/%RELEASE%/%ASSET%" || (rmdir /S /Q "%TMP%"& exit /b 1)
curl.exe -fsSL --retry 3 -o "%TMP%\checksums.txt" "https://github.com/%REPO%/releases/download/%RELEASE%/checksums.txt" || (rmdir /S /Q "%TMP%"& exit /b 1)
for /f "tokens=1" %%A in ('findstr /I /C:"%ASSET%" "%TMP%\checksums.txt"') do set "EXPECTED=%%A"
for /f %%A in ('powershell -NoProfile -Command "(Get-FileHash -Algorithm SHA256 -LiteralPath '%TMP%\%ASSET%').Hash"') do set "ACTUAL=%%A"
if "%EXPECTED%"=="" (rmdir /S /Q "%TMP%"& exit /b 1)
if /I not "%EXPECTED%"=="%ACTUAL%" (rmdir /S /Q "%TMP%"& exit /b 1)
mkdir "%APP_DIR%" 2>nul
move /Y "%TMP%\%ASSET%" "%APP_DIR%\pan-vlessbank.exe" >nul || exit /b 1
rmdir /S /Q "%TMP%"
echo Installed: %APP_DIR%\pan-vlessbank.exe
