# Instalador portátil (pen USB)

Leva a pasta do projeto na pen e instala o sistema noutro computador,
**sem internet** (binários pré-compilados) e **sem Go instalado**.

## Preparar a pen (uma vez, num computador com Go)

```bash
bash instalador/descarregar-cache.sh
```

- Constrói binários para Linux/Windows/macOS (amd64 e arm64) em `instalador/binarios/`.
- (Opcional) `bash instalador/descarregar-cache.sh --cache-toolchains` guarda também
  o toolchain Go em `instalador/cache/` — só se quiser compilar offline.

Depois copie a pasta do projeto inteira para a pen.

## Instalar no computador de destino

| Sistema | Ficheiro a abrir | Como |
|---|---|---|
| **Windows** | `instalar.bat` | Duplo clique |
| **Linux** | `instalar.sh` | `bash instalar.sh` |
| **macOS** | `instalar.sh` | `bash instalar.sh` |

O instalador:

1. copia o binário da pen para uma pasta local
   (Windows: `%LOCALAPPDATA%\WiFiController`; Linux/macOS: `~/wifi-controller`);
2. cria `config.env` editável (nome, moeda, prefixo de vales, porta, planos);
3. cria o **start**:
   - Windows: `WiFi Controller.bat` no Ambiente de Trabalho;
   - Linux: `WiFi Controller` no Ambiente de Trabalho (`.desktop`);
   - macOS: `WiFi Controller.command` no Ambiente de Trabalho;
4. ao clicar no start: arranca o servidor (se ainda não estiver a correr),
   espera responder e **abre o navegador** com o sistema pronto a usar.

## Primeiro uso (dentro do sistema)

1. O sistema arranca sem contas. Aberto no navegador:
   - se ainda não houver conta, crie-a via API (ver abaixo) — uma única vez;
2. depois, login normal no painel.

```bash
# Uma vez, com o sistema a correr:
curl -X POST http://localhost:8080/api/v1/auth/primeira-conta \
  -H 'Content-Type: application/json' \
  -d '{"nome":"Operador","email":"voce@exemplo.local","senha":"MudarEsta1"}'
```

## Parar

- Windows: `parar.bat` na pasta de instalação (ou fechar pelo Gestor de Tarefas).
- Linux/macOS: `~/wifi-controller/parar.sh`

## Desinstalar

- Windows: apagar `%LOCALAPPDATA%\WiFiController` + `WiFi Controller.bat` no Ambiente de Trabalho.
- Linux/macOS: apagar `~/wifi-controller` + o atalho no Ambiente de Trabalho.
