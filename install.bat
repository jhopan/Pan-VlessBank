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
mkdir "%TMP%" || goto :fail
curl.exe -fsSL --retry 3 -o "%TMP%\%ASSET%" "%BASE_URL%/%ASSET%" || goto :fail
curl.exe -fsSL --retry 3 -o "%TMP%\checksums.txt" "%BASE_URL%/checksums.txt" || goto :fail
for /f "tokens=1" %%A in ('findstr /R /C:" %ASSET%$" "%TMP%\checksums.txt"') do set "EXPECTED=%%A"
if "%EXPECTED%"=="" goto :fail
for /f "tokens=1" %%A in ('certutil -hashfile "%TMP%\%ASSET%" SHA256 ^| findstr /R "^[0-9A-F][0-9A-F]"') do set "ACTUAL=%%A"
if /I not "%EXPECTED%"=="%ACTUAL%" goto :fail
mkdir "%APP_DIR%" 2>nul
move /Y "%TMP%\%ASSET%" "%APP_DIR%\vlessbank.exe" >nul || goto :fail
rmdir /S /Q "%TMP%"
echo Installed: %APP_DIR%\vlessbank.exe
echo Release: %RELEASE%
echo State lama dipertahankan: accounts.txt, config.json, settings, token, dan log tidak diubah.
echo Jalankan: "%APP_DIR%\vlessbank.exe"
exit /b 0
:fail
echo ERROR: install gagal. Binary lama tidak diganti.
rmdir /S /Q "%TMP%" 2>nul
exit /b 1
