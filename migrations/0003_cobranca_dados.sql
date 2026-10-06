-- 0003_cobranca_dados.sql
-- Liga a cobrança por dados ao fluxo RADIUS.
--
-- Duas adições:
--   1. sessoes.limite_dados_mb, copiado do plano no momento da autenticação.
--      Guardar o valor no início impede que uma alteração ao plano a meio da
--      sessão mude as regras já aceitas pelo cliente.
--   2. sessoes.session_id, que associa a sessão à sessão RADIUS. O router
--      volta a enviar este valor no Accounting-Request, e é por ele que o
--      consumo é atribuído à sessão correcta.
--
-- O consumo continua em dados_mb (REAL, em MB).
--------------------------------------------------------------------------------
ALTER TABLE sessoes ADD COLUMN limite_dados_mb INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessoes ADD COLUMN session_id    TEXT    NOT NULL DEFAULT '';
CREATE INDEX idx_sessoes_session ON sessoes(session_id);
