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
│   ├── auth/                # contas, sessões de acesso, papéis (dono/operador/suporte)
│   ├── vouchers/            # vales: geração em lote, ativação, revogação, expiração
│   ├── plans/               # planos de acesso (preço, duração, velocidade, dados)
│   ├── sessions/            # sessões de clientes, desconexão, expiração automática
│   ├── users/               # utilizadores registados no portal
│   ├── networks/            # redes WiFi e respetiva topologia
│   ├── devices/             # inventário de routers, switches e pontos de acesso
│   ├── radius/              # validação de credenciais para o servidor RADIUS
│   ├── mikrotik/            # cliente da API REST do RouterOS
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
- uma conta de dono (credenciais indicadas no arranque — **anote-as**);
- quatro planos de acesso de exemplo (1 Hora, 3 Horas, 1 Dia, 7 Dias).

### Credenciais iniciais

Define-as para não depender dos valores gerados:

```bash
WIFIC_ADMIN_EMAIL=admin@kimbo.ao
WIFIC_ADMIN_SENHA=Kimbo2026
```

## Configuração

Todas as opções são variáveis de ambiente com o prefixo `WIFIC_`:

| Variável | Omissão | Descrição |
|---|---|---|
| `WIFIC_PORTA` | `8080` | Porta do servidor HTTP |
| `WIFIC_ENDERECO` | `0.0.0.0` | Interface de escuta |
| `WIFIC_BANCO` | `data/controller.db` | Ficheiro SQLite |
| `WIFIC_MIGRACOES` | `migrations` | Diretório das migrações |
| `WIFIC_AMBIENTE` | `desenvolvimento` | `producao` ativa validações mais estritas |
| `WIFIC_DURACAO_SESSAO` | `12h` | Validade do token de acesso |
| `WIFIC_CUSTO_HASH` | `12` | Custo do bcrypt (mínimo 12 em produção) |
| `WIFIC_ADMIN_EMAIL` | gerado | Email da conta de dono inicial |
| `WIFIC_ADMIN_SENHA` | gerada | Senha da conta de dono inicial |
| `WIFIC_MIKROTIK_ENDERECO` | `192.168.88.1` | Endereço do router |
| `WIFIC_MIKROTIK_USUARIO` | `admin` | Utilizador da API do router |
| `WIFIC_MIKROTIK_SENHA` | — | Senha da API (vazia = integração desativada) |
| `WIFIC_MIKROTIK_PORTA_API` | `8728` | Porta da API do router |
| `WIFIC_RADIUS_ENDERECO` | `127.0.0.1` | Endereço do servidor RADIUS |
| `WIFIC_RADIUS_PORTA` | `1812` | Porta do servidor RADIUS |

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
| `GET` | `/api/v1/publico/portal?codigo=ZG-XXXX-XXXX` | Validar um código (consulta) |
| `POST` | `/api/v1/publico/validar` | Validar um código sem o consumir |
| `POST` | `/api/v1/publico/sessao` | Autenticar cliente e abrir sessão |

### Autenticação e contas

| Método | Caminho | Papel |
|---|---|---|
| `POST` | `/api/v1/auth/logout` | qualquer |
| `GET` | `/api/v1/auth/sessao` | qualquer |
| `GET` | `/api/v1/contas` | qualquer |
| `POST` | `/api/v1/contas` | dono |
| `PATCH` | `/api/v1/contas/{id}/estado` | dono |
| `PATCH` | `/api/v1/contas/{id}/papel` | dono |

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
  -d '{"email":"admin@kimbo.ao","senha":"Kimbo2026"}' | jq -r .token)

# Gerar 50 vales do plano 1
curl -s -X POST localhost:8080/api/v1/vales/lote \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"plano_id":1,"quantidade":50,"validade_dias":90}'

# Autenticar um cliente a partir do portal cativo
curl -s -X POST localhost:8080/api/v1/publico/sessao \
  -H 'Content-Type: application/json' \
  -d '{"codigo":"ZG-XXXX-XXXX","endereco_ip":"192.168.1.50",
       "endereco_mac":"DC:A6:32:01:77:2E","tipo_dispositivo":"iphone"}'
```

---

## Decisões de conceção

**Dinheiro em centavos.** `preco_centavos` é um inteiro, nunca vírgula
flutuante. `10000` = 100 Kz. Evita erros de arredondamento.

**Códigos de vale ofuscados.** Alfabeto sem `0/O` e `1/I`, formato `ZG-XXXX-XXXX`,
gerados com `crypto/rand` (não `math/rand`).

**Um vale, uma utilização.** A ativação faz `UPDATE ... WHERE estado = 'disponivel'`
e verifica as linhas afetadas. Dois pedidos simultâneos com o mesmo código: só um
consegue, o outro recebe "código já utilizado".

**Senhas com bcrypt** e verificação em tempo constante, para não revelar quais
emails existem (timing attack). O token de sessão é opaco, guardado apenas como
SHA-256 — um dump da base de dados não permite fazer login.

**Proteção contra lockout.** A última conta de dono não pode ser desativada nem
rebaixada, e trocar a senha invalida as outras sessões.

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

- arranque, migrações e dados iniciais (conta de dono + 4 planos);
- autenticação, papéis e contas de equipa;
- planos, geração de vales em lote, validação e **uso único**;
- autenticação de clientes pelo portal, com velocidade e prazo herdados do plano;
- redes, utilizadores, portal cativo e análises;
- manutenção periódica e encerramento gracioso (SIGINT/SIGTERM).

A integração com o router MikroTik está implementada (API REST do RouterOS) e
desativa-se sozinha quando não há credenciais configuradas.

### Limitação conhecida

O endpoint `PATCH /api/v1/dispositivos/{id}/telemetria` falha com
`no such column: tempo_actividade` quando o servidor corre sobre a base de dados
criada pelo mesmo arranque, embora a coluna exista e a mesma instrução funcione
executada diretamente sobre o ficheiro. O mesmo código, isolado fora do servidor
de longa duração, funciona corretamente — o problema não está no SQL nem na
estrutura da tabela. Requer investigação num ambiente com disco e toolchain
estáveis antes de ser considerado resolvido.
