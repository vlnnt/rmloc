@echo off
setlocal

cd /d "%~dp0"

echo Building RMLOC for Windows...

set "OUTPUT_DIR=rmloc-windows"

if not exist "%OUTPUT_DIR%" (
    mkdir "%OUTPUT_DIR%"
)

set "CGO_ENABLED=0"
set "GOOS=windows"
set "GOARCH=amd64"

go build ^
    -trimpath ^
    -ldflags="-s -w" ^
    -o "%OUTPUT_DIR%\rmloc.exe" ^
    ./cmd/rmloc

if errorlevel 1 (
    echo.
    echo Build failed.
    exit /b 1
)

if exist "prompts\" (
    xcopy "prompts\*" "%OUTPUT_DIR%\prompts\" /E /I /Y >nul

    if errorlevel 1 (
        echo.
        echo Failed to copy prompts.
        exit /b 1
    )
)

echo.
echo Build completed:
echo   %OUTPUT_DIR%\rmloc.exe
echo   %OUTPUT_DIR%\prompts\

endlocal
