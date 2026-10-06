-- 0004_generico.sql
-- Torna o sistema genérico: sem papel de "dono" e sem nome de rede fixo.
--
-- As contas passam a ser todas iguais: o papel de "dono" deixa de existir
-- na aplicação. A coluna papel é mantida na base de dados por compatibilidade,
-- mas a aplicação ignora-a e escreve sempre 'operador'.
--
-- O nome de rede fixo "WiFi do Kimbo" passa a "WiFi Controller"; a partir
-- daqui o nome é gerido pelo operador via PATCH /api/v1/portal.
--------------------------------------------------------------------------------
UPDATE contas SET papel = 'operador' WHERE papel <> 'operador';

UPDATE utilizadores SET papel = 'operador' WHERE papel = 'dono';

UPDATE portal SET nome_rede = 'WiFi Controller'
WHERE nome_rede = 'WiFi do Kimbo' OR nome_rede LIKE '%Kimbo%';
