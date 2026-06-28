@echo off
setlocal

set "ROOT_DIR=%~dp0"
if "%ROOT_DIR:~-1%"=="\" set "ROOT_DIR=%ROOT_DIR:~0,-1%"

set "BACKEND_DIR=%ROOT_DIR%"
set "FRONTEND_DIR=%ROOT_DIR%\frontend"

if not exist "%BACKEND_DIR%\cmd\server\main.go" goto :path_error
if not exist "%FRONTEND_DIR%\package.json" goto :path_error

where go >nul 2>nul
if errorlevel 1 goto :go_error

where npm.cmd >nul 2>nul
if errorlevel 1 goto :npm_error

if not exist "%FRONTEND_DIR%\node_modules\.bin\vite.cmd" goto :install_frontend
goto :start_all

:install_frontend
echo Frontend dependencies are missing. Installing...
cd /d "%FRONTEND_DIR%"
if errorlevel 1 goto :path_error
npm.cmd install --include=dev
if errorlevel 1 goto :frontend_install_error
cd /d "%ROOT_DIR%"
if not exist "%FRONTEND_DIR%\node_modules\.bin\vite.cmd" goto :frontend_install_error
goto :start_all

:start_all
echo [1/2] Start backend...
start "SecMgmt Go Backend" cmd /k "cd /d ""%BACKEND_DIR%"" && go run ./cmd/server"

echo [2/2] Start frontend...
start "SecMgmt Go Frontend" cmd /k "cd /d ""%FRONTEND_DIR%"" && npm.cmd run dev"

echo Done.
echo Backend: http://127.0.0.1:8000
echo Frontend: http://127.0.0.1:5173
echo.
echo Stop: run "%ROOT_DIR%\stop.bat" or close the backend/frontend command windows.
echo.
echo If the frontend fails, check whether "%FRONTEND_DIR%\node_modules" exists and install dependencies first.
exit /b 0

:path_error
echo ERROR: Project path is invalid.
echo BACKEND_DIR=%BACKEND_DIR%
echo FRONTEND_DIR=%FRONTEND_DIR%
exit /b 1

:go_error
echo ERROR: Go is not installed or not added to PATH.
exit /b 1

:npm_error
echo ERROR: Node.js/npm is not installed or not added to PATH.
exit /b 1

:frontend_install_error
echo ERROR: Frontend dependency installation failed.
exit /b 1
