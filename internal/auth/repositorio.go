package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Repositorio guarda as contas de administração no SQLite.
type Repositorio struct {
	db *sql.DB
}

// NovoRepositorio cria o repositório sobre uma conexão de base de dados.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

const colunasConta = `id, nome, email, senha_hash, papel, estado, ultimo_acesso, criado_em`

// BuscarPorEmail devolve a conta com o email indicado (comparação case-insensitive).
func (r *Repositorio) BuscarPorEmail(ctx context.Context, email string) (Conta, error) {
	linha := r.db.QueryRowContext(ctx,
		`SELECT `+colunasConta+` FROM contas WHERE email = ? COLLATE NOCASE`,
		strings.ToLower(strings.TrimSpace(email)))
	return lerConta(linha)
}

// BuscarPorID devolve a conta com o identificador indicado.
func (r *Repositorio) BuscarPorID(ctx context.Context, id int64) (Conta, error) {
	linha := r.db.QueryRowContext(ctx, `SELECT `+colunasConta+` FROM contas WHERE id = ?`, id)
	return lerConta(linha)
}

// Listar devolve todas as contas ordenadas por nome.
func (r *Repositorio) Listar(ctx context.Context) ([]Conta, error) {
	linhas, err := r.db.QueryContext(ctx, `SELECT `+colunasConta+` FROM contas ORDER BY nome`)
	if err != nil {
		return nil, fmt.Errorf("não foi possível listar as contas: %w", err)
	}
	defer linhas.Close()

	contas := []Conta{}
	for linhas.Next() {
		c, err := lerConta(linhas)
		if err != nil {
			return nil, err
		}
		contas = append(contas, c)
	}
	return contas, linhas.Err()
}

// Criar insere uma nova conta e devolve o ID atribuído.
func (r *Repositorio) Criar(ctx context.Context, c CriarConta, hashSenha string) (int64, error) {
	estado := EstadoDesativado
	if c.AtivoOuPadrao() {
		estado = EstadoAtivo
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO contas (nome, email, senha_hash, papel, estado) VALUES (?, ?, ?, ?, ?)`,
		c.Nome, c.Email, hashSenha, string(c.Papel), estado)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrEmailEmUso
		}
		return 0, fmt.Errorf("não foi possível criar a conta: %w", err)
	}
	return res.LastInsertId()
}

// AtualizarEstadoAtivaDesativa muda o estado da conta.
func (r *Repositorio) AtualizarEstado(ctx context.Context, id int64, ativo bool) error {
	estado := EstadoDesativado
	if ativo {
		estado = EstadoAtivo
	}
	res, err := r.db.ExecContext(ctx, `UPDATE contas SET estado = ? WHERE id = ?`, estado, id)
	if err != nil {
		return fmt.Errorf("não foi possível alterar o estado da conta: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrContaNaoEncontrada
	}
	return nil
}

// AtualizarPapel altera o papel da conta.
func (r *Repositorio) AtualizarPapel(ctx context.Context, id int64, papel Papel) error {
	res, err := r.db.ExecContext(ctx, `UPDATE contas SET papel = ? WHERE id = ?`, string(papel), id)
	if err != nil {
		return fmt.Errorf("não foi possível alterar o papel da conta: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrContaNaoEncontrada
	}
	return nil
}

// AtualizarSenha substitui a senha de uma conta.
func (r *Repositorio) AtualizarSenha(ctx context.Context, id int64, hashSenha string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE contas SET senha_hash = ? WHERE id = ?`, hashSenha, id)
	if err != nil {
		return fmt.Errorf("não foi possível atualizar a senha: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrContaNaoEncontrada
	}
	return nil
}

// RegistarAcesso atualiza a marca de último acesso.
func (r *Repositorio) RegistarAcesso(ctx context.Context, id int64, quando time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE contas SET ultimo_acesso = ? WHERE id = ?`, quando, id)
	return err
}

// ContarDonos devolve quantas contas com papel de dono existem (proteção contra lockout).
func (r *Repositorio) ContarDonos(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contas WHERE papel = ? AND estado = ?`,
		string(PapelDono), EstadoAtivo).Scan(&total)
	return total, err
}

// Existe devolve true se existir alguma conta (usado no arranque para criar a primeira).
func (r *Repositorio) Existe(ctx context.Context) (bool, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contas`).Scan(&total); err != nil {
		return false, err
	}
	return total > 0, nil
}

// Erros de domínio do módulo de autenticação.
var (
	ErrContaNaoEncontrada = errors.New("conta não encontrada")
	ErrEmailEmUso        = errors.New("este email já está registado")
)

// linhaScanner abstrai *sql.Row e *sql.Rows para a leitura de uma conta.
type linhaScanner interface {
	Scan(dest ...any) error
}

func lerConta(l linhaScanner) (Conta, error) {
	var c Conta
	var ultimoAcesso sql.NullTime

	if err := l.Scan(&c.ID, &c.Nome, &c.Email, &c.SenhaHash, &c.Papel, &c.Estado, &ultimoAcesso, &c.CriadoEm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Conta{}, ErrContaNaoEncontrada
		}
		return Conta{}, fmt.Errorf("não foi possível ler a conta: %w", err)
	}
	if ultimoAcesso.Valid {
		t := ultimoAcesso.Time
		c.UltimoAcesso = &t
	}
	return c, nil
}
