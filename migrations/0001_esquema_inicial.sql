-- 0001_esquema_inicial.sql
-- Esquema base do WiFi Controller: contas, redes, planos, vales, sessões e portal.
--------------------------------------------------------------------------------
-- Contas de administração (equipa do painel)
--------------------------------------------------------------------------------
CREATE TABLE contas (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    nome          TEXT    NOT NULL,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    senha_hash    TEXT    NOT NULL,
    papel         TEXT    NOT NULL DEFAULT 'operador'
                  CHECK (papel IN ('dono', 'operador', 'suporte')),
    estado        TEXT    NOT NULL DEFAULT 'ativo'
                  CHECK (estado IN ('ativo', 'desativado')),
    ultimo_acesso TIMESTAMP,
    criado_em     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE sessoes_admin (
    token_hash  TEXT PRIMARY KEY,
    conta_id    INTEGER NOT NULL REFERENCES contas(id) ON DELETE CASCADE,
    agente      TEXT,
    endereco_ip TEXT,
    expira_em   TIMESTAMP NOT NULL,
    criado_em   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sessoes_admin_conta ON sessoes_admin(conta_id);
CREATE INDEX idx_sessoes_admin_expira ON sessoes_admin(expira_em);
--------------------------------------------------------------------------------
-- Redes
--------------------------------------------------------------------------------
CREATE TABLE redes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    nome       TEXT    NOT NULL UNIQUE,
    descricao  TEXT    NOT NULL DEFAULT '',
    subnet     TEXT    NOT NULL DEFAULT '',
    estado     TEXT    NOT NULL DEFAULT 'ativa' CHECK (estado IN ('ativa', 'inativa')),
    criado_em  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
--------------------------------------------------------------------------------
-- Vales (vouchers) de acesso
--------------------------------------------------------------------------------
CREATE TABLE vales (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    codigo     TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    plano_id   INTEGER NOT NULL REFERENCES planos(id) ON DELETE RESTRICT,
    estado     TEXT    NOT NULL DEFAULT 'disponivel'
               CHECK (estado IN ('disponivel', 'ativo', 'expirado', 'revogado')),
    expira_em  TIMESTAMP,
    usado_em   TIMESTAMP,
    usado_por  TEXT,
    criado_em  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_vales_estado ON vales(estado);
CREATE INDEX idx_vales_plano ON vales(plano_id);
CREATE INDEX idx_vales_expira ON vales(expira_em);
--------------------------------------------------------------------------------
-- Sessões de utilizador (ligações autenticadas)
--------------------------------------------------------------------------------
CREATE TABLE sessoes (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    vale_id          INTEGER REFERENCES vales(id) ON DELETE SET NULL,
    rede_id          INTEGER REFERENCES redes(id) ON DELETE SET NULL,
    utilizador       TEXT    NOT NULL,
    telefone         TEXT,
    endereco_ip      TEXT    NOT NULL,
    endereco_mac     TEXT    NOT NULL,
    tipo_dispositivo TEXT    NOT NULL DEFAULT 'desconhecido',
    velocidade       TEXT    NOT NULL DEFAULT '',
    limite_velocidade INTEGER NOT NULL DEFAULT 0,
    dados_mb         REAL    NOT NULL DEFAULT 0,
    iniciada_em      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    terminada_em     TIMESTAMP,
    estado           TEXT    NOT NULL DEFAULT 'ativa'
                     CHECK (estado IN ('ativa', 'terminada', 'expirada')),
    motivo_fim       TEXT
);
CREATE INDEX idx_sessoes_estado ON sessoes(estado);
CREATE INDEX idx_sessoes_utilizador ON sessoes(utilizador);
CREATE INDEX idx_sessoes_mac ON sessoes(endereco_mac);
CREATE INDEX idx_sessoes_iniciada ON sessoes(iniciada_em);
--------------------------------------------------------------------------------
-- Utilizadores do portal
--------------------------------------------------------------------------------
CREATE TABLE utilizadores (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    nome       TEXT    NOT NULL,
    email      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    telefone   TEXT,
    papel      TEXT    NOT NULL DEFAULT 'operador',
    estado     TEXT    NOT NULL DEFAULT 'ativo' CHECK (estado IN ('ativo', 'desativado')),
    criado_em  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
--------------------------------------------------------------------------------
-- Dispositivos de rede (routers, switches, pontos de acesso)
--------------------------------------------------------------------------------
CREATE TABLE dispositivos (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    nome             TEXT    NOT NULL,
    tipo             TEXT    NOT NULL CHECK (tipo IN ('router', 'switch', 'ponto_acesso')),
    endereco_ip      TEXT    NOT NULL,
    estado           TEXT    NOT NULL DEFAULT 'offline'
                     CHECK (estado IN ('online', 'offline')),
    tempo_actividade TEXT    NOT NULL DEFAULT '',
    clientes         INTEGER NOT NULL DEFAULT 0,
    banda            TEXT    NOT NULL DEFAULT '',
    firmware         TEXT    NOT NULL DEFAULT '',
    cpu              TEXT    NOT NULL DEFAULT '',
    memoria          TEXT    NOT NULL DEFAULT '',
    visto_em         TIMESTAMP,
    criado_em        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_dispositivos_nome ON dispositivos(nome);
--------------------------------------------------------------------------------
-- Definições do portal cativo (registo único)
--------------------------------------------------------------------------------
CREATE TABLE portal (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    nome_rede        TEXT    NOT NULL DEFAULT 'WiFi do Kimbo',
    mensagem         TEXT    NOT NULL DEFAULT 'Introduza o seu vale',
    cor_fundo        TEXT    NOT NULL DEFAULT '#0b1220',
    cor_destaque     TEXT    NOT NULL DEFAULT '#3b82f6',
    termos           TEXT    NOT NULL DEFAULT '',
    mensagem_sucesso TEXT    NOT NULL DEFAULT '',
    publicado        INTEGER NOT NULL DEFAULT 0 CHECK (publicado IN (0, 1)),
    atualizado_em    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO portal (id) VALUES (1);
--------------------------------------------------------------------------------
-- Métricas horárias (preenchidas pelo coletor de tráfego)
--------------------------------------------------------------------------------
CREATE TABLE metricas_hora (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    instante      TIMESTAMP NOT NULL UNIQUE,
    download_kbps INTEGER NOT NULL DEFAULT 0,
    upload_kbps   INTEGER NOT NULL DEFAULT 0,
    utilizadores  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_metricas_instante ON metricas_hora(instante);
--------------------------------------------------------------------------------
-- Registo de auditoria das ações administrativas
--------------------------------------------------------------------------------
CREATE TABLE auditoria (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    conta_id    INTEGER REFERENCES contas(id) ON DELETE SET NULL,
    acao        TEXT    NOT NULL,
    alvo        TEXT    NOT NULL DEFAULT '',
    detalhes    TEXT    NOT NULL DEFAULT '',
    endereco_ip TEXT,
    criado_em   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_auditoria_criado ON auditoria(criado_em);
--------------------------------------------------------------------------------
-- Planos de acesso
--------------------------------------------------------------------------------
CREATE TABLE planos (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    nome             TEXT    NOT NULL UNIQUE,
    preco_centavos   INTEGER NOT NULL CHECK (preco_centavos >= 0),
    duracao_minutos  INTEGER NOT NULL CHECK (duracao_minutos > 0),
    velocidade_kbps  INTEGER NOT NULL CHECK (velocidade_kbps > 0),
    limite_dados_mb  INTEGER NOT NULL DEFAULT 0,  -- 0 = ilimitado
    max_dispositivos INTEGER NOT NULL DEFAULT 1 CHECK (max_dispositivos > 0),
    ativo            INTEGER NOT NULL DEFAULT 1 CHECK (ativo IN (0, 1)),
    criado_em        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    atualizado_em    TIMESTAMP
);
CREATE INDEX idx_planos_ativo ON planos(ativo);
