#!/usr/bin/env bash
# instalar.sh — instalador para Linux e macOS.
#
# Uso (a partir da pen, na pasta do projeto copiada):
#   bash instalar.sh
#
# O que faz:
#   1. escolhe o binário pré-compilado da pen para o seu sistema
#      (funciona SEM internet e SEM Go instalado);
#   2. se não houver binário, descarrega o Go e compila (precisa de internet);
#   3. instala em ~/wifi-controller: binário + ficheiro config.env + arrancador;
#   4. cria um start no Ambiente de Trabalho (clicável) que abre o navegador
#      com o software pronto a usar.
set -euo pipefail

AQUI="$(cd "$(dirname "$0")" && pwd)"
BINARIOS="$AQUI/binarios"
DESTINO="$HOME/wifi-controller"
NOME_SISTEMA="WiFi Controller"
PORTA=8080

DETECTAR_SO=$(uname -s)
DETECTAR_ARQ=$(uname -m)
case "$DETECTAR_SO" in
  Linux*) SO=linux ;;
  Darwin*) SO=darwin ;;
  MINGW*|MSYS*|CYGWIN*)
    # Git Bash no Windows: o instalador nativo é o .bat.
    echo "Windows detetado via Git Bash — a usar o instalador nativo..."
    exec cmd //c "$(cygpath -w "$AQUI/instalar.bat")"
    ;;
  *) echo "Sistema não suportado: $DETECTAR_SO"; exit 1 ;;
esac
case "$DETECTAR_ARQ" in
  x86_64|amd64) ARQ=amd64 ;;
  aarch64|arm64) ARQ=arm64 ;;
  *) echo "Arquitectura não suportada: $DETECTAR_ARQ"; exit 1 ;;
esac

echo "==> Sistema detetado: $SO/$ARQ"
mkdir -p "$DESTINO"

# --- 1. Binário -------------------------------------------------------------
BINARIO="$BINARIOS/wificontroller-${SO}-${ARQ}"
if [ -f "$BINARIO" ]; then
  echo "==> A copiar o binário pré-compilado da pen (sem internet)."
  cp "$BINARIO" "$DESTINO/wificontroller"
else
  echo "==> Sem binário pré-compilado: vou compilar. É preciso Go ou internet."
  # Tenta Go local; se não houver, descarrega o toolchain para a própria pasta.
  if ! command -v go >/dev/null 2>&1; then
    GO_VERSAO=1.27.1
    FICHEIRO="go${GO_VERSAO}.${SO}-${ARQ}.tar.gz"
    echo "==> A descarregar Go $GO_VERSAO..."
    curl -fL --retry 3 -o "/tmp/$FICHEIRO" "https://go.dev/dl/$FICHEIRO"
    mkdir -p "$DESTINO/toolchain"
    tar -C "$DESTINO/toolchain" -xzf "/tmp/$FICHEIRO"
    export PATH="$DESTINO/toolchain/go/bin:$PATH"
  fi
  (cd "$AQUI/.." && CGO_ENABLED=0 go build -o "$DESTINO/wificontroller" ./cmd/server)
fi
chmod +x "$DESTINO/wificontroller"

# --- 2. Configuração local ---------------------------------------------------
if [ ! -f "$DESTINO/config.env" ]; then
  cat > "$DESTINO/config.env" <<EOF
