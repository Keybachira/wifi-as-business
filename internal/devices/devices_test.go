package devices

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// novoBancoDeTeste cria um esquema mínimo com a tabela dispositivos.
//
// Reproduz a estrutura da migração 0001: só as colunas que o repositório usa.
func novoBancoDeTeste(t *testing.T) *sql.DB {
	t.Helper()

	banco, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatalf("não foi possível abrir a base de dados: %v", err)
	}
	t.Cleanup(func() { _ = banco.Close() })

	_, err = banco.Exec(`
		CREATE TABLE dispositivos (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			nome             TEXT    NOT NULL,
			tipo             TEXT    NOT NULL,
			endereco_ip      TEXT    NOT NULL,
			estado           TEXT    NOT NULL DEFAULT 'offline',
			tempo_actividade TEXT    NOT NULL DEFAULT '',
			clientes         INTEGER NOT NULL DEFAULT 0,
			banda            TEXT    NOT NULL DEFAULT '',
			firmware         TEXT    NOT NULL DEFAULT '',
			cpu              TEXT    NOT NULL DEFAULT '',
			memoria          TEXT    NOT NULL DEFAULT '',
			visto_em         TIMESTAMP,
			criado_em        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		t.Fatalf("não foi possível criar a tabela: %v", err)
	}

	inserir := "INSERT INTO dispositivos (nome, tipo, endereco_ip) VALUES ('x', 'router', '10.0.0.1')"
	if _, err := banco.Exec(inserir); err != nil {
		t.Fatalf("não foi possível inserir: %v", err)
	}
	return banco
}

// A telemetria tem de gravar todos os campos. A instrução foi reescrita depois
// de o nome da coluna tempo_atividade ter ficado partido, o que o SQLite
// reportava como "no such column: tempo_actividade" — um erro que não aponta
// para a causa. Este teste impede a regressão.
func TestAtualizarTelemetriaGravaTodosOsCampos(t *testing.T) {
	banco := novoBancoDeTeste(t)
	repo := NovoRepositorio(banco)

	id, err := repo.Criar(context.Background(), NovoDispositivo{
		Nome:       "Router Principal",
		Tipo:       TipoRouter,
		EnderecoIP: "192.168.88.1",
	})
	if err != nil {
		t.Fatalf("não foi possível criar o dispositivo: %v", err)
	}

	erro := repo.AtualizarTelemetria(context.Background(), id, Telemetria{
		Estado:   EstadoOnline,
		Uptime:   "5d2h",
		Clientes: 7,
		Banda:    "100Mbps",
		Firmware: "7.15",
		CPU:      "12%",
		Memoria:  "40%",
	})
	if erro != nil {
		t.Fatalf("a telemetria não foi gravada: %v", erro)
	}

	guardado, err := repo.Buscar(context.Background(), id)
	if err != nil {
		t.Fatalf("não foi possível ler o dispositivo: %v", err)
	}

	if guardado.Estado != EstadoOnline {
		t.Errorf("estado: esperava online, recebeu %q", guardado.Estado)
	}
	if guardado.TempoAtividade != "5d2h" {
		t.Errorf("tempo de atividade: esperava 5d2h, recebeu %q", guardado.TempoAtividade)
	}
	if guardado.Clientes != 7 {
		t.Errorf("clientes: esperava 7, recebeu %d", guardado.Clientes)
	}
	if guardado.Banda != "100Mbps" {
		t.Errorf("banda: esperava 100Mbps, recebeu %q", guardado.Banda)
	}
	if guardado.Firmware != "7.15" || guardado.CPU != "12%" || guardado.Memoria != "40%" {
		t.Errorf("métricas incompletas: %+v", guardado)
	}
	if guardado.VistoEm == nil {
		t.Error("visto_em devia ter sido preenchido")
	}
}

// Qualquer estado inválido é normalizado para offline, sem falhar.
func TestAtualizarTelemetriaNormalizaEstado(t *testing.T) {
	banco := novoBancoDeTeste(t)
	repo := NovoRepositorio(banco)

	id, _ := repo.Criar(context.Background(), NovoDispositivo{
		Nome: "Router", Tipo: TipoRouter, EnderecoIP: "10.0.0.1",
	})

	if err := repo.AtualizarTelemetria(context.Background(), id, Telemetria{Estado: "inventado"}); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	guardado, _ := repo.Buscar(context.Background(), id)
	if guardado.Estado != EstadoOffline {
		t.Errorf("esperava offline, recebeu %q", guardado.Estado)
	}
}

func TestAtualizarTelemetriaDispositivoInexistente(t *testing.T) {
	banco := novoBancoDeTeste(t)
	repo := NovoRepositorio(banco)

	if err := repo.AtualizarTelemetria(context.Background(), 999, Telemetria{}); err != ErrNaoEncontrado {
		t.Fatalf("esperava ErrNaoEncontrado, recebeu %v", err)
	}
}

// Documenta o comportamento real do driver, que contraria a suposição inicial.
//
// A hipótese de que o driver recusasse quebras de linha estava errada: uma
// vírgula no fim da linha anterior, uma quebra antes da vírgula e uma quebra
// dentro de um SELECT são todas aceites. A única forma que falha é partir um
// identificador ao meio, e é esse o caso que produzia o erro enganador.
func TestDriverAceitaQuebrasMasRecusaIdentificadoresPartidos(t *testing.T) {
	banco := novoBancoDeTeste(t)

	casos := []struct {
		nome     string
		consulta string
		falha    bool
	}{
		{"uma linha só",
			"UPDATE dispositivos SET estado = ?, tempo_actividade = ? WHERE id = ?", false},
		{"quebra antes de uma vírgula",
			"UPDATE dispositivos SET estado = ?\n, tempo_actividade = ? WHERE id = ?", false},
		{"quebra depois de uma vírgula",
			"UPDATE dispositivos SET estado = ?,\n tempo_actividade = ? WHERE id = ?", false},
		{"quebra dentro de um SELECT",
			"SELECT id, nome,\n clientes FROM dispositivos WHERE id = ?", false},
		{"identificador partido por quebra de linha",
			"UPDATE dispositivos SET tempo_\n atividade = ? WHERE id = ?", true},
		{"identificador partido por separador",
			"UPDATE dispositivos SET tempo\tatividade = ? WHERE id = ?", true},
	}

	for _, c := range casos {
		var err error
		if c.nome[0] == 'S' {
			_, err = banco.Query(c.consulta, 1)
		} else {
			_, err = banco.Exec(c.consulta, "online", "5d2h", 1)
		}
		obtido := err != nil
		if obtido != c.falha {
			t.Errorf("%s: esperava falha=%v, obteve %v (erro: %v)",
				c.nome, c.falha, obtido, err)
		}
	}
}
