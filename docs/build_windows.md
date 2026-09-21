# Build Intructions for Windows Platform

## Introduction

For better results and avoid problems, consider a clean Windows 11 install or VM to begin with.

Recommended minimum hardware: 16GB RAM, 200GB SSD, 4 cores x64 CPU.

## Install all the necessary tools

### Install Git

    winget install Git.Git

### Install Visual Studio 2026 and Dotnet SDKs

    winget install Microsoft.VisualStudio.Community --silent --accept-package-agreements --accept-source-agreements --override "--wait --quiet --add ProductLang En-us --add Microsoft.VisualStudio.Workload.VCTools --add Microsoft.Net.Component.4.8.TargetingPack --add Microsoft.VisualStudio.Workload.ManagedDesktop --add Microsoft.VisualStudio.Workload.NetCrossPlat --add Microsoft.VisualStudio.Workload.CoreEditor --add Microsoft.VisualStudio.Workload.NativeDesktop --includeRecommended"

    winget install Microsoft.DotNet.SDK.8
    winget install Microsoft.NuGet

    rem git clone https://github.com/microsoft/vcpkg C:\vcpkg
    rem C:\vcpkg\bootstrap-vcpkg.bat
    rem setx VCPKG_ROOT C:\vcpkg
    rem setx PATH "C:\vcpkg;%PATH%"

If msbuild.exe is not on the PATH, find it and add it:

    "%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe" -latest -products * -requires Microsoft.Component.MSBuild -find MSBuild\**\Bin\MSBuild.exe
    Should print: C:\Program Files\Microsoft Visual Studio\18\Community\MSBuild\Current\Bin\MSBuild.exe
    setx PATH "%PATH%;C:\Program Files\Microsoft Visual Studio\18\Community\MSBuild\Current\Bin"

### Install Python

    winget install Python.Python.3.14

### Install CMake 

    winget install Kitware.CMake

## Install NSISBI 3.12 

    https://sourceforge.net/projects/nsisbi/files/latest/download

Install Access Control plugin for NSIS
    https://nsis.sourceforge.io/mediawiki/images/4/4a/AccessControl.zip
    Copy AccessControl.dll from Plugins\i386-ansi to NSIS\Plugins\x86-ansi
    Copy AccessControl.dll from Plugins\i386-unicode to NSIS\Plugins\x86-unicode

### Install GO 

    winget install GoLang.Go

### Install JDK

    winget install EclipseAdoptium.Temurin.26.JDK

### Apache Maven
    winget install chocolatey
    choco install maven (As Admin)

### Install Nodejs

    winget install OpenJS.NodeJS

## Clone the repo

    git clone --recurse-submodules https://github.com/riclolsen/json-scada --config core.autocrlf=input c:\json-scada
	
	or the development repo
	git clone --recurse-submodules https://github.com/json-scada/json-scada-devel --config core.autocrlf=input c:\json-scada

## Compiling and building the source code

Open "x64 Native Tools Command Prompt for VS".

    cd \json-scada\platform-windows
    build.bat

## Build the NSIS installer

Create and fill up the "platform-windows" folders

    cd \json-scada\platform-windows
    mkdir grafana-runtime
    mkdir inkscape-runtime
    mkdir jdk-runtime
    mkdir metabase-runtime
    mkdir mongodb-compass-runtime
    mkdir mongodb-runtime
    mkdir nginx_php-runtime
    mkdir nodejs-runtime
    mkdir nodered-runtime
    mkdir postgresql-runtime
    mkdir telegraf-runtime

Fill up the folders with the respective runtimes (grab from another JSON-SCADA installation or download from the internet).

Add the following files to \json-scada\platform-windows:

    nssm.exe
    ffmpeg.exe
    sounder.exe
    OpcWatch.exe
    vc_redist.x64.exe
    aspnetcore-runtime-8.0.23-win-x64.exe (or newer as in the json-scada.nsi file)
    dotnet-runtime-8.0.23-win-x64.exe (or newer as in the json-scada.nsi file)
    dotnet-runtime-10.0.2-win-x64.exe (or newer as in the json-scada.nsi file)
    OPC Core Components Redistributable (x64) 3.00.108.msi (or newer as in the json-scada.nsi file)

Build the installer

    cd \json-scada\platform-windows
    mkdir installer-release
    "C:\Program Files (x86)\NSIS\makensis.exe" json-scada.nsi
	Or use the NSIS GUI app for better progress feedback.	
	This build can take hours to finish!
	

