// Package users gere os utilizadores registados no portal.
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Estados possíveis de um utilizador.
const (
	EstadoAtivo      = "ativo"
	EstadoDesativado = "desativado"
)

// Erros do repositório.
var (
	ErrNaoEncontrado = errors.New("utilizador não encontrado")
	ErrEmailEmUso    = errors.New("este email já está registado")
)

// Utilizador é uma pessoa com acesso ao portal ou à gestão de rede.
type Utilizador struct {
	ID        int64     `json:"id"`
	Nome      string     `json:"nome"`
	Email     string     `json:"email"`
	Telefone  string     `json:"telefone,omitempty"`
	Papel     string     `json:"papel"`
	Estado    string     `json:"estado"`
	CriadoEm  time.Time  `json:"criado_em"`
	SessoesAtivas int    `json:"sessoes_ativas"`
}

// Ativo indica se o utilizador pode autenticar-se.
func (u Utilizador) Ativo() bool { return u.Estado == EstadoAtivo }

// NovoUtilizador são os dados para registar um utilizador.
type NovoUtilizador struct {
	Nome     string `json:"nome"`
	Email    string `json:"email"`
	Telefone string `json:"telefone,omitempty"`
	Papel    string `json:"papel,omitempty"`
	Ativo    *bool  `json:"ativo,omitempty"`
}

// Validar normaliza e valida os dados de registo.
func (n *NovoUtilizador) Validar() error {
	n.Nome = strings.TrimSpace(n.Nome)
	n.Email = strings.ToLower(strings.TrimSpace(n.Email))
	n.Telefone = strings.TrimSpace(n.Telefone)
	n.Papel = strings.TrimSpace(n.Papel)

	if n.Nome == "" {
		return errors.New("o nome é obrigatório")
	}
	if n.Email == "" || !strings.Contains(n.Email, "@") || strings.ContainsAny(n.Email, " \t") {
		return errors.New("o email indicado é inválido")
	}
	if n.Papel == "" {
		n.Papel = "operador"
	}
	if n.Ativo == nil {
		v := true
		n.Ativo = &v
	}
	return nil
}

// AlternarEstado são os dados para ativar/desativar um utilizador.
type AlternarEstado struct {
	Ativo bool `json:"ativo"`
}

// Repositorio persiste os utilizadores no SQLite.
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório de utilizadores.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

// Listar devolve os utilizadores, opcionalmente filtrando por estado.
func (r *Repositorio) Listar(ctx context.Context, estado string) ([]Utilizador, error) {
	query := `SELECT u.id, u.nome, u.email, COALESCE(u.telefone, ''), u.papel, u.estado, u.criado_em,
			(SELECT COUNT(*) FROM sessoes s WHERE s.utilizador = u.nome AND s.estado = 'ativa')
		FROM utilizadores u WHERE 1 = 1`
	var args []any
	if estado != "" {
		query += ` AND u.estado = ?`
		args = append(args, estado)
	}
	query += ` ORDER BY u.nome`

	linhas, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("não foi possível listar os utilizadores: %w", err)
	}
	defer linhas.Close()

	utilizadores := []Utilizador{}
	for linhas.Next() {
		var u Utilizador
		if err := linhas.Scan(&u.ID, &u.Nome, &u.Email, &u.Telefone, &u.Papel, &u.Estado,
			&u.CriadoEm, &u.SessoesAtivas); err != nil {
			return nil, err
		}
		utilizadores = append(utilizadores, u)
	}
	return utilizadores, linhas.Err()
}

// Buscar devolve o utilizador pelo identificador.
func (r *Repositorio) Buscar(ctx context.Context, id int64) (Utilizador, error) {
	var u Utilizador
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.nome, u.email, COALESCE(u.telefone, ''), u.papel, u.estado, u.criado_em,
			(SELECT COUNT(*) FROM sessoes s WHERE s.utilizador = u.nome AND s.estado = 'ativa')
		FROM utilizadores u WHERE u.id = ?`, id).
		Scan(&u.ID, &u.Nome, &u.Email, &u.Telefone, &u.Papel, &u.Estado, &u.CriadoEm, &u.SessoesAtivas)
	if errors.Is(err, sql.ErrNoRows) {
		return Utilizador{}, ErrNaoEncontrado
	}
	if err != nil {
		return Utilizador{}, fmt.Errorf("não foi possível ler o utilizador: %w", err)
	}
	return u, nil
}

// Criar regista um novo utilizador.
func (r *Repositorio) Criar(ctx context.Context, n NovoUtilizador) (int64, error) {
	estado := EstadoDesativado
	if *n.Ativo {
		estado = EstadoAtivo
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO utilizadores (nome, email, telefone, papel, estado) VALUES (?, ?, ?, ?, ?)`,
		n.Nome, n.Email, nuloSeVazio(n.Telefone), n.Papel, estado)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrEmailEmUso
		}
		return 0, fmt.Errorf("não foi possível registar o utilizador: %w", err)
	}
	return res.LastInsertId()
}

// AtualizarEstado ativa ou desativa o utilizador.
func (r *Repositorio) AtualizarEstado(ctx context.Context, id int64, ativo bool) error {
	estado := EstadoDesativado
	if ativo {
		estado = EstadoAtivo
	}
	res, err := r.db.ExecContext(ctx, `UPDATE utilizadores SET estado = ? WHERE id = ?`, estado, id)
	if err != nil {
		return fmt.Errorf("não foi possível atualizar o utilizador: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// Eliminar remove o utilizador.
func (r *Repositorio) Eliminar(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM utilizadores WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("não foi possível eliminar o utilizador: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// ContarAtivos devolve o número de utilizadores ativos.
func (r *Repositorio) ContarAtivos(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM utilizadores WHERE estado = ?`, EstadoAtivo).Scan(&total)
	return total, err
}

func nuloSeVazio(s string) any {
	if s == "" {
		return nil
	}
	return s
}
