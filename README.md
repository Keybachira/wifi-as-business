# WiFi Controller

Controlador de acesso WiFi para pequenos negócios (hotéis, cafés, escritórios).
Permite vender **vales** de acesso por tempo e dados, controlar **sessões** de
clientes, gerir a rede e integrá-se com routers **MikroTik** via RADIUS/API.

---

## Estrutura

```
wifi-as-business/
├── cmd/
│   └── server/
│       └── main.go          # arranque: configuração, DI, rotas, manutenção
├── internal/
│   ├── auth/                # contas de acesso ao painel (sem papéis de privilégio)
│   ├── vouchers/            # vales: geração em lote, ativação, revogação, expiração
│   ├── plans/               # planos de acesso (preço, duração, velocidade, dados)
│   ├── sessions/            # sessões de clientes, desconexão, expiração automática
│   ├── users/               # utilizadores registados no portal
│   ├── networks/            # redes WiFi e respetiva topologia
│   ├── devices/             # inventário de routers, switches e pontos de acesso
│   ├── radius/              # validação de credenciais: serviço e servidor RADIUS
│   ├── mikrotik/            # cliente da API REST do RouterOS
│   ├── pfsense/             # cliente da API REST do pfSense
│   ├── limitadores/         # agregador de routers sob gestão
│   ├── portal/              # configuração do portal cativo
│   ├── analytics/           # métricas agregadas para o painel
│   ├── system/              # configuração, erros HTTP, servidor, middleware
│   └── database/            # ligação SQLite e migrações versionadas
├── migrations/              # ficheiros .sql aplicados por ordem de versão
├── web/
│   └── frontend/            # aplicação React (painel de administração e portal)
├── config/                  # configuração de exemplo
├── data/                    # controller.db (SQLite)
├── go.mod
└── README.md
```

Cada módulo segue a mesma divisão em camadas:

| Ficheiro | Responsabilidade |
|---|---|
| `modelo.go` / `<módulo>.go` | Tipos, regras de validação, formatação |
| `repositorio.go` | Acesso ao SQLite (SQL isolado do resto do código) |
| `servico.go` | Lógica de negócio, permissões, efeitos colaterais |
| `handler.go` | Rotas HTTP, leitura do pedido, tradução de erros |

---

O servidor é **genérico e funciona 100% local**: nome, moeda e prefixo dos
vales são configuráveis, os dados ficam num ficheiro SQLite local e não há
conta imposta no arranque — a primeira conta é criada por si.

## Requisitos

