@echo off
REM Launcher for tot-mcp MCP server on Windows.
REM Downloads the binary on first run, then runs it.

set "BINARY=%CLAUDE_PLUGIN_DATA%\bin\tot-mcp.exe"

if not exist "%BINARY%" (
    call "%~dp0install.bat"
    if errorlevel 1 exit /b 1
)

"%BINARY%" %*
