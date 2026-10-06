package sessions

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	// Driver SQLite em Go puro, o mesmo que a aplicação usa.
	_ "modernc.org/sqlite"
)

// novoBancoDeTeste cria o esquema mínimo: planos, vales e sessões.
//
// A cobrança de dados atravessa as três tabelas, por isso o teste precisa das
// três: um plano define o limite, um vale liga o plano à sessão, e a sessão
// acumula o consumo.
func novoBancoDeTeste(t *testing.T) *sql.DB {
	t.Helper()

	banco, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatalf("não foi possível abrir a base de dados: %v", err)
	}
	t.Cleanup(func() { _ = banco.Close() })

	_, err = banco.Exec(esquemaCobranca)
	if err != nil {
		t.Fatalf("não foi possível criar o esquema: %v", err)
	}
	return banco
}

// esquemaCobranca reproduz as tabelas usadas pela cobrança de dados.
const esquemaCobranca = `
CREATE TABLE planos (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	nome             TEXT    NOT NULL,
	preco_centavos   INTEGER NOT NULL,
	duracao_minutos  INTEGER NOT NULL,
	velocidade_kbps  INTEGER NOT NULL,
	limite_dados_mb  INTEGER NOT NULL DEFAULT 0,
	max_dispositivos INTEGER NOT NULL DEFAULT 1,
	ativo            INTEGER NOT NULL DEFAULT 1,
	criado_em        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	atualizado_em    TIMESTAMP
);
CREATE TABLE vales (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	codigo     TEXT    NOT NULL,
	plano_id   INTEGER NOT NULL REFERENCES planos(id),
	estado     TEXT    NOT NULL DEFAULT 'disponivel',
	expira_em  TIMESTAMP,
	usado_em   TIMESTAMP,
	usado_por  TEXT,
	criado_em  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE sessoes (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	vale_id          INTEGER REFERENCES vales(id),
	rede_id          INTEGER,
	utilizador       TEXT    NOT NULL,
	telefone         TEXT,
	endereco_ip      TEXT    NOT NULL,
	endereco_mac     TEXT    NOT NULL,
	tipo_dispositivo TEXT    NOT NULL DEFAULT 'desconhecido',
	velocidade       TEXT    NOT NULL DEFAULT '',
	limite_velocidade INTEGER NOT NULL DEFAULT 0,
	limite_dados_mb  INTEGER NOT NULL DEFAULT 0,
	dados_mb         REAL    NOT NULL DEFAULT 0,
	session_id       TEXT    NOT NULL DEFAULT '',
	iniciada_em      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	terminada_em     TIMESTAMP,
	estado           TEXT    NOT NULL DEFAULT 'ativa',
	motivo_fim       TEXT
)`

