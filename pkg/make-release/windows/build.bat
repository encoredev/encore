@echo off
rem SPDX-License-Identifier: MIT
rem Copyright (C) 2019-2021 WireGuard LLC. All Rights Reserved.

setlocal enableextensions enabledelayedexpansion
set BUILDDIR=%~dp0
set ROOT=%BUILDDIR%\..\..\..
set DST=%ROOT%\dist\windows_amd64
set PATH=%BUILDDIR%.deps\llvm-mingw\bin;%BUILDDIR%.deps;%PATH%
set PATHEXT=.EXE;.CMD

if "%ENCORE_VERSION%" == "" (
	echo ENCORE_VERSION not set
	exit /b 1
)

if "%ENCORE_GOROOT%" == "" (
	echo ENCORE_GOROOT not set
	exit /b 1
)

:: Get absolute path
cd %ENCORE_GOROOT% || exit /b 1
set ENCORE_GOROOT=%CD%

cd /d %BUILDDIR% || exit /b 1

if exist .deps\prepared goto :build
:installdeps
	rmdir /s /q .deps 2> NUL
	mkdir .deps || goto :error
	cd .deps || goto :error
	call :download llvm-mingw-msvcrt.zip https://download.wireguard.com/windows-toolchain/distfiles/llvm-mingw-20201020-msvcrt-x86_64.zip 2e46593245090df96d15e360e092f0b62b97e93866e0162dca7f93b16722b844 || goto :error
	call :download wintun.zip https://www.wintun.net/builds/wintun-0.10.2.zip fcd9f62f1bd5a550fcb9c21fbb5d6a556214753ccbbd1a3ebad4d318ec9dcbef || goto :error
	call :download wix-binaries.zip https://github.com/wixtoolset/wix3/releases/download/wix3112rtm/wix311-binaries.zip 2c1888d5d1dba377fc7fa14444cf556963747ff9a0a289a3599cf09da03b9e2e || goto :error
	copy /y NUL prepared > NUL || goto :error
	cd .. || goto :error

:build
	set GOOS=windows
	call :build_plat amd64 x86_64 amd64 || goto :error
	call :copy_artifacts || goto :error

:success
	echo [+] Success!
	exit /b 0

:download
	echo [+] Downloading %1
	curl -#fLo %1 %2 || exit /b 1
	echo [+] Verifying %1
	for /f %%a in ('CertUtil -hashfile %1 SHA256 ^| findstr /r "^[0-9a-f]*$"') do if not "%%a"=="%~3" exit /b 1
	echo [+] Extracting %1
	tar -xf %1 %~4 || exit /b 1
	echo [+] Cleaning up %1
	del %1 || exit /b 1
	goto :eof

:build_plat
	rmdir /S /Q "%DST%"
	mkdir %DST%\bin >NUL 2>&1
	echo [+] Assembling resources
	x86_64-w64-mingw32-windres -I ".deps\wintun\bin\amd64" -i resources.rc -o "%ROOT%\cli\cmd\encore\resources_amd64.syso" -O coff -c 65001 || exit /b %errorlevel%
	set GOARCH=amd64
	echo [+] Building
	go build -tags load_wintun_from_rsrc -ldflags "-X 'encr.dev/internal/version.Version=v%ENCORE_VERSION%'" -o "%DST%\bin\encore.exe" "%ROOT%\cli\cmd\encore" || exit /b 1
	go build -trimpath -o "%DST%\bin\git-remote-encore.exe" "%ROOT%\cli\cmd\git-remote-encore" || exit /b 1
	goto :eof

:copy_artifacts
	echo [+] Copying files
	xcopy /S /I /E /H /Q "%ENCORE_GOROOT%" "%DST%\encore-go" || exit /b 1
	xcopy /S /I /E /H /Q "%ROOT%\runtimes\go" "%DST%\runtimes\go" || exit /b 1
	goto :eof

:error
	echo [-] Failed with error #%errorlevel%.
	cmd /c exit %errorlevel%
