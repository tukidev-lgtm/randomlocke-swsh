@echo off
REM Compila Randomlocke.exe (necesitas Go 1.22 o superior: https://go.dev/dl/)
cd /d "%~dp0"
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o ..\dist\Randomlocke.exe .
if %errorlevel% neq 0 (echo Error al compilar & pause & exit /b 1)
echo Listo: dist\Randomlocke.exe
pause
