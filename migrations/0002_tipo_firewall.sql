-- 0002_tipo_firewall.sql
-- Acrescenta o tipo 'firewall' aos dispositivos, para que um pfSense possa ser
-- registado no inventário ao lado dos routers MikroTik.
--
-- O SQLite não altera uma restrição CHECK já criada: a tabela tem de ser
-- reconstruida. Os dados são copiados e os índices recriados.
--------------------------------------------------------------------------------
CREATE TABLE dispositivos_novo (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    nome             TEXT    NOT NULL,
    tipo             TEXT    NOT NULL CHECK (tipo IN ('router', 'switch', 'ponto_acesso', 'firewall')),
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
INSERT INTO dispositivos_novo (id, nome, tipo, endereco_ip, estado, tempo_actividade, clientes, banda, firmware, cpu, memoria, visto_em, criado_em)
    SELECT id, nome, tipo, endereco_ip, estado, tempo_actividade, clientes, banda, firmware, cpu, memoria, visto_em, criado_em FROM dispositivos;
DROP TABLE dispositivos;
ALTER TABLE dispositivos_novo RENAME TO dispositivos;
CREATE UNIQUE INDEX idx_dispositivos_nome ON dispositivos(nome);