// registoSilencioso evita poluir a saída dos testes.
func registoSilencioso() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// criarPlano insere um plano com o limite de dados indicado.
func criarPlano(t *testing.T, banco *sql.DB, limiteMB int) int64 {
	t.Helper()
	res, err := banco.Exec(
		`INSERT INTO planos (nome, preco_centavos, duracao_minutos, velocidade_kbps, limite_dados_mb)
		 VALUES ('Plano Teste', 10000, 60, 5000, ?)`, limiteMB)
	if err != nil {
		t.Fatalf("não foi possível criar o plano: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// criarSessao insere uma sessão ligada a um vale do plano indicado.
//
// O limite é herdado do plano dentro do próprio INSERT, como em produção.
func criarSessao(t *testing.T, banco *sql.DB, planoID int64, sessionID string) int64 {
	t.Helper()

	res, err := banco.Exec(
		`INSERT INTO vales (codigo, plano_id, estado) VALUES ('ZG-TESTE-0001', ?, 'ativo')`, planoID)
	if err != nil {
		t.Fatalf("não foi possível criar o vale: %v", err)
	}
	valeID, _ := res.LastInsertId()

	res, err = banco.Exec(
		`INSERT INTO sessoes (vale_id, utilizador, endereco_ip, endereco_mac, limite_dados_mb, session_id)
		 VALUES (?, 'Guest', '192.168.1.50', 'DC:A6:32:01:77:2E',
		         (SELECT limite_dados_mb FROM planos WHERE id = ?), ?)`,
		valeID, planoID, sessionID)
	if err != nil {
		t.Fatalf("não foi possível criar a sessão: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// O limite do plano é copiado para a sessão no arranque, para que uma
// alteração posterior ao plano não mude as regras já acordadas.
func TestSessaoHerdaLimiteDoPlano(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 2048)
	criarSessao(t, banco, planoID, "sess-1")

	repo := NovoRepositorio(banco)
	sessao, err := repo.BuscarAtivaPorMAC(context.Background(), "DC:A6:32:01:77:2E")
	if err != nil {
		t.Fatalf("não foi possível ler a sessão: %v", err)
	}
	if sessao.LimiteDadosMB != 2048 {
		t.Fatalf("esperava limite de 2048 MB, recebeu %d", sessao.LimiteDadosMB)
	}
	if sessao.RestanteDados() != 2048 {
		t.Fatalf("esperava 2048 MB restantes, recebeu %v", sessao.RestanteDados())
	}
}

// O consumo abaixo do limite é gravado mas não corta a sessão.
func TestConsumoAbaixoDoLimiteNaoCorta(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 100)
	criarSessao(t, banco, planoID, "sess-1")

	srv := NovoServico(NovoRepositorio(banco), registoSilencioso())
	encerrada, err := srv.RegistrarConsumo(context.Background(), "sess-1", 40, 10)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if encerrada {
		t.Fatal("a sessão não devia ser cortada abaixo do limite")
	}

	sessao, _ := srv.Obter(context.Background(), 1)
	if sessao.DadosMB != 50 {
		t.Fatalf("esperava 50 MB consumidos, recebeu %v", sessao.DadosMB)
	}
	if !sessao.AindaAtiva() {
		t.Error("a sessão devia continuar ativa")
	}
}

// Ao atingir o limite a sessão é terminada com o motivo correcto.
func TestConsumoNoLimiteCortaSessao(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 100)
	criarSessao(t, banco, planoID, "sess-1")

	srv := NovoServico(NovoRepositorio(banco), registoSilencioso())
	encerrada, err := srv.RegistrarConsumo(context.Background(), "sess-1", 80, 30)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if !encerrada {
		t.Fatal("a sessão devia ser cortada ao atingir o limite")
	}

	sessao, _ := srv.Obter(context.Background(), 1)
	if sessao.AindaAtiva() {
		t.Error("a sessão devia estar terminada")
	}
	if sessao.MotivoFim != "limite de dados atingido" {
		t.Errorf("motivo inesperado: %q", sessao.MotivoFim)
	}
	// O consumo fica registado mesmo após o corte, para a cobrança fechar certa.
	if sessao.DadosMB != 110 {
		t.Errorf("esperava 110 MB registados, recebeu %v", sessao.DadosMB)
	}
}

// Um plano sem limite (0) nunca corta, por mais dados que se consuma.
func TestPlanoIlimitadoNuncaCorta(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 0)
	criarSessao(t, banco, planoID, "sess-1")

	srv := NovoServico(NovoRepositorio(banco), registoSilencioso())
	encerrada, err := srv.RegistrarConsumo(context.Background(), "sess-1", 50000, 10000)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if encerrada {
		t.Fatal("um plano ilimitado nunca deve cortar a sessão")
	}

	sessao, _ := srv.Obter(context.Background(), 1)
	if sessao.RestanteDados() != -1 {
		t.Errorf("um plano ilimitado devia reportar -1, recebeu %v", sessao.RestanteDados())
	}
	if !sessao.AindaAtiva() {
		t.Error("a sessão devia continuar ativa")
	}
}

// Os contadores do router são absolutos, não incrementos: enviar o mesmo
// valor três vezes não pode triplicar o consumo.
func TestContadoresAbsolutosNaoDuplicam(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 1000)
	criarSessao(t, banco, planoID, "sess-1")

	srv := NovoServico(NovoRepositorio(banco), registoSilencioso())
	for i := 0; i < 3; i++ {
		if _, err := srv.RegistrarConsumo(context.Background(), "sess-1", 30, 10); err != nil {
			t.Fatalf("não esperava erro: %v", err)
		}
	}

	sessao, _ := srv.Obter(context.Background(), 1)
	if sessao.DadosMB != 40 {
		t.Fatalf("esperava 40 MB (valor absoluto), recebeu %v", sessao.DadosMB)
	}
}

// Uma sessão que o router não conhece não é erro: pode ter sido aberta antes
// do último arranque do servidor.
func TestSessionIDDesconhecidoNaoFalha(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 100)
	criarSessao(t, banco, planoID, "sess-1")

	srv := NovoServico(NovoRepositorio(banco), registoSilencioso())
	encerrada, err := srv.RegistrarConsumo(context.Background(), "sessao-do-fantasma", 10, 10)
	if err != nil {
		t.Fatalf("uma sessão desconhecida não deve gerar erro: %v", err)
	}
	if encerrada {
		t.Fatal("nada devia ser cortado")
	}
}

