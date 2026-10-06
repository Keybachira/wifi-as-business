@echo off
rem instalar.bat - instalador Windows (clicavel, da pen)
rem
rem Uso: duplo clique a partir da pasta do projeto copiada da pen.
rem
rem O que faz:
rem   1. copia o binario pre-compilado da pen para %LOCALAPPDATA%\WiFiController
rem      (funciona SEM internet e SEM Go instalado);
rem   2. se nao houver binario, descarrega o Go e compila (precisa de internet);
rem   3. cria config.env, start.bat e parar.bat;
rem   4. cria um atalho "WiFi Controller" no Ambiente de Trabalho que arranca
rem      o sistema e abre o navegador.

setlocal enabledelayedexpansion
set "AQUI=%~dp0"
set "BINARIOS=%AQUI%binarios\"
set "DESTINO=%LOCALAPPDATA%\WiFiController"
set "PORTA=8080"

echo === WiFi Controller - Instalador Windows ===
echo.

rem --- Arquitectura -----------------------------------------------------------
set "ARQ=amd64"
if "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "ARQ=arm64"
echo Sistema detetado: windows/%ARQ%

rem --- Pasta de destino -------------------------------------------------------
if not exist "%DESTINO%" mkdir "%DESTINO%"

rem --- 1. Binario --------------------------------------------------------------
if exist "%BINARIOS%wificontroller-windows-%ARQ%.exe" (
  echo A copiar o binario pre-compilado da pen...
  copy /Y "%BINARIOS%wificontroller-windows-%ARQ%.exe" "%DESTINO%\wificontroller.exe" >nul
) else (
  echo Nao ha binario pre-compilado: vou compilar com o Go.
  where go >nul 2>&1
  if errorlevel 1 (
    echo A descarregar o Go...
    set "GOVER=1.27.1"
    set "GOZIP=%TEMP%\go%GOVER%.windows-%ARQ%.zip"
    if not exist "%GOZIP%" curl -fL --retry 3 -o "%GOZIP%" "https://go.dev/dl/go%GOVER%.windows-%ARQ%.zip"
    if not exist "%DESTINO%\toolchain" mkdir "%DESTINO%\toolchain"
    tar -xf "%GOZIP%" -C "%DESTINO%\toolchain"
    set "PATH=%DESTINO%\toolchain\go\bin;%PATH%"
  )
  pushd "%AQUI%.."
  set "GOPATH=%DESTINO%\gopath"
  set "GOCACHE=%DESTINO%\gocache"
  call go build -o "%DESTINO%\wificontroller.exe" ./cmd/server
  popd
)

if not exist "%DESTINO%\wificontroller.exe" (
  echo ERRO: nao foi possivel instalar o binario.
  pause
  exit /b 1
)

rem --- 2. Configuracao local -----------------------------------------------------
if not exist "%DESTINO%\config.env" (
  (
    echo # Configuracao do WiFi Controller - edite e reinicie para aplicar.
    echo WIFIC_NOME=WiFi Controller
    echo WIFIC_MOEDA=
    echo WIFIC_PREFIXO_VALE=VA
    echo WIFIC_PORTA=%PORTA%
    echo WIFIC_BANCO=%DESTINO%\data.db
    echo WIFIC_MIGRACOES=%DESTINO%\migrations
    echo WIFIC_PLANOS_INICIAIS=
  ) > "%DESTINO%\config.env"
  if not exist "%DESTINO%\migrations" mkdir "%DESTINO%\migrations"
  copy /Y "%AQUI%..\migrations\*.sql" "%DESTINO%\migrations\" >nul
  echo config.env criado em %DESTINO%\config.env
)

rem --- 2b. Frontend compilado (sem ele o navegador nao tem pagina) ---------------
rem O servidor procura web\frontend\dist a partir da pasta de execucao.
if exist "%AQUI%..\web\frontend\dist" (
  if exist "%DESTINO%\web\frontend\dist" rmdir /s /q "%DESTINO%\web\frontend\dist"
  mkdir "%DESTINO%\web\frontend\dist"
  xcopy /e /i /q /y "%AQUI%..\web\frontend\dist" "%DESTINO%\web\frontend\dist" >nul
  echo Frontend copiado para %DESTINO%\web\frontend\dist
) else (
  echo AVISO: web\frontend\dist nao existe na pen; a API funcionara sem painel.
)

rem --- 3. Arrancador (start.bat) ---------------------------------------------------
(
  echo @echo off
  echo cd /d "%%~dp0"
  echo rem start.bat - arranca o sistema e abre o navegador
  echo for /f "tokens=2 delims==" %%%%P in ^('findstr /b "WIFIC_PORTA=" config.env'^) do set PORTA=%%%%P
  echo set "PORTA=%%PORTA%%"
  echo if "%%PORTA%%"=="" set "PORTA=%PORTA%"
  echo curl -s -m 2 "http://localhost:%%PORTA%%/api/v1/auth/saude" ^>nul 2^>^&1
  echo if not errorlevel 1 goto ja_correndo
  echo echo A arrancar o WiFi Controller...
  echo start "" /b "%DESTINO%\wificontroller.exe"^>run.log 2^>^&1
  echo for /l %%%%I in (1,1,30^) do ^(
  echo   ping -n 2 127.0.0.1 ^>nul
  echo   curl -s -m 2 "http://localhost:%%PORTA%%/api/v1/auth/saude" ^>nul 2^>^&1
  echo   if not errorlevel 1 goto aberto
  echo ^)
  echo echo O servidor nao respondeu em 30s. Veja run.log
  echo pause
  echo exit /b 1
  echo :aberto
  echo start "" "http://localhost:%%PORTA%%"
  echo echo A correr em http://localhost:%%PORTA%%
  echo echo Para parar: parar.bat
  echo ping -n 4 127.0.0.1 ^>nul
  echo exit /b 0
  echo :ja_correndo
  echo start "" "http://localhost:%%PORTA%%"
  echo echo O sistema ja estava a correr em http://localhost:%%PORTA%%
  echo ping -n 4 127.0.0.1 ^>nul
) > "%DESTINO%\start.bat"

rem --- 4. Parar ----------------------------------------------------------------------
(
  echo @echo off
  echo cd /d "%%~dp0"
  echo taskkill /f /im wificontroller.exe ^>nul 2^>^&1
  echo echo Parado.
  echo ping -n 3 127.0.0.1 ^>nul
) > "%DESTINO%\parar.bat"

rem --- 5. Atalho no Ambiente de Trabalho ------------------------------------------------
rem O Windows não cria .lnk por script sem PowerShell; em vez disso criamos um
rem "start minimalista" clicavel: um .bat no Ambiente de Trabalho.
(
  echo @echo off
  echo call "%DESTINO%\start.bat"
) > "%USERPROFILE%\Desktop\WiFi Controller.bat"

echo.
echo === Instalacao concluida ===
echo Clique em "WiFi Controller.bat" no Ambiente de Trabalho para usar.
echo Pasta: %DESTINO%
pause
exit /b 0
