@echo off
chcp 65001 >nul
echo ====================================
echo Running Property-Based Tests
echo ====================================
echo.

cd /d "%~dp0"

echo Running Property 5: Timeframe Isolation...
go test -v -run TestProperty_TimeframeIsolation

if %errorlevel% neq 0 (
    echo.
    echo ❌ Property test failed
    pause
    exit /b 1
)

echo.
echo Running Property 10: Concurrent Access Safety...
go test -v -run TestProperty_ConcurrentAccessSafety

if %errorlevel% neq 0 (
    echo.
    echo ❌ Property test failed
    pause
    exit /b 1
)

echo.
echo ✅ All property tests passed
echo.
pause