# Configuração do WiFi Controller — edite e reinicie para aplicar.
WIFIC_NOME=WiFi Controller
WIFIC_MOEDA=
WIFIC_PREFIXO_VALE=VA
WIFIC_PORTA=$PORTA
WIFIC_BANCO=$DESTINO/data.db
WIFIC_MIGRACOES=$AQUI/../migrations
WIFIC_PLANOS_INICIAIS=
EOF
  # Migrações junto ao binário para instalações sem a pen ligada.
  mkdir -p "$DESTINO/migrations"
  cp "$AQUI"/../migrations/*.sql "$DESTINO/migrations/"
  sed -i.bak "s|WIFIC_MIGRACOES=$AQUI/../migrations|WIFIC_MIGRACOES=$DESTINO/migrations|" "$DESTINO/config.env" && rm -f "$DESTINO/config.env.bak"
  echo "==> config.env criado em $DESTINO/config.env (edite à vontade)."
fi

# --- 2b. Frontend compilado (sem ele o navegador não tem página) -------------
# O servidor procura web/frontend/dist a partir da pasta de execução.
if [ -d "$AQUI/../web/frontend/dist" ]; then
  rm -rf "$DESTINO/web/frontend/dist"
  mkdir -p "$DESTINO/web/frontend/dist"
  cp -r "$AQUI/../web/frontend/dist/." "$DESTINO/web/frontend/dist/"
  echo "==> Frontend copiado para $DESTINO/web/frontend/dist"
else
  echo "==> AVISO: web/frontend/dist não existe na pen; a API funcionará sem painel."
fi

# --- 3. Arrancador (start.sh) ----------------------------------------------
cat > "$DESTINO/start.sh" <<'EOF'
#!/usr/bin/env bash
# start.sh — arranca o WiFi Controller e abre o navegador.
cd "$(dirname "$0")"
PORTA=$(grep -E '^WIFIC_PORTA=' config.env 2>/dev/null | cut -d= -f2)
PORTA=${PORTA:-8090}

# Já está a correr? Só abre o navegador.
if curl -s -m 2 "http://localhost:$PORTA/api/v1/auth/saude" >/dev/null 2>&1; then
  URL="http://localhost:$PORTA"
  case "$(uname -s)" in
    Darwin*) open "$URL" ;;
    *) xdg-open "$URL" 2>/dev/null || sensible-browser "$URL" 2>/dev/null || firefox "$URL" 2>/dev/null || echo "Abra $URL no navegador." ;;
  esac
  echo "O sistema já estava a correr em $URL"
  exit 0
fi

echo "A arrancar o WiFi Controller..."
nohup ./wificontroller >> run.log 2>&1 &
echo $! > run.pid

# Espera o servidor responder e abre o navegador.
for i in $(seq 1 30); do
  if curl -s -m 2 "http://localhost:$PORTA/api/v1/auth/saude" >/dev/null 2>&1; then
    URL="http://localhost:$PORTA"
    case "$(uname -s)" in
      Darwin*) open "$URL" ;;
      *) xdg-open "$URL" 2>/dev/null || sensible-browser "$URL" 2>/dev/null || firefox "$URL" 2>/dev/null || echo "Abra $URL no navegador." ;;
    esac
    echo ""
    echo "✔ A correr em $URL"
    echo "  Para parar: ./parar.sh"
    exit 0
  fi
  sleep 1
done

echo "O servidor não respondeu em 30s. Veja o log: tail run.log"
exit 1
EOF
chmod +x "$DESTINO/start.sh"

# --- 4. Parar ---------------------------------------------------------------
cat > "$DESTINO/parar.sh" <<'EOF'
#!/usr/bin/env bash
# parar.sh — para o WiFi Controller.
cd "$(dirname "$0")"
if [ -f run.pid ]; then
  PID=$(cat run.pid)
  if kill "$PID" 2>/dev/null; then
    echo "Parado (PID $PID)."
  fi
  rm -f run.pid
else
  pkill -f wificontroller 2>/dev/null && echo "Parado." || echo "Não estava a correr."
fi
EOF
chmod +x "$DESTINO/parar.sh"

# --- 5. Atalho no Ambiente de Trabalho -------------------------------------
AREA="$HOME/Desktop"
[ -d "$HOME/Área de Trabalho" ] && AREA="$HOME/Área de Trabalho"
if [ -d "$AREA" ]; then
  if [ "$(uname -s)" = "Darwin" ]; then
    cat > "$AREA/WiFi Controller.command" <<EOF
#!/usr/bin/env bash
exec "$DESTINO/start.sh"
EOF
    chmod +x "$AREA/WiFi Controller.command"
  else
    cat > "$AREA/wifi-controller.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=WiFi Controller
Comment=Arranca o sistema e abre o navegador
Exec=$DESTINO/start.sh
Icon=network-wireless
Terminal=true
Categories=Network;
EOF
    chmod +x "$AREA/wifi-controller.desktop"
  fi
  echo "==> Atalho \"WiFi Controller\" criado no Ambiente de Trabalho."
else
  echo "==> Ambiente de Trabalho não encontrado; use $DESTINO/start.sh"
fi

echo ""
echo "✔ Instalação concluída em $DESTINO"
echo "  Clique em \"WiFi Controller\" no Ambiente de Trabalho, ou corra:"
echo "  $DESTINO/start.sh"