- **Go 1.22+**
- **SQLite** através do driver [`modernc.org/sqlite`](https://modernc.org/sqlite)
  (escrito em Go puro, **não precisa de CGO nem de compilador C**)

## Arranque

```bash
go mod download
go run ./cmd/server
```

Ou compilar:

```bash
go build -o wificontroller.exe ./cmd/server
./wificontroller.exe
```

O servidor escuta em `http://localhost:8080` por omissão.

Na primeira execução são criados automaticamente:

- o ficheiro `data/controller.db` com todas as tabelas;
- **nada mais**: não há contas nem planos predefinidos.

### Primeira conta (sem "dono")

Não existe papel de dono nem conta criada pelo servidor. Duas formas:

1. **Pela API** (recomendado) — só funciona enquanto não existir nenhuma conta:

```bash
curl -s -X POST localhost:8080/api/v1/auth/primeira-conta \
  -H 'Content-Type: application/json' \
  -d '{"nome":"Operador","email":"operador@exemplo.local","senha":"MudarEsta1"}'
```

Para saber se o sistema ainda precisa de configuração:
`GET /api/v1/auth/estado-inicial` → `{"precisa_configuracao":true}`.

2. **Por configuração** — para arranques desacompanhados:

```bash
WIFIC_ADMIN_EMAIL=operador@exemplo.local
WIFIC_ADMIN_SENHA=MudarEsta1
```

### Configuração

Toda a configuração vem de variáveis com o prefixo `WIFIC_`. Além do ambiente,
o servidor lê **um ficheiro local** no arranque — o primeiro que existir:

1. o caminho indicado em `WIFIC_FICHEIRO`;
2. `config.env` na pasta de execução;
3. `.env` na pasta de execução.

Variáveis já definidas no ambiente **não** são sobrescritas pelo ficheiro.
Modelo completo em [`config/config.example.env`](config/config.example.env).

| Variável | Omissão | Descrição |
|---|---|---|
| `WIFIC_NOME` | `WiFi Controller` | Nome do sistema (saúde, logs) |
| `WIFIC_MOEDA` | vazia | Moeda exibida (ex.: `Kz`, `USD`); vazia = sem símbolo |
| `WIFIC_PREFIXO_VALE` | `VA` | Prefixo dos códigos de vale (1–4 caracteres) |
| `WIFIC_PLANOS_INICIAIS` | vazia | Planos criados no arranque (ver abaixo) |
| `WIFIC_ADMIN_EMAIL` / `WIFIC_ADMIN_SENHA` | vazias | Cria a primeira conta no arranque |
| `WIFIC_FICHEIRO` | — | Ficheiro de configuração local a usar |
| `WIFIC_PORTA` | `8080` | Porta do servidor HTTP |
| `WIFIC_ENDERECO` | `0.0.0.0` | Interface de escuta |
| `WIFIC_BANCO` | `data/controller.db` | Ficheiro SQLite |
| `WIFIC_MIGRACOES` | `migrations` | Diretório das migrações |
| `WIFIC_AMBIENTE` | `desenvolvimento` | `producao` ativa validações mais estritas |
| `WIFIC_DURACAO_SESSAO` | `12h` | Validade do token de acesso |
| `WIFIC_CUSTO_HASH` | `12` | Custo do bcrypt (mínimo 12 em produção) |
| `WIFIC_MIKROTIK_ENDERECO` | `192.168.88.1` | Endereço do router |
| `WIFIC_MIKROTIK_USUARIO` | `admin` | Utilizador da API do router |
| `WIFIC_MIKROTIK_SENHA` | — | Senha da API (vazia = integração desativada) |
| `WIFIC_MIKROTIK_PORTA_API` | `8728` | Porta da API do router |
| `WIFIC_RADIUS_ENDERECO` | `127.0.0.1` | Endereço onde o servidor RADIUS escuta |
| `WIFIC_RADIUS_PORTA` | `1812` | Porta UDP do servidor RADIUS |
| `WIFIC_RADIUS_SEGREDO` | — | Segredo partilhado com o router (vazio = servidor UDP desativado) |
| `WIFIC_PFSENSE_ENDERECO` | — | Endereço do pfSense, com esquema e porta (vazio = integração desativada) |
| `WIFIC_PFSENSE_USUARIO` | `admin` | Utilizador da API do pfSense |
| `WIFIC_PFSENSE_SENHA` | — | Senha de acesso, quando não se usa chave de API |
| `WIFIC_PFSENSE_CHAVE_API` | — | Chave de API do pfSense; tem precedência sobre utilizador/senha |
| `WIFIC_PFSENSE_ALIAS` | — | Alias do pfSense que autoriza os IPs dos clientes |

**Planos iniciais opcionais** (`WIFIC_PLANOS_INICIAIS`), separados por vírgula:

```
Nome;preco_centavos;duracao_min;velocidade_kbps[;dados_mb][;max_dispositivos]
```

Exemplo:

```bash
WIFIC_PLANOS_INICIAIS="1 Hora;100;60;5000;1024;1, 1 Dia;500;1440;20000;8192;2"
```

Sem esta variável o sistema arranca sem planos — crie-os via API ou painel.

Em `producao` o arranque **falha** se o segredo JWT estiver no valor
predefinido ou se o custo do hash for inferior a 12.

---

## Frontend

O painel de administração e o portal cativo são uma aplicação React + TypeScript
em `web/frontend/`, servida pelo próprio backend em produção.

### Desenvolvimento

Dois terminais. Primeiro o backend:

```bash
go run ./cmd/server          # API em http://localhost:8080
```

Depois o frontend, com recarga automática:

```bash
cd web/frontend
npm install
npm run dev                  # http://localhost:5173
```

O Vite encaminha todas as chamadas `/api` para `http://localhost:8080`, por isso
não há CORS nem URLs configuradas na aplicação. Para apontar a outro backend:

```bash
VIFIC_API=http://192.168.1.10:8080 npm run dev
```

### Produção

```bash
cd web/frontend
npm run build                 # gera web/frontend/dist
```

Depois basta arrancar o backend: ele deteta `web/frontend/dist` e serve a
aplicação em `/`, com a API no mesmo endereço e porta. Não é preciso nenhum
servidor web adicional.

Detalhe do cache em produção:

- `index.html` é servido com `no-cache`, para que um novo build chegue
  imediatamente aos utilizadores;
- os ficheiros em `assets/` têm o hash do conteúdo no nome e são cacheados
  durante um ano (`immutable`).

Qualquer rota desconhecida devolve o `index.html`, para que recarregar uma página
interna da aplicação não dê 404. A pasta de código-fonte **nunca** é servida.

### Scripts

| Script | O que faz |
|---|---|
| `npm run dev` | Servidor de desenvolvimento com HMR e proxy para a API |
| `npm run build` | Compila para `dist/` |
| `npm run preview` | Serve `dist/` localmente, também com proxy |
| `npm run typecheck` | `tsc --noEmit` |

---

## API

Prefixo: `/api/v1`. Salvo indicação em contrário, todas as rotas exigem o cabeçalho:

```
Authorization: Bearer <token>
```

### Rotas públicas

| Método | Caminho | Descrição |
|---|---|---|
| `GET` | `/api/v1/auth/saude` | Estado de saúde do serviço |
| `POST` | `/api/v1/auth/login` | Autenticar e obter token |
| `GET` | `/api/v1/portal/publico` | Aparência do portal cativo |
| `GET` | `/api/v1/publico/portal?codigo=VA-XXXX-XXXX` | Validar um código (consulta) |
| `POST` | `/api/v1/publico/validar` | Validar um código sem o consumir |
| `POST` | `/api/v1/publico/sessao` | Autenticar cliente e abrir sessão |

### Autenticação e contas

| Método | Caminho | Papel |
|---|---|---|
| `GET` | `/api/v1/auth/estado-inicial` | público |
| `POST` | `/api/v1/auth/primeira-conta` | público (só sem contas) |
| `POST` | `/api/v1/auth/logout` | qualquer |
| `GET` | `/api/v1/auth/sessao` | qualquer |
| `GET` | `/api/v1/contas` | qualquer |
| `POST` | `/api/v1/contas` | qualquer conta |
| `PATCH` | `/api/v1/contas/{id}/estado` | qualquer conta (não a própria) |
| `PATCH` | `/api/v1/contas/{id}/papel` | qualquer conta |

### Planos, vales e sessões

| Método | Caminho |
|---|---|
| `GET` `POST` | `/api/v1/planos` |
| `GET` `PATCH` `DELETE` | `/api/v1/planos/{id}` |
| `GET` | `/api/v1/vales` |
| `POST` | `/api/v1/vales/lote` |
| `GET` | `/api/v1/vales/resumo` |
| `GET` | `/api/v1/vales/{id}` |
| `POST` | `/api/v1/vales/{id}/revogar` |
| `GET` | `/api/v1/sessoes` |
| `GET` | `/api/v1/sessoes/ativas` |
| `GET` `DELETE` | `/api/v1/sessoes/{id}` |

### Rede, portal e análises

| Método | Caminho |
|---|---|
| `GET` `POST` | `/api/v1/redes` |
| `GET` `DELETE` | `/api/v1/redes/{id}` |
| `POST` | `/api/v1/redes/{id}/alternar` |
| `GET` `POST` | `/api/v1/dispositivos` |
| `GET` `DELETE` | `/api/v1/dispositivos/{id}` |
| `PATCH` | `/api/v1/dispositivos/{id}/telemetria` |
| `GET` | `/api/v1/dispositivos/resumo` |
| `GET` `POST` | `/api/v1/utilizadores` |
| `GET` `DELETE` | `/api/v1/utilizadores/{id}` |
| `PATCH` | `/api/v1/utilizadores/{id}/estado` |
| `GET` `PATCH` | `/api/v1/portal` |
| `POST` | `/api/v1/portal/publicar` |
| `GET` | `/api/v1/analises/resumo` |
| `GET` | `/api/v1/analises/painel?periodo=7` |

### Formato dos erros

```json
{
  "erro": {
    "codigo": "conflito",
    "mensagem": "Já existe um plano com este nome",
    "campo": ""
  }
}
```

### Exemplo

```bash
# Autenticar
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"operador@exemplo.local","senha":"MudarEsta1"}' | jq -r .token)

# Primeiro uso: criar a conta inicial (só funciona sem contas existentes)
# curl -s -X POST localhost:8080/api/v1/auth/primeira-conta \
#   -H 'Content-Type: application/json' \
#   -d '{"nome":"Operador","email":"operador@exemplo.local","senha":"MudarEsta1"}'

# Gerar 50 vales do plano 1
curl -s -X POST localhost:8080/api/v1/vales/lote \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"plano_id":1,"quantidade":50,"validade_dias":90}'

# Autenticar um cliente a partir do portal cativo
curl -s -X POST localhost:8080/api/v1/publico/sessao \
  -H 'Content-Type: application/json' \
  -d '{"codigo":"VA-XXXX-XXXX","endereco_ip":"192.168.1.50',
       "endereco_mac":"DC:A6:32:01:77:2E","tipo_dispositivo":"iphone"}'
```

---

## Decisões de conceção

**Dinheiro em centavos.** `preco_centavos` é um inteiro, nunca vírgula
flutuante. `10000` = 100 unidades da moeda configurada. Evita erros de arredondamento.

**Códigos de vale ofuscados.** Alfabeto sem `0/O` e `1/I`, formato `VA-XXXX-XXXX`
(prefixo configurável via `WIFIC_PREFIXO_VALE`), gerados com `crypto/rand`
(não `math/rand`).

**Um vale, uma utilização.** A ativação faz `UPDATE ... WHERE estado = 'disponivel'`
e verifica as linhas afetadas. Dois pedidos simultâneos com o mesmo código: só um
consegue, o outro recebe "código já utilizado".

**Senhas com bcrypt** e verificação em tempo constante, para não revelar quais
emails existem (timing attack). O token de sessão é opaco, guardado apenas como
SHA-256 — um dump da base de dados não permite fazer login.

**Sem dono, sem privilégios fixos.** Todas as contas ativas têm os mesmos
direitos. Como salvaguarda, uma conta não pode desativar a própria conta com
sessão iniciada; trocar a senha invalida as outras sessões.

**Consultas parametrizadas** em todo o lado. A única concatenação de SQL é na
construção dinâmica de filtros, onde apenas colunas e operadores conhecidos são
interpolados — nunca valores do utilizador.

**Migrações incrementais.** Os ficheiros `NNNN_descricao.sql` são aplicados por
ordem e registados em `registo_migracoes`. Cada migração corre numa transação:
ou aplica inteira, ou não aplica nada.

**Dinheiro e tempos explícitos.** Durações em minutos, datas em UTC, para evitar
ambiguidades de fuso horário.

## Tarefas periódicas

De 5 em 5 minutos o servidor:

1. remove tokens de sessão expirados;
2. marca como expirados os vales cujo prazo terminou;
3. termina as sessões cujo plano já não é válido.

## Segurança

- Senhas com bcrypt (custo 12 por omissão, mínimo obrigatório em produção)
- Token de sessão opaco, armazenado apenas como SHA-256
- CORS restrito à origem que faz o pedido
- Cabeçalhos `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`
- Corpo dos pedidos limitado a 1 MB
- Consultas parametrizadas
- Prazos de expiração em todas as respostas
- O arranque falha em `producao` se as credenciais por omissão não forem substituídas

## Estado atual

Verificado ponta a ponta com o servidor em execução:

- arranque, migrações e primeira conta via API;
- autenticação, papéis e contas de equipa;
- planos, geração de vales em lote, validação e **uso único**;
- autenticação de clientes pelo portal, com velocidade e prazo herdados do plano;
- redes, utilizadores, portal cativo e análises;
- dispositivos e telemetria (`PATCH /telemetria` grava todos os campos);
- servidor RADIUS em UDP a autenticar vales reais;
- migração `0002` a reconstruir a tabela preservando os dados existentes;
- manutenção periódica e encerramento gracioso (SIGINT/SIGTERM).

A integração com o router MikroTik está implementada (API REST do RouterOS) e
desativa-se sozinha quando não há credenciais configuradas.

### Testes

```bash
go test ./...
```

Cobrem o protocolo RADIUS (cifra, timeout, rejeição, lixo em UDP), a integração
com o pfSense (ARP, alias, Traffic Shaper) e os agregadores de equipamento.
O módulo `devices` tem testes que documentam o comportamento do driver SQLite
perante quebras de linha em SQL — a regra que causou um erro enganador.

## Routers suportados

O controlador sabe falar com dois tipos de equipamento:

| Equipamento | Papel | Estado |
|---|---|---|
| **MikroTik** | router principal | Implementado |
| **pfSense** | alternativa, em paralelo | Implementado |

O inventário de dispositivos aceita `router`, `switch`, `ponto_acesso` e
`firewall`, para que um pfSense seja registado ao lado dos routers MikroTik.

O MikroTik continua a ser o router principal. O pfSense é uma segunda opção
que entra apenas quando `WIFIC_PFSENSE_ENDERECO` e uma credencial estão
definidas; sem elas o arranque é idêntico ao de sempre.

Os dois implementam a mesma interface (`LimitadorRede`), e um agregador
notifica **ambos** em cada desconexão. Se um falhar, o outro continua a ser
avisado e a sessão é dada como terminada — um router em baixo não pode
impedir que o outro corte a navegação.

A aplicação da velocidade do plano segue um caminho separado, porque os
equipamentos não falam a mesma língua: o pfSense limita por endereço IP e o
MikroTik por endereço MAC. Um agregador traduz o IP da sessão para MAC
(consultando a tabela ARP) antes de mandar a regra ao router principal.
A aplicação acontece **depois** de a sessão estar registada: uma falha do
equipamento deixa o cliente ligado, e só avisa no registo.

### Como o pfSense desliga um cliente

O pfSense não expõe sessões de clientes fora do captive portal, por isso a
desconexão é feita pela tabela ARP:

1. o endereço MAC é procurado na tabela ARP para obter o IP;
2. esse IP é removido do alias configurado em `WIFIC_PFSENSE_ALIAS`.

É esse alias — e não o estado interno do firewall — que governa quem navega.
Sem ele, o arranque avisa e a desconexão falha com uma mensagem explícita.

### Validação de vouchers por RADIUS

O servidor RADIUS escuta em UDP (`WIFIC_RADIUS_PORTA`, 1812 por omissão) e
implementa o suficiente do RFC 2865 para um pfSense ou MikroTik validar vales:

- descodifica o `Access-Request` e valida a cifra do `User-Password`;
- consulta o vale na base de dados (o mesmo do portal cativo);
- responde `Access-Accept` com `Session-Timeout` em segundos — é este valor
  que faz o firewall desligar o cliente sozinho quando o plano acaba;
- ou `Access-Reject`, sem mensagem, para não revelar que o vale existe;
- responde `Accounting-Response` a pedidos de fecho de sessão.

O mapeamento usado é: `User-Name` = endereço MAC do equipamento,
`User-Password` = código do vale.

O servidor só arranca com `WIFIC_RADIUS_SEGREDO` definido. Sem esse segredo
partilhado não há forma de validar um pedido, e abrir a porta 1812 seria apenas
uma superfície de ataque — por isso fica desligado e o portal cativo continua a
funcionar normalmente. O segredo tem de ser **igual** ao configurado no router:
se divergirem, a senha não se descodifica e o acesso é recusado, mas o motivo
não é óbvio do lado do router.

Para testar contra um servidor a correr, existe uma sonda independente:

```bash
go run ./cmd/radiusprobe \
  -endereco 127.0.0.1:1812 \
  -segredo 'segredo-partilhado' \
  -utilizador DC:A6:32:01:77:2E \
  -vale VA-XXXX-XXXX
```

Devolve `Access-Accept` com o `Session-Timeout`, ou `Access-Reject`.

### Limitação conhecida — accounting sem persistência

O `Accounting-Request` é aceite e respondido, mas os octetos e o tempo
consumidos **não são gravados**. A cobrança pelo uso real depende de o
`dados_mb` das sessões passar a ser preenchido. O plano tem `limite_dados_mb`,
mas nada o aplica hoje ao consumo.
