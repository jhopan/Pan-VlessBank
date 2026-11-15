@echo off
setlocal EnableExtensions EnableDelayedExpansion
set "REPO=%VLESSBANK_REPO%"
if "%REPO%"=="" set "REPO=jhopan/vless-bank"
set "RELEASE=%VLESSBANK_RELEASE%"
if "%RELEASE%"=="" set "RELEASE=latest"
set "APP_DIR=%VLESSBANK_DIR%"
if "%APP_DIR%"=="" set "APP_DIR=%USERPROFILE%\pan-vlessbank"
set "ASSET=vlessbank_windows_amd64.exe"
set "BASE_URL=https://github.com/%REPO%/releases/download/%RELEASE%"
set "TMP=%TEMP%\vlessbank-install-%RANDOM%%RANDOM%"
mkdir "%TMP%" || (echo ERROR: cannot create temp directory& exit /b 1)
curl.exe -fsSL --retry 3 -o "%TMP%\%ASSET%" "%BASE_URL%/%ASSET%" || (echo ERROR: binary download failed& rmdir /S /Q "%TMP%"& exit /b 1)
curl.exe -fsSL --retry 3 -o "%TMP%\checksums.txt" "%BASE_URL%/checksums.txt" || (echo ERROR: checksum download failed& rmdir /S /Q "%TMP%"& exit /b 1)
for /f "tokens=1" %%A in ('findstr /I /C:"%ASSET%" "%TMP%\checksums.txt"') do set "EXPECTED=%%A"
if "%EXPECTED%"=="" (echo ERROR: checksum not found& rmdir /S /Q "%TMP%"& exit /b 1)
for /f %%A in ('powershell -NoProfile -Command "(Get-FileHash -Algorithm SHA256 -LiteralPath '%TMP%\%ASSET%').Hash"') do set "ACTUAL=%%A"
if /I not "%EXPECTED%"=="%ACTUAL%" (echo ERROR: checksum mismatch& rmdir /S /Q "%TMP%"& exit /b 1)
mkdir "%APP_DIR%" 2>nul
move /Y "%TMP%\%ASSET%" "%APP_DIR%\vlessbank.exe" >nul || (echo ERROR: binary replacement failed& rmdir /S /Q "%TMP%"& exit /b 1)
rmdir /S /Q "%TMP%"
echo Installed: %APP_DIR%\vlessbank.exe
echo Release: %RELEASE%
echo State preserved: accounts.txt, config.json, settings, tokens, and logs were not changed.
echo Run: "%APP_DIR%\vlessbank.exe"
