@echo off
REM Install tot-mcp binary from GitHub releases on Windows.
REM Called by launch.bat. Stores binary in %CLAUDE_PLUGIN_DATA%\bin\.

setlocal enabledelayedexpansion

set "REPO=viraf-pro/tree-of-thoughts"
set "BINARY_NAME=tot-mcp.exe"
set "INSTALL_DIR=%CLAUDE_PLUGIN_DATA%\bin"
set "PLUGIN_ROOT=%CLAUDE_PLUGIN_ROOT%"
set "VERSION_FILE=%CLAUDE_PLUGIN_DATA%\.installed-version"
set "ARCHIVE=tot-mcp-windows-amd64.exe"

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"

REM Read target version from plugin.json
set "PLUGIN_JSON=%PLUGIN_ROOT%\.claude-plugin\plugin.json"
if not exist "%PLUGIN_JSON%" set "PLUGIN_JSON=%PLUGIN_ROOT%\.codex-plugin\plugin.json"
set "TARGET="
if exist "%PLUGIN_JSON%" (
    for /f "tokens=2 delims=:, " %%a in ('findstr /c:"\"version\"" "%PLUGIN_JSON%"') do (
        set "TARGET=v%%~a"
    )
)

REM Fallback to latest release
if "%TARGET%"=="" (
    for /f "tokens=2 delims=:, " %%a in ('curl -fsSL "https://api.github.com/repos/%REPO%/releases/latest" 2^>nul ^| findstr /c:"\"tag_name\""') do (
        set "TARGET=%%~a"
    )
)

if "%TARGET%"=="" (
    echo tot-mcp: could not determine target version >&2
    exit /b 1
)

REM Check if already installed at correct version
if exist "%INSTALL_DIR%\%BINARY_NAME%" if exist "%VERSION_FILE%" (
    set /p INSTALLED=<"%VERSION_FILE%"
    if "!INSTALLED!"=="%TARGET%" exit /b 0
)

REM Download pre-built binary
set "BASE_URL=https://github.com/%REPO%/releases/download/%TARGET%"
set "TMPFILE=%TEMP%\%ARCHIVE%"

echo Downloading tot-mcp %TARGET% for Windows...
curl -fsSL --retry 3 "%BASE_URL%/%ARCHIVE%" -o "%TMPFILE%" 2>nul
if errorlevel 1 (
    echo tot-mcp: download failed >&2
    goto :try_build
)

copy /y "%TMPFILE%" "%INSTALL_DIR%\%BINARY_NAME%" >nul
del /f "%TMPFILE%" 2>nul
echo %TARGET%>"%VERSION_FILE%"
echo tot-mcp %TARGET% installed successfully.
exit /b 0

:try_build
REM Fallback: build from source
where go >nul 2>&1
if errorlevel 1 (
    echo tot-mcp: could not download binary or build from source. Install Go or check https://github.com/%REPO%/releases >&2
    exit /b 1
)

cd /d "%PLUGIN_ROOT%"
go build -ldflags "-s -w" -o "%INSTALL_DIR%\%BINARY_NAME%" .
echo built-from-source>"%VERSION_FILE%"
exit /b 0
