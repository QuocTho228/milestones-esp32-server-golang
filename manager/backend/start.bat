@echo off
setlocal enabledelayedexpansion
echo === Script khoi dong backend he thong quan ly Milestones ===

REM Kiem tra tham so
if "%1"=="help" goto :help
if "%1"=="-h" goto :help
if "%1"=="--help" goto :help

REM Thiet lap duong dan file cau hinh
set CONFIG_FILE=manager/backend/config/config.json
set RESET_DB=

if "%1"=="dev" (
    set CONFIG_FILE=manager/backend/config/config.dev.json
    echo Su dung cau hinh moi truong development: %CONFIG_FILE%
) else if "%1"=="prod" (
    set CONFIG_FILE=manager/backend/config/config.prod.json
    echo Su dung cau hinh moi truong production: %CONFIG_FILE%
) else if "%1"=="reset" (
    set RESET_DB=-reset-db
    echo Reset lai database va dung cau hinh mac dinh: %CONFIG_FILE%
) else if "%1"=="reset-dev" (
    set CONFIG_FILE=manager/backend/config/config.dev.json
    set RESET_DB=-reset-db
    echo Reset lai database va dung cau hinh moi truong development: %CONFIG_FILE%
) else if "%1"=="custom" (
    if "%2"=="" (
        echo Loi: vui long chi dinh duong dan file cau hinh
        echo Cach su dung: start.bat custom config.json
        pause
        exit /b 1
    )
    set CONFIG_FILE=%2
    echo Su dung cau hinh tuy chinh: %CONFIG_FILE%
) else if "%1"=="" (
    echo Su dung cau hinh mac dinh: %CONFIG_FILE%
) else (
    echo Tham so khong xac dinh: %1
    echo Dung 'start.bat help' de xem tro giup
    pause
    exit /b 1
)

REM Kiem tra file cau hinh co ton tai hay khong
if not exist "%CONFIG_FILE%" (
    echo Loi: file cau hinh khong ton tai: %CONFIG_FILE%
    pause
    exit /b 1
)

REM Vao thu muc backend
cd manager\backend

REM Cai dat dependency
echo Dang cai dat dependency Go...
go mod tidy

REM Khoi dong service
echo Dang khoi dong service...
if not "%RESET_DB%"=="" (
    echo Canh bao: database se bi reset, toan bo du lieu se bi xoa!
    set /p confirm=Ban co chac chan muon tiep tuc khong? (y/N): 
    if not "!confirm!"=="y" if not "!confirm!"=="Y" (
        echo Da huy thao tac
        pause
        exit /b 0
    )
    go run main.go -config="..\..\%CONFIG_FILE%" %RESET_DB%
) else (
    go run main.go -config="..\..\%CONFIG_FILE%"
)
goto :end

:help
echo Cach su dung:
echo   start.bat                    # Su dung file cau hinh mac dinh
echo   start.bat dev                # Su dung cau hinh moi truong development
echo   start.bat prod               # Su dung cau hinh moi truong production
echo   start.bat custom config.json # Su dung file cau hinh tuy chinh
echo   start.bat reset              # Reset lai database va dung cau hinh mac dinh
echo   start.bat reset-dev          # Reset lai database va dung cau hinh moi truong development
echo   start.bat help               # Hien thi thong tin tro giup

:end
pause