// A rotina periódica apanha o consumo que o router não reportou, por estar
// desligado ou mal configurado para accounting.
func TestExcederDadosApanhaConsumoNaoReportado(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 50)
	criarSessao(t, banco, planoID, "sess-1")

	if _, err := banco.Exec(`UPDATE sessoes SET dados_mb = 999`); err != nil {
		t.Fatalf("não foi possível simular o consumo: %v", err)
	}

	srv := NovoServico(NovoRepositorio(banco), registoSilencioso())
	n, err := srv.ExcederDados(context.Background())
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperava 1 sessão cortada, obteve %d", n)
	}
	if s, _ := srv.Obter(context.Background(), 1); s.AindaAtiva() {
		t.Error("a sessão devia ter sido terminada")
	}
}

// Uma sessão já terminada não volta a ser cortada pela rotina periódica.
func TestExcederDadosIgnoraSessoesTerminadas(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 50)
	criarSessao(t, banco, planoID, "sess-1")

	if _, err := banco.Exec(
		`UPDATE sessoes SET dados_mb = 999, estado = 'terminada'`); err != nil {
		t.Fatalf("não foi possível preparar a sessão: %v", err)
	}

	srv := NovoServico(NovoRepositorio(banco), registoSilencioso())
	n, err := srv.ExcederDados(context.Background())
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if n != 0 {
		t.Fatalf("esperava 0 sessões, obteve %d", n)
	}
}

// MarcarExcedidas tem de marcar todas as sessões indicadas de uma vez.
func TestMarcarExcedidasVariasSessoes(t *testing.T) {
	banco := novoBancoDeTeste(t)
	planoID := criarPlano(t, banco, 10)
	id1 := criarSessao(t, banco, planoID, "s1")

	res, err := banco.Exec(
		`INSERT INTO vales (codigo, plano_id, estado) VALUES ('ZG-TESTE-0002', ?, 'ativo')`, planoID)
	if err != nil {
		t.Fatalf("não foi possível criar o segundo vale: %v", err)
	}
	vale2, _ := res.LastInsertId()
	res, _ = banco.Exec(
		`INSERT INTO sessoes (vale_id, utilizador, endereco_ip, endereco_mac, limite_dados_mb, dados_mb)
		 VALUES (?, 'Guest2', '192.168.1.51', 'AA:BB:CC:DD:EE:FF', 10, 999)`, vale2)
	id2, _ := res.LastInsertId()

	repo := NovoRepositorio(banco)
	if err := repo.MarcarExcedidas(context.Background(), []int64{id1, id2}); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if _, err := repo.BuscarAtivaPorMAC(context.Background(), "DC:A6:32:01:77:2E"); err == nil {
		t.Error("a primeira sessão devia ter sido terminada")
	}
	if _, err := repo.BuscarAtivaPorMAC(context.Background(), "AA:BB:CC:DD:EE:FF"); err == nil {
		t.Error("a segunda sessão devia ter sido terminada")
	}
}

// Uma lista vazia não pode gerar SQL inválido (IN ()).
func TestMarcarExcedidasComListaVazia(t *testing.T) {
	banco := novoBancoDeTeste(t)
	repo := NovoRepositorio(banco)
	if err := repo.MarcarExcedidas(context.Background(), nil); err != nil {
		t.Fatalf("uma lista vazia não deve gerar erro: %v", err)
	}
}
