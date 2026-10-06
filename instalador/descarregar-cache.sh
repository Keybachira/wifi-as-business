#!/usr/bin/env bash
# descarregar-cache.sh — prepara a pen para instalações SEM internet.
#
# Corra isto UMA VEZ num computador com Go e internet (pode ser este):
#   bash descarregar-cache.sh
#
# O que faz:
#   1. constrói o binário do servidor para as plataformas comuns
#      (Linux, Windows e macOS, amd64/arm64) em instalador/binarios/;
#   2. com --cache-toolchains, descarrega também o toolchain Go para
#      instalador/cache/ (só necessário se quiser construir a partir do
#      código-fonte num computador sem Go).
#
# Depois disto, a pen instala o sistema em qualquer computador mesmo
# sem internet.
set -euo pipefail

AQUI="$(cd "$(dirname "$0")" && pwd)"
RAIZ="$(cd "$AQUI/.." && pwd)"
GO_VERSAO="${GO_VERSAO:-1.27.1}"
BINARIOS="$AQUI/binarios"
CACHE="$AQUI/cache"

mkdir -p "$BINARIOS" "$CACHE"

# --- 1. Binários multiplataforma -------------------------------------------
echo "==> A construir binários (pode levar alguns minutos)..."
for PLAT in "linux amd64" "linux arm64" "windows amd64" "windows arm64" "darwin amd64" "darwin arm64"; do
  set -- $PLAT
  SO=$1; ARQ=$2
  EXT=""
  [ "$SO" = "windows" ] && EXT=".exe"
  SAIDA="$BINARIOS/wificontroller-${SO}-${ARQ}${EXT}"
  echo "    $SO/$ARQ"
  (cd "$RAIZ" && CGO_ENABLED=0 GOOS="$SO" GOARCH="$ARQ" go build -trimpath -o "$SAIDA" ./cmd/server)
done
echo "    OK — binários em instalador/binarios/"

# --- 2. Toolchains Go (opcional, só para construir offline) ----------------
if [ "${1:-}" = "--cache-toolchains" ]; then
  for PLAT in "linux amd64" "linux arm64" "windows amd64" "darwin amd64" "darwin arm64"; do
    set -- $PLAT
    SO=$1; ARQ=$2
    if [ "$SO" = "windows" ]; then FICHEIRO="go${GO_VERSAO}.${SO}-${ARQ}.zip"; else FICHEIRO="go${GO_VERSAO}.${SO}-${ARQ}.tar.gz"; fi
    DESTINO="$CACHE/$FICHEIRO"
    if [ -s "$DESTINO" ]; then
      echo "==> $FICHEIRO já está em cache."
      continue
    fi
    echo "==> A descarregar $FICHEIRO..."
    curl -fL --retry 3 -o "$DESTINO" "https://go.dev/dl/$FICHEIRO"
  done
  # Módulos Go para construir sem internet (o frontend já vai compilado).
  (cd "$RAIZ" && go mod download) >/dev/null 2>&1 || true
  echo "==> Toolchains em instalador/cache/."
fi

echo ""
echo "Pen pronta. Copie a pasta do projeto inteira para a pen."
echo "No computador de destino, corra instalar.sh (Linux/macOS) ou instalar.bat (Windows)."
