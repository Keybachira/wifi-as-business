package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Driver SQLite puro em Go (sem dependência de CGO).
	_ "modernc.org/sqlite"
)

// Banco encapsula o pool de conexões SQLite.
type Banco struct {
	*sql.DB
	caminho string
}

// Abrir cria/abre o ficheiro SQLite e aplica as definições de robustez.
func Abrir(caminho string) (*Banco, error) {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return nil, fmt.Errorf("não foi possível criar o diretório do banco: %w", err)
	}

	// WAL melhora a concorrência de leituras; busy_timeout evita "database is locked".
	dsn := caminho + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir o banco: %w", err)
	}

	// SQLite suporta uma escrita por vez; pool pequeno evita contenção.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("o banco não respondeu: %w", err)
	}

	return &Banco{DB: db, caminho: caminho}, nil
}

// Caminho devolve o caminho do ficheiro SQLite.
func (b *Banco) Caminho() string { return b.caminho }

// Fechar encerra as conexões de forma graciosa.
func (b *Banco) Fechar() error { return b.DB.Close() }

// EmTransação executa fn dentro de uma transação, fazendo rollback em caso de erro.
func (b *Banco) EmTransação(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := b.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("não foi possível iniciar a transação: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// RegistrarTabela devolve o query de INSERT completo para uma tabela com RETURNING id.
func (b *Banco) TodasAsTabelas(ctx context.Context) ([]string, error) {
	linhas, err := b.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()

	var tabelas []string
	for linhas.Next() {
		var nome string
		if err := linhas.Scan(&nome); err != nil {
			return nil, err
		}
		tabelas = append(tabelas, nome)
	}
	return tabelas, linhas.Err()
}
