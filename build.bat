@echo off
setlocal
set "LDFLAGS=-s -w"

if not exist outp mkdir outp

echo ========================================
echo Befree-go cross-platform build
echo ========================================
echo.

echo [1/3] Building Windows amd64...
set "GOOS=windows"
set "GOARCH=amd64"
go build -ldflags="%LDFLAGS%" -trimpath -o .\outp\Befree-go.exe .
if errorlevel 1 exit /b 1
echo.

echo [2/3] Building Linux amd64...
set "GOOS=linux"
set "GOARCH=amd64"
go build -ldflags="%LDFLAGS%" -trimpath -o .\outp\Befree-go_linux64 .
if errorlevel 1 exit /b 1
echo.

echo [3/3] Building macOS arm64...
set "GOOS=darwin"
set "GOARCH=arm64"
go build -ldflags="%LDFLAGS%" -trimpath -o .\outp\Befree-go_darwin_arm64 .
if errorlevel 1 exit /b 1
echo.

echo ========================================
echo Build completed
echo ========================================
echo.
echo Output files:
echo   Befree-go.exe
echo   Befree-go_linux64
echo   Befree-go_darwin_arm64
echo.
pause
