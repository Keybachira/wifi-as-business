package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Migrador aplica ficheiros .sql versionados de forma incremental e idempotente.
type Migrador struct {
	dir    string
	banco  *Banco
	registo *slog.Logger
}

// NovoMigrador cria o migrador apontando para o diretório de ficheiros .sql.
func NovoMigrador(banco *Banco, diretorio string, registo *slog.Logger) *Migrador {
	return &Migrador{dir: diretorio, banco: banco, registo: registo}
}

// Aplicar executa todas as migrações ainda não registadas.
func (m *Migrador) Aplicar(ctx context.Context) error {
	if err := m.criarTabelaRegisto(ctx); err != nil {
		return err
	}

	ficheiros, err := m.listar()
	if err != nil {
		return err
	}
	if len(ficheiros) == 0 {
		m.registo.Warn("nenhuma migração encontrada", "diretorio", m.dir)
		return nil
	}

	aplicadas, err := m.aplicadas(ctx)
	if err != nil {
		return err
	}

	count := 0
	for _, f := range ficheiros {
		if aplicadas[f.nome] {
			continue
		}
		if err := m.aplicar(ctx, f); err != nil {
			return fmt.Errorf("falha ao aplicar a migração %s: %w", f.nome, err)
		}
		count++
	}

	m.registo.Info("migrações concluídas", "novas", count, "total", len(ficheiros))
	return nil
}

type ficheiroMigracao struct {
	nome     string
	versao   int
	caminho  string
	conteudo string
}

// listar devolve as migrações ordenadas pela versão (prefixo numérico do nome).
func (m *Migrador) listar() ([]ficheiroMigracao, error) {
	entradas, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler o diretório de migrações: %w", err)
	}

	var ficheiros []ficheiroMigracao
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		nome := e.Name()
		versao, err := strconv.Atoi(separarVersao(nome))
		if err != nil {
			return nil, fmt.Errorf("nome de migração inválido %q: use o formato NNNN_descricao.sql", nome)
		}
		conteudo, err := os.ReadFile(filepath.Join(m.dir, nome))
		if err != nil {
			return nil, err
		}
		ficheiros = append(ficheiros, ficheiroMigracao{nome: nome, versao: versao, caminho: nome, conteudo: string(conteudo)})
	}

	sort.Slice(ficheiros, func(i, j int) bool { return ficheiros[i].versao < ficheiros[j].versao })
	return ficheiros, nil
}

// aplicar executa uma migração dentro de transação e regista a versão.
func (m *Migrador) aplicar(ctx context.Context, f ficheiroMigracao) error {
	inicio := time.Now()
	if err := m.banco.EmTransação(ctx, func(tx *sql.Tx) error {
		// O driver SQLite executa uma instrução por chamada, por isso o
		// ficheiro é dividido em comandos individuais.
		for i, comando := range dividirComandos(f.conteudo) {
			if _, err := tx.ExecContext(ctx, comando); err != nil {
				return fmt.Errorf("instrução %d: %w", i+1, err)
			}
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO registo_migracoes (nome, versao) VALUES (?, ?)`, f.nome, f.versao)
		return err
	}); err != nil {
		return err
	}
	m.registo.Info("migração aplicada", "ficheiro", f.nome, "duracao", time.Since(inicio).String())
	return nil
}

// dividirComandos separa o SQL em comandos individuais, removendo comentários.
// O esquema deste projeto não usa ponto e vírgula dentro de literais, por isso
// uma divisão simples é suficiente e mais segura do que um parser completo.
func dividirComandos(texto string) []string {
	var comandos []string
	var atual strings.Builder

	for _, bruta := range strings.Split(texto, "\n") {
		// Remove o comentário de linha, preservando o código anterior à marca.
		linha := bruta
		if i := strings.Index(linha, "--"); i >= 0 {
			linha = linha[:i]
		}
		if strings.TrimSpace(linha) == "" {
			continue
		}

		atual.WriteString(linha)
		atual.WriteString("\n")

		if strings.HasSuffix(strings.TrimSpace(linha), ";") {
			comandos = append(comandos, strings.TrimSpace(atual.String()))
			atual.Reset()
		}
	}

	// Comando final sem ponto e vírgula.
	if resto := strings.TrimSpace(atual.String()); resto != "" {
		comandos = append(comandos, resto)
	}
	return comandos
}

// aplicadas devolve o conjunto de migrações já registadas.
func (m *Migrador) aplicadas(ctx context.Context) (map[string]bool, error) {
	linhas, err := m.banco.QueryContext(ctx, `SELECT nome FROM registo_migracoes`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()

	conjunto := map[string]bool{}
	for linhas.Next() {
		var nome string
		if err := linhas.Scan(&nome); err != nil {
			return nil, err
		}
		conjunto[nome] = true
	}
	return conjunto, linhas.Err()
}

func (m *Migrador) criarTabelaRegisto(ctx context.Context) error {
	_, err := m.banco.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS registo_migracoes (
			nome        TEXT PRIMARY KEY,
			versao      INTEGER NOT NULL,
			aplicada_em TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	return err
}

// separarVersao extrai os dígitos iniciais do nome do ficheiro.
func separarVersao(nome string) string {
	i := 0
	for i < len(nome) && nome[i] >= '0' && nome[i] <= '9' {
		i++
	}
	if i == 0 {
		return "x"
	}
	return nome[:i]
}
