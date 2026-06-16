@echo off
:: Change working directory to the directory where this script is located
cd /d "%~dp0"

echo ==========================================================
echo          TORBI ANDROID GO BRIDGE COMPILER
echo ==========================================================
echo.

:: Check Android SDK and NDK paths.
set "SDK_PATH=C:\Android"
if not exist "%SDK_PATH%" (
    set "SDK_PATH=%LOCALAPPDATA%\Android\Sdk"
)
if not exist "%SDK_PATH%" (
    echo [ERROR] Android SDK not found at C:\Android or %LOCALAPPDATA%\Android\Sdk
    echo Please install the Android SDK via Android Studio or Command-line tools.
    pause
    exit /b 1
)

:: Find the latest NDK directory
set "NDK_ROOT="
for /d %%d in ("%SDK_PATH%\ndk\*") do (
    set "NDK_ROOT=%%d"
)

if "%NDK_ROOT%"=="" (
    echo [ERROR] Android NDK not found in %SDK_PATH%\ndk
    echo Please open Android Studio, go to SDK Manager, SDK Tools and install NDK.
    pause
    exit /b 1
)

echo Found Android NDK at: %NDK_ROOT%

:: Find NDK clang toolchain compiler path for arm64
set "CLANG_PATH="
for /f "delims=" %%i in ('dir /b /s "%NDK_ROOT%\*aarch64-linux-android*-clang.cmd" 2^>nul') do (
    set "CLANG_PATH=%%i"
)

if "%CLANG_PATH%"=="" (
    echo [ERROR] Android NDK Clang compiler not found inside %NDK_ROOT%
    pause
    exit /b 1
)

echo Using compiler: %CLANG_PATH%
echo.

:: Create target directories in Flutter project
if not exist "client\android\app\src\main\jniLibs\arm64-v8a" (
    mkdir "client\android\app\src\main\jniLibs\arm64-v8a"
)

echo Compiling Go core engine for Android (arm64-v8a)...
set CGO_ENABLED=1
set GOOS=android
set GOARCH=arm64
set CC=%CLANG_PATH%

go build -ldflags "-checklinkname=0" -buildmode=c-shared -o client\android\app\src\main\jniLibs\arm64-v8a\libtorbi.so bridge\bridge.go

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Go compilation failed! Check compiler or environment logs.
    pause
    exit /b 1
)

echo.
echo SUCCESS: Compiled libtorbi.so and saved to client/android/app/src/main/jniLibs/arm64-v8a/
pause